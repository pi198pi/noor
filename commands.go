package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// handleSlashCommand dispatches slash commands. Returns true if the input
// was handled as a command (no further API call needed).
func handleSlashCommand(input string, cfg *Config, messages *[]Message, mcp **MCPClient, api *APIClient, sessionCost *float64, session *Session) bool {
	parts := strings.SplitN(strings.TrimPrefix(input, "/"), " ", 2)
	cmd := strings.ToLower(parts[0])
	args := ""
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}

	switch cmd {
	case "help":
		printHelp(cfg, *mcp)

	case "clear":
		fmt.Print("\033[H\033[2J")
		*messages = initMessages(cfg)
		showBanner(cfg)

	case "reset":
		*messages = initMessages(cfg)
		printSuccess("Context reset.")

	case "model":
		handleModelCommand(args, cfg)

	case "style":
		handleStyleCommand(args, cfg, messages)

	case "theme":
		handleThemeCommand(args, cfg)

	case "system":
		handleSystemCommand(args, cfg, messages)

	case "history":
		handleHistoryCommand(args, messages, cfg, session)

	case "tools":
		handleToolsCommand(cfg, *mcp)

	case "mcp":
		handleMCPCommand(args, mcp)

	case "imagine":
		handleImagineCommand(args, cfg, api)

	case "image":
		handleImageCommand(args, cfg, messages, *mcp, api, sessionCost)

	case "export":
		handleExportCommand(args, messages, cfg)

	case "copy":
		handleCopyCommand(messages)

	case "retry":
		handleRetryCommand(cfg, messages, *mcp, api, sessionCost)

	case "edit":
		handleEditCommand(cfg, messages, *mcp, api, sessionCost)

	case "budget":
		handleBudgetCommand(args, cfg)

	case "compress":
		handleCompressCommand(cfg, messages, api)

	case "search":
		handleSearchCommand(args, messages, cfg, session)

	case "freeze":
		handleFreezeCommand(messages)

	case "attach":
		handleAttachCommand()

	default:
		printWarning("Unknown command: /" + cmd + "  (type /help for commands)")
	}

	return true
}

// handleMCPCommand starts, stops, or shows status of the MCP server.
//
//	/mcp                  → status
//	/mcp <command>        → start a new MCP server (replaces existing)
//	/mcp stop             → close the current server
func handleMCPCommand(args string, mcp **MCPClient) {
	if args == "" {
		if *mcp == nil {
			printInfo("No MCP server connected. Use: /mcp <command>")
			return
		}
		printInfo(fmt.Sprintf("MCP connected — %d tools loaded", len((*mcp).Tools())))
		return
	}

	if args == "stop" {
		if *mcp == nil {
			printWarning("No MCP server to stop.")
			return
		}
		(*mcp).Close()
		*mcp = nil
		printSuccess("MCP server stopped.")
		return
	}

	// Replace any existing connection
	if *mcp != nil {
		(*mcp).Close()
		*mcp = nil
	}

	spin := NewSpinner("starting MCP server...")
	spin.Start()
	client, err := NewMCPClient(args)
	spin.Stop()

	if err != nil {
		printError("MCP failed to start: " + err.Error())
		return
	}
	*mcp = client
	printSuccess(fmt.Sprintf("MCP started — %d tools loaded", len(client.Tools())))
}

func handleModelCommand(args string, cfg *Config) {
	if args == "" {
		chosen := RunPicker(OpenRouterModels, cfg.Model)
		if chosen != "" && chosen != cfg.Model {
			cfg.Model = chosen
			if err := saveModel(cfg.Model); err != nil {
				printWarning("Could not save model preference: " + err.Error())
			}
			printSuccess("Model → " + cfg.Model)
		}
	} else {
		cfg.Model = args
		if err := saveModel(cfg.Model); err != nil {
			printWarning("Could not save model preference: " + err.Error())
		}
		printSuccess("Model → " + cfg.Model)
	}
}

func handleThemeCommand(args string, cfg *Config) {
	chosen := args
	if chosen == "" {
		chosen = RunPicker(ThemeNames(), currentTheme)
	}
	if chosen == "" {
		return
	}
	if !applyTheme(chosen) {
		printWarning("Unknown theme. Options: " + strings.Join(ThemeNames(), ", "))
		return
	}
	cfg.Theme = chosen
	if err := saveTheme(chosen); err != nil {
		printWarning("Could not save theme: " + err.Error())
	}
	printSuccess("Theme → " + chosen)
}

func handleStyleCommand(args string, cfg *Config, messages *[]Message) {
	chosen := args
	if chosen == "" {
		chosen = RunPicker([]string{"markdown", "plain", "concise", "raw"}, cfg.Style)
	}
	if chosen == "" {
		return
	}
	if _, ok := stylePrompts[chosen]; !ok {
		printWarning("Unknown style. Options: markdown, plain, concise, raw")
		return
	}
	cfg.Style = chosen
	*messages = initMessages(cfg)
	printSuccess("Style → " + chosen + " (context reset)")
}

func handleSystemCommand(args string, cfg *Config, messages *[]Message) {
	if args == "" {
		// Open a multi-line editor pre-populated with current prompt.
		text := cfg.SystemPrompt
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewText().
					Title("System prompt").
					Description("ctrl+d submit · esc cancel").
					Value(&text),
			),
		).WithShowHelp(false).WithKeyMap(huhKeyMap())
		if err := form.Run(); err != nil {
			printInfo("Cancelled.")
			return
		}
		args = strings.TrimSpace(text)
		if args == "" {
			printInfo("Cancelled.")
			return
		}
	}
	cfg.SystemPrompt = args
	*messages = initMessages(cfg)
	printSuccess("System prompt updated (context reset)")
}

func handleHistoryCommand(args string, messages *[]Message, cfg *Config, session *Session) {
	if args == "clear" {
		handleHistoryClear()
		return
	}

	sessions, err := LoadSessions()
	if err != nil || len(sessions) == 0 {
		printInfo("No sessions found.")
		return
	}
	chosen := RunHistoryPicker(sessions)
	if chosen == nil {
		return
	}
	// Replace system prompt with current config, then append history messages
	newMsgs := initMessages(cfg)
	for _, m := range chosen.Messages {
		if m.Role != "system" {
			newMsgs = append(newMsgs, m)
		}
	}
	*messages = newMsgs
	cfg.Model = chosen.Model
	if err := saveModel(cfg.Model); err != nil {
		printWarning("Could not save model preference: " + err.Error())
	}
	// Reuse the original session's StartTime so SaveSession overwrites
	// the original file rather than creating a new one.
	if session != nil {
		session.StartTime = chosen.StartTime
		session.Model = chosen.Model
	}
	printSuccess(fmt.Sprintf("Resumed session from %s  ·  %s",
		chosen.StartTime.Format("Jan 02 15:04"), chosen.Model))
}

// handleHistoryClear deletes every saved session after a confirmation prompt.
func handleHistoryClear() {
	sessions, _ := LoadSessions()
	if len(sessions) == 0 {
		printInfo("No sessions to clear.")
		return
	}

	var confirm bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Delete all %d sessions?", len(sessions))).
				Description("This cannot be undone.").
				Affirmative("Delete").
				Negative("Cancel").
				Value(&confirm),
		),
	).WithShowHelp(false).WithKeyMap(huhKeyMap())

	if err := form.Run(); err != nil || !confirm {
		printInfo("Cancelled.")
		return
	}

	n, err := ClearAllSessions()
	if err != nil {
		printError("Clear failed: " + err.Error())
		return
	}
	printSuccess(fmt.Sprintf("Deleted %d session(s).", n))
}

func handleToolsCommand(cfg *Config, mcp *MCPClient) {
	tools := buildTools(cfg, mcp)
	if len(tools) == 0 {
		printInfo("No tools loaded.")
		return
	}

	rows := make([]table.Row, 0, len(tools))
	for _, t := range tools {
		source := "built-in"
		if mcp != nil {
			for _, mt := range mcp.Tools() {
				if mt.Function.Name == t.Function.Name {
					source = "mcp"
					break
				}
			}
		}
		desc := t.Function.Description
		if len(desc) > 80 {
			desc = desc[:77] + "..."
		}
		rows = append(rows, table.Row{t.Function.Name, source, desc})
	}

	cols := []table.Column{
		{Title: "Tool", Width: 28},
		{Title: "Source", Width: 8},
		{Title: "Description", Width: 80},
	}

	tbl := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(min(len(rows)+1, 18)),
	)

	hs := table.DefaultStyles()
	hs.Header = hs.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorBorder).
		BorderBottom(true).
		Bold(true).
		Foreground(colorProvider)
	hs.Selected = hs.Selected.
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(colorProvider).
		Bold(true)
	tbl.SetStyles(hs)

	p := tea.NewProgram(tableModel{table: tbl})
	if _, err := p.Run(); err != nil {
		printError(err.Error())
	}
}

type tableModel struct {
	table table.Model
}

func (m tableModel) Init() tea.Cmd { return nil }
func (m tableModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "q", "esc", "ctrl+c", "enter":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}
func (m tableModel) View() string {
	return "\n" + m.table.View() + "\n  " +
		styleDim.Render("↑/↓ navigate · q close")
}

// pendingAttachments holds @-references queued by /attach. They get prepended
// to the next user message and cleared.
var pendingAttachments []string

// imageExtensions is the set of file types /attach refuses to handle —
// these belong to /image instead, which sends them as proper multimodal
// content rather than reading their bytes as text.
var imageExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true,
	".gif": true, ".webp": true, ".bmp": true,
}

// handleAttachCommand opens the filepicker and queues the chosen file as an
// @-reference to be prepended to the next user message.
func handleAttachCommand() {
	path, err := RunFilePicker("Attach file", nil)
	if err != nil {
		printError("Filepicker failed: " + err.Error())
		return
	}
	if path == "" {
		printInfo("Cancelled.")
		return
	}
	if imageExtensions[strings.ToLower(filepath.Ext(path))] {
		printWarning(fmt.Sprintf("'%s' is an image — use /image %s instead",
			filepath.Base(path), path))
		return
	}
	pendingAttachments = append(pendingAttachments, "@"+path)
	printSuccess(fmt.Sprintf("Attached → %s  (queued: %d)", path, len(pendingAttachments)))
}

// drainAttachments returns any queued @-references as a single space-separated
// prefix string and clears the queue. Returns "" when nothing is queued.
func drainAttachments() string {
	if len(pendingAttachments) == 0 {
		return ""
	}
	out := strings.Join(pendingAttachments, " ") + " "
	pendingAttachments = nil
	return out
}

// handleFreezeCommand exports the last assistant response to a PNG image
// using the `freeze` CLI (https://github.com/charmbracelet/freeze).
func handleFreezeCommand(messages *[]Message) {
	freezeBin := findFreezeBinary()
	if freezeBin == "" {
		printWarning("freeze not installed. Install: go install github.com/charmbracelet/freeze@latest")
		printInfo("Tip: ensure $(go env GOPATH)/bin is on your PATH.")
		return
	}

	var lastAssistant string
	for i := len(*messages) - 1; i >= 0; i-- {
		if (*messages)[i].Role == "assistant" {
			if c, ok := (*messages)[i].Content.(string); ok {
				lastAssistant = c
			}
			break
		}
	}
	if lastAssistant == "" {
		printWarning("No assistant response to freeze.")
		return
	}

	// Prefer the first fenced code block; fall back to the full response.
	content := lastAssistant
	lang := ""
	if start := strings.Index(content, "```"); start >= 0 {
		rest := content[start+3:]
		if nl := strings.Index(rest, "\n"); nl >= 0 {
			lang = strings.TrimSpace(rest[:nl])
			rest = rest[nl+1:]
		}
		if end := strings.Index(rest, "```"); end >= 0 {
			content = rest[:end]
		}
	}

	out := "noor-freeze-" + time.Now().Format("20060102-150405") + ".png"
	args := []string{"--output", out}
	if lang != "" {
		args = append(args, "--language", lang)
	}
	cmd := exec.Command(freezeBin, args...)
	cmd.Stdin = strings.NewReader(content)
	if err := cmd.Run(); err != nil {
		printError("freeze failed: " + err.Error())
		return
	}
	printSuccess("Frozen → " + out)
}

// findFreezeBinary returns the path to the `freeze` binary, checking PATH
// first then $GOPATH/bin and ~/go/bin (so users don't need to put GOPATH on PATH).
func findFreezeBinary() string {
	if p, err := exec.LookPath("freeze"); err == nil {
		return p
	}
	candidates := []string{}
	if gp := os.Getenv("GOPATH"); gp != "" {
		candidates = append(candidates, filepath.Join(gp, "bin", "freeze"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, "go", "bin", "freeze"))
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

func handleImagineCommand(args string, cfg *Config, api *APIClient) {
	if args == "" {
		printWarning("Usage: /imagine <prompt>")
		return
	}
	if !isImageModel(cfg.Model) {
		printWarning("Switch to an image model first with /model (e.g. google/gemini-2.5-flash-image)")
		return
	}
	generateImage([]Message{{Role: "user", Content: args}}, api, cfg)
}

func handleImageCommand(args string, cfg *Config, messages *[]Message, mcp *MCPClient, api *APIClient, sessionCost *float64) {
	if args == "" {
		// Open a filepicker to choose an image.
		path, err := RunFilePicker("Select image", []string{".png", ".jpg", ".jpeg", ".gif", ".webp"})
		if err != nil || path == "" {
			printInfo("Cancelled.")
			return
		}
		args = path
	}
	paramParts := strings.SplitN(args, " ", 2)
	imgPath := paramParts[0]
	prompt := "What do you see in this image?"
	if len(paramParts) > 1 && strings.TrimSpace(paramParts[1]) != "" {
		prompt = paramParts[1]
	}
	msg, err := BuildImageMessage(imgPath, prompt)
	if err != nil {
		printError(err.Error())
		return
	}
	if isImageModel(cfg.Model) {
		generateImage([]Message{msg}, api, cfg)
	} else {
		*messages = append(*messages, msg)
		_, cost := sendMessage(context.Background(), api, cfg, messages, mcp)
		if sessionCost != nil {
			*sessionCost += cost
		}
	}
}

func handleExportCommand(args string, messages *[]Message, cfg *Config) {
	var lastAssistant string
	for i := len(*messages) - 1; i >= 0; i-- {
		if (*messages)[i].Role == "assistant" {
			if c, ok := (*messages)[i].Content.(string); ok {
				lastAssistant = c
			}
			break
		}
	}
	if lastAssistant == "" {
		printWarning("No assistant response to export.")
		return
	}
	filename := AppName + "-response-" + time.Now().Format("20060102-150405") + ".html"
	if args != "" {
		filename = args
	}
	if err := ExportResponse(lastAssistant, cfg.Model, filename); err != nil {
		printError(err.Error())
	} else {
		printSuccess("Exported → " + filename)
	}
}

// handleCopyCommand copies the last assistant response to the clipboard.
func handleCopyCommand(messages *[]Message) {
	var lastAssistant string
	for i := len(*messages) - 1; i >= 0; i-- {
		if (*messages)[i].Role == "assistant" {
			if c, ok := (*messages)[i].Content.(string); ok {
				lastAssistant = c
			}
			break
		}
	}
	if lastAssistant == "" {
		printWarning("No assistant response to copy.")
		return
	}
	if err := CopyToClipboard(lastAssistant); err != nil {
		printError("Copy failed: " + err.Error())
		return
	}
	printSuccess(fmt.Sprintf("Copied to clipboard (%d chars)", len(lastAssistant)))
}

// handleRetryCommand regenerates the last assistant response by re-sending
// the most recent user message with the same context.
func handleRetryCommand(cfg *Config, messages *[]Message, mcp *MCPClient, api *APIClient, sessionCost *float64) {
	lastUserIdx := -1
	for i := len(*messages) - 1; i >= 0; i-- {
		if (*messages)[i].Role == "user" {
			lastUserIdx = i
			break
		}
	}
	if lastUserIdx < 0 {
		printWarning("No user message to retry.")
		return
	}
	// Truncate everything after the last user message (drops assistant + tool messages)
	*messages = (*messages)[:lastUserIdx+1]

	_, cost := sendMessage(context.Background(), api, cfg, messages, mcp)
	if sessionCost != nil {
		*sessionCost += cost
	}
}

// handleEditCommand lets the user edit the last user message in-place
// and regenerates the response.
func handleEditCommand(cfg *Config, messages *[]Message, mcp *MCPClient, api *APIClient, sessionCost *float64) {
	lastUserIdx := -1
	var lastUserText string
	for i := len(*messages) - 1; i >= 0; i-- {
		if (*messages)[i].Role == "user" {
			if t, ok := (*messages)[i].Content.(string); ok {
				lastUserText = t
				lastUserIdx = i
			}
			break
		}
	}
	if lastUserIdx < 0 {
		printWarning("No user message to edit.")
		return
	}

	prompt := fmt.Sprintf("\n%s %s\n",
		styleWarning.Render("EDIT"),
		styleDim.Render("❯"),
	)
	newText, ok := ReadInputWithDefault(prompt, lastUserText)
	if !ok || newText == "" {
		printInfo("Cancelled.")
		return
	}

	// Truncate everything from the last user message onward, replace it
	*messages = (*messages)[:lastUserIdx]
	*messages = append(*messages, Message{Role: "user", Content: sanitize(newText)})

	_, cost := sendMessage(context.Background(), api, cfg, messages, mcp)
	if sessionCost != nil {
		*sessionCost += cost
	}
}

// handleSearchCommand searches all past sessions for the given query and
// presents matches in a picker. Selecting one resumes that session.
func handleSearchCommand(query string, messages *[]Message, cfg *Config, session *Session) {
	if query == "" {
		printWarning("Usage: /search <query>")
		return
	}
	matches, err := SearchSessions(query)
	if err != nil {
		printError("Search failed: " + err.Error())
		return
	}
	if len(matches) == 0 {
		printInfo("No sessions matched: " + query)
		return
	}
	printInfo(fmt.Sprintf("%d session(s) matched", len(matches)))

	chosen := RunHistoryPicker(matches)
	if chosen == nil {
		return
	}
	newMsgs := initMessages(cfg)
	for _, m := range chosen.Messages {
		if m.Role != "system" {
			newMsgs = append(newMsgs, m)
		}
	}
	*messages = newMsgs
	cfg.Model = chosen.Model
	if err := saveModel(cfg.Model); err != nil {
		printWarning("Could not save model preference: " + err.Error())
	}
	if session != nil {
		session.StartTime = chosen.StartTime
		session.Model = chosen.Model
	}
	printSuccess(fmt.Sprintf("Resumed session from %s  ·  %s",
		chosen.StartTime.Format("Jan 02 15:04"), chosen.Model))
}

// handleBudgetCommand shows today's spending and the configured daily limit.
// `args` can be a number to set a new daily limit (saved to config).
func handleBudgetCommand(args string, cfg *Config) {
	// Always show today's spend first
	today := TodayCost()
	if cfg.DailyBudget > 0 {
		pct := today / cfg.DailyBudget * 100
		printInfo(fmt.Sprintf("Today: $%.4f / $%.2f  (%.0f%%)", today, cfg.DailyBudget, pct))
	} else {
		printInfo(fmt.Sprintf("Today: $%.4f  (no limit set)", today))
	}

	// If no arg, prompt for a new limit via huh.
	if args == "" {
		var input string
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewInput().
					Title("Daily budget (USD)").
					Description("blank or 0 to disable").
					Placeholder(fmt.Sprintf("%.2f", cfg.DailyBudget)).
					Value(&input).
					Validate(func(s string) error {
						if s == "" {
							return nil
						}
						v, err := strconv.ParseFloat(s, 64)
						if err != nil || v < 0 {
							return fmt.Errorf("must be a non-negative number")
						}
						return nil
					}),
			),
		).WithShowHelp(false).WithKeyMap(huhKeyMap())
		if err := form.Run(); err != nil {
			return
		}
		args = strings.TrimSpace(input)
		if args == "" {
			return
		}
	}

	v, err := strconv.ParseFloat(args, 64)
	if err != nil || v < 0 {
		printWarning("Usage: /budget [amount]  (e.g. /budget 2.00)")
		return
	}
	cfg.DailyBudget = v
	if err := saveSetting("DAILY_BUDGET", fmt.Sprintf("%.2f", v)); err != nil {
		printWarning("Could not save budget: " + err.Error())
	} else {
		printSuccess(fmt.Sprintf("Daily budget set to $%.2f", v))
	}
}

// handleCompressCommand summarizes the conversation so far and replaces
// the context with the summary, freeing up tokens.
func handleCompressCommand(cfg *Config, messages *[]Message, api *APIClient) {
	if len(*messages) < 4 {
		printInfo("Not enough conversation to compress.")
		return
	}

	spin := NewSpinner("compressing context...")
	spin.Start()

	// Build a summarization request from the existing messages
	summarizationPrompt := "Summarize the conversation so far in a compact form that " +
		"preserves: code samples, decisions made, key facts, and any unresolved questions. " +
		"Use bullet points. Be terse but complete."

	summarizeMsgs := append([]Message{}, *messages...)
	summarizeMsgs = append(summarizeMsgs, Message{Role: "user", Content: summarizationPrompt})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := api.Request(ctx, summarizeMsgs, cfg, nil)
	spin.Stop()

	if err != nil {
		printError("Compression failed: " + err.Error())
		return
	}

	beforeTokens := approxTokens(*messages)

	// Replace messages with: system + summary + (no further history)
	newMsgs := initMessages(cfg)
	newMsgs = append(newMsgs, Message{
		Role:    "assistant",
		Content: "[Conversation summary so far]\n\n" + result.Content,
	})
	*messages = newMsgs

	afterTokens := approxTokens(*messages)
	saved := beforeTokens - afterTokens
	printSuccess(fmt.Sprintf("Context compressed: ~%dK → ~%dK tokens (%dK saved)",
		beforeTokens/1000, afterTokens/1000, saved/1000))
}

// generateImage sends a prompt/image to an image-capable model and saves
// any returned images to disk.
func generateImage(msgs []Message, api *APIClient, cfg *Config) {
	spin := NewSpinner("generating image...")
	spin.Start()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	result, err := api.Request(ctx, msgs, cfg, nil)
	spin.Stop()

	if err != nil {
		printError(err.Error())
		return
	}

	if len(result.Images) == 0 {
		if result.Content != "" {
			fmt.Print(renderMarkdown(result.Content))
		} else {
			printError("No image returned. Make sure you selected an image model.")
		}
		return
	}

	for _, img := range result.Images {
		path, err := SaveGeneratedImage(img)
		if err != nil {
			printError("Failed to save image: " + err.Error())
			continue
		}
		printSuccess("Image saved → " + path)
	}
}

// printHelp shows the in-chat help message.
func printHelp(cfg *Config, mcp *MCPClient) {
	sep()
	cmds := [][2]string{
		{"/help", "Show this"},
		{"/clear", "Clear screen and reset conversation"},
		{"/reset", "Reset conversation context"},
		{"/model [name]", "Pick or switch model"},
		{"/style <name>", "Switch style (markdown/plain/concise/raw)"},
		{"/theme [name]", "Switch color theme"},
		{"/system <prompt>", "Set system prompt"},
		{"/history", "Browse and resume a past session"},
		{"/history clear", "Delete all saved sessions"},
		{"/search <query>", "Search past sessions and resume one"},
		{"/tools", "List loaded tools"},
		{"/mcp [cmd|stop]", "Start, stop, or show status of MCP server"},
		{"/image <path> [prompt]", "Analyze image or edit it (image models)"},
		{"/imagine <prompt>", "Generate an image (image models)"},
		{"/export [file]", "Export — default .html, or .py/.js/.go to extract code"},
		{"/copy", "Copy last response to clipboard"},
		{"/retry", "Regenerate last response"},
		{"/edit", "Edit last message and regenerate"},
		{"/budget [amount]", "Show or set daily cost limit"},
		{"/compress", "Summarize history to free up context"},
		{"/freeze", "Export last code block to PNG (requires `freeze`)"},
		{"/attach", "Pick a file to include with your next message"},
		{"weather <city>", "Live weather"},
		{"@file/url/git", "Include file, URL, or git context (@diff/status/log)"},
		{"exit", "Quit"},
	}
	for _, c := range cmds {
		fmt.Printf("  %-28s %s\n", styleCmd.Render(c[0]), styleDim.Render(c[1]))
	}
	sep()
	fmt.Printf("  %s %s  ·  %s %s\n",
		styleDim.Render("model:"), styleProvider.Render(cfg.Model),
		styleDim.Render("style:"), styleTemp.Render(cfg.Style),
	)
	sep()
	fmt.Println()
}
