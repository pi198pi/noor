package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	AppName          = "noor"
	AppVersion       = "2.0.0"
	DefaultAPIURL    = "https://openrouter.ai/api/v1/chat/completions"
	DefaultModel     = "anthropic/claude-haiku-4.5"
	DefaultTemp      = 0.7
	DefaultMaxTokens = 2000
	MaxHistorySize   = 10
	MaxToolOutput    = 12000
)

var OpenRouterModels = []string{
	"── CHAT ──",
	"anthropic/claude-opus-4.7",
	"anthropic/claude-sonnet-4.6",
	"anthropic/claude-haiku-4.5",
	"openai/gpt-5.5",
	"openai/gpt-5.4",
	"openai/gpt-5.4-nano",
	"openai/gpt-4o",
	"openai/gpt-4o-mini",
	"google/gemini-2.5-pro",
	"google/gemini-2.5-flash",
	"deepseek/deepseek-v4-pro",
	"deepseek/deepseek-v4-flash",
	"deepseek/deepseek-v3.2",
	"mistralai/mistral-large",
	"z-ai/glm-5.1",
	"z-ai/glm-5-turbo",
	"moonshotai/kimi-k2.6",
	"minimax/minimax-m2.7",
	"x-ai/grok-4.3",
	"── IMAGES ──",
	"google/gemini-2.5-flash-image",
	"google/gemini-3.1-flash-image-preview",
}

var stylePrompts = map[string]string{
	"markdown": "You are a helpful assistant. Format your responses using markdown when appropriate.",
	"plain":    "You are a helpful assistant. Use plain text without markdown formatting.",
	"concise":  "You are a helpful assistant. Be concise and direct. Keep responses brief.",
	"raw":      "",
}

type Config struct {
	APIURL       string
	APIKey       string
	Model        string
	Temperature  float64
	MaxTokens    int
	Style        string
	Theme        string
	SystemPrompt string
	MCPServer    string
	NoHistory    bool
	ConfigFile   string
	TinyFishKey  string
	DailyBudget  float64 // USD, 0 = no limit
	Debug        bool
}

func defaultConfig() Config {
	return Config{
		APIURL:      DefaultAPIURL,
		Model:       DefaultModel,
		Temperature: DefaultTemp,
		MaxTokens:   DefaultMaxTokens,
		Style:       "markdown",
	}
}

func parseFlags() Config {
	cfg := defaultConfig()

	flag.StringVar(&cfg.ConfigFile, "config", "", "Custom config file")
	flag.StringVar(&cfg.ConfigFile, "c", "", "Custom config file")
	flag.StringVar(&cfg.Style, "style", cfg.Style, "Response style: markdown, plain, concise, raw")
	flag.StringVar(&cfg.Style, "S", cfg.Style, "Response style")
	flag.StringVar(&cfg.SystemPrompt, "system-prompt", "", "System prompt")
	flag.StringVar(&cfg.SystemPrompt, "p", "", "System prompt")
	flag.Float64Var(&cfg.Temperature, "temp", cfg.Temperature, "Temperature 0.0-2.0")
	flag.Float64Var(&cfg.Temperature, "t", cfg.Temperature, "Temperature")
	flag.IntVar(&cfg.MaxTokens, "max-tokens", cfg.MaxTokens, "Max tokens")
	flag.IntVar(&cfg.MaxTokens, "m", cfg.MaxTokens, "Max tokens")
	flag.StringVar(&cfg.MCPServer, "mcp-server", "", "MCP stdio server command")
	flag.BoolVar(&cfg.NoHistory, "no-history", false, "Disable session history")
	flag.BoolVar(&cfg.Debug, "debug", false, "Enable debug logging to stderr")

	showVer := flag.Bool("version", false, "Show version")

	flag.Usage = showHelpText
	flag.Parse()

	if *showVer {
		fmt.Printf("%s %s\n", AppName, AppVersion)
		os.Exit(0)
	}

	return cfg
}

func loadConfig(cfg *Config) {
	path := cfg.ConfigFile
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		path = filepath.Join(home, ".config", AppName, "config")
	}

	if f, err := os.Open(path); err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			k, v := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
			switch k {
			case "OPENROUTER_API_KEY":
				if cfg.APIKey == "" {
					cfg.APIKey = v
				}
			case "TINYFISH_API_KEY":
				if cfg.TinyFishKey == "" {
					cfg.TinyFishKey = v
				}
			case "TEMPERATURE":
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					cfg.Temperature = f
				}
			case "MAX_TOKENS":
				if i, err := strconv.Atoi(v); err == nil {
					cfg.MaxTokens = i
				}
			case "MODEL":
				// Config file MODEL takes effect only if the user hasn't
				// explicitly set a different model via flags or env.
				if cfg.Model == DefaultModel {
					cfg.Model = v
				}
			case "THEME":
				cfg.Theme = v
			case "DAILY_BUDGET":
				if f, err := strconv.ParseFloat(v, 64); err == nil {
					cfg.DailyBudget = f
				}
			}
		}
	}

	if v := os.Getenv("OPENROUTER_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("TINYFISH_API_KEY"); v != "" {
		cfg.TinyFishKey = v
	}

	// Validate and clamp temperature
	if cfg.Temperature < 0 {
		cfg.Temperature = 0
	}
	if cfg.Temperature > 2.0 {
		cfg.Temperature = 2.0
	}
	if cfg.MaxTokens < 1 {
		cfg.MaxTokens = 1
	}
}

// saveSetting writes or replaces a KEY=value line in ~/.config/<AppName>/config.
func saveSetting(key, value string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("finding home directory: %w", err)
	}
	dir := filepath.Join(home, ".config", AppName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}
	path := filepath.Join(dir, "config")

	var lines []string
	if f, err := os.Open(path); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
		}
		f.Close()
	}

	prefix := key + "="
	found := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), prefix) {
			lines[i] = prefix + value
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, prefix+value)
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

func saveModel(model string) error { return saveSetting("MODEL", model) }
func saveTheme(theme string) error { return saveSetting("THEME", theme) }
