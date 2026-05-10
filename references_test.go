package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandReferencesFile(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "test.txt")
	if err := os.WriteFile(f, []byte("file content here"), 0644); err != nil {
		t.Fatal(err)
	}

	expanded, summaries := ExpandReferences("read @" + f)
	if !strings.Contains(expanded, "file content here") {
		t.Errorf("expected file content in expansion, got:\n%s", expanded)
	}
	if len(summaries) != 1 {
		t.Errorf("expected 1 summary, got %d", len(summaries))
	}
	if !strings.Contains(summaries[0], "test.txt") {
		t.Errorf("summary = %q", summaries[0])
	}
}

func TestExpandReferencesURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body><p>hello world</p></body></html>"))
	}))
	defer server.Close()

	oldClient := httpClient
	httpClient = server.Client()
	defer func() { httpClient = oldClient }()

	expanded, summaries := ExpandReferences("check @" + server.URL)
	if !strings.Contains(expanded, "hello world") {
		t.Errorf("expected plain text in expansion, got:\n%s", expanded)
	}
	if strings.Contains(expanded, "<html>") {
		t.Error("HTML should have been stripped")
	}
	if len(summaries) != 1 {
		t.Errorf("expected 1 summary, got %d", len(summaries))
	}
}

func TestExpandReferencesMaxCount(t *testing.T) {
	tmp := t.TempDir()
	input := ""
	for i := 0; i < maxRefCount+2; i++ {
		f := filepath.Join(tmp, fmt.Sprintf("f%d.txt", i))
		os.WriteFile(f, []byte("x"), 0644)
		if i > 0 {
			input += " "
		}
		input += "@" + f
	}

	expanded, summaries := ExpandReferences(input)
	if len(summaries) != maxRefCount {
		t.Errorf("expected %d summaries (max), got %d", maxRefCount, len(summaries))
	}
	// Remaining refs should be left as-is (no [Content of...] expansion)
	contentMarkers := strings.Count(expanded, "[Content of")
	if contentMarkers != maxRefCount {
		t.Errorf("expected %d content markers, got %d", maxRefCount, contentMarkers)
	}
}

func TestExpandReferencesMissingFile(t *testing.T) {
	expanded, summaries := ExpandReferences("read @/nonexistent/path/file.txt")
	if !strings.Contains(expanded, "@/nonexistent/path/file.txt") {
		t.Errorf("expected unchanged ref, got:\n%s", expanded)
	}
	if len(summaries) != 0 {
		t.Errorf("expected 0 summaries, got %d", len(summaries))
	}
}

func TestStripHTML(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<p>hello</p>", "hello"},
		{"<script>alert(1)</script>hi", "hi"},
		{"<style>body{}</style>text", "text"},
		{"<div><span>  a  b  </span></div>", "a b"},
		{"", ""},
		{"plain text", "plain text"},
	}

	for _, tt := range tests {
		got := stripHTML(tt.input)
		if got != tt.expected {
			t.Errorf("stripHTML(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		n        int
		expected string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{2 * 1024 * 1024, "2.0 MB"},
	}

	for _, tt := range tests {
		got := formatBytes(tt.n)
		if got != tt.expected {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.n, got, tt.expected)
		}
	}
}

func TestReadLocalFile(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "small.txt")
	os.WriteFile(f, []byte("hello"), 0644)

	content, label, err := readLocalFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "hello" {
		t.Errorf("content = %q", content)
	}
	if label != "5 B" {
		t.Errorf("label = %q", label)
	}
}

func TestReadLocalFileTruncated(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "big.txt")
	data := make([]byte, maxRefSize+100)
	for i := range data {
		data[i] = 'x'
	}
	os.WriteFile(f, data, 0644)

	content, label, err := readLocalFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content) != maxRefSize {
		t.Errorf("content len = %d, want %d", len(content), maxRefSize)
	}
	if !strings.Contains(label, "truncated") {
		t.Errorf("label = %q, expected 'truncated'", label)
	}
}

func TestReadLocalFileDirectory(t *testing.T) {
	tmp := t.TempDir()
	_, _, err := readLocalFile(tmp)
	if err == nil {
		t.Fatal("expected error for directory")
	}
}

func TestReadLocalFileMissing(t *testing.T) {
	_, _, err := readLocalFile("/nonexistent/file.txt")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestFetchURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); !strings.Contains(ua, AppName) {
			t.Errorf("bad user-agent: %q", ua)
		}
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte("<html><body><p>fetched content</p></body></html>"))
	}))
	defer server.Close()

	oldClient := httpClient
	httpClient = server.Client()
	defer func() { httpClient = oldClient }()

	content, label, err := fetchURL(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(content, "fetched content") {
		t.Errorf("content = %q", content)
	}
	if strings.Contains(content, "<html>") {
		t.Error("HTML should have been stripped")
	}
	if label == "" {
		t.Error("expected non-empty label")
	}
}

func TestFetchURLErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	oldClient := httpClient
	httpClient = server.Client()
	defer func() { httpClient = oldClient }()

	_, _, err := fetchURL(server.URL)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("error = %q, want 404 mention", err.Error())
	}
}
