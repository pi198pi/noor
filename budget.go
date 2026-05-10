package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// budgetData persists daily cost totals to disk so the user can be warned
// when approaching their configured daily budget.
type budgetData struct {
	Daily map[string]float64 `json:"daily"` // YYYY-MM-DD → USD
}

func budgetPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache", AppName, "budget.json")
}

func loadBudget() *budgetData {
	b := &budgetData{Daily: make(map[string]float64)}
	path := budgetPath()
	if path == "" {
		return b
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return b
	}
	_ = json.Unmarshal(data, b)
	if b.Daily == nil {
		b.Daily = make(map[string]float64)
	}
	return b
}

func saveBudget(b *budgetData) error {
	path := budgetPath()
	if path == "" {
		return fmt.Errorf("cannot determine home directory")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	// Garbage-collect entries older than 60 days
	cutoff := time.Now().AddDate(0, 0, -60).Format("2006-01-02")
	for k := range b.Daily {
		if k < cutoff {
			delete(b.Daily, k)
		}
	}
	data, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

// AddDailyCost records `amount` against today's running total and persists
// it. Returns the new daily total.
func AddDailyCost(amount float64) float64 {
	b := loadBudget()
	today := time.Now().Format("2006-01-02")
	b.Daily[today] += amount
	_ = saveBudget(b)
	return b.Daily[today]
}

// TodayCost returns today's running total without modifying it.
func TodayCost() float64 {
	b := loadBudget()
	return b.Daily[time.Now().Format("2006-01-02")]
}

// BudgetStatus returns a warning level (0/50/80/100) and a message
// when daily cost crosses certain thresholds.
func BudgetStatus(daily, limit float64) (level int, message string) {
	if limit <= 0 {
		return 0, ""
	}
	pct := daily / limit
	switch {
	case pct >= 1.0:
		return 100, fmt.Sprintf("Daily budget exceeded: $%.4f / $%.2f", daily, limit)
	case pct >= 0.8:
		return 80, fmt.Sprintf("Daily budget at %.0f%%: $%.4f / $%.2f", pct*100, daily, limit)
	case pct >= 0.5:
		return 50, fmt.Sprintf("Daily budget at %.0f%%: $%.4f / $%.2f", pct*100, daily, limit)
	}
	return 0, ""
}
