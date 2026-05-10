package main

import (
	"testing"
	"time"
)

func TestSaveAndLoadSessions(t *testing.T) {
	tmp := t.TempDir()
	historyDirOverride = tmp
	defer func() { historyDirOverride = "" }()

	session := &Session{
		Provider:  "openrouter",
		Model:     "test-model",
		StartTime: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		Messages: []Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi there"},
		},
	}

	if err := SaveSession(session); err != nil {
		t.Fatalf("SaveSession: %v", err)
	}

	sessions, err := LoadSessions()
	if err != nil {
		t.Fatalf("LoadSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if sessions[0].Model != "test-model" {
		t.Errorf("Model = %q", sessions[0].Model)
	}
	if len(sessions[0].Messages) != 2 {
		t.Errorf("Messages = %d", len(sessions[0].Messages))
	}
	if sessions[0].EndTime.IsZero() {
		t.Error("expected EndTime to be set")
	}
}

func TestSearchSessions(t *testing.T) {
	tmp := t.TempDir()
	historyDirOverride = tmp
	defer func() { historyDirOverride = "" }()

	s1 := &Session{
		Provider:  "openrouter",
		Model:     "m1",
		StartTime: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
		Messages:  []Message{{Role: "user", Content: "python tutorial"}},
	}
	s2 := &Session{
		Provider:  "openrouter",
		Model:     "m2",
		StartTime: time.Date(2024, 1, 15, 11, 0, 0, 0, time.UTC),
		Messages:  []Message{{Role: "user", Content: "golang tutorial"}},
	}
	SaveSession(s1)
	SaveSession(s2)

	matches, err := SearchSessions("python")
	if err != nil {
		t.Fatalf("SearchSessions: %v", err)
	}
	if len(matches) != 1 {
		t.Errorf("expected 1 match, got %d", len(matches))
	}
	if len(matches) > 0 && matches[0].Model != "m1" {
		t.Errorf("matched wrong session: %q", matches[0].Model)
	}

	matches, err = SearchSessions("tutorial")
	if err != nil {
		t.Fatalf("SearchSessions: %v", err)
	}
	if len(matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(matches))
	}

	matches, err = SearchSessions("rust")
	if err != nil {
		t.Fatalf("SearchSessions: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

func TestClearAllSessions(t *testing.T) {
	tmp := t.TempDir()
	historyDirOverride = tmp
	defer func() { historyDirOverride = "" }()

	SaveSession(&Session{
		Provider:  "openrouter",
		Model:     "m",
		StartTime: time.Now(),
		Messages:  []Message{{Role: "user", Content: "x"}},
	})

	n, err := ClearAllSessions()
	if err != nil {
		t.Fatalf("ClearAllSessions: %v", err)
	}
	if n != 1 {
		t.Errorf("cleared %d, want 1", n)
	}

	sessions, _ := LoadSessions()
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions after clear, got %d", len(sessions))
	}
}

func TestCleanupHistory(t *testing.T) {
	tmp := t.TempDir()
	historyDirOverride = tmp
	defer func() { historyDirOverride = "" }()

	for i := 0; i < MaxHistorySize+2; i++ {
		SaveSession(&Session{
			Provider:  "openrouter",
			Model:     "m",
			StartTime: time.Date(2024, 1, 1, 0, i, 0, 0, time.UTC),
			Messages:  []Message{{Role: "user", Content: "msg"}},
		})
	}

	CleanupHistory()

	sessions, _ := LoadSessions()
	if len(sessions) != MaxHistorySize {
		t.Errorf("expected %d sessions after cleanup, got %d", MaxHistorySize, len(sessions))
	}
}

func TestSessionPreview(t *testing.T) {
	s := Session{
		Messages: []Message{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "first question"},
			{Role: "assistant", Content: "answer"},
		},
	}
	preview := sessionPreview(s)
	if preview != "first question" {
		t.Errorf("preview = %q", preview)
	}

	s2 := Session{Messages: []Message{{Role: "assistant", Content: "only assistant"}}}
	if sessionPreview(s2) != "(no messages)" {
		t.Errorf("preview = %q", sessionPreview(s2))
	}
}

func TestDeleteSession(t *testing.T) {
	tmp := t.TempDir()
	historyDirOverride = tmp
	defer func() { historyDirOverride = "" }()

	start := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	SaveSession(&Session{
		Provider:  "openrouter",
		Model:     "m",
		StartTime: start,
		Messages:  []Message{{Role: "user", Content: "x"}},
	})

	sessions, _ := LoadSessions()
	if len(sessions) != 1 {
		t.Fatal("expected 1 session")
	}

	if err := DeleteSession(sessions[0]); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	sessions, _ = LoadSessions()
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions after delete, got %d", len(sessions))
	}
}
