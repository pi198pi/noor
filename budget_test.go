package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBudgetAddAndToday(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "budget.json")
	budgetPathOverride = tmp
	defer func() { budgetPathOverride = "" }()

	total := AddDailyCost(0.5)
	if total != 0.5 {
		t.Errorf("total = %f, want 0.5", total)
	}

	total = AddDailyCost(0.3)
	if total != 0.8 {
		t.Errorf("total = %f, want 0.8", total)
	}

	today := TodayCost()
	if today != 0.8 {
		t.Errorf("today = %f, want 0.8", today)
	}
}

func TestBudgetStatus(t *testing.T) {
	tests := []struct {
		daily      float64
		limit      float64
		wantLevel  int
		wantPrefix string
	}{
		{0.3, 1.0, 0, ""},
		{0.6, 1.0, 50, "Daily budget at 60%"},
		{0.85, 1.0, 80, "Daily budget at 85%"},
		{1.2, 1.0, 100, "Daily budget exceeded"},
		{0.5, 0, 0, ""},
	}

	for _, tt := range tests {
		level, msg := BudgetStatus(tt.daily, tt.limit)
		if level != tt.wantLevel {
			t.Errorf("BudgetStatus(%f, %f) level = %d, want %d", tt.daily, tt.limit, level, tt.wantLevel)
		}
		if tt.wantPrefix != "" && !strings.HasPrefix(msg, tt.wantPrefix) {
			t.Errorf("BudgetStatus(%f, %f) msg = %q, want prefix %q", tt.daily, tt.limit, msg, tt.wantPrefix)
		}
	}
}
