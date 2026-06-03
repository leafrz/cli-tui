package main

import (
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// headerTickMsg treibt Header-Animationen (rotate/marquee/context) und die Uhr.
type headerTickMsg time.Time

func headerTickCmd(animated bool) tea.Cmd {
	d := time.Second
	if animated {
		d = 200 * time.Millisecond
	}
	return tea.Tick(d, func(t time.Time) tea.Msg { return headerTickMsg(t) })
}

// rootModel ist das Dashboard: globaler Header + Routing zum aktiven Modul.
type rootModel struct {
	modules []Module
	active  int

	width  int
	height int

	header      headerConfig
	headerFrame int

	// In-App Header-Editor
	editing   bool
	editInput textinput.Model
}

func newRoot() *rootModel {
	st := loadState()

	ei := textinput.New()
	ei.Prompt = "› "
	ei.PromptStyle = lipgloss.NewStyle().Foreground(colTeal)
	ei.TextStyle = lipgloss.NewStyle().Foreground(colCream)
	ei.CharLimit = 80
	ei.Width = 40

	return &rootModel{
		modules:   []Module{newRadioModule()},
		header:    st.Header.withDefaults(),
		editInput: ei,
	}
}

func (r *rootModel) activeModule() Module { return r.modules[r.active] }

func (r *rootModel) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(r.modules)+1)
	for _, m := range r.modules {
		cmds = append(cmds, m.Init())
	}
	cmds = append(cmds, headerTickCmd(r.header.animated()))
	return tea.Batch(cmds...)
}

func (r *rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.width = msg.Width
		r.height = msg.Height
		// an das aktive Modul weiterreichen (für Listen-Sizing etc.)
		mod, cmd := r.activeModule().Update(msg)
		r.modules[r.active] = mod
		return r, cmd

	case headerTickMsg:
		r.headerFrame++
		return r, headerTickCmd(r.header.animated())

	case tea.KeyMsg:
		key := msg.String()

		// Header-Editor hat Vorrang, solange offen.
		if r.editing {
			switch key {
			case "enter":
				r.header.Text = r.editInput.Value()
				r.header.Mode = headerStatic
				r.editing = false
				return r, r.saveHeaderCmd()
			case "esc":
				r.editing = false
				return r, nil
			}
			var cmd tea.Cmd
			r.editInput, cmd = r.editInput.Update(msg)
			return r, cmd
		}

		// Globale Dashboard-Tasten.
		switch key {
		case "ctrl+t":
			// Der laufende Header-Tick passt seine Rate beim nächsten Tick
			// automatisch an den neuen Modus an (kein zweiter Tick nötig).
			r.header = r.header.next()
			return r, r.saveHeaderCmd()
		case "ctrl+e":
			r.editing = true
			r.editInput.SetValue(r.header.Text)
			r.editInput.CursorEnd()
			r.editInput.Focus()
			return r, textinput.Blink
		}
	}

	// Alles andere ans aktive Modul.
	mod, cmd := r.activeModule().Update(msg)
	r.modules[r.active] = mod
	return r, cmd
}

func (r *rootModel) View() string {
	header := r.headerView()
	headerH := lipgloss.Height(header)

	contentH := r.height - headerH
	if contentH < 1 {
		contentH = 1
	}

	var content string
	if r.editing {
		content = lipgloss.Place(r.width, contentH, lipgloss.Center, lipgloss.Center,
			r.headerEditorView())
	} else {
		content = r.activeModule().View(r.width, contentH)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, content)
}

// headerView rendert den globalen Header (Titel/Animation links, Uhr rechts).
func (r *rootModel) headerView() string {
	status := r.activeModule().Status()
	left := headerTextStyle.Render(
		headerText(r.header, r.headerFrame, r.headerWidthForText(), status),
	)

	clock := clockStyle.Render(time.Now().Format("15:04"))

	spacerW := r.width - lipgloss.Width(left) - lipgloss.Width(clock) - 2
	if spacerW < 1 {
		spacerW = 1
	}
	bar := lipgloss.JoinHorizontal(lipgloss.Bottom,
		left,
		lipgloss.NewStyle().Width(spacerW).Render(""),
		clock,
	)

	rule := horizontalRule(r.width - 2)
	return lipgloss.NewStyle().Padding(0, 1).Render(
		lipgloss.JoinVertical(lipgloss.Left, bar, rule),
	)
}

// headerWidthForText begrenzt die Breite des animierten Header-Texts, damit die
// Uhr rechts immer Platz hat.
func (r *rootModel) headerWidthForText() int {
	w := r.width - 12 // Platz für Uhr + Ränder
	if w < 10 {
		w = 10
	}
	if w > 60 {
		w = 60
	}
	return w
}

func (r *rootModel) headerEditorView() string {
	prompt := labelStyle.Render("set header text")
	input := lipgloss.NewStyle().Width(40).Render(r.editInput.View())
	help := helpStyle.Render("enter: save (mode → static)   ·   esc: cancel")
	card := cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, prompt, "", input))
	return lipgloss.JoinVertical(lipgloss.Center, card, "", help)
}

// saveHeaderCmd persistiert NUR die Header-Config (merge, ohne andere Felder
// zu überschreiben).
func (r *rootModel) saveHeaderCmd() tea.Cmd {
	h := r.header
	return func() tea.Msg {
		_ = updateState(func(s *persistedState) { s.Header = h })
		return nil
	}
}
