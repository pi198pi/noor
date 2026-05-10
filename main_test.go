package main

import (
	"testing"
)

func TestSanitize(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello\nworld", "hello\nworld"},
		{"hello\tworld", "hello\tworld"},
		{"hello\x00world", "helloworld"},
		{"hello\x01world", "helloworld"},
		{"normal text", "normal text"},
	}

	for _, tt := range tests {
		got := sanitize(tt.input)
		if got != tt.expected {
			t.Errorf("sanitize(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestParseWeather(t *testing.T) {
	loc, ok := parseWeather("weather London")
	if !ok || loc != "London" {
		t.Errorf("expected London, got %q, ok=%v", loc, ok)
	}

	loc, ok = parseWeather("WEATHER  New York ")
	if !ok || loc != "New York" {
		t.Errorf("expected 'New York', got %q, ok=%v", loc, ok)
	}

	loc, ok = parseWeather("hello world")
	if ok {
		t.Error("expected not weather")
	}
}
