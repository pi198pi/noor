package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

// streamResponse performs a streaming API call, showing an animated progress
// indicator (spinner + character count) on a single line. Once the stream
// completes, the indicator clears and the full response renders through
// glamour for proper markdown formatting. This keeps the terminal clean and
// avoids the cursor-positioning issues that come with re-rendering scrolled
// content.
func streamResponse(ctx context.Context, api *APIClient, cfg *Config, messages *[]Message, tools []Tool) (*APIResult, error) {
	var charsReceived int64

	stop := make(chan struct{})
	done := make(chan struct{})

	go func() {
		defer close(done)
		frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-stop:
				fmt.Print("\r\033[K")
				return
			case <-ticker.C:
				n := atomic.LoadInt64(&charsReceived)
				label := "thinking..."
				if n > 0 {
					label = "receiving... " + formatChars(int(n))
				}
				fmt.Printf("\r  %s %s",
					styleSpinner.Render(frames[i]),
					styleDim.Render(label),
				)
				i = (i + 1) % len(frames)
			}
		}
	}()

	result, err := api.Stream(ctx, *messages, cfg, tools, func(chunk string) {
		atomic.AddInt64(&charsReceived, int64(len(chunk)))
	})

	close(stop)
	<-done

	if err != nil {
		return nil, fmt.Errorf("streaming failed: %w", err)
	}

	if result.Content != "" {
		fmt.Print(strings.TrimLeft(renderMarkdown(result.Content), "\n"))
	}

	return result, nil
}

// formatChars formats a character count as "1.2K chars" or "342 chars".
func formatChars(n int) string {
	if n >= 1000 {
		return fmt.Sprintf("%.1fK chars", float64(n)/1000)
	}
	return fmt.Sprintf("%d chars", n)
}

// executeToolCall runs a single tool call, dispatching based on the tool name.
func executeToolCall(tc ToolCall, cfg *Config, mcp *MCPClient) string {
	name := tc.Function.Name

	var args map[string]interface{}
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
		return fmt.Sprintf("error parsing tool arguments: %v", err)
	}

	if name == "web_search" {
		query, _ := args["query"].(string)
		return tinyfishWebSearch(query, cfg.TinyFishKey)
	}

	if mcp != nil {
		fmt.Printf("\r\033[K  %s", styleInfo.Render("⚙ "+name+"..."))
		output, err := mcp.CallTool(name, args)
		fmt.Print("\r\033[K")
		if err != nil {
			return fmt.Sprintf("tool error: %v", err)
		}
		return output
	}

	return fmt.Sprintf("unknown tool: %s", name)
}

// buildTools assembles the list of tools available during a chat turn.
func buildTools(cfg *Config, mcp *MCPClient) []Tool {
	var tools []Tool
	if cfg.TinyFishKey != "" {
		tools = append(tools, builtinWebSearchTool)
	}
	if mcp != nil {
		tools = append(tools, mcp.Tools()...)
	}
	return tools
}
