package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportResponseMarkdown(t *testing.T) {
	tmp := t.TempDir()
	filename := filepath.Join(tmp, "test.md")
	content := "# Hello\n\nThis is **bold** and `code`.\n\n```go\nfmt.Println(\"hi\")\n```\n"

	if err := ExportResponse(content, "test-model", filename); err != nil {
		t.Fatalf("ExportResponse: %v", err)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Should be raw markdown, not HTML-wrapped
	if string(data) != content {
		t.Errorf("content mismatch:\ngot:\n%s\nwant:\n%s", string(data), content)
	}

	// Should NOT contain HTML tags
	if strings.Contains(string(data), "<html>") || strings.Contains(string(data), "<body>") {
		t.Error("markdown export should not contain HTML tags")
	}
}

func TestExportResponseCode(t *testing.T) {
	tmp := t.TempDir()
	filename := filepath.Join(tmp, "test.py")
	content := "Some intro text.\n\n```python\ndef hello():\n    print('world')\n```\n\nMore text."

	if err := ExportResponse(content, "test-model", filename); err != nil {
		t.Fatalf("ExportResponse: %v", err)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Should extract just the code block
	want := "def hello():\n    print('world')"
	if string(data) != want {
		t.Errorf("code = %q, want %q", string(data), want)
	}
}

func TestExportResponseEmptyContent(t *testing.T) {
	err := ExportResponse("   ", "model", "out.md")
	if err == nil {
		t.Fatal("expected error for empty content")
	}
	if !strings.Contains(err.Error(), "no response") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestExportResponseHTML(t *testing.T) {
	tmp := t.TempDir()
	filename := filepath.Join(tmp, "export.html")
	content := "# Hello\n"

	if err := ExportResponse(content, "test-model", filename); err != nil {
		t.Fatalf("ExportResponse: %v", err)
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Should contain rendered HTML (goldmark converts # Hello to <h1>)
	if !strings.Contains(string(data), "<h1>") {
		t.Errorf("HTML export should contain rendered <h1>, got:\n%s", string(data))
	}
}

func TestExtractCodeBlock(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			"```python\nprint(1)\n```",
			"print(1)",
		},
		{
			"~~~go\nfmt.Println(1)\n~~~",
			"fmt.Println(1)",
		},
		{
			"some text\n\n```\ncode here\n```",
			"code here",
		},
		{
			"no code block here",
			"",
		},
		{
			"```\nline1\nline2\n```\n\n```\nother\n```",
			"line1\nline2",
		},
	}

	for _, tt := range tests {
		got := extractCodeBlock(tt.input)
		if got != tt.expected {
			t.Errorf("extractCodeBlock(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
