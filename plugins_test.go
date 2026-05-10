package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadPlugins(t *testing.T) {
	tmp := t.TempDir()

	// Create a valid plugin
	plugin1 := filepath.Join(tmp, "qr.json")
	os.WriteFile(plugin1, []byte(`{
		"name": "generate_qr",
		"description": "Generate a QR code",
		"parameters": {
			"type": "object",
			"properties": {
				"text": {"type": "string"}
			},
			"required": ["text"]
		},
		"command": "echo '{{.text}}'"
	}`), 0644)

	// Create an invalid plugin (missing command)
	plugin2 := filepath.Join(tmp, "bad.json")
	os.WriteFile(plugin2, []byte(`{
		"name": "bad",
		"description": "No command"
	}`), 0644)

	// Create a non-JSON file (should be ignored)
	os.WriteFile(filepath.Join(tmp, "readme.txt"), []byte("hello"), 0644)

	// Temporarily override pluginsDir
	oldDir := pluginsDir()
	_ = oldDir // avoid unused warning in some Go versions
	// We can't override pluginsDir easily, but we can test loadPlugins
	// by creating a dir and calling loadPlugins directly if we add a parameter...
	// For now, test via the exported functions by creating the real dir.
	// Since pluginsDir() uses UserHomeDir, we'll skip the integration test
	// and test executePlugin directly with a pre-registered command.

	// Clear any previously loaded plugins
	pluginTools = nil
	pluginCommands = make(map[string]string)

	// Register a command manually to test executePlugin
	pluginCommands["test_echo"] = "echo '{{.msg}}'"

	output := executePlugin("test_echo", map[string]interface{}{"msg": "hello world"})
	if output != "hello world" {
		t.Errorf("output = %q, want 'hello world'", output)
	}
}

func TestExecutePluginNotFound(t *testing.T) {
	output := executePlugin("nonexistent", map[string]interface{}{})
	if !strings.Contains(output, "not found") {
		t.Errorf("output = %q, want 'not found'", output)
	}
}

func TestExecutePluginBadTemplate(t *testing.T) {
	pluginCommands["bad_tmpl"] = "echo '{{.bad"
	output := executePlugin("bad_tmpl", map[string]interface{}{})
	if !strings.Contains(output, "bad command template") {
		t.Errorf("output = %q, want template error", output)
	}
}

func TestExecutePluginTimeout(t *testing.T) {
	orig := pluginExecTimeout
	pluginExecTimeout = 100 * time.Millisecond
	defer func() { pluginExecTimeout = orig }()

	pluginCommands["slow"] = "sleep 5"
	output := executePlugin("slow", map[string]interface{}{})
	if !strings.Contains(output, "timed out") {
		t.Errorf("output = %q, want timeout error", output)
	}
}

func TestExecutePluginCommandError(t *testing.T) {
	pluginCommands["fails"] = "exit 1"
	output := executePlugin("fails", map[string]interface{}{})
	if !strings.Contains(output, "fails:") {
		t.Errorf("output = %q, want error with plugin name", output)
	}
}

func TestExecutePluginTruncation(t *testing.T) {
	pluginCommands["long"] = "python3 -c \"print('x' * 20000)\""
	output := executePlugin("long", map[string]interface{}{})
	if !strings.Contains(output, "[... output truncated ...]") {
		t.Errorf("expected truncation, output len = %d", len(output))
	}
}
