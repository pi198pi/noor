package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const maxToolCalls = 5

// chatLoop is the main interactive REPL. It reads user input, dispatches
// slash commands or chat messages, and streams responses.
func chatLoop(cfg *Config, mcp *MCPClient) {
	apiClient := NewAPIClient(cfg.APIURL, cfg.APIKey)

	session := &Session{
		Provider:  "openrouter",
		Model:     cfg.Model,
		StartTime: time.Now(),
	}
	var sessionCost float64

	showBanner(cfg)
	messages := initMessages(cfg)

	for {
		prompt := fmt.Sprintf("\n%s %s %s\n",
			connDot(),
			styleProvider.Render("OR"),
			styleDim.Render("❯"),
		)
		input, ok := ReadInput(prompt)
		if !ok {
			break
		}
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			if s := formatCost(sessionCost); s != "" {
				fmt.Println(styleDim.Render("  session cost: ") + styleSuccess.Render(s))
			}
			fmt.Println(styleDim.Render("  goodbye"))
			break
		}

		loc, isWeather := parseWeather(input)
		if isWeather {
			handleWeather(loc)
			continue
		}

		if strings.HasPrefix(input, "/") {
			if handleSlashCommand(input, cfg, &messages, &mcp, apiClient, &sessionCost, session) {
				continue
			}
		}

		// Prepend any /attach-queued @-references to this message.
		if prefix := drainAttachments(); prefix != "" {
			input = prefix + input
		}

		// Expand any @-references (files, git shortcuts, URLs) before sending
		expanded, summaries := ExpandReferences(input)
		for _, s := range summaries {
			printInfo("📎 Included: " + s)
		}

		// Suggest a better model if the input warrants it
		if suggested := suggestModel(input, cfg.Model); suggested != "" {
			printInfo(fmt.Sprintf("💡 Tip: try `/model %s` for better results", suggested))
		}

		messages = append(messages, Message{Role: "user", Content: sanitize(expanded)})

		// Per-request context that Ctrl+C cancels (without killing the app)
		ctx, cancel := context.WithCancel(context.Background())
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT)
		go func() {
			select {
			case <-sigCh:
				cancel()
			case <-ctx.Done():
			}
		}()

		response, cost := sendMessage(ctx, apiClient, cfg, &messages, mcp, sessionCost)
		signal.Stop(sigCh)
		cancel()

		sessionCost += cost
		if cost > 0 && cfg.DailyBudget > 0 {
			daily := AddDailyCost(cost)
			if level, msg := BudgetStatus(daily, cfg.DailyBudget); level > 0 {
				switch level {
				case 100:
					printError("💸 " + msg)
				case 80:
					printWarning("💰 " + msg)
				case 50:
					printInfo("💵 " + msg)
				}
			}
		}

		if response != "" && !cfg.NoHistory {
			session.Messages = messages
		}
	}

	if !cfg.NoHistory && len(messages) > 1 {
		session.Messages = messages
		if err := SaveSession(session); err == nil {
			CleanupHistory()
		}
	}

	// Close any in-process MCP server (one started via /mcp during the session).
	if mcp != nil {
		mcp.Close()
	}
}

// setupSignals handles SIGTERM gracefully (closes MCP and exits).
// SIGINT (Ctrl+C) is handled per-request to cancel in-flight calls,
// not the whole application.
func setupSignals(mcp *MCPClient) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM)
	go func() {
		<-sigCh
		if mcp != nil {
			mcp.Close()
		}
		os.Exit(0)
	}()
}

// sendMessage sends a single user prompt to the API, handles streaming and
// any tool-call loops, and returns the assistant response and cost.
func sendMessage(ctx context.Context, api *APIClient, cfg *Config, messages *[]Message, mcp *MCPClient, sessionCost float64) (string, float64) {
	tools := buildTools(cfg, mcp)
	toolCallCount := 0
	forcedSummary := false
	start := time.Now()

	assistantHeader()
	for {
		result, err := streamResponse(ctx, api, cfg, messages, tools)
		if err != nil {
			printError(err.Error())
			// Remove the user message that caused the error
			*messages = (*messages)[:len(*messages)-1]
			return "", 0
		}

		// No tool calls — we're done
		if len(result.ToolCalls) == 0 {
			*messages = append(*messages, Message{Role: "assistant", Content: result.Content})
			assistantFooter(time.Since(start), result.Tokens, approxTokens(*messages), cfg.Model, result.Cost, sessionCost+result.Cost)
			return result.Content, result.Cost
		}

		// Execute tool calls
		toolCallCount += len(result.ToolCalls)
		*messages = append(*messages, Message{
			Role:      "assistant",
			Content:   result.Content,
			ToolCalls: result.ToolCalls,
		})

		for _, tc := range result.ToolCalls {
			output := executeToolCall(tc, cfg, mcp)
			*messages = append(*messages, Message{
				Role:       "tool",
				Content:    output,
				Name:       tc.Function.Name,
				ToolCallID: tc.ID,
			})
		}

		// Force a final answer if we've made too many tool calls
		if toolCallCount >= maxToolCalls && !forcedSummary {
			forcedSummary = true
			*messages = append(*messages, Message{
				Role:    "user",
				Content: "Please summarize what you found and give your best answer now.",
			})
		}
	}
}

// sanitize removes control characters from input, keeping tabs, newlines,
// and printable characters.
func sanitize(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if r >= 0x20 || r == '\t' || r == '\n' {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// isImageModel reports whether the model supports image modalities.
func isImageModel(model string) bool {
	return strings.Contains(model, "-image")
}

// parseWeather checks if input is a weather query.
func parseWeather(input string) (string, bool) {
	lower := strings.ToLower(input)
	if strings.HasPrefix(lower, "weather ") {
		return strings.TrimSpace(input[8:]), true
	}
	return "", false
}

// initMessages builds the initial message list with system prompt.
func initMessages(cfg *Config) []Message {
	date := "Current date and time (UTC): " + time.Now().UTC().Format("January 2, 2006 15:04:05 UTC") + "."
	var parts []string
	parts = append(parts, date)
	if cfg.SystemPrompt != "" {
		parts = append(parts, cfg.SystemPrompt)
	} else if p, ok := stylePrompts[cfg.Style]; ok && p != "" {
		parts = append(parts, p)
	}
	return []Message{{Role: "system", Content: strings.Join(parts, "\n\n")}}
}
