package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

// Plugin is a user-defined tool loaded from ~/.config/noor/plugins/*.json.
// The command field is a Go text/template that receives the tool arguments
// as its data context. Arguments with spaces should be quoted in the template.
type Plugin struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
	Command     string                 `json:"command"`
}

// pluginCommands maps tool name → command template for dispatch.
// Populated once at startup by loadPlugins.
var pluginCommands = make(map[string]string)

func pluginsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", AppName, "plugins")
}

// loadPlugins recursively scans ~/.config/noor/plugins/ and all subdirs
// for *.json files. Each subdirectory acts as a category (purely cosmetic —
// the tool name must be unique across all plugins).
func loadPlugins() []Tool {
	dir := pluginsDir()
	if dir == "" {
		return nil
	}

	var tools []Tool
	var files []string

	// Walk all subdirectories, depth-first
	_ = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			Log.Debug("plugins walk error", "path", path, "err", err)
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(d.Name()) == ".json" {
			files = append(files, path)
		}
		return nil
	})

	for _, path := range files {
		name := filepath.Base(path)
		data, err := os.ReadFile(path)
		if err != nil {
			Log.Debug("plugin read error", "file", name, "err", err)
			continue
		}

		var p Plugin
		if err := json.Unmarshal(data, &p); err != nil {
			Log.Debug("plugin parse error", "file", name, "err", err)
			continue
		}

		if p.Name == "" || p.Command == "" {
			Log.Debug("plugin missing name or command", "file", name)
			continue
		}

		// Register command for executeToolCall dispatch
		pluginCommands[p.Name] = p.Command

		tools = append(tools, Tool{
			Type: "function",
			Function: ToolFunction{
				Name:        p.Name,
				Description: p.Description,
				Parameters:  p.Parameters,
			},
		})
		Log.Debug("plugin loaded", "name", p.Name, "file", name)
	}

	return tools
}

// pluginExecTimeout is the maximum time a plugin command may run.
var pluginExecTimeout = 30 * time.Second

// executePlugin runs the plugin command template with args as the data context.
// The command is executed via sh -c so shell features work naturally.
// A timeout prevents runaway plugins.
func executePlugin(name string, args map[string]interface{}) string {
	cmdTemplate, ok := pluginCommands[name]
	if !ok {
		return fmt.Sprintf("plugin not found: %s", name)
	}

	tmpl, err := template.New("plugin").Parse(cmdTemplate)
	if err != nil {
		return fmt.Sprintf("plugin %s: bad command template: %v", name, err)
	}

	var cmdBuf strings.Builder
	if err := tmpl.Execute(&cmdBuf, args); err != nil {
		return fmt.Sprintf("plugin %s: template execution failed: %v", name, err)
	}

	Log.Debug("plugin execute", "name", name, "command", cmdBuf.String())

	ctx, cancel := context.WithTimeout(context.Background(), pluginExecTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", cmdBuf.String())
	output, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Sprintf("plugin %s: timed out after 30s", name)
		}
		if exitErr, ok := err.(*exec.ExitError); ok && len(exitErr.Stderr) > 0 {
			return fmt.Sprintf("plugin %s: %s", name, string(exitErr.Stderr))
		}
		return fmt.Sprintf("plugin %s: %v", name, err)
	}

	out := strings.TrimSpace(string(output))
	if len(out) > MaxToolOutput {
		out = out[:MaxToolOutput] + "\n[... output truncated ...]"
	}
	return out
}
