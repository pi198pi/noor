package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"text/template"
	"time"
)

// ─── --list-models ───────────────────────────────────────────────────────────

func printModelList() {
	for _, m := range OpenRouterModels {
		if isHeader(m) {
			fmt.Println(m)
			continue
		}
		ctx := ""
		if w, ok := modelCtxWindow[m]; ok {
			ctx = fmt.Sprintf("  %s", formatCtx(w))
		}
		fmt.Printf("  %s%s\n", m, ctx)
	}
}

// ─── config validate ─────────────────────────────────────────────────────────

// validateConfig checks the setup and prints a report. Returns the number of
// errors found (0 = all good).
func validateConfig() int {
	errors := 0

	// 1. Config file
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("✗ Cannot determine home directory")
		return 1
	}
	cfgPath := filepath.Join(home, ".config", AppName, "config")

	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		fmt.Printf("✗ Config file not found: %s\n", cfgPath)
		errors++
	} else {
		fmt.Printf("✓ Config file: %s\n", cfgPath)
	}

	// 2. API key
	cfg := defaultConfig()
	loadConfig(&cfg)
	if cfg.APIKey == "" {
		fmt.Println("✗ OPENROUTER_API_KEY is not set (config file or environment)")
		errors++
	} else {
		fmt.Println("✓ OPENROUTER_API_KEY set")

		// 3. API key validity — lightweight ping
		if err := pingOpenRouter(cfg.APIKey); err != nil {
			fmt.Printf("✗ OpenRouter API unreachable: %v\n", err)
			errors++
		} else {
			fmt.Println("✓ OpenRouter API reachable")
		}
	}

	// 4. TinyFish key (optional but validate if present)
	if cfg.TinyFishKey != "" {
		fmt.Println("✓ TINYFISH_API_KEY set")
		if err := pingTinyFish(cfg.TinyFishKey); err != nil {
			fmt.Printf("✗ TinyFish API unreachable: %v\n", err)
			errors++
		} else {
			fmt.Println("✓ TinyFish API reachable")
		}
	}

	// 5. Plugins
	pluginDir := filepath.Join(home, ".config", AppName, "plugins")
	var pluginFiles []string
	_ = filepath.WalkDir(pluginDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(d.Name()) == ".json" {
			pluginFiles = append(pluginFiles, path)
		}
		return nil
	})

	if len(pluginFiles) == 0 {
		fmt.Println("— No plugins found (optional)")
	} else {
		for _, path := range pluginFiles {
			rel, _ := filepath.Rel(pluginDir, path)
			data, err := os.ReadFile(path)
			if err != nil {
				fmt.Printf("✗ Plugin %s: cannot read: %v\n", rel, err)
				errors++
				continue
			}

			var p Plugin
			if err := json.Unmarshal(data, &p); err != nil {
				fmt.Printf("✗ Plugin %s: invalid JSON: %v\n", rel, err)
				errors++
				continue
			}

			if p.Name == "" {
				fmt.Printf("✗ Plugin %s: missing \"name\" field\n", rel)
				errors++
				continue
			}
			if p.Command == "" {
				fmt.Printf("✗ Plugin %s: missing \"command\" field\n", rel)
				errors++
				continue
			}

			if _, err := template.New("validate").Parse(p.Command); err != nil {
				fmt.Printf("✗ Plugin %s (%s): bad template: %v\n", rel, p.Name, err)
				errors++
				continue
			}

			fmt.Printf("✓ Plugin %s → %s\n", rel, p.Name)
		}
	}

	// 6. Budget
	b := loadBudget()
	today := b.Daily[nowDate()]
	if today > 0 && cfg.DailyBudget > 0 {
		pct := today / cfg.DailyBudget * 100
		fmt.Printf("ℹ Budget today: $%.4f / $%.2f (%.0f%%)\n", today, cfg.DailyBudget, pct)
	} else if today > 0 {
		fmt.Printf("ℹ Budget today: $%.4f (no limit)\n", today)
	}

	// 7. Theme
	if t, ok := themes[cfg.Theme]; ok {
		fmt.Printf("ℹ Theme: %s\n", t.Name)
	} else {
		fmt.Printf("ℹ Theme: default\n")
	}

	fmt.Println()
	if errors == 0 {
		fmt.Println("All checks passed ✓")
	} else {
		fmt.Printf("%d error(s) found — fix before running noor\n", errors)
	}

	return errors
}

// pingOpenRouter makes a lightweight GET to the OpenRouter models endpoint
// to verify the API key is valid and the service is reachable.
func pingOpenRouter(apiKey string) error {
	req, err := http.NewRequest("GET", "https://openrouter.ai/api/v1/models", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

// pingTinyFish makes a lightweight search query to verify the TinyFish API key.
func pingTinyFish(apiKey string) error {
	req, err := http.NewRequest("GET", "https://api.search.tinyfish.ai/?query=test&language=en", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

// nowDate returns today's date as YYYY-MM-DD.
func nowDate() string {
	return time.Now().Format("2006-01-02")
}
