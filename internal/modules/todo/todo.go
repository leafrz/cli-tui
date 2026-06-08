package todo

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/leafrz/dashboard/internal/config"
	"github.com/leafrz/dashboard/internal/core"
	"github.com/leafrz/dashboard/internal/ui"
)

type todoModule struct {
	width, height int
	items         []config.TodoItem
	cursor        int

	inputActive bool // Eingabefeld offen (neu oder bearbeiten)
	editIdx     int  // -1 = neuer Task, sonst Index des bearbeiteten Tasks
	input       textinput.Model
}

func New() *todoModule {
	in := textinput.New()
	in.Prompt = "› "
	in.Placeholder = "task…"
	in.CharLimit = 120
	in.Width = 40

	m := &todoModule{items: config.Load().Todos, editIdx: -1, input: in}
	m.restyle()
	return m
}

func (m *todoModule) Name() string  { return "todo" }
func (m *todoModule) Init() tea.Cmd { return nil }

func (m *todoModule) Status() string {
	if len(m.items) == 0 {
		return ""
	}
	done := 0
	for _, it := range m.items {
		if it.Done {
			done++
		}
	}
	return fmt.Sprintf("todo %d/%d", done, len(m.items))
}

func (m *todoModule) restyle() {
	m.input.PromptStyle = lipgloss.NewStyle().Foreground(ui.ColTeal)
	m.input.TextStyle = lipgloss.NewStyle().Foreground(ui.ColCream)
	m.input.PlaceholderStyle = lipgloss.NewStyle().Foreground(ui.ColFaint)
	m.input.Cursor.Style = lipgloss.NewStyle().Foreground(ui.ColPeach)
}

func (m *todoModule) save() tea.Cmd {
	items := append([]config.TodoItem(nil), m.items...)
	return func() tea.Msg {
		_ = config.Update(func(s *config.State) { s.Todos = items })
		return nil
	}
}

// startInput öffnet das Eingabefeld: idx == -1 für neuen Task, sonst bearbeiten.
func (m *todoModule) startInput(idx int) {
	m.inputActive = true
	m.editIdx = idx
	if idx >= 0 {
		m.input.SetValue(m.items[idx].Text)
	} else {
		m.input.SetValue("")
	}
	m.input.CursorEnd()
	m.input.Focus()
}

func (m *todoModule) closeInput() {
	m.inputActive = false
	m.editIdx = -1
	m.input.SetValue("")
	m.input.Blur()
}

func (m *todoModule) Update(msg tea.Msg) (core.Module, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case core.FocusMsg:
		m.items = config.Load().Todos // könnte extern geändert sein

	case core.ThemeChangedMsg:
		m.restyle()

	case tea.KeyMsg:
		if m.inputActive {
			switch msg.String() {
			case "enter":
				val := strings.TrimSpace(m.input.Value())
				if val == "" {
					m.closeInput()
					return m, nil
				}
				if m.editIdx >= 0 {
					m.items[m.editIdx].Text = val
					m.closeInput()
					return m, m.save()
				}
				// neuer Task -> anhängen, im Add-Modus bleiben für schnelles Erfassen
				m.items = append(m.items, config.TodoItem{Text: val})
				m.cursor = len(m.items) - 1
				m.input.SetValue("")
				return m, m.save()
			case "esc":
				m.closeInput()
				return m, nil
			}
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}

		switch msg.String() {
		case "esc", "q":
			return m, core.GoToLauncher
		case "a":
			m.startInput(-1)
			return m, textinput.Blink
		case "e":
			if m.cursor < len(m.items) {
				m.startInput(m.cursor)
				return m, textinput.Blink
			}
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case " ", "enter":
			if m.cursor < len(m.items) {
				m.items[m.cursor].Done = !m.items[m.cursor].Done
				return m, m.save()
			}
		case "d":
			if m.cursor < len(m.items) {
				m.items = append(m.items[:m.cursor], m.items[m.cursor+1:]...)
				if m.cursor >= len(m.items) && m.cursor > 0 {
					m.cursor--
				}
				return m, m.save()
			}
		case "c":
			kept := m.items[:0]
			for _, it := range m.items {
				if !it.Done {
					kept = append(kept, it)
				}
			}
			m.items = kept
			if m.cursor >= len(m.items) && m.cursor > 0 {
				m.cursor = len(m.items) - 1
			}
			return m, m.save()
		}
	}
	return m, nil
}

func (m *todoModule) View(width, height int) string {
	m.width, m.height = width, height

	openText := lipgloss.NewStyle().Foreground(ui.ColCream)
	doneText := lipgloss.NewStyle().Foreground(ui.ColDim).Strikethrough(true)
	openBox := lipgloss.NewStyle().Foreground(ui.ColPurple)
	doneBox := lipgloss.NewStyle().Foreground(ui.ColGood)
	cursorStyle := lipgloss.NewStyle().Foreground(ui.ColPeach)

	rows := []string{ui.LabelStyle.Render("todo"), ""}
	if len(m.items) == 0 {
		rows = append(rows, ui.DimStyle.Render("no tasks yet — press 'a' to add"))
	}
	for i, it := range m.items {
		cursor := "  "
		if i == m.cursor && !m.inputActive {
			cursor = cursorStyle.Render("▸ ")
		}
		box, ts := openBox.Render("[ ]"), openText
		if it.Done {
			box, ts = doneBox.Render("[x]"), doneText
		}
		rows = append(rows, cursor+box+" "+ts.Render(it.Text))
	}
	if m.inputActive {
		label := "add: "
		if m.editIdx >= 0 {
			label = "edit: "
		}
		rows = append(rows, "", ui.LabelStyle.Render(label)+m.input.View())
	}

	card := ui.CardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
	help := ui.HelpStyle.Render("a add · e edit · space/enter toggle · d delete · c clear done · esc back")
	body := lipgloss.JoinVertical(lipgloss.Center, card, "", help)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, body)
}
