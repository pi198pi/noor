package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

type inputModel struct {
	ta        textarea.Model
	value     string
	submitted bool
	cancelled bool
	width     int
	hint      string // shows under the input on tab when there are multiple matches
}

// slashCommands is the canonical list used for tab completion.
var slashCommands = []string{
	"help", "clear", "reset",
	"model", "style", "theme", "system",
	"history", "search",
	"tools", "mcp", "image", "imagine",
	"export", "copy", "retry", "edit",
	"budget", "compress",
	"freeze", "attach",
}

// pathTakingCommands lists slash commands whose first argument is a file path.
// Tab on `/<cmd> <prefix>` completes <prefix> against the filesystem.
var pathTakingCommands = map[string]bool{
	"image":  true,
	"export": true,
}

func newInputModel() inputModel {
	ta := textarea.New()
	ta.Placeholder = "Type a message..."
	ta.Focus()
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.Prompt = ""
	ta.SetHeight(3)

	w, _, _ := term.GetSize(int(os.Stdout.Fd()))
	if w <= 8 {
		w = 80
	}
	ta.SetWidth(w - 4)

	ta.KeyMap.InsertNewline = key.NewBinding(
		key.WithKeys("ctrl+n", "alt+enter"),
		key.WithHelp("ctrl+n", "new line"),
	)

	// Use terminal-native styling — no extra lipgloss borders
	noStyle := lipgloss.NewStyle()
	ta.FocusedStyle.Base = noStyle
	ta.FocusedStyle.CursorLine = noStyle
	ta.BlurredStyle.Base = noStyle
	ta.BlurredStyle.CursorLine = noStyle

	// Style the cursor and text
	ta.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#00C4CC"))
	ta.FocusedStyle.Prompt = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	ta.FocusedStyle.Text = lipgloss.NewStyle().Foreground(lipgloss.Color("#F3F4F6"))

	return inputModel{ta: ta, width: w - 4}
}

func (m inputModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m inputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width - 4
		if m.width < 40 {
			m.width = 40
		}
		m.ta.SetWidth(m.width)
		return m, nil
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEnter:
			val := strings.TrimSpace(m.ta.Value())
			if val != "" {
				m.value = val
				m.submitted = true
				return m, tea.Quit
			}
			return m, nil
		case tea.KeyCtrlC, tea.KeyEsc:
			m.cancelled = true
			return m, tea.Quit
		case tea.KeyTab:
			completed, hint := completeInput(m.ta.Value())
			if completed != "" {
				m.ta.SetValue(completed)
				m.ta.CursorEnd()
			}
			m.hint = hint
			return m, nil
		}
		// Any non-tab keypress clears the completion hint
		m.hint = ""
	}

	var cmd tea.Cmd
	m.ta, cmd = m.ta.Update(msg)

	// Auto-grow height when content wraps, capped at 10 lines
	info := m.ta.LineInfo()
	need := (m.ta.LineCount() - 1) + info.Height
	if need < 3 {
		need = 3
	}
	if need > m.ta.Height() {
		m.ta.SetHeight(min(need, 10))
	}

	return m, cmd
}

func (m inputModel) View() string {
	// Compact hint bar — left/cancel, right/send
	hintLeft := styleDim.Render("Enter · send  ·  Tab · complete")
	hintRight := styleDim.Render("Ctrl+N · newline  Esc · cancel")
	// Pad to fill width
	padding := m.width - lipgloss.Width(hintLeft) - lipgloss.Width(hintRight)
	if padding < 2 {
		padding = 2
	}
	hintLine := hintLeft + strings.Repeat(" ", padding) + hintRight

	view := m.ta.View() + "\n" + hintLine
	if m.hint != "" {
		view += "\n" + styleDim.Render("  "+m.hint)
	}
	return view
}

// ReadInput shows an interactive textarea and returns the submitted text.
// Returns ("", false) if cancelled or on error.
func ReadInput(prompt string) (string, bool) {
	return ReadInputWithDefault(prompt, "")
}

// completeInput returns (newText, hintText) given the current textarea value.
// Supports completing slash commands and @-references.
func completeInput(text string) (string, string) {
	// Find what we're completing — check for an unfinished /command or @ref
	// at the very end of the text (i.e. cursor position).

	// Slash command at start of input
	if strings.HasPrefix(text, "/") && !strings.Contains(text, " ") {
		prefix := text[1:]
		var matches []string
		for _, cmd := range slashCommands {
			if strings.HasPrefix(cmd, prefix) {
				matches = append(matches, cmd)
			}
		}
		if len(matches) == 0 {
			return "", "no matching command"
		}
		if len(matches) == 1 {
			return "/" + matches[0] + " ", ""
		}
		// Multiple matches: extend to longest common prefix
		common := longestCommonPrefix(matches)
		if common != prefix {
			return "/" + common, "matches: " + strings.Join(matches, "  ")
		}
		return "", "matches: " + strings.Join(matches, "  ")
	}

	// /command <path-prefix> — complete the first argument as a file path
	// for slash commands listed in pathTakingCommands.
	if strings.HasPrefix(text, "/") {
		if space := strings.Index(text, " "); space > 0 {
			cmd := text[1:space]
			rest := text[space+1:]
			// Only complete the first argument — once a second word starts,
			// the user is past the path and into the prompt/options.
			if pathTakingCommands[cmd] && !strings.ContainsAny(rest, " \t\n") {
				completions := completeFileRef(rest)
				if len(completions) == 0 {
					return "", "no matching files"
				}
				if len(completions) == 1 {
					return text[:space+1] + completions[0], ""
				}
				common := longestCommonPrefix(completions)
				if common != rest {
					return text[:space+1] + common, "matches: " + strings.Join(truncList(completions, 5), "  ")
				}
				return "", "matches: " + strings.Join(truncList(completions, 5), "  ")
			}
		}
	}

	// @-reference: find last @ in text and complete file paths after it
	if idx := strings.LastIndex(text, "@"); idx >= 0 {
		// Only complete if there's no space after the @
		after := text[idx+1:]
		if !strings.ContainsAny(after, " \t\n") {
			completions := completeFileRef(after)
			if len(completions) == 0 {
				return "", "no matching files"
			}
			if len(completions) == 1 {
				return text[:idx+1] + completions[0], ""
			}
			common := longestCommonPrefix(completions)
			if common != after {
				return text[:idx+1] + common, "matches: " + strings.Join(truncList(completions, 5), "  ")
			}
			return "", "matches: " + strings.Join(truncList(completions, 5), "  ")
		}
	}

	return "", ""
}

// completeFileRef returns possible file/dir completions for the given prefix.
func completeFileRef(prefix string) []string {
	dir := "."
	base := prefix
	if i := strings.LastIndex(prefix, "/"); i >= 0 {
		dir = prefix[:i+1]
		base = prefix[i+1:]
	}

	expandedDir := dir
	if strings.HasPrefix(dir, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			expandedDir = home + dir[1:]
		}
	}
	entries, err := os.ReadDir(expandedDir)
	if err != nil {
		return nil
	}
	var matches []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(base, ".") {
			continue // skip dotfiles unless explicitly requested
		}
		if !strings.HasPrefix(name, base) {
			continue
		}
		full := dir + name
		if e.IsDir() {
			full += "/"
		}
		matches = append(matches, full)
	}
	return matches
}

func longestCommonPrefix(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	prefix := strs[0]
	for _, s := range strs[1:] {
		for !strings.HasPrefix(s, prefix) {
			if len(prefix) == 0 {
				return ""
			}
			prefix = prefix[:len(prefix)-1]
		}
	}
	return prefix
}

func truncList(items []string, max int) []string {
	if len(items) <= max {
		return items
	}
	out := append([]string{}, items[:max]...)
	return append(out, fmt.Sprintf("(+%d more)", len(items)-max))
}

// ReadInputWithDefault is like ReadInput but pre-fills the textarea with
// the given default text. Useful for the /edit command.
func ReadInputWithDefault(prompt, defaultText string) (string, bool) {
	fmt.Print(prompt)
	model := newInputModel()
	if defaultText != "" {
		model.ta.SetValue(defaultText)
	}
	p := tea.NewProgram(model, tea.WithOutput(os.Stdout))
	m, err := p.Run()
	if err != nil {
		return "", false
	}
	result := m.(inputModel)
	if result.cancelled || !result.submitted {
		return "", false
	}
	return result.value, true
}
