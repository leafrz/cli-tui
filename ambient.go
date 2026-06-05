package main

import (
	"math/rand"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/leafrz/dashboard/radio"
)

// --- cell grid (für Overlay von Uhr/Infos über Animation) ------------------

type cell struct {
	ch  rune
	col lipgloss.Color // "" = keine Farbe (Default-Vordergrund)
}

type grid struct {
	w, h  int
	cells []cell
}

func newGrid(w, h int) *grid {
	g := &grid{w: w, h: h, cells: make([]cell, w*h)}
	for i := range g.cells {
		g.cells[i].ch = ' '
	}
	return g
}

func (g *grid) set(x, y int, ch rune, col lipgloss.Color) {
	if x < 0 || y < 0 || x >= g.w || y >= g.h {
		return
	}
	g.cells[y*g.w+x] = cell{ch, col}
}

func (g *grid) stampText(x, y int, s string, col lipgloss.Color) {
	for i, r := range []rune(s) {
		g.set(x+i, y, r, col)
	}
}

// render fasst gleichfarbige Runs pro Zeile zusammen (weniger ANSI-Escapes).
func (g *grid) render() string {
	var sb strings.Builder
	for y := 0; y < g.h; y++ {
		x := 0
		for x < g.w {
			c := g.cells[y*g.w+x]
			j := x
			var run []rune
			for j < g.w && g.cells[y*g.w+j].col == c.col {
				run = append(run, g.cells[y*g.w+j].ch)
				j++
			}
			if c.col == "" {
				sb.WriteString(string(run))
			} else {
				sb.WriteString(lipgloss.NewStyle().Foreground(c.col).Render(string(run)))
			}
			x = j
		}
		if y < g.h-1 {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// star wird von der Starfield-Szene genutzt (scenes.go).
type star struct{ x, y, z float64 }

// --- messages --------------------------------------------------------------

type ambientTickMsg time.Time

func ambientTick() tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(t time.Time) tea.Msg { return ambientTickMsg(t) })
}

// --- module ----------------------------------------------------------------

type ambientModule struct {
	player        *radio.Player
	width, height int
	frame         int
	style         int
	showClock     bool
	clock24       bool
	autoRotate    bool
	rotateCounter int
	showHelp      bool
	ticking       bool

	rng        *rand.Rand
	scenes     []scene
	specLevels []float64 // Mini-Spektrum (wenn Radio läuft)
	cfg        ambientConfig

	// weather
	weatherCfg      weatherConfig
	weatherLine     string
	weatherAt       time.Time
	weatherFetching bool

	// location editor
	editing   bool
	editInput textinput.Model
}

func newAmbientModule(p *radio.Player) *ambientModule {
	ei := textinput.New()
	ei.Prompt = "› "
	ei.Placeholder = "city (empty = auto by IP)"
	ei.PromptStyle = lipgloss.NewStyle().Foreground(colTeal)
	ei.TextStyle = lipgloss.NewStyle().Foreground(colCream)
	ei.CharLimit = 60
	ei.Width = 36

	st := loadState()
	scenes := buildScenes()

	m := &ambientModule{
		player:     p,
		rng:        rand.New(rand.NewSource(time.Now().UnixNano())),
		scenes:     scenes,
		weatherCfg: st.Weather,
		cfg:        st.Ambient,
		editInput:  ei,
	}
	// Vorlieben anwenden (Zero-Value = sinnvolle Defaults).
	m.showClock = !st.Ambient.HideClock
	m.clock24 = !st.Ambient.Clock12
	m.autoRotate = st.Ambient.Rotate
	for i, sc := range scenes {
		if sc.name() == st.Ambient.Scene {
			m.style = i
		}
	}
	return m
}

// persistPrefsCmd speichert die Ambient-Vorlieben (merge; Idle-Felder bleiben).
func (m *ambientModule) persistPrefsCmd() tea.Cmd {
	cfg := m.cfg
	cfg.Scene = m.scenes[m.style].name()
	cfg.HideClock = !m.showClock
	cfg.Clock12 = !m.clock24
	cfg.Rotate = m.autoRotate
	m.cfg = cfg
	return func() tea.Msg {
		_ = updateState(func(s *persistedState) { s.Ambient = cfg })
		return nil
	}
}

// sampleSpectrum füllt das Mini-Spektrum aus dem laufenden Audio.
func (m *ambientModule) sampleSpectrum() {
	const bands = 24
	if len(m.specLevels) != bands {
		m.specLevels = make([]float64, bands)
	}
	spec := m.player.Spectrum(bands)
	if spec == nil {
		for i := range m.specLevels {
			m.specLevels[i] *= 0.8
		}
		return
	}
	for i := range m.specLevels {
		if spec[i] > m.specLevels[i] {
			m.specLevels[i] = spec[i]
		} else {
			m.specLevels[i] = m.specLevels[i]*0.82 + spec[i]*0.18
		}
	}
}

func (m *ambientModule) Name() string   { return "ambient" }
func (m *ambientModule) Status() string { return "" }
func (m *ambientModule) Init() tea.Cmd  { return nil }

// shouldFetchWeather: Backoff 20s bei Fehlschlag, sonst alle 15 Minuten.
func (m *ambientModule) shouldFetchWeather() bool {
	if m.weatherFetching || m.weatherCfg.Mode == "off" {
		return false
	}
	interval := 15 * time.Minute
	if m.weatherLine == "" {
		interval = 20 * time.Second
	}
	return time.Since(m.weatherAt) > interval
}

func (m *ambientModule) maybeWeather() tea.Cmd {
	if !m.shouldFetchWeather() {
		return nil
	}
	m.weatherFetching = true
	m.weatherAt = time.Now()
	return weatherCmd(m.weatherCfg)
}

// persistWeatherCmd speichert NUR die Wetter-Config (merge).
func (m *ambientModule) persistWeatherCmd() tea.Cmd {
	cfg := m.weatherCfg
	return func() tea.Msg {
		_ = updateState(func(s *persistedState) { s.Weather = cfg })
		return nil
	}
}

func (m *ambientModule) Update(msg tea.Msg) (Module, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case focusMsg:
		cmds := []tea.Cmd{}
		if !m.ticking {
			m.ticking = true
			cmds = append(cmds, ambientTick())
		}
		if c := m.maybeWeather(); c != nil {
			cmds = append(cmds, c)
		}
		return m, tea.Batch(cmds...)

	case ambientTickMsg:
		m.frame++
		m.advance()
		if m.player != nil {
			m.sampleSpectrum()
		}
		cmds := []tea.Cmd{ambientTick()}
		if m.autoRotate {
			m.rotateCounter++
			if m.rotateCounter >= 333 { // ~30s bei 90ms
				m.style = (m.style + 1) % len(m.scenes)
				m.rotateCounter = 0
				cmds = append(cmds, m.persistPrefsCmd())
			}
		}
		if c := m.maybeWeather(); c != nil {
			cmds = append(cmds, c)
		}
		return m, tea.Batch(cmds...)

	case weatherMsg:
		m.weatherFetching = false
		if msg.text != "" {
			m.weatherLine = msg.text
		}
		return m, nil

	case tea.KeyMsg:
		k := msg.String()

		// Standort-Editor hat Vorrang.
		if m.editing {
			switch k {
			case "enter":
				val := strings.TrimSpace(m.editInput.Value())
				if val == "" {
					m.weatherCfg.Mode, m.weatherCfg.City = "auto", ""
				} else {
					m.weatherCfg.Mode, m.weatherCfg.City = "manual", val
				}
				m.editing = false
				m.weatherLine = ""        // sichtbares Refetch erzwingen
				m.weatherAt = time.Time{} // shouldFetch true
				return m, tea.Batch(m.persistWeatherCmd(), m.maybeWeather())
			case "esc":
				m.editing = false
				return m, nil
			}
			var cmd tea.Cmd
			m.editInput, cmd = m.editInput.Update(msg)
			return m, cmd
		}

		if m.showHelp {
			if k == "esc" || k == "?" || k == "q" {
				m.showHelp = false
			}
			return m, nil
		}
		switch k {
		case "esc", "q", "backspace":
			m.ticking = false
			return m, goToLauncher
		case "?":
			m.showHelp = true
		case " ", "s":
			m.style = (m.style + 1) % len(m.scenes)
			m.rotateCounter = 0
			return m, m.persistPrefsCmd()
		case "S":
			m.style = (m.style - 1 + len(m.scenes)) % len(m.scenes)
			m.rotateCounter = 0
			return m, m.persistPrefsCmd()
		case "r":
			m.autoRotate = !m.autoRotate
			m.rotateCounter = 0
			return m, m.persistPrefsCmd()
		case "c":
			m.showClock = !m.showClock
			return m, m.persistPrefsCmd()
		case "h":
			m.clock24 = !m.clock24
			return m, m.persistPrefsCmd()
		case "w":
			m.editing = true
			m.editInput.SetValue(m.weatherCfg.City)
			m.editInput.CursorEnd()
			m.editInput.Focus()
			return m, textinput.Blink
		}
	}
	return m, nil
}

func (m *ambientModule) advance() {
	m.scenes[m.style].advance(m.width, m.height, m.rng)
}

func (m *ambientModule) View(width, height int) string {
	m.width, m.height = width, height
	if m.editing {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, m.locationEditorView())
	}
	if m.showHelp {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, m.helpView())
	}
	if width < 4 || height < 4 {
		return ""
	}

	g := newGrid(width, height)
	m.scenes[m.style].draw(g, m.rng)
	if m.showClock {
		m.drawClock(g)
	}

	scene := m.scenes[m.style].name()
	if m.autoRotate {
		scene += " ↻"
	}
	hint := "space: scene · r: rotate · c: clock · w: location · ?: help · esc   (" +
		scene + ")"
	g.stampText(centerX(width, hint), height-1, hint, colDim)

	return g.render()
}

func (m *ambientModule) locationEditorView() string {
	prompt := labelStyle.Render("weather location")
	input := lipgloss.NewStyle().Width(38).Render(m.editInput.View())
	hint := helpStyle.Render("enter: save   ·   empty = auto by IP   ·   esc: cancel")
	card := cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, prompt, "", input))
	return lipgloss.JoinVertical(lipgloss.Center, card, "", hint)
}

// centerX liefert die linke Startspalte, um s (nach Runen) zu zentrieren.
func centerX(width int, s string) int {
	x := (width - len([]rune(s))) / 2
	if x < 0 {
		x = 0
	}
	return x
}

// nowPlaying liefert die "läuft gerade"-Zeile (oder "").
func (m *ambientModule) nowPlaying() string {
	if m.player == nil {
		return ""
	}
	playing, _, _ := m.player.GetStatus()
	if !playing {
		return ""
	}
	if meta := m.player.GetMetadata(); meta != "" {
		return "♫ " + meta
	}
	return "● live"
}

// --- big clock -------------------------------------------------------------

var clockFont = map[rune][]string{
	'0': {"████", "█  █", "█  █", "█  █", "████"},
	'1': {"  █ ", " ██ ", "  █ ", "  █ ", " ███"},
	'2': {"████", "   █", "████", "█   ", "████"},
	'3': {"████", "   █", " ███", "   █", "████"},
	'4': {"█  █", "█  █", "████", "   █", "   █"},
	'5': {"████", "█   ", "████", "   █", "████"},
	'6': {"████", "█   ", "████", "█  █", "████"},
	'7': {"████", "   █", "  █ ", " █  ", " █  "},
	'8': {"████", "█  █", "████", "█  █", "████"},
	'9': {"████", "█  █", "████", "   █", "████"},
	':': {"    ", "  █ ", "    ", "  █ ", "    "},
}

func (m *ambientModule) drawClock(g *grid) {
	now := time.Now()
	text := now.Format("15:04")
	if !m.clock24 {
		text = now.Format("03:04")
	}

	const gw, gh = 4, 5
	const gap = 1
	const sx, sy = 2, 2

	baseW := len(text)*(gw+gap) - gap
	totalW := baseW * sx
	totalH := gh * sy

	ox := (m.width - totalW) / 2
	oy := (m.height-totalH)/2 - 2

	for i, ch := range text {
		glyph, ok := clockFont[ch]
		if !ok {
			continue
		}
		baseX := i * (gw + gap)
		for r := 0; r < gh; r++ {
			rrow := []rune(glyph[r])
			for c := 0; c < gw && c < len(rrow); c++ {
				if rrow[c] != '█' {
					continue
				}
				for dy := 0; dy < sy; dy++ {
					for dx := 0; dx < sx; dx++ {
						g.set(ox+(baseX+c)*sx+dx, oy+r*sy+dy, '█', colPeach)
					}
				}
			}
		}
	}

	// Info-Zeilen unter der Uhr.
	type infoLine struct {
		s string
		c lipgloss.Color
	}
	dateFmt := "Mon 02 Jan · 15:04:05"
	if !m.clock24 {
		dateFmt = "Mon 02 Jan · 03:04:05 PM"
	}
	lines := []infoLine{{now.Format(dateFmt), colDim}}
	if m.weatherLine != "" {
		lines = append(lines, infoLine{m.weatherLine, colTeal})
	}
	np := m.nowPlaying()
	if np != "" {
		lines = append(lines, infoLine{np, colMauve})
	}
	for i, ln := range lines {
		g.stampText(centerX(m.width, ln.s), oy+totalH+1+i, ln.s, ln.c)
	}

	// Mini-Spektrum, wenn das Radio läuft.
	if np != "" && len(m.specLevels) > 0 {
		g.stampBars(m.specLevels, m.width/2, oy+totalH+1+len(lines)+1, 3)
	}
}

// stampBars zeichnet ein kompaktes Spektrum (ein Band = Balken + Lücke),
// horizontal um cx zentriert, von topY nach unten height Zeilen.
func (g *grid) stampBars(levels []float64, cx, topY, height int) {
	bars := len(levels)
	startX := cx - bars // bars*2 Spalten breit / 2
	for b, lv := range levels {
		x := startX + b*2
		for r := 0; r < height; r++ {
			cellFromBottom := height - r
			if ch := barRune(lv, cellFromBottom, height); ch != ' ' {
				g.set(x, topY+r, ch, eqRowColor(cellFromBottom, height))
			}
		}
	}
}

func (m *ambientModule) helpView() string {
	sections := []helpSection{
		{title: "ambient", rows: [][2]string{
			{"space / s", "next scene"},
			{"S", "previous scene"},
			{"r", "auto-rotate scenes"},
			{"c", "toggle clock"},
			{"h", "12 / 24-hour clock"},
			{"w", "set weather location (city / auto)"},
			{"esc / q", "back to dashboard"},
			{"?", "toggle this help"},
		}},
	}
	return helpOverlay("ambient · help", sections,
		"auto-screensaver after ~2 min idle · any key wakes")
}
