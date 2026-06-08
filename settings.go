package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var settingsLabels = []string{
	"theme",
	"header mode",
	"header text",
	"weather",
	"weather city",
	"idle screensaver",
	"idle timeout",
	"auto-rotate scenes",
	"clock format",
}

const settingsCount = 9

// settingsModule ist die zentrale Konfigurationsseite. Änderungen werden sofort
// persistiert und per reloadConfigMsg an Root + Module verteilt.
type settingsModule struct {
	width, height int
	cursor        int
	st            persistedState

	editing   bool
	editIdx   int
	editInput textinput.Model
}

func newSettingsModule() *settingsModule {
	ei := textinput.New()
	ei.Prompt = "› "
	ei.CharLimit = 80
	ei.Width = 36
	m := &settingsModule{st: loadState(), editInput: ei}
	m.restyle()
	return m
}

func (m *settingsModule) Name() string   { return "settings" }
func (m *settingsModule) Status() string { return "" }
func (m *settingsModule) Init() tea.Cmd  { return nil }

func (m *settingsModule) restyle() {
	m.editInput.PromptStyle = lipgloss.NewStyle().Foreground(colTeal)
	m.editInput.TextStyle = lipgloss.NewStyle().Foreground(colCream)
	m.editInput.Cursor.Style = lipgloss.NewStyle().Foreground(colPeach)
}

func (m *settingsModule) Update(msg tea.Msg) (Module, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case focusMsg:
		m.st = loadState()

	case reloadConfigMsg:
		m.st = loadState()

	case themeChangedMsg:
		m.restyle()

	case tea.KeyMsg:
		k := msg.String()
		if m.editing {
			switch k {
			case "enter":
				m.applyText(m.editIdx, strings.TrimSpace(m.editInput.Value()))
				m.editing = false
				return m, m.save()
			case "esc":
				m.editing = false
				return m, nil
			}
			var cmd tea.Cmd
			m.editInput, cmd = m.editInput.Update(msg)
			return m, cmd
		}
		switch k {
		case "esc", "q":
			return m, goToLauncher
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < settingsCount-1 {
				m.cursor++
			}
		case "left":
			return m, m.change(-1)
		case "right":
			return m, m.change(1)
		case "enter", " ":
			return m, m.enter()
		}
	}
	return m, nil
}

// enter: Textfelder öffnen den Editor, alles andere schaltet weiter.
func (m *settingsModule) enter() tea.Cmd {
	switch m.cursor {
	case 2: // header text
		m.editIdx = 2
		m.editInput.SetValue(m.st.Header.Text)
		m.openEditor()
		return textinput.Blink
	case 4: // weather city
		m.editIdx = 4
		m.editInput.SetValue(m.st.Weather.City)
		m.openEditor()
		return textinput.Blink
	default:
		return m.change(1)
	}
}

func (m *settingsModule) openEditor() {
	m.editing = true
	m.editInput.CursorEnd()
	m.editInput.Focus()
}

// change mutiert die Einstellung am Cursor und persistiert.
func (m *settingsModule) change(dir int) tea.Cmd {
	switch m.cursor {
	case 0: // theme
		m.st.Theme = cycleList(themeNames(), m.st.Theme, dir)
		applyTheme(themeByName(m.st.Theme))
	case 1: // header mode
		m.st.Header.Mode = cycleList(headerModes, m.st.Header.Mode, dir)
	case 3: // weather mode
		m.st.Weather.Mode = cycleList([]string{"auto", "manual", "off"}, m.st.Weather.Mode, dir)
	case 5: // idle on/off
		m.st.Ambient.IdleOff = !m.st.Ambient.IdleOff
	case 6: // idle timeout
		s := m.st.Ambient.IdleSecs
		if s <= 0 {
			s = 120
		}
		s += dir * 30
		if s < 30 {
			s = 30
		}
		if s > 1800 {
			s = 1800
		}
		m.st.Ambient.IdleSecs = s
	case 7: // auto-rotate
		m.st.Ambient.Rotate = !m.st.Ambient.Rotate
	case 8: // clock format
		m.st.Ambient.Clock12 = !m.st.Ambient.Clock12
	default:
		return nil
	}
	return m.save()
}

func (m *settingsModule) applyText(idx int, val string) {
	switch idx {
	case 2:
		m.st.Header.Text = val
	case 4:
		m.st.Weather.City = val
		if val != "" {
			m.st.Weather.Mode = "manual"
		} else if m.st.Weather.Mode == "manual" {
			m.st.Weather.Mode = "auto"
		}
	}
}

// save persistiert NUR die Config-Felder (Favoriten etc. bleiben) und stößt das
// Neuladen in Root + Modulen an.
func (m *settingsModule) save() tea.Cmd {
	st := m.st
	return tea.Batch(
		func() tea.Msg {
			_ = updateState(func(s *persistedState) {
				s.Theme = st.Theme
				s.Header = st.Header
				s.Weather = st.Weather
				s.Ambient = st.Ambient
			})
			return nil
		},
		reloadConfig,
	)
}

func (m *settingsModule) value(i int) string {
	a := m.st.Ambient
	switch i {
	case 0:
		return m.st.Theme
	case 1:
		return m.st.Header.Mode
	case 2:
		if m.st.Header.Text == "" {
			return "(empty)"
		}
		return m.st.Header.Text
	case 3:
		return m.st.Weather.Mode
	case 4:
		if m.st.Weather.City == "" {
			return "(auto by IP)"
		}
		return m.st.Weather.City
	case 5:
		if a.IdleOff {
			return "off"
		}
		return "on"
	case 6:
		if a.IdleOff {
			return "—"
		}
		s := a.IdleSecs
		if s <= 0 {
			s = 120
		}
		return fmt.Sprintf("%ds", s)
	case 7:
		if a.Rotate {
			return "on"
		}
		return "off"
	case 8:
		if a.Clock12 {
			return "12-hour"
		}
		return "24-hour"
	}
	return ""
}

func (m *settingsModule) View(width, height int) string {
	m.width, m.height = width, height

	if m.editing {
		title := labelStyle.Render("edit " + settingsLabels[m.editIdx])
		input := lipgloss.NewStyle().Width(38).Render(m.editInput.View())
		hint := helpStyle.Render("enter: save   ·   esc: cancel")
		card := cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, title, "", input))
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
			lipgloss.JoinVertical(lipgloss.Center, card, "", hint))
	}

	valStyle := lipgloss.NewStyle().Foreground(colTeal)
	rows := []string{labelStyle.Render("settings"), ""}
	for i := 0; i < settingsCount; i++ {
		cursor := "  "
		ls := lipgloss.NewStyle().Foreground(colCream)
		if i == m.cursor {
			cursor = lipgloss.NewStyle().Foreground(colPeach).Render("▸ ")
			ls = lipgloss.NewStyle().Foreground(colPeach).Bold(true)
		}
		label := ls.Width(20).Render(settingsLabels[i])
		rows = append(rows, cursor+label+valStyle.Render(m.value(i)))
	}

	card := cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
	help := helpStyle.Render("↑/↓ select   ·   ←/→ change   ·   enter edit   ·   esc back")
	body := lipgloss.JoinVertical(lipgloss.Center, card, "", help)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, body)
}

// cycleList liefert das nächste/vorherige Element (zyklisch).
func cycleList(list []string, cur string, dir int) string {
	idx := 0
	for i, v := range list {
		if v == cur {
			idx = i
			break
		}
	}
	n := len(list)
	idx = (idx + dir + n) % n
	return list[idx]
}
