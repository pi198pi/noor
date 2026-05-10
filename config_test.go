package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFromFile(t *testing.T) {
	// Ensure env does not override file values
	t.Setenv("OPENROUTER_API_KEY", "")
	t.Setenv("TINYFISH_API_KEY", "")

	tmp := t.TempDir()
	configFile := filepath.Join(tmp, "config")
	content := `OPENROUTER_API_KEY=file-key
TEMPERATURE=0.5
MAX_TOKENS=500
MODEL=openai/gpt-4o
THEME=cyberpunk
DAILY_BUDGET=2.50
`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := defaultConfig()
	cfg.ConfigFile = configFile
	loadConfig(&cfg)

	if cfg.APIKey != "file-key" {
		t.Errorf("APIKey = %q, want file-key", cfg.APIKey)
	}
	if cfg.Temperature != 0.5 {
		t.Errorf("Temperature = %f, want 0.5", cfg.Temperature)
	}
	if cfg.MaxTokens != 500 {
		t.Errorf("MaxTokens = %d, want 500", cfg.MaxTokens)
	}
	if cfg.Model != "openai/gpt-4o" {
		t.Errorf("Model = %q, want openai/gpt-4o", cfg.Model)
	}
	if cfg.Theme != "cyberpunk" {
		t.Errorf("Theme = %q, want cyberpunk", cfg.Theme)
	}
	if cfg.DailyBudget != 2.50 {
		t.Errorf("DailyBudget = %f, want 2.50", cfg.DailyBudget)
	}
}

func TestLoadConfigEnvOverride(t *testing.T) {
	tmp := t.TempDir()
	configFile := filepath.Join(tmp, "config")
	os.WriteFile(configFile, []byte("OPENROUTER_API_KEY=file-key\n"), 0644)

	t.Setenv("OPENROUTER_API_KEY", "env-key")
	t.Setenv("TINYFISH_API_KEY", "tf-key")

	cfg := defaultConfig()
	cfg.ConfigFile = configFile
	loadConfig(&cfg)

	if cfg.APIKey != "env-key" {
		t.Errorf("APIKey = %q, want env-key (env overrides file)", cfg.APIKey)
	}
	if cfg.TinyFishKey != "tf-key" {
		t.Errorf("TinyFishKey = %q, want tf-key", cfg.TinyFishKey)
	}
}

func TestLoadConfigClamping(t *testing.T) {
	tmp := t.TempDir()
	configFile := filepath.Join(tmp, "config")
	os.WriteFile(configFile, []byte("TEMPERATURE=-1\nMAX_TOKENS=0\n"), 0644)

	cfg := defaultConfig()
	cfg.ConfigFile = configFile
	loadConfig(&cfg)

	if cfg.Temperature != 0 {
		t.Errorf("Temperature clamped to 0, got %f", cfg.Temperature)
	}
	if cfg.MaxTokens != 1 {
		t.Errorf("MaxTokens clamped to 1, got %d", cfg.MaxTokens)
	}
}

func TestLoadConfigModelFromFileOnlyWhenDefault(t *testing.T) {
	tmp := t.TempDir()
	configFile := filepath.Join(tmp, "config")
	os.WriteFile(configFile, []byte("MODEL=openai/gpt-4o\n"), 0644)

	cfg := defaultConfig()
	cfg.Model = "anthropic/claude-opus-4.7"
	cfg.ConfigFile = configFile
	loadConfig(&cfg)

	if cfg.Model != "anthropic/claude-opus-4.7" {
		t.Errorf("Model = %q, should not have been overridden from config", cfg.Model)
	}
}

func TestLoadConfigModelFromFileWhenDefault(t *testing.T) {
	tmp := t.TempDir()
	configFile := filepath.Join(tmp, "config")
	os.WriteFile(configFile, []byte("MODEL=openai/gpt-4o\n"), 0644)

	cfg := defaultConfig()
	cfg.ConfigFile = configFile
	loadConfig(&cfg)

	if cfg.Model != "openai/gpt-4o" {
		t.Errorf("Model = %q, want openai/gpt-4o", cfg.Model)
	}
}
