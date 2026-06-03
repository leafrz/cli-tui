package main

import (
	"fmt"
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

// goToLauncherMsg bringt das Dashboard zurück zum Startmenü.
type goToLauncherMsg struct{}

func goToLauncher() tea.Msg { return goToLauncherMsg{} }

// launcherEntry ist ein Eintrag im Startmenü. module == nil => "coming soon".
type launcherEntry struct {
	icon   string
	name   string
	desc   string
	module Module
}

func (e launcherEntry) available() bool { return e.module != nil }

// rootModel ist das Dashboard: Launcher + globaler Header + Routing zum Modul.
type rootModel struct {
	entries []launcherEntry
	active  int // -1 = Launcher, sonst Index in entries
	cursor  int // Auswahl im Launcher

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
		entries: []launcherEntry{
			{icon: "📻", name: "internet radio", desc: "stream stations worldwide", module: newRadioModule()},
			{icon: "☀", name: "weather", desc: "coming soon", module: nil},
			{icon: "✶", name: "placeholder", desc: "coming soon", module: nil},
		},
		active:    -1,
		header:    st.Header.withDefaults(),
		editInput: ei,
	}
}

func (r *rootModel) inLauncher() bool { return r.active < 0 }

func (r *rootModel) activeModule() Module {
	if r.inLauncher() {
		return nil
	}
	return r.entries[r.active].module
}

func (r *rootModel) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(r.entries)+1)
	for i := range r.entries {
		if r.entries[i].module != nil {
			cmds = append(cmds, r.entries[i].module.Init())
		}
	}
	cmds = append(cmds, headerTickCmd(r.header.animated()))
	return tea.Batch(cmds...)
}

func (r *rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.width = msg.Width
		r.height = msg.Height
		// an alle echten Module weiterreichen (Listen-Sizing etc.)
		var cmds []tea.Cmd
		for i := range r.entries {
			if r.entries[i].module != nil {
				mod, c := r.entries[i].module.Update(msg)
				r.entries[i].module = mod
				cmds = append(cmds, c)
			}
		}
		return r, tea.Batch(cmds...)

	case headerTickMsg:
		r.headerFrame++
		return r, headerTickCmd(r.header.animated())

	case goToLauncherMsg:
		r.active = -1
		return r, nil

	case tea.KeyMsg:
		key := msg.String()

		// 1) Header-Editor hat Vorrang.
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

		// 2) Globale Dashboard-Tasten (überall außer im Editor).
		switch key {
		case "ctrl+t":
			r.header = r.header.next()
			return r, r.saveHeaderCmd()
		case "ctrl+e":
			r.editing = true
			r.editInput.SetValue(r.header.Text)
			r.editInput.CursorEnd()
			r.editInput.Focus()
			return r, textinput.Blink
		}

		// 3) Launcher-Navigation (kein aktives Modul).
		if r.inLauncher() {
			switch key {
			case "ctrl+c", "q":
				return r, tea.Quit
			case "up", "k":
				if r.cursor > 0 {
					r.cursor--
				}
			case "down", "j":
				if r.cursor < len(r.entries)-1 {
					r.cursor++
				}
			case "enter":
				if r.entries[r.cursor].available() {
					r.active = r.cursor
				}
			}
			return r, nil
		}
	}

	// 4) Alles andere ans aktive Modul.
	if mod := r.activeModule(); mod != nil {
		newMod, cmd := mod.Update(msg)
		r.entries[r.active].module = newMod
		return r, cmd
	}
	return r, nil
}

func (r *rootModel) View() string {
	header := r.headerView()
	headerH := lipgloss.Height(header)

	contentH := r.height - headerH
	if contentH < 1 {
		contentH = 1
	}

	var content string
	switch {
	case r.editing:
		content = lipgloss.Place(r.width, contentH, lipgloss.Center, lipgloss.Center,
			r.headerEditorView())
	case r.inLauncher():
		content = r.launcherView(contentH)
	default:
		content = r.activeModule().View(r.width, contentH)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, content)
}

// launcherView rendert das Startmenü.
func (r *rootModel) launcherView(contentH int) string {
	title := labelStyle.Render("what do you wanna do?")

	rows := []string{title, ""}
	for i, e := range r.entries {
		cursor := "  "
		nameStyle := lipgloss.NewStyle().Foreground(colCream)
		if !e.available() {
			nameStyle = lipgloss.NewStyle().Foreground(colDim)
		}
		if i == r.cursor {
			cursor = lipgloss.NewStyle().Foreground(colPeach).Render("▸ ")
			nameStyle = nameStyle.Bold(true).Foreground(colPeach)
		}
		name := nameStyle.Render(fmt.Sprintf("%s  %-16s", e.icon, e.name))
		desc := dimStyle.Render(e.desc)
		rows = append(rows, cursor+name+"  "+desc)
	}

	card := cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
	help := helpStyle.Render(
		"↑/↓: select   ·   enter: open   ·   ctrl+t: header   ·   ctrl+e: edit   ·   ctrl+c: quit")
	menu := lipgloss.JoinVertical(lipgloss.Center, card, "", help)

	return lipgloss.Place(r.width, contentH, lipgloss.Center, lipgloss.Center, menu)
}

// headerView rendert den globalen Header (Titel/Animation links, Uhr rechts).
func (r *rootModel) headerView() string {
	status := ""
	if mod := r.activeModule(); mod != nil {
		status = mod.Status()
	}
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

// headerWidthForText begrenzt die Breite des animierten Header-Texts.
func (r *rootModel) headerWidthForText() int {
	w := r.width - 12
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

// saveHeaderCmd persistiert NUR die Header-Config (merge).
func (r *rootModel) saveHeaderCmd() tea.Cmd {
	h := r.header
	return func() tea.Msg {
		_ = updateState(func(s *persistedState) { s.Header = h })
		return nil
	}
}
