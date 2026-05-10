package main

import (
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// ─── Color Palette (populated by applyTheme) ──────────────────────────────────

var (
	colorProvider lipgloss.Color
	colorBorder   lipgloss.Color
	colorSep      lipgloss.Color
	colorResponse lipgloss.Color
	colorError    lipgloss.Color
	colorWarning  lipgloss.Color
	colorInfo     lipgloss.Color
	colorSuccess  lipgloss.Color
	colorDim      lipgloss.Color
	colorCmd      lipgloss.Color
	colorTemp     lipgloss.Color
	colorCtxBar   lipgloss.Color
)

// ─── Style Helpers (populated by applyTheme) ──────────────────────────────────

var (
	styleProvider lipgloss.Style
	styleDim      lipgloss.Style
	styleBold     lipgloss.Style
	styleError    lipgloss.Style
	styleWarning  lipgloss.Style
	styleInfo     lipgloss.Style
	styleSuccess  lipgloss.Style
	styleCmd      lipgloss.Style
	styleResponse lipgloss.Style
	styleSpinner  lipgloss.Style
	styleTemp     lipgloss.Style
	styleSep      lipgloss.Style
	styleBorder   lipgloss.Style
	styleCtxBar   lipgloss.Style
	bannerStyle   lipgloss.Style
)

var currentTheme = "default"

func init() {
	applyTheme("default")
}

// applyTheme rebuilds all package-level styles from the named theme.
// Returns true if the theme was found and applied.
func applyTheme(name string) bool {
	t, ok := themes[name]
	if !ok {
		return false
	}
	currentTheme = name

	colorProvider = t.Provider
	colorBorder = t.Border
	colorSep = t.Sep
	colorResponse = t.Response
	colorError = t.Error
	colorWarning = t.Warning
	colorInfo = t.Info
	colorSuccess = t.Success
	colorDim = t.Dim
	colorCmd = t.Cmd
	colorTemp = t.Temp
	colorCtxBar = t.CtxBar

	styleProvider = lipgloss.NewStyle().Foreground(colorProvider).Bold(true)
	styleDim = lipgloss.NewStyle().Foreground(colorDim)
	styleBold = lipgloss.NewStyle().Bold(true)
	styleError = lipgloss.NewStyle().Foreground(colorError).Bold(true)
	styleWarning = lipgloss.NewStyle().Foreground(colorWarning)
	styleInfo = lipgloss.NewStyle().Foreground(colorInfo)
	styleSuccess = lipgloss.NewStyle().Foreground(colorSuccess)
	styleCmd = lipgloss.NewStyle().Foreground(colorCmd)
	styleResponse = lipgloss.NewStyle().Foreground(colorResponse)
	styleSpinner = lipgloss.NewStyle().Foreground(colorProvider)
	styleTemp = lipgloss.NewStyle().Foreground(colorTemp)
	styleSep = lipgloss.NewStyle().Foreground(colorSep)
	styleBorder = lipgloss.NewStyle().Foreground(colorBorder)
	styleCtxBar = lipgloss.NewStyle().Foreground(colorCtxBar)

	bannerStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(0, 2).
		Bold(true)

	return true
}

// ─── Glamour Markdown Renderer ────────────────────────────────────────────────

var (
	glamourRenderer    *glamour.TermRenderer
	glamourRendererErr error
	glamourWidth       int
	glamourMu          sync.Mutex
)

// getGlamourRenderer returns a renderer matched to the current terminal width.
// It rebuilds the renderer when the terminal has been resized.
func getGlamourRenderer() (*glamour.TermRenderer, error) {
	width := 100
	if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && w > 0 {
		width = w - 4
		if width < 40 {
			width = 40
		}
	}

	glamourMu.Lock()
	defer glamourMu.Unlock()

	if glamourRenderer == nil || width != glamourWidth {
		glamourRenderer, glamourRendererErr = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(width),
		)
		glamourWidth = width
	}
	return glamourRenderer, glamourRendererErr
}

func renderMarkdown(text string) string {
	r, err := getGlamourRenderer()
	if err != nil {
		return text
	}
	out, err := r.Render(text)
	if err != nil {
		return text
	}
	return out
}

// ─── Connection Indicator ─────────────────────────────────────────────────────

// connStatus tracks the latest API call result. 0 = unknown, 1 = ok, 2 = error.
var connStatus int32 // atomic via sync/atomic

func setConnOK()    { atomic.StoreInt32(&connStatus, 1) }
func setConnError() { atomic.StoreInt32(&connStatus, 2) }

func connDot() string {
	switch atomic.LoadInt32(&connStatus) {
	case 1:
		return lipgloss.NewStyle().Foreground(colorSuccess).Render("●")
	case 2:
		return lipgloss.NewStyle().Foreground(colorError).Render("●")
	default:
		return lipgloss.NewStyle().Foreground(colorDim).Render("●")
	}
}

// ─── Banner ───────────────────────────────────────────────────────────────────

func showBanner(cfg *Config) {
	modelLabel := cfg.Model
	if idx := strings.LastIndex(modelLabel, "/"); idx >= 0 {
		modelLabel = modelLabel[idx+1:]
	}
	model := providerStyle(cfg.Model).Render(modelLabel)
	title := fmt.Sprintf("⬡ %s %s  ·  %s OpenRouter  ·  %s",
		AppName, AppVersion, connDot(), model)
	fmt.Println(bannerStyle.Render(title))
	fmt.Println(styleDim.Render("  /help for commands  ·  exit to quit"))
	fmt.Println()
}

// ─── Separator ────────────────────────────────────────────────────────────────

func sep() {
	w := termWidth()
	if w > 80 {
		w = 80
	}
	if w < 40 {
		w = 40
	}
	fmt.Println(styleSep.Render(strings.Repeat("─", w)))
}

// ─── Assistant Header ─────────────────────────────────────────────────────────

func assistantHeader() {
	fmt.Printf("\r\033[K%s %s\n",
		styleProvider.Render("OR"),
		connDot(),
	)
}

// ─── Context Window Tracking ──────────────────────────────────────────────────

// Context windows verified against OpenRouter's models API.
var modelCtxWindow = map[string]int{
	"anthropic/claude-opus-4.7":             1000000,
	"anthropic/claude-sonnet-4.6":           1000000,
	"anthropic/claude-haiku-4.5":            200000,
	"openai/gpt-5.5":                        1050000,
	"openai/gpt-5.4":                        1050000,
	"openai/gpt-5.4-nano":                   400000,
	"openai/gpt-4o":                         128000,
	"openai/gpt-4o-mini":                    128000,
	"google/gemini-2.5-pro":                 1048576,
	"google/gemini-2.5-flash":               1048576,
	"google/gemini-2.5-flash-image":         32768,
	"google/gemini-3.1-flash-image-preview": 65536,
	"deepseek/deepseek-v4-pro":              1048576,
	"deepseek/deepseek-v4-flash":            1048576,
	"deepseek/deepseek-v3.2":                131072,
	"mistralai/mistral-large":               128000,
	"z-ai/glm-5.1":                          202752,
	"z-ai/glm-5-turbo":                      202752,
	"moonshotai/kimi-k2.6":                  262142,
	"minimax/minimax-m2.7":                  196608,
	"x-ai/grok-4.3":                         1000000,
}

func ctxWindowForModel(model string) int {
	if w, ok := modelCtxWindow[model]; ok {
		return w
	}
	return 128000
}

func formatCtx(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.0fM", float64(n)/1000000)
	}
	return fmt.Sprintf("%dK", n/1000)
}

func formatCost(c float64) string {
	if c <= 0 {
		return ""
	}
	if c < 0.0001 {
		return "<$0.0001"
	}
	return fmt.Sprintf("$%.4f", c)
}

// ─── Assistant Footer ─────────────────────────────────────────────────────────

func assistantFooter(elapsed time.Duration, tokens int, ctxTokens int, model string, cost float64, sessionTotal float64) {
	// Line 1: timing · tokens · rate · cost · total
	parts := []string{styleTemp.Render(fmt.Sprintf("%.1fs", elapsed.Seconds()))}
	if tokens > 0 {
		parts = append(parts, styleTemp.Render(fmt.Sprintf("%dt", tokens)))
		if elapsed.Seconds() > 0 {
			rate := float64(tokens) / elapsed.Seconds()
			parts = append(parts, styleTemp.Render(fmt.Sprintf("%.0f t/s", rate)))
		}
	}
	if s := formatCost(cost); s != "" {
		parts = append(parts, styleSuccess.Render(s))
	}
	if sessionTotal > 0 {
		parts = append(parts, styleDim.Render(formatCost(sessionTotal)+" total"))
	}
	fmt.Printf("%s\n", styleDim.Render(strings.Join(parts, "  ·  ")))

	// Line 2: context bar
	window := ctxWindowForModel(model)
	used := ctxTokens
	remaining := window - used
	if remaining < 0 {
		remaining = 0
	}
	pct := math.Min(float64(used)/float64(window), 1.0)

	// Warn when approaching context limit
	if pct >= 0.8 {
		warnMsg := fmt.Sprintf("⚠ Context %.0f%% full — consider /compress or /reset", pct*100)
		fmt.Println(styleWarning.Render("  " + warnMsg))
	}

	// Render a compact ASCII progress bar
	barWidth := 20
	filled := int(math.Round(pct * float64(barWidth)))
	if filled > barWidth {
		filled = barWidth
	}
	bar := styleCtxBar.Render(strings.Repeat("█", filled))
	empty := styleDim.Render(strings.Repeat("░", barWidth-filled))

	pctStr := fmt.Sprintf("%d%%", int(pct*100))
	if pct < 0.01 {
		pctStr = fmt.Sprintf("%.1f%%", pct*100)
	}
	fmt.Printf("  %s%s  %s\n",
		bar, empty,
		styleDim.Render(fmt.Sprintf("%s  ·  %s used  ·  %s left",
			pctStr, formatCtx(used), formatCtx(remaining))),
	)
}

// ─── Print Helpers ────────────────────────────────────────────────────────────

func printError(msg string) {
	fmt.Println(styleError.Render("  ✗  " + msg))
}

func printWarning(msg string) {
	fmt.Println(styleWarning.Render("  ⚠  " + msg))
}

func printInfo(msg string) {
	fmt.Println(styleInfo.Render("  ℹ  " + msg))
}

func printSuccess(msg string) {
	fmt.Println(styleSuccess.Render("  ✓  " + msg))
}

// ─── Spinner ──────────────────────────────────────────────────────────────────

type Spinner struct {
	stop  chan struct{}
	done  chan struct{}
	label string
}

func NewSpinner(label string) *Spinner {
	return &Spinner{
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
		label: label,
	}
}

func (s *Spinner) Start() {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-s.stop:
				fmt.Print("\r\033[K")
				return
			case <-ticker.C:
				fmt.Printf("\r  %s %s",
					styleSpinner.Render(frames[i]),
					styleDim.Render(s.label),
				)
				i = (i + 1) % len(frames)
			}
		}
	}()
}

func (s *Spinner) Stop() {
	close(s.stop)
	<-s.done
}

// ─── Token Estimation ─────────────────────────────────────────────────────────

func approxTokens(messages []Message) int {
	total := 0
	for _, m := range messages {
		switch c := m.Content.(type) {
		case string:
			total += len(c)
		case []interface{}:
			for _, part := range c {
				if p, ok := part.(map[string]interface{}); ok {
					if t, ok := p["text"].(string); ok {
						total += len(t)
					}
				}
			}
		}
	}
	return total / 4
}

// ─── Terminal Width ───────────────────────────────────────────────────────────

func termWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return 80
	}
	return w
}

// ─── Help Text (--help flag) ──────────────────────────────────────────────────

func showHelpText() {
	fmt.Println()
	fmt.Println(bannerStyle.Render(fmt.Sprintf("⬡ %s %s", AppName, AppVersion)))
	fmt.Println()
	fmt.Println(styleBold.Render("USAGE"))
	fmt.Printf("  %s [options]\n\n", styleCmd.Render(AppName))

	fmt.Println(styleBold.Render("OPTIONS"))
	opts := [][2]string{
		{"-h, --help", "Show this help"},
		{"--version", "Show version"},
		{"-c, --config FILE", "Custom config file"},
		{"-S, --style STYLE", "Response style: markdown, plain, concise, raw"},
		{"-p, --system-prompt STR", "Set system prompt"},
		{"-t, --temp FLOAT", "Temperature (0.0–2.0, default: 0.7)"},
		{"-m, --max-tokens INT", "Max tokens (default: 2000)"},
		{"--mcp-server CMD", "MCP stdio server command"},
		{"--no-history", "Disable session history"},
		{"--debug", "Enable debug logging to stderr"},
	}
	for _, o := range opts {
		fmt.Printf("  %-28s %s\n", styleCmd.Render(o[0]), styleDim.Render(o[1]))
	}

	fmt.Println()
	fmt.Println(styleBold.Render("SLASH COMMANDS"))
	cmds := [][2]string{
		{"/help", "Show available commands"},
		{"/clear", "Clear screen and reset conversation"},
		{"/reset", "Reset conversation context only"},
		{"/model [name]", "Interactive picker or switch model directly"},
		{"/style <name>", "Switch response style"},
		{"/theme [name]", "Switch color theme (default/cyberpunk/ocean/forest/sunset/minimal)"},
		{"/system <prompt>", "Set custom system prompt"},
		{"/history", "Browse and resume a past session"},
		{"/history clear", "Delete all saved sessions"},
		{"/search <query>", "Search past sessions for text and resume"},
		{"/tools", "List loaded MCP tools"},
		{"/mcp [cmd|stop]", "Start/stop MCP server (e.g. /mcp python3 mcp.py)"},
		{"/image <path> [prompt]", "Analyze image or edit it (image models)"},
		{"/imagine <prompt>", "Generate an image (image models)"},
		{"/export [file]", "Export — default .html, or .py/.js/.go to extract code"},
		{"/copy", "Copy last response to clipboard"},
		{"/retry", "Regenerate last response"},
		{"/edit", "Edit last message and regenerate"},
		{"/budget [amount]", "Show today's cost or set daily limit"},
		{"/compress", "Summarize history to free up context"},
		{"/freeze", "Export last code block to PNG"},
		{"/attach", "Pick a file to include with your next message"},
		{"weather <city>", "Fetch live weather"},
		{"@file/url/git", "Include @file.go, @https://..., or @diff/status/log"},
		{"exit", "Exit the chat"},
	}
	for _, c := range cmds {
		fmt.Printf("  %-28s %s\n", styleCmd.Render(c[0]), styleDim.Render(c[1]))
	}

	fmt.Println()
	fmt.Println(styleBold.Render("ENVIRONMENT"))
	fmt.Printf("  %-28s %s\n", styleCmd.Render("OPENROUTER_API_KEY"), styleDim.Render("Required. API key from openrouter.ai."))
	fmt.Printf("  %-28s %s\n", styleCmd.Render("TINYFISH_API_KEY"), styleDim.Render("Optional. Enables built-in web search via TinyFish."))
	fmt.Println()
}
