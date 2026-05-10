package main

import (
	"testing"
)

func TestDegToCompass(t *testing.T) {
	tests := []struct {
		deg      int
		expected string
	}{
		{0, "N"},
		{45, "NE"},
		{90, "E"},
		{135, "SE"},
		{180, "S"},
		{225, "SW"},
		{270, "W"},
		{315, "NW"},
		{360, "N"},
		{22, "N"},
		{67, "NE"},
	}

	for _, tt := range tests {
		got := degToCompass(tt.deg)
		if got != tt.expected {
			t.Errorf("degToCompass(%d) = %s, want %s", tt.deg, got, tt.expected)
		}
	}
}

func TestWeatherCode(t *testing.T) {
	tests := []struct {
		code     int
		expected string
	}{
		{0, "Clear sky"},
		{1, "Partly cloudy"},
		{2, "Partly cloudy"},
		{3, "Partly cloudy"},
		{45, "Foggy"},
		{48, "Foggy"},
		{51, "Drizzle"},
		{61, "Rain"},
		{71, "Snow"},
		{77, "Snow grains"},
		{80, "Rain showers"},
		{95, "Thunderstorm"},
		{96, "Thunderstorm with hail"},
		{99, "Thunderstorm with hail"},
		{999, "Unknown"},
	}

	for _, tt := range tests {
		got := weatherCode(tt.code)
		if got != tt.expected {
			t.Errorf("weatherCode(%d) = %s, want %s", tt.code, got, tt.expected)
		}
	}
}

func TestTempColorStyle(t *testing.T) {
	for _, temp := range []float64{-10, 5, 15, 25, 35} {
		_ = tempColorStyle(temp)
	}
}
