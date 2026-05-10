package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

const htmlTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>%s</title>
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; max-width: 860px; margin: 40px auto; padding: 0 24px; color: #1a1a1a; line-height: 1.7; }
  header { border-bottom: 1px solid #e0e0e0; padding-bottom: 12px; margin-bottom: 32px; color: #666; font-size: 0.85em; }
  h1,h2,h3,h4 { margin-top: 1.6em; margin-bottom: 0.4em; font-weight: 600; }
  h1 { font-size: 1.6em; } h2 { font-size: 1.35em; } h3 { font-size: 1.15em; }
  code { background: #f4f4f4; padding: 2px 5px; border-radius: 4px; font-size: 0.9em; font-family: "SF Mono", Menlo, Consolas, monospace; }
  pre { background: #f4f4f4; padding: 16px; border-radius: 6px; overflow-x: auto; }
  pre code { background: none; padding: 0; }
  blockquote { border-left: 4px solid #d0d0d0; margin: 0; padding-left: 16px; color: #555; }
  table { border-collapse: collapse; width: 100%%; margin: 16px 0; }
  th, td { border: 1px solid #ddd; padding: 8px 12px; text-align: left; }
  th { background: #f4f4f4; font-weight: 600; }
  a { color: #0066cc; } hr { border: none; border-top: 1px solid #e0e0e0; margin: 24px 0; }
  @media print { body { margin: 0; } }
</style>
</head>
<body>
<header>%s %s &nbsp;·&nbsp; %s &nbsp;·&nbsp; %s</header>
%s
</body>
</html>`

func ExportResponse(content, model, filename string) error {
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("no response to export")
	}

	ext := strings.ToLower(filepath.Ext(filename))

	// Markdown export — write raw markdown as-is
	if ext == ".md" {
		return writeFile(filename, content)
	}

	// Code file — extract first code block or fall back to raw content
	if ext != "" && ext != ".html" {
		code := extractCodeBlock(content)
		if code == "" {
			code = content
		}
		return writeFile(filename, code)
	}

	// HTML export (default)
	if ext == "" {
		filename += ".html"
	}

	md := goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Table),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)
	var body strings.Builder
	if err := md.Convert([]byte(content), &body); err != nil {
		return fmt.Errorf("rendering markdown: %w", err)
	}

	title := AppName + " export " + time.Now().Format("2006-01-02")
	out := fmt.Sprintf(htmlTemplate,
		title, AppName, AppVersion, model,
		time.Now().Format("Jan 2, 2006 15:04"),
		body.String(),
	)
	return writeFile(filename, out)
}

func writeFile(filename, content string) error {
	dir := filepath.Dir(filename)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory: %w", err)
		}
	}
	return os.WriteFile(filename, []byte(content), 0644)
}

// extractCodeBlock pulls the content of the first fenced code block
// (supports both ``` and ~~~ fences).
func extractCodeBlock(s string) string {
	lines := strings.Split(s, "\n")
	var result []string
	var fence string
	inside := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !inside {
			if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
				inside = true
				fence = trimmed[:3]
			}
			continue
		}
		if strings.HasPrefix(trimmed, fence) {
			break
		}
		result = append(result, line)
	}
	return strings.TrimSpace(strings.Join(result, "\n"))
}
