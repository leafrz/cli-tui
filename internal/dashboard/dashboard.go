package dashboard

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leafrz/dashboard/internal/config"
	"github.com/leafrz/dashboard/internal/core"
	"github.com/leafrz/dashboard/internal/ui"

	"github.com/leafrz/dashboard/internal/audio"
	"github.com/leafrz/dashboard/internal/modules/ambient"
	"github.com/leafrz/dashboard/internal/modules/hosts"
	"github.com/leafrz/dashboard/internal/modules/radio"
	"github.com/leafrz/dashboard/internal/modules/settings"
	"github.com/leafrz/dashboard/internal/modules/sysmon"
	"github.com/leafrz/dashboard/internal/modules/todo"
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

// idleTickMsg prüft im Sekundentakt auf Inaktivität (Auto-Screensaver).
type idleTickMsg time.Time

func idleTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return idleTickMsg(t) })
}

// launcherEntry ist ein Eintrag im Startmenü. module == nil => "coming soon".
type launcherEntry struct {
	icon   string
	name   string
	desc   string
	module core.Module
}

func (e launcherEntry) available() bool { return e.module != nil }

// rootModel ist das Dashboard: Launcher + globaler Header + Routing zum Modul.
type rootModel struct {
	entries []launcherEntry
	active  int // -1 = Launcher, sonst Index in entries
	cursor  int // Auswahl im Launcher

	width  int
	height int

	showHelp bool // globale Hilfe (nur im Launcher)

	theme string // aktueller Theme-Name

	header      config.HeaderConfig
	headerFrame int

	// In-App Header-Editor
	editing   bool
	editInput textinput.Model

	// Auto-Screensaver (Idle)
	lastInput   time.Time
	idleActive  bool
	prevActive  int
	idleTimeout time.Duration
	ambientIdx  int

	player *audio.Player // geteilt; für globalen Now-Playing-Footer + Media-Tasten
}

func NewRoot() *rootModel {
	st := config.Load()
	ui.ApplyTheme(ui.ThemeByName(st.Theme)) // Palette setzen, bevor irgendwas rendert

	// Ein geteilter Player für Radio + Visualizer.
	player := audio.NewPlayer()

	ei := textinput.New()
	ei.Prompt = "› "
	ei.PromptStyle = lipgloss.NewStyle().Foreground(ui.ColTeal)
	ei.TextStyle = lipgloss.NewStyle().Foreground(ui.ColCream)
	ei.CharLimit = 80
	ei.Width = 40

	r := &rootModel{
		entries: []launcherEntry{
			{icon: "📻", name: "internet radio", desc: "stream stations worldwide", module: radio.New(player)},
			{icon: "📊", name: "system monitor", desc: "cpu · memory · disk · network", module: sysmon.New()},
			{icon: "🌌", name: "ambient", desc: "screensaver + clock + weather", module: ambient.New(player)},
			{icon: "✓", name: "todo", desc: "tasks & checklist", module: todo.New()},
			{icon: "⇄", name: "ssh hosts", desc: "saved hosts · status · connect", module: hosts.New()},
			{icon: "⚙", name: "settings", desc: "theme · header · weather · screensaver", module: settings.New()},
			{icon: "☀", name: "weather", desc: "coming soon", module: nil},
		},
		active:      -1,
		theme:       st.Theme,
		header:      st.Header.WithDefaults(),
		editInput:   ei,
		lastInput:   time.Now(),
		idleTimeout: st.Ambient.IdleTimeout(),
		ambientIdx:  -1,
		player:      player,
	}
	for i := range r.entries {
		if r.entries[i].name == "ambient" {
			r.ambientIdx = i
		}
	}
	return r
}

func (r *rootModel) inLauncher() bool { return r.active < 0 }

func (r *rootModel) activeModule() core.Module {
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
	cmds = append(cmds, headerTickCmd(r.header.Animated()))
	cmds = append(cmds, idleTick())
	return tea.Batch(cmds...)
}

func (r *rootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.width = msg.Width
		r.height = msg.Height
		// an alle echten core.Module weiterreichen (Listen-Sizing etc.)
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
		return r, headerTickCmd(r.header.Animated())

	case idleTickMsg:
		if r.idleTimeout > 0 && r.ambientIdx >= 0 && !r.idleActive && !r.editing &&
			r.active != r.ambientIdx && time.Since(r.lastInput) > r.idleTimeout {
			r.prevActive = r.active
			r.active = r.ambientIdx
			r.idleActive = true
			return r, tea.Batch(core.FocusModule, idleTick())
		}
		return r, idleTick()

	case core.GoToLauncherMsg:
		r.active = -1
		return r, nil

	case core.SwitchModuleMsg:
		for i := range r.entries {
			if r.entries[i].name == msg.Name && r.entries[i].module != nil {
				r.active = i
				return r, core.FocusModule
			}
		}
		return r, nil

	case core.ThemeChangedMsg:
		// an ALLE core.Module weiterreichen, damit auch inaktive ihre Styles anpassen.
		var cmds []tea.Cmd
		for i := range r.entries {
			if r.entries[i].module != nil {
				mod, c := r.entries[i].module.Update(msg)
				r.entries[i].module = mod
				cmds = append(cmds, c)
			}
		}
		return r, tea.Batch(cmds...)

	case core.ReloadConfigMsg:
		// Eigene (Root-)Config neu laden ...
		st := config.Load()
		r.header = st.Header.WithDefaults()
		r.theme = st.Theme
		r.idleTimeout = st.Ambient.IdleTimeout()
		ui.ApplyTheme(ui.ThemeByName(st.Theme))
		// ... an alle core.Module weiterreichen + Komponenten neu einfärben.
		cmds := []tea.Cmd{core.ThemeChanged}
		for i := range r.entries {
			if r.entries[i].module != nil {
				mod, c := r.entries[i].module.Update(msg)
				r.entries[i].module = mod
				cmds = append(cmds, c)
			}
		}
		return r, tea.Batch(cmds...)

	case tea.KeyMsg:
		key := msg.String()
		r.lastInput = time.Now()

		// Auto-Screensaver aktiv? Erste Taste weckt nur auf (keine Aktion).
		if r.idleActive {
			r.idleActive = false
			r.active = r.prevActive
			return r, core.FocusModule
		}

		// 1) Header-Editor hat Vorrang.
		if r.editing {
			switch key {
			case "enter":
				r.header.Text = r.editInput.Value()
				r.header.Mode = config.HeaderStatic
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
			r.header = r.header.Next()
			return r, r.saveHeaderCmd()
		case "ctrl+e":
			r.editing = true
			r.editInput.SetValue(r.header.Text)
			r.editInput.CursorEnd()
			r.editInput.Focus()
			return r, textinput.Blink
		case "ctrl+p":
			r.theme = ui.NextThemeName(r.theme)
			ui.ApplyTheme(ui.ThemeByName(r.theme))
			return r, tea.Batch(r.saveThemeCmd(), core.ThemeChanged)

		// Globale Media-Tasten (aus jedem Modul): Pause + Volume.
		case "ctrl+@", "ctrl+ ": // ctrl+space
			if playing, _, _ := r.player.GetStatus(); playing {
				r.player.TogglePause()
			}
			return r, nil
		case "ctrl+up":
			r.player.AdjustVolume(0.1)
			return r, nil
		case "ctrl+down":
			r.player.AdjustVolume(-0.1)
			return r, nil
		}

		// 3) Launcher-Navigation (kein aktives Modul).
		if r.inLauncher() {
			// Globale Hilfe hat Vorrang, solange offen.
			if r.showHelp {
				if key == "esc" || key == "?" || key == "q" {
					r.showHelp = false
				}
				return r, nil
			}
			switch key {
			case "?":
				r.showHelp = true
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
					return r, core.FocusModule // Modul (neu) anstoßen
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
	footer := r.footerView()
	contentH := r.height - lipgloss.Height(header) - lipgloss.Height(footer)
	if contentH < 1 {
		contentH = 1
	}

	var content string
	switch {
	case r.editing:
		content = lipgloss.Place(r.width, contentH, lipgloss.Center, lipgloss.Center,
			r.headerEditorView())
	case r.inLauncher() && r.showHelp:
		content = lipgloss.Place(r.width, contentH, lipgloss.Center, lipgloss.Center,
			globalHelpView(r.theme))
	case r.inLauncher():
		content = r.launcherView(contentH)
	default:
		content = r.activeModule().View(r.width, contentH)
	}

	return lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
}

// footerView ist die globale Now-Playing-/Volume-Leiste (in jedem Modul sichtbar).
func (r *rootModel) footerView() string {
	playing, paused, vol := r.player.GetStatus()
	muted := r.player.IsMuted()

	var left string
	if playing {
		icon, col := "▶", ui.ColMauve
		if paused {
			icon, col = "❚❚", ui.ColPeach
		}
		label := r.player.StationName()
		if track := r.player.GetMetadata(); track != "" {
			if label != "" {
				label = track + "  ·  " + label
			} else {
				label = track
			}
		}
		if label == "" {
			label = "live"
		}
		left = lipgloss.NewStyle().Foreground(col).Render(icon + " " + truncate(label, r.width-24))
	} else {
		left = ui.DimStyle.Render("○ nothing playing")
	}

	var right string
	if muted {
		right = lipgloss.NewStyle().Foreground(ui.ColPeach).Render("🔇 muted")
	} else {
		right = ui.DimStyle.Render("vol ") + ui.RenderVolumeBar(vol, 10) +
			ui.DimStyle.Render(fmt.Sprintf(" %3.0f%%", vol*100))
	}

	spacerW := r.width - lipgloss.Width(left) - lipgloss.Width(right) - 4
	if spacerW < 1 {
		spacerW = 1
	}
	row := lipgloss.JoinHorizontal(lipgloss.Bottom,
		left, lipgloss.NewStyle().Width(spacerW).Render(""), right)

	return lipgloss.NewStyle().Padding(0, 1).Render(
		lipgloss.JoinVertical(lipgloss.Left, ui.HorizontalRule(r.width-2), row))
}

// truncate kürzt s (nach Runen) auf max Zeichen mit "…".
func truncate(s string, max int) string {
	if max < 1 {
		max = 1
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}

// launcherView rendert das Startmenü.
func (r *rootModel) launcherView(contentH int) string {
	title := ui.LabelStyle.Render("what do you wanna do?")

	rows := []string{title, ""}
	for i, e := range r.entries {
		cursor := "  "
		nameStyle := lipgloss.NewStyle().Foreground(ui.ColCream)
		if !e.available() {
			nameStyle = lipgloss.NewStyle().Foreground(ui.ColDim)
		}
		if i == r.cursor {
			cursor = lipgloss.NewStyle().Foreground(ui.ColPeach).Render("▸ ")
			nameStyle = nameStyle.Bold(true).Foreground(ui.ColPeach)
		}
		name := nameStyle.Render(fmt.Sprintf("%s  %-16s", e.icon, e.name))
		desc := ui.DimStyle.Render(e.desc)
		rows = append(rows, cursor+name+"  "+desc)
	}

	card := ui.CardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
	help := ui.HelpStyle.Render(fmt.Sprintf(
		"↑/↓: select   ·   enter: open   ·   ctrl+p: theme (%s)   ·   ?: help   ·   ctrl+c: quit",
		r.theme))
	menu := lipgloss.JoinVertical(lipgloss.Center, card, "", help)

	return lipgloss.Place(r.width, contentH, lipgloss.Center, lipgloss.Center, menu)
}

// globalHelpView rendert die GLOBALE Hilfe (dashboard-weite Befehle).
func globalHelpView(theme string) string {
	sections := []ui.HelpSection{
		{Title: "navigation", Rows: [][2]string{
			{"↑/↓", "select module"},
			{"enter", "open module"},
			{"esc", "back to dashboard (from a module)"},
		}},
		{Title: "appearance", Rows: [][2]string{
			{"ctrl+t", "cycle header mode"},
			{"ctrl+e", "edit header text"},
			{"ctrl+p", "cycle theme (now: " + theme + ")"},
		}},
		{Title: "playback (from anywhere)", Rows: [][2]string{
			{"ctrl+space", "play / pause"},
			{"ctrl+↑/↓", "volume"},
		}},
		{Title: "app", Rows: [][2]string{
			{"?", "toggle this help"},
			{"ctrl+c", "quit"},
		}},
	}
	return ui.HelpOverlay("dashboard · help", sections, "? or esc to close")
}

// headerView rendert den globalen Header (Titel/Animation links, Uhr rechts).
func (r *rootModel) headerView() string {
	status := ""
	if mod := r.activeModule(); mod != nil {
		status = mod.Status()
	}
	left := ui.HeaderTextStyle.Render(
		headerText(r.header, r.headerFrame, r.headerWidthForText(), status),
	)

	clock := ui.ClockStyle.Render(time.Now().Format("15:04"))

	spacerW := r.width - lipgloss.Width(left) - lipgloss.Width(clock) - 2
	if spacerW < 1 {
		spacerW = 1
	}
	bar := lipgloss.JoinHorizontal(lipgloss.Bottom,
		left,
		lipgloss.NewStyle().Width(spacerW).Render(""),
		clock,
	)

	rule := ui.HorizontalRule(r.width - 2)
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
	prompt := ui.LabelStyle.Render("set header text")
	input := lipgloss.NewStyle().Width(40).Render(r.editInput.View())
	help := ui.HelpStyle.Render("enter: save (mode → static)   ·   esc: cancel")
	card := ui.CardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, prompt, "", input))
	return lipgloss.JoinVertical(lipgloss.Center, card, "", help)
}

// saveHeaderCmd persistiert NUR die Header-Config (merge).
func (r *rootModel) saveHeaderCmd() tea.Cmd {
	h := r.header
	return func() tea.Msg {
		_ = config.Update(func(s *config.State) { s.Header = h })
		return nil
	}
}

// saveThemeCmd persistiert NUR den Theme-Namen (merge).
func (r *rootModel) saveThemeCmd() tea.Cmd {
	name := r.theme
	return func() tea.Msg {
		_ = config.Update(func(s *config.State) { s.Theme = name })
		return nil
	}
}
