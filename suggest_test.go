package main

import (
	"strings"
	"testing"
)

func TestSuggestModel(t *testing.T) {
	tests := []struct {
		input        string
		currentModel string
		want         string
	}{
		// Image generation
		{"generate image of a cat", DefaultModel, "google/gemini-2.5-flash-image"},
		{"generate image of a cat", "google/gemini-2.5-flash-image", ""},
		{"draw a picture", DefaultModel, "google/gemini-2.5-flash-image"},
		{"create image", DefaultModel, "google/gemini-2.5-flash-image"},
		{"edit this photo", DefaultModel, "google/gemini-2.5-flash-image"},

		// Code-heavy (2+ code words) → suggest Sonnet
		{"def foo():\n    import os", DefaultModel, "anthropic/claude-sonnet-4.6"},
		{"function bar() {\n    const x = 1\n}", DefaultModel, "anthropic/claude-sonnet-4.6"},
		{"class Foo:\n    def bar(self): pass", DefaultModel, "anthropic/claude-sonnet-4.6"},
		{"def foo():\n    import os", "anthropic/claude-sonnet-4.6", ""},

		// Long input → suggest Sonnet
		{strings.Repeat("x", 2000), DefaultModel, "anthropic/claude-sonnet-4.6"},
		{strings.Repeat("x", 2000), "anthropic/claude-sonnet-4.6", ""},

		// Reasoning → suggest Opus
		{"prove that P=NP", DefaultModel, "anthropic/claude-opus-4.7"},
		{"design a system for scaling", DefaultModel, "anthropic/claude-opus-4.7"},
		{"complexity analysis of quicksort", DefaultModel, "anthropic/claude-opus-4.7"},
		{"prove that P=NP", "anthropic/claude-opus-4.7", ""},

		// No suggestion needed
		{"hello world", DefaultModel, ""},
		{"what is the weather", DefaultModel, ""},
	}

	for _, tt := range tests {
		got := suggestModel(tt.input, tt.currentModel)
		if got != tt.want {
			t.Errorf("suggestModel(%q, %q) = %q, want %q", tt.input, tt.currentModel, got, tt.want)
		}
	}
}
