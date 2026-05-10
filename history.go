package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Session struct {
	Provider  string    `json:"provider"`
	Model     string    `json:"model"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Messages  []Message `json:"messages"`
}

var historyDirOverride string // test hook

func historyDir() string {
	if historyDirOverride != "" {
		return historyDirOverride
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".cache", AppName, "history")
}

func SaveSession(session *Session) error {
	dir := historyDir()
	if dir == "" {
		return fmt.Errorf("cannot determine home directory")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating history directory: %w", err)
	}

	session.EndTime = time.Now()
	id := session.StartTime.Format("20060102-150405")
	path := filepath.Join(dir, fmt.Sprintf("%s.json", id))

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling session: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

func LoadSessions() ([]Session, error) {
	dir := historyDir()
	if dir == "" {
		return nil, fmt.Errorf("cannot determine home directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading history directory: %w", err)
	}

	var sessions []Session
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var s Session
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}
		sessions = append(sessions, s)
	}

	// newest first
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartTime.After(sessions[j].StartTime)
	})
	return sessions, nil
}

// SearchSessions filters sessions to those containing `query` (case-insensitive)
// in any user or assistant message. Returns sessions newest-first.
func SearchSessions(query string) ([]Session, error) {
	all, err := LoadSessions()
	if err != nil {
		return nil, err
	}
	q := strings.ToLower(query)
	var matches []Session
	for _, s := range all {
		for _, m := range s.Messages {
			if m.Role != "user" && m.Role != "assistant" {
				continue
			}
			if t, ok := m.Content.(string); ok {
				if strings.Contains(strings.ToLower(t), q) {
					matches = append(matches, s)
					break
				}
			}
		}
	}
	return matches, nil
}

func sessionPreview(s Session) string {
	for _, m := range s.Messages {
		if m.Role != "user" {
			continue
		}
		text := ""
		switch c := m.Content.(type) {
		case string:
			text = c
		}
		if text == "" {
			continue
		}
		if len(text) > 50 {
			text = text[:50] + "..."
		}
		return text
	}
	return "(no messages)"
}

// ClearAllSessions deletes every saved session JSON. Returns the number of
// files removed.
func ClearAllSessions() (int, error) {
	dir := historyDir()
	if dir == "" {
		return 0, fmt.Errorf("cannot determine home directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("reading history directory: %w", err)
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err == nil {
			count++
		}
	}
	return count, nil
}

// DeleteSession removes a single session file matching the given StartTime.
func DeleteSession(s Session) error {
	dir := historyDir()
	if dir == "" {
		return fmt.Errorf("cannot determine home directory")
	}
	id := s.StartTime.Format("20060102-150405")
	return os.Remove(filepath.Join(dir, id+".json"))
}

func CleanupHistory() {
	dir := historyDir()
	if dir == "" {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}

	sort.Strings(files)
	for len(files) > MaxHistorySize {
		_ = os.Remove(files[0])
		files = files[1:]
	}
}
