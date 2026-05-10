package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
)

// huhKeyMap returns a KeyMap that also lets esc cancel the form (huh's
// default only binds ctrl+c).
func huhKeyMap() *huh.KeyMap {
	km := huh.NewDefaultKeyMap()
	km.Quit = key.NewBinding(key.WithKeys("ctrl+c", "esc"))
	return km
}

// ─── Model Picker (huh-based, searchable) ────────────────────────────────────

// RunPicker shows a searchable picker. Returns selected option or "" if cancelled.
// Headers (lines starting with "──") are rendered as group separators.
func RunPicker(models []string, current string) string {
	opts := make([]huh.Option[string], 0, len(models))
	currentGroup := ""
	for _, m := range models {
		if isHeader(m) {
			currentGroup = strings.Trim(m, " ─")
			continue
		}
		label := m
		if w, ok := modelCtxWindow[m]; ok {
			label = fmt.Sprintf("%-44s  %s", m, formatCtx(w))
		}
		if currentGroup != "" {
			label = fmt.Sprintf("[%s] %s", currentGroup, label)
		}
		opts = append(opts, huh.NewOption(label, m))
	}

	var sel string = current
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Select model").
				Description("type to filter · ↑/↓ navigate · enter select · esc cancel").
				Options(opts...).
				Filtering(true).
				Height(15).
				Value(&sel),
		),
	).WithShowHelp(false).WithShowErrors(false).WithKeyMap(huhKeyMap())

	if err := form.Run(); err != nil {
		return ""
	}
	if sel == current {
		return ""
	}
	return sel
}

func isHeader(s string) bool {
	return strings.HasPrefix(s, "──")
}

// ─── History Picker (split-pane: list + viewport preview) ───────────────────

type historyPickerModel struct {
	sessions []Session
	cursor   int
	preview  viewport.Model
	width    int
	height   int
	selected int
	quit     bool
	ready    bool
}

func (m historyPickerModel) Init() tea.Cmd { return nil }

func (m historyPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		previewW := msg.Width - 50
		if previewW < 30 {
			previewW = 30
		}
		previewH := msg.Height - 5
		if previewH < 5 {
			previewH = 5
		}
		if !m.ready {
			m.preview = viewport.New(previewW, previewH)
			m.ready = true
		} else {
			m.preview.Width = previewW
			m.preview.Height = previewH
		}
		m.preview.SetContent(sessionFullPreview(m.sessions[m.cursor]))

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			m.quit = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				if m.ready {
					m.preview.SetContent(sessionFullPreview(m.sessions[m.cursor]))
					m.preview.GotoTop()
				}
			}
		case "down", "j":
			if m.cursor < len(m.sessions)-1 {
				m.cursor++
				if m.ready {
					m.preview.SetContent(sessionFullPreview(m.sessions[m.cursor]))
					m.preview.GotoTop()
				}
			}
		case "pgup":
			m.preview.HalfPageUp()
		case "pgdown":
			m.preview.HalfPageDown()
		case "enter", " ":
			m.selected = m.cursor
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m historyPickerModel) View() string {
	if !m.ready {
		return "loading…"
	}
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(colorProvider)
	header := headerStyle.Render("  Resume session")

	var listSb strings.Builder
	maxList := m.height - 5
	if maxList < 5 {
		maxList = 5
	}
	start := 0
	if m.cursor >= maxList {
		start = m.cursor - maxList + 1
	}
	end := start + maxList
	if end > len(m.sessions) {
		end = len(m.sessions)
	}

	for i := start; i < end; i++ {
		s := m.sessions[i]
		date := s.StartTime.Format("Jan 02 15:04")
		model := s.Model
		if idx := strings.LastIndex(model, "/"); idx >= 0 {
			model = model[idx+1:]
		}
		line := fmt.Sprintf("%-12s  %-22s", date, model)
		if i == m.cursor {
			listSb.WriteString(styleProvider.Render("▶ ") + lipgloss.NewStyle().Bold(true).Foreground(colorProvider).Render(line) + "\n")
		} else {
			listSb.WriteString("  " + styleDim.Render(line) + "\n")
		}
	}

	listW := 42
	listPane := lipgloss.NewStyle().Width(listW).Render(listSb.String())
	previewPane := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(0, 1).
		Render(m.preview.View())

	body := lipgloss.JoinHorizontal(lipgloss.Top, listPane, previewPane)
	footer := styleDim.Render("  ↑/↓ list · pgup/pgdn scroll preview · enter resume · q cancel")

	return lipgloss.JoinVertical(lipgloss.Left, header, "", body, "", footer)
}

func RunHistoryPicker(sessions []Session) *Session {
	if len(sessions) == 0 {
		return nil
	}
	p := tea.NewProgram(
		historyPickerModel{sessions: sessions, selected: -1},
		tea.WithAltScreen(),
	)
	m, err := p.Run()
	if err != nil {
		return nil
	}
	result := m.(historyPickerModel)
	if result.quit || result.selected < 0 {
		return nil
	}
	return &result.sessions[result.selected]
}

// sessionFullPreview returns a multi-line preview of the messages in a session,
// suitable for the viewport scroll-pane.
func sessionFullPreview(s Session) string {
	var sb strings.Builder
	sb.WriteString(styleDim.Render(fmt.Sprintf("%s · %s\n\n", s.StartTime.Format("Mon Jan 02 15:04"), s.Model)))
	for _, msg := range s.Messages {
		if msg.Role == "system" {
			continue
		}
		text, ok := msg.Content.(string)
		if !ok {
			continue
		}
		role := strings.ToUpper(msg.Role)
		switch msg.Role {
		case "user":
			sb.WriteString(styleProvider.Render(role) + "\n")
		case "assistant":
			sb.WriteString(styleSuccess.Render(role) + "\n")
		default:
			sb.WriteString(styleDim.Render(role) + "\n")
		}
		sb.WriteString(text + "\n\n")
	}
	return sb.String()
}
