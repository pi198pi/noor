package main

import (
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type filePickerModel struct {
	picker   filepicker.Model
	title    string
	selected string
	quit     bool
	width    int
	height   int
}

func (m filePickerModel) Init() tea.Cmd { return m.picker.Init() }

func (m filePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.picker.SetHeight(msg.Height - 4)
	case tea.KeyMsg:
		if k := msg.String(); k == "ctrl+c" || k == "esc" || k == "q" {
			m.quit = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.picker, cmd = m.picker.Update(msg)
	if didSelect, path := m.picker.DidSelectFile(msg); didSelect {
		m.selected = path
		return m, tea.Quit
	}
	return m, cmd
}

func (m filePickerModel) View() string {
	header := lipgloss.NewStyle().Bold(true).Foreground(colorProvider).Render("  " + m.title)
	footer := styleDim.Render("  ↑/↓ navigate · enter open/select · esc cancel")
	return strings.Join([]string{header, "", m.picker.View(), footer}, "\n")
}

// RunFilePicker shows a filepicker rooted at the current directory and returns
// the absolute path of the selected file. allowedExts is a list of extensions
// (lowercased, with leading dot) — empty means any file is allowed.
func RunFilePicker(title string, allowedExts []string) (string, error) {
	fp := filepicker.New()
	fp.AllowedTypes = allowedExts
	fp.ShowHidden = false
	fp.AutoHeight = false
	fp.Height = 18

	p := tea.NewProgram(
		filePickerModel{picker: fp, title: title},
		tea.WithAltScreen(),
	)
	m, err := p.Run()
	if err != nil {
		return "", err
	}
	res := m.(filePickerModel)
	if res.quit {
		return "", nil
	}
	return res.selected, nil
}
