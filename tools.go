package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

// pluginTools is cached after first load to avoid re-reading the plugins dir
// on every chat turn.
var pluginTools []Tool

// ─── Web Search ──────────────────────────────────────────────────────────────

var builtinWebSearchTool = Tool{
	Type: "function",
	Function: ToolFunction{
		Name:        "web_search",
		Description: "Search the web for real-time or very recent information. Only use this when the answer requires up-to-date data (news, live prices, current events, today's weather). Do NOT use for general knowledge, coding, math, explanations, or anything you already know.",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query",
				},
			},
			"required": []string{"query"},
		},
	},
}

// ─── Weather ─────────────────────────────────────────────────────────────────

type weatherData struct {
	city      string
	tempC     float64
	feelsLike float64
	windKph   float64
	windDir   string
	condition string
}

func handleWeather(location string) {
	fmt.Printf("%s %s\n", styleInfo.Render("⛅ fetching weather:"), styleDim.Render(location))

	wd, err := fetchWeather(location)
	if err != nil {
		printError("Weather: " + err.Error())
		return
	}

	tempF := wd.tempC*9/5 + 32
	tc := tempColorStyle(wd.tempC)

	sep()
	fmt.Printf("  %s  %s\n", styleBold.Render(wd.city), styleDim.Render(wd.condition))
	fmt.Printf("  Temp:  %s  %s\n",
		tc.Render(fmt.Sprintf("%.1f°C", wd.tempC)),
		styleDim.Render(fmt.Sprintf("(%.1f°F)", tempF)),
	)
	fmt.Printf("  Feels: %s\n", tc.Render(fmt.Sprintf("%.1f°C", wd.feelsLike)))
	fmt.Printf("  Wind:  %s %s\n",
		styleInfo.Render(fmt.Sprintf("%.1f km/h", wd.windKph)),
		styleDim.Render(wd.windDir),
	)
	sep()
	fmt.Println()
}

func fetchWeather(location string) (*weatherData, error) {
	geoURL := fmt.Sprintf(
		"https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=en&format=json",
		url.QueryEscape(location),
	)
	gResp, err := httpClient.Get(geoURL)
	if err != nil {
		return nil, fmt.Errorf("geocoding failed: %w", err)
	}
	defer gResp.Body.Close()
	gBody, err := io.ReadAll(gResp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading geocoding response: %w", err)
	}

	var geo struct {
		Results []struct {
			Name      string  `json:"name"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
			Country   string  `json:"country"`
		} `json:"results"`
	}
	if err := json.Unmarshal(gBody, &geo); err != nil || len(geo.Results) == 0 {
		return nil, fmt.Errorf("location not found: %s", location)
	}

	g := geo.Results[0]
	city := g.Name
	if g.Country != "" {
		city += ", " + g.Country
	}

	wURL := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f"+
			"&current=temperature_2m,apparent_temperature,wind_speed_10m,wind_direction_10m,weathercode"+
			"&wind_speed_unit=kmh",
		g.Latitude, g.Longitude,
	)
	wResp, err := httpClient.Get(wURL)
	if err != nil {
		return nil, fmt.Errorf("weather fetch failed: %w", err)
	}
	defer wResp.Body.Close()
	wBody, err := io.ReadAll(wResp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading weather response: %w", err)
	}

	var weather struct {
		Current struct {
			Temperature2m       float64 `json:"temperature_2m"`
			ApparentTemperature float64 `json:"apparent_temperature"`
			WindSpeed10m        float64 `json:"wind_speed_10m"`
			WindDirection10m    int     `json:"wind_direction_10m"`
			Weathercode         int     `json:"weathercode"`
		} `json:"current"`
	}
	if err := json.Unmarshal(wBody, &weather); err != nil {
		return nil, fmt.Errorf("weather parse failed")
	}

	c := weather.Current
	return &weatherData{
		city:      city,
		tempC:     c.Temperature2m,
		feelsLike: c.ApparentTemperature,
		windKph:   c.WindSpeed10m,
		windDir:   degToCompass(c.WindDirection10m),
		condition: weatherCode(c.Weathercode),
	}, nil
}

func degToCompass(deg int) string {
	dirs := []string{"N", "NE", "E", "SE", "S", "SW", "W", "NW"}
	idx := int(math.Round(float64(deg)/45)) % 8
	return dirs[idx]
}

func weatherCode(code int) string {
	switch {
	case code == 0:
		return "Clear sky"
	case code == 1 || code == 2 || code == 3:
		return "Partly cloudy"
	case code == 45 || code == 48:
		return "Foggy"
	case code == 51 || code == 53 || code == 55:
		return "Drizzle"
	case code == 56 || code == 57:
		return "Freezing drizzle"
	case code == 61 || code == 63 || code == 65:
		return "Rain"
	case code == 66 || code == 67:
		return "Freezing rain"
	case code == 71 || code == 73 || code == 75:
		return "Snow"
	case code == 77:
		return "Snow grains"
	case code == 80 || code == 81 || code == 82:
		return "Rain showers"
	case code == 85 || code == 86:
		return "Snow showers"
	case code == 95:
		return "Thunderstorm"
	case code == 96 || code == 99:
		return "Thunderstorm with hail"
	default:
		return "Unknown"
	}
}

func tempColorStyle(c float64) lipgloss.Style {
	switch {
	case c < 0:
		return styleInfo
	case c < 10:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#60A5FA"))
	case c < 20:
		return styleSuccess
	case c < 30:
		return styleWarning
	default:
		return styleError
	}
}

// ─── Image ───────────────────────────────────────────────────────────────────

var imageMIME = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
}

// EncodeImage reads a local image file and returns (base64, mimeType, error).
func EncodeImage(path string) (string, string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	mime, ok := imageMIME[ext]
	if !ok {
		return "", "", fmt.Errorf("unsupported image type %q — use png, jpg, gif, or webp", ext)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("cannot read image: %w", err)
	}
	return base64.StdEncoding.EncodeToString(data), mime, nil
}

// BuildImageMessage builds a user Message with inline image + text.
func BuildImageMessage(imagePath, prompt string) (Message, error) {
	b64, mime, err := EncodeImage(imagePath)
	if err != nil {
		return Message{}, err
	}
	content := []map[string]interface{}{
		{"type": "text", "text": prompt},
		{"type": "image_url", "image_url": map[string]string{
			"url": fmt.Sprintf("data:%s;base64,%s", mime, b64),
		}},
	}
	return Message{Role: "user", Content: content}, nil
}

// SaveGeneratedImage saves an ImageResult to ~/Pictures/<AppName>/ and returns the path.
func SaveGeneratedImage(img ImageResult) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "Pictures", AppName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	ext := "png"
	if strings.Contains(img.MimeType, "jpeg") || strings.Contains(img.MimeType, "jpg") {
		ext = "jpg"
	} else if strings.Contains(img.MimeType, "webp") {
		ext = "webp"
	}

	filename := filepath.Join(dir, "img-"+time.Now().Format("20060102-150405")+"."+ext)

	if img.Data != "" {
		data, err := base64.StdEncoding.DecodeString(img.Data)
		if err != nil {
			return "", fmt.Errorf("decoding image: %w", err)
		}
		return filename, os.WriteFile(filename, data, 0644)
	}

	if img.URL != "" {
		resp, err := httpClient.Get(img.URL)
		if err != nil {
			return "", fmt.Errorf("downloading image: %w", err)
		}
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return filename, os.WriteFile(filename, data, 0644)
	}

	return "", fmt.Errorf("no image data in response")
}

// ─── Clipboard ───────────────────────────────────────────────────────────────

// CopyToClipboard sends text to the system clipboard. It tries multiple
// backends: Wayland (wl-copy), X11 (xclip, xsel), and macOS (pbcopy).
func CopyToClipboard(text string) error {
	candidates := [][]string{
		{"wl-copy"},
		{"xclip", "-selection", "clipboard"},
		{"xsel", "--clipboard", "--input"},
		{"pbcopy"},
	}
	for _, args := range candidates {
		if _, err := exec.LookPath(args[0]); err != nil {
			continue
		}
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("no clipboard tool found (install wl-clipboard, xclip, or xsel)")
}
