package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	maxRefSize       = 200 * 1024 // 200KB per reference
	maxRefCount      = 10         // max refs in a single message
	maxFetchedURLLen = 1024 * 1024
)

var refPattern = regexp.MustCompile(`@(\S+)`)

// ExpandReferences scans the user input for @-references (files, git shortcuts,
// or URLs) and replaces each with its actual content. Returns the expanded
// message and a list of human-readable summaries of what was included.
func ExpandReferences(input string) (string, []string) {
	matches := refPattern.FindAllStringIndex(input, -1)
	if len(matches) == 0 {
		return input, nil
	}

	var summaries []string
	count := 0
	expanded := refPattern.ReplaceAllStringFunc(input, func(match string) string {
		if count >= maxRefCount {
			return match
		}
		ref := match[1:] // strip leading @
		content, label, err := resolveReference(ref)
		if err != nil {
			return match // leave as-is on failure
		}
		count++
		summaries = append(summaries, fmt.Sprintf("%s (%s)", ref, label))
		return fmt.Sprintf("%s\n\n[Content of %s]\n```\n%s\n```\n",
			match, ref, content)
	})

	return expanded, summaries
}

// resolveReference looks up the content for a given @-reference.
// Returns (content, sizeLabel, error).
func resolveReference(ref string) (string, string, error) {
	// URL
	if strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://") {
		return fetchURL(ref)
	}

	// Git shortcuts
	switch ref {
	case "diff":
		return runGitCmd("diff")
	case "diff-staged":
		return runGitCmd("diff", "--staged")
	case "status":
		return runGitCmd("status", "--short")
	case "log":
		return runGitCmd("log", "-10", "--oneline")
	case "branch":
		return runGitCmd("branch", "--show-current")
	}

	// Local file
	return readLocalFile(ref)
}

// readLocalFile reads a file from disk, capping the size at maxRefSize.
func readLocalFile(path string) (string, string, error) {
	// Expand ~ to home directory
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", "", err
	}
	if info.IsDir() {
		return "", "", fmt.Errorf("%s is a directory", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", "", err
	}
	truncated := false
	if len(data) > maxRefSize {
		data = data[:maxRefSize]
		truncated = true
	}
	label := formatBytes(int(info.Size()))
	if truncated {
		label += ", truncated"
	}
	return string(data), label, nil
}

// runGitCmd runs `git <args>` and returns stdout.
func runGitCmd(args ...string) (string, string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", "", fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
	}
	output := strings.TrimSpace(string(out))
	if len(output) > maxRefSize {
		output = output[:maxRefSize] + "\n[... truncated ...]"
	}
	return output, formatBytes(len(output)), nil
}

// fetchURL downloads a URL and returns its body as text. HTML is reduced
// to plain text by stripping tags.
func fetchURL(url string) (string, string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; "+AppName+"-cli/"+AppVersion+")")
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return "", "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFetchedURLLen))
	if err != nil {
		return "", "", err
	}

	text := string(body)
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "html") {
		text = stripHTML(text)
	}

	if len(text) > maxRefSize {
		text = text[:maxRefSize] + "\n[... truncated ...]"
	}
	return text, formatBytes(len(text)), nil
}

// stripHTML removes script/style tags (with content) and all other HTML
// tags, leaving only readable text. Whitespace is collapsed.
var (
	scriptStyleRe = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</(script|style)>`)
	htmlTagRe     = regexp.MustCompile(`<[^>]+>`)
	whitespaceRe  = regexp.MustCompile(`\s+`)
)

func stripHTML(s string) string {
	s = scriptStyleRe.ReplaceAllString(s, " ")
	s = htmlTagRe.ReplaceAllString(s, " ")
	s = whitespaceRe.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// formatBytes returns a human-friendly size string.
func formatBytes(n int) string {
	switch {
	case n >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	case n >= 1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
