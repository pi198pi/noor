package main

import "github.com/charmbracelet/lipgloss"

// Theme is a complete color palette. applyTheme rebuilds all package-level
// style variables when the theme changes.
type Theme struct {
	Name     string
	Provider lipgloss.Color
	Border   lipgloss.Color
	Sep      lipgloss.Color
	Response lipgloss.Color
	Error    lipgloss.Color
	Warning  lipgloss.Color
	Info     lipgloss.Color
	Success  lipgloss.Color
	Dim      lipgloss.Color
	Cmd      lipgloss.Color
	Temp     lipgloss.Color
	CtxBar   lipgloss.Color
}

var themes = map[string]Theme{
	"default": {
		Name:     "default",
		Provider: "#00C4CC",
		Border:   "#8B5CF6",
		Sep:      "#374151",
		Response: "#F3F4F6",
		Error:    "#EF4444",
		Warning:  "#F59E0B",
		Info:     "#3B82F6",
		Success:  "#10B981",
		Dim:      "#6B7280",
		Cmd:      "#FCD34D",
		Temp:     "#94A3B8",
		CtxBar:   "#A78BFA",
	},
	"cyberpunk": {
		Name:     "cyberpunk",
		Provider: "#FF00FF",
		Border:   "#00FFFF",
		Sep:      "#1F2937",
		Response: "#F0ABFC",
		Error:    "#FF0055",
		Warning:  "#FFD400",
		Info:     "#00E5FF",
		Success:  "#39FF14",
		Dim:      "#6B7280",
		Cmd:      "#FFD400",
		Temp:     "#FF6FB5",
		CtxBar:   "#FF00FF",
	},
	"ocean": {
		Name:     "ocean",
		Provider: "#06B6D4",
		Border:   "#0EA5E9",
		Sep:      "#1E293B",
		Response: "#E0F2FE",
		Error:    "#F87171",
		Warning:  "#FBBF24",
		Info:     "#38BDF8",
		Success:  "#34D399",
		Dim:      "#64748B",
		Cmd:      "#7DD3FC",
		Temp:     "#94A3B8",
		CtxBar:   "#22D3EE",
	},
	"forest": {
		Name:     "forest",
		Provider: "#22C55E",
		Border:   "#15803D",
		Sep:      "#1F2937",
		Response: "#ECFDF5",
		Error:    "#DC2626",
		Warning:  "#F59E0B",
		Info:     "#10B981",
		Success:  "#84CC16",
		Dim:      "#6B7280",
		Cmd:      "#A3E635",
		Temp:     "#86EFAC",
		CtxBar:   "#22C55E",
	},
	"sunset": {
		Name:     "sunset",
		Provider: "#FB923C",
		Border:   "#F97316",
		Sep:      "#292524",
		Response: "#FFEDD5",
		Error:    "#EF4444",
		Warning:  "#FACC15",
		Info:     "#FB7185",
		Success:  "#FBBF24",
		Dim:      "#78716C",
		Cmd:      "#FCD34D",
		Temp:     "#F59E0B",
		CtxBar:   "#FB923C",
	},
	"minimal": {
		Name:     "minimal",
		Provider: "#FFFFFF",
		Border:   "#A0A0A0",
		Sep:      "#404040",
		Response: "#E5E5E5",
		Error:    "#E5484D",
		Warning:  "#D4A017",
		Info:     "#909090",
		Success:  "#A0A0A0",
		Dim:      "#606060",
		Cmd:      "#FFFFFF",
		Temp:     "#A0A0A0",
		CtxBar:   "#FFFFFF",
	},
}

func ThemeNames() []string {
	names := make([]string, 0, len(themes))
	// Keep "default" first, others alphabetical
	for n := range themes {
		if n != "default" {
			names = append(names, n)
		}
	}
	// simple sort
	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			if names[j] < names[i] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}
	return append([]string{"default"}, names...)
}

// ─── Provider Badges ──────────────────────────────────────────────────────────

var providerColors = map[string]lipgloss.Color{
	"anthropic":  "#D97757",
	"openai":     "#10A37F",
	"google":     "#4285F4",
	"meta-llama": "#A855F7",
	"deepseek":   "#06B6D4",
	"x-ai":       "#FFFFFF",
	"mistralai":  "#FF6F61",
	"z-ai":       "#FCD34D",
	"moonshotai": "#A78BFA",
	"minimax":    "#14B8A6",
}

func providerStyle(model string) lipgloss.Style {
	for prefix, color := range providerColors {
		if len(model) >= len(prefix) && model[:len(prefix)] == prefix {
			return lipgloss.NewStyle().Foreground(color).Bold(true)
		}
	}
	return styleProvider
}
