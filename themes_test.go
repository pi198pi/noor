package main

import (
	"slices"
	"testing"
)

func TestApplyTheme(t *testing.T) {
	if !applyTheme("default") {
		t.Error("expected default theme to exist")
	}
	if applyTheme("nonexistent") {
		t.Error("expected nonexistent theme to fail")
	}
	if currentTheme != "default" {
		t.Errorf("currentTheme = %q, want default", currentTheme)
	}

	// Switch to another theme and verify
	if !applyTheme("ocean") {
		t.Error("expected ocean theme to exist")
	}
	if currentTheme != "ocean" {
		t.Errorf("currentTheme = %q, want ocean", currentTheme)
	}

	// Restore default for other tests
	applyTheme("default")
}

func TestThemeNames(t *testing.T) {
	names := ThemeNames()
	if len(names) == 0 {
		t.Fatal("expected themes")
	}
	if names[0] != "default" {
		t.Errorf("first theme = %q, want default", names[0])
	}
	// Check sorted after default
	for i := 2; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("themes not sorted: %v", names)
			break
		}
	}
	expected := []string{"cyberpunk", "default", "forest", "minimal", "ocean", "sunset"}
	for _, name := range expected {
		if !slices.Contains(names, name) {
			t.Errorf("missing theme: %s", name)
		}
	}
}

func TestProviderStyle(t *testing.T) {
	tests := []struct {
		model  string
		prefix string
	}{
		{"anthropic/claude-haiku", "anthropic"},
		{"openai/gpt-4", "openai"},
		{"google/gemini", "google"},
		{"deepseek/deepseek", "deepseek"},
		{"x-ai/grok", "x-ai"},
		{"unknown/model", ""},
	}

	for _, tt := range tests {
		style := providerStyle(tt.model)
		fg := style.GetForeground()
		if tt.prefix != "" {
			expectedColor := providerColors[tt.prefix]
			if fg != expectedColor {
				t.Errorf("providerStyle(%q) foreground = %v, want %v", tt.model, fg, expectedColor)
			}
		}
		// All should be bold
		if !style.GetBold() {
			t.Errorf("providerStyle(%q) should be bold", tt.model)
		}
	}
}
