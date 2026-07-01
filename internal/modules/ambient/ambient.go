package ambient

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leafrz/dashboard/internal/config"
	"github.com/leafrz/dashboard/internal/core"
	"github.com/leafrz/dashboard/internal/ui"

	"github.com/leafrz/dashboard/internal/audio"
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
	player        *audio.Player
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
	cfg        config.AmbientConfig

	// Kiosk-Modus (--autostart): der große Enter-Buzzer ist die einzige Taste.
	// 1x = Volume up, 2x = Volume down, lang (Key-Repeat) = Hotfix.
	kiosk         bool
	volFlashUntil time.Time // solange in der Zukunft: großes Volume-Overlay zeigen

	// Enter-Gesten-Erkennung (Buzzer sendet nur Key-Down + Key-Repeat).
	enterLast  time.Time // Zeitpunkt des letzten Enter
	enterCount int       // Drücke im aktuellen Fenster
	rapidRun   int       // aufeinanderfolgende Repeat-Events (<150ms Abstand)
	longFired  bool      // Hotfix in diesem Burst schon ausgelöst

	// Hotfix-Counter (Meme: jede Woche ein Hotfix).
	hotfixCount      int
	hotfixFlashUntil time.Time
	editingHotfix    bool

	// weather
	weatherCfg      config.WeatherConfig
	weatherLine     string
	weatherAt       time.Time
	weatherFetching bool

	// location editor
	editing   bool
	editInput textinput.Model
}

func New(p *audio.Player) *ambientModule {
	ei := textinput.New()
	ei.Prompt = "› "
	ei.Placeholder = "city (empty = auto by IP)"
	ei.PromptStyle = lipgloss.NewStyle().Foreground(ui.ColTeal)
	ei.TextStyle = lipgloss.NewStyle().Foreground(ui.ColCream)
	ei.CharLimit = 60
	ei.Width = 36

	st := config.Load()
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
	m.setDvdLogo(st.Header.Text)
	m.hotfixCount = st.Hotfixes
	return m
}

// setDvdLogo writes the header text into the dvd bounce scene logo.
func (m *ambientModule) setDvdLogo(text string) {
	for _, sc := range m.scenes {
		if d, ok := sc.(*dvdScene); ok {
			if text != "" {
				d.logo = text
			}
			return
		}
	}
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
		_ = config.Update(func(s *config.State) { s.Ambient = cfg })
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
		_ = config.Update(func(s *config.State) { s.Weather = cfg })
		return nil
	}
}

func (m *ambientModule) Update(msg tea.Msg) (core.Module, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case core.AutostartMsg:
		// Autostart: Szenen-Rotation einschalten + Kiosk-Modus (Enter-Buzzer).
		m.autoRotate = true
		m.rotateCounter = 0
		m.kiosk = true
		return m, nil

	case core.FocusMsg:
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

	case core.ReloadConfigMsg:
		st := config.Load()
		m.weatherCfg = st.Weather
		m.cfg = st.Ambient
		m.showClock = !st.Ambient.HideClock
		m.clock24 = !st.Ambient.Clock12
		m.autoRotate = st.Ambient.Rotate
		for i, sc := range m.scenes {
			if sc.name() == st.Ambient.Scene {
				m.style = i
			}
		}
		m.setDvdLogo(st.Header.Text)
		m.hotfixCount = st.Hotfixes
		m.weatherLine = ""
		m.weatherAt = time.Time{} // beim nächsten Tick/Focus neu holen (oder aus)
		return m, nil

	case enterResolveMsg:
		m.resolveEnter(time.Now())
		return m, nil

	case tea.KeyMsg:
		k := msg.String()

		// Hotfix-Counter-Editor hat Vorrang.
		if m.editingHotfix {
			switch k {
			case "enter":
				if n, err := strconv.Atoi(strings.TrimSpace(m.editInput.Value())); err == nil && n >= 0 {
					m.hotfixCount = n
					m.editingHotfix = false
					return m, m.persistHotfixCmd()
				}
				return m, nil // ungültige Eingabe -> Editor bleibt offen
			case "esc":
				m.editingHotfix = false
				return m, nil
			}
			var cmd tea.Cmd
			m.editInput, cmd = m.editInput.Update(msg)
			return m, cmd
		}

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
		case "enter":
			// Kiosk: der große Enter-Buzzer ist die einzige Taste.
			// 1x = lauter, 2x = leiser, lang (Key-Repeat) = Hotfix.
			if m.kiosk {
				return m, m.handleEnter(time.Now())
			}
		case "e":
			// Kiosk: Hotfix-Counter editieren (mit Tastatur am Gerät).
			if m.kiosk {
				m.editingHotfix = true
				m.editInput.SetValue(fmt.Sprintf("%d", m.hotfixCount))
				m.editInput.CursorEnd()
				m.editInput.Focus()
				return m, textinput.Blink
			}
		case "esc", "q", "backspace":
			m.ticking = false
			return m, core.GoToLauncher
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

// --- Kiosk: Enter-Buzzer-Gesten ---------------------------------------------
// Terminals melden kein Key-Up. Ein gehaltener Buzzer erzeugt Key-Repeat:
// erst ~500ms Pause, dann Events im ~30ms-Takt. Darauf baut die Erkennung:
//
//	Repeat-Burst (>=3 Events mit <150ms Abstand)  -> langer Druck  -> Hotfix
//	sonst: Drücke zählen, nach 650ms Ruhe auflösen -> 1x lauter, 2x leiser
//
// resolveAfter MUSS größer sein als die OS-Repeat-Verzögerung, sonst feuert
// "lauter" vor jedem Hotfix.
const (
	rapidGap     = 150 * time.Millisecond
	resolveAfter = 650 * time.Millisecond
	volStep      = 0.1
)

type enterResolveMsg time.Time

// handleEnter verarbeitet ein Enter-Event des Buzzers (testbar über now).
func (m *ambientModule) handleEnter(now time.Time) tea.Cmd {
	gap := now.Sub(m.enterLast)
	m.enterLast = now

	if gap < rapidGap {
		m.rapidRun++
	} else {
		m.rapidRun = 0
		if gap > resolveAfter {
			m.longFired = false // neuer Burst
		}
	}

	// Key-Repeat erkannt = langer Druck -> Hotfix (einmal pro Burst).
	if m.rapidRun >= 3 {
		if m.longFired {
			return nil
		}
		m.longFired = true
		m.enterCount = 0
		return m.hotfixCmd()
	}
	if m.longFired {
		return nil // Ausläufer des Repeat-Bursts schlucken
	}

	m.enterCount++
	return tea.Tick(resolveAfter+50*time.Millisecond,
		func(t time.Time) tea.Msg { return enterResolveMsg(t) })
}

// resolveEnter löst das Drück-Fenster auf, sobald lange genug Ruhe war.
func (m *ambientModule) resolveEnter(now time.Time) {
	if now.Sub(m.enterLast) < resolveAfter {
		return // es kam noch was; ein späterer Tick übernimmt
	}
	n := m.enterCount
	m.enterCount = 0
	if m.longFired {
		m.longFired = false // Burst beendet, nichts weiter tun
		return
	}
	switch {
	case n == 1:
		m.stepVolume(+1)
	case n >= 2:
		m.stepVolume(-1)
	}
}

// stepVolume ändert die Lautstärke um volStep in dir (+1/-1), hebt Mute auf.
func (m *ambientModule) stepVolume(dir float64) {
	if m.player == nil {
		return
	}
	if m.player.IsMuted() {
		m.player.ToggleMute()
	}
	m.player.AdjustVolume(volStep * dir)
	m.volFlashUntil = time.Now().Add(2500 * time.Millisecond)
}

// hotfixCmd erhöht den Hotfix-Counter, persistiert ihn und spielt einen
// zufälligen Meme-Sound aus dem sounds/-Ordner neben dem Binary.
func (m *ambientModule) hotfixCmd() tea.Cmd {
	m.hotfixCount++
	m.hotfixFlashUntil = time.Now().Add(3 * time.Second)
	return tea.Batch(
		m.persistHotfixCmd(),
		func() tea.Msg { _ = audio.PlayRandomSFX("sounds"); return nil },
	)
}

func (m *ambientModule) persistHotfixCmd() tea.Cmd {
	n := m.hotfixCount
	return func() tea.Msg {
		_ = config.Update(func(s *config.State) { s.Hotfixes = n })
		return nil
	}
}

// drawHotfixBox zeichnet den permanenten Hotfix-Counter oben rechts (Kiosk).
func (m *ambientModule) drawHotfixBox(g *grid) {
	label := fmt.Sprintf(" ⚑ hotfix #%d ", m.hotfixCount)
	w := len([]rune(label))
	x := g.w - w - 4
	if x < 0 {
		x = 0
	}
	g.stampText(x, 0, "╭"+strings.Repeat("─", w)+"╮", ui.ColPurple)
	g.stampText(x, 1, "│", ui.ColPurple)
	g.stampText(x+1, 1, label, ui.ColPeach)
	g.stampText(x+1+w, 1, "│", ui.ColPurple)
	g.stampText(x, 2, "╰"+strings.Repeat("─", w)+"╯", ui.ColPurple)
}

// drawHotfixFlash zeigt nach einem Long-Press kurz ein großes Banner.
func (m *ambientModule) drawHotfixFlash(g *grid) {
	msg := fmt.Sprintf("★  HOTFIX #%d SHIPPED  ★", m.hotfixCount)
	w := len([]rune(msg))
	y := g.h / 4
	x := centerX(g.w, msg)
	g.stampText(x-2, y-1, "╔"+strings.Repeat("═", w+2)+"╗", ui.ColError)
	g.stampText(x-2, y, "║ ", ui.ColError)
	g.stampText(x, y, msg, ui.ColPeach)
	g.stampText(x+w, y, " ║", ui.ColError)
	g.stampText(x-2, y+1, "╚"+strings.Repeat("═", w+2)+"╝", ui.ColError)
}

// drawVolumeOverlay zeichnet die große Kiosk-Lautstärkeanzeige (raumtauglich).
func (m *ambientModule) drawVolumeOverlay(g *grid) {
	_, _, vol := m.player.GetStatus()

	barW := m.width / 2
	if barW > 48 {
		barW = 48
	}
	if barW < 10 {
		barW = 10
	}
	filled := int(vol*float64(barW) + 0.5)

	label := fmt.Sprintf("volume %3.0f%%", vol*100)
	y := m.height * 3 / 4
	g.stampText(centerX(m.width, label), y-1, label, ui.ColPeach)

	x := (m.width - barW) / 2
	for i := 0; i < barW; i++ {
		ch, col := '░', ui.ColFaint
		if i < filled {
			ch, col = '█', ui.ColPeach
		}
		g.set(x+i, y, ch, col)
	}
}

func (m *ambientModule) advance() {
	m.scenes[m.style].advance(m.width, m.height, m.rng)
}

func (m *ambientModule) View(width, height int) string {
	m.width, m.height = width, height
	if m.editingHotfix {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, m.hotfixEditorView())
	}
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
	if m.player != nil && time.Now().Before(m.volFlashUntil) {
		m.drawVolumeOverlay(g)
	}
	if m.kiosk {
		m.drawHotfixBox(g)
		if time.Now().Before(m.hotfixFlashUntil) {
			m.drawHotfixFlash(g)
		}
	}

	scene := m.scenes[m.style].name()
	if m.autoRotate {
		scene += " ↻"
	}
	hint := "space: scene · r: rotate · c: clock · w: location · ?: help · esc   (" +
		scene + ")"
	g.stampText(centerX(width, hint), height-1, hint, ui.ColDim)

	return g.render()
}

func (m *ambientModule) hotfixEditorView() string {
	prompt := ui.LabelStyle.Render("hotfix counter")
	input := lipgloss.NewStyle().Width(38).Render(m.editInput.View())
	hint := ui.HelpStyle.Render("enter: save   ·   esc: cancel")
	card := ui.CardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, prompt, "", input))
	return lipgloss.JoinVertical(lipgloss.Center, card, "", hint)
}

func (m *ambientModule) locationEditorView() string {
	prompt := ui.LabelStyle.Render("weather location")
	input := lipgloss.NewStyle().Width(38).Render(m.editInput.View())
	hint := ui.HelpStyle.Render("enter: save   ·   empty = auto by IP   ·   esc: cancel")
	card := ui.CardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, prompt, "", input))
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
						g.set(ox+(baseX+c)*sx+dx, oy+r*sy+dy, '█', ui.ColPeach)
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
	lines := []infoLine{{now.Format(dateFmt), ui.ColDim}}
	if m.weatherLine != "" {
		lines = append(lines, infoLine{m.weatherLine, ui.ColTeal})
	}
	np := m.nowPlaying()
	if np != "" {
		lines = append(lines, infoLine{np, ui.ColMauve})
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
	// Balken sitzen auf startX, startX+2, … startX+2(bars-1); sichtbare Breite
	// 2*bars-1 (keine Lücke am Ende). Mitte = startX+(bars-1) -> auf cx legen.
	startX := cx - (bars - 1)
	for b, lv := range levels {
		x := startX + b*2
		for r := 0; r < height; r++ {
			cellFromBottom := height - r
			if ch := ui.BarRune(lv, cellFromBottom, height); ch != ' ' {
				g.set(x, topY+r, ch, ui.EqRowColor(cellFromBottom, height))
			}
		}
	}
}

func (m *ambientModule) helpView() string {
	sections := []ui.HelpSection{
		{Title: "ambient", Rows: [][2]string{
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
	if m.kiosk {
		sections = append(sections, ui.HelpSection{Title: "kiosk buzzer", Rows: [][2]string{
			{"enter 1x", "volume up"},
			{"enter 2x", "volume down"},
			{"enter hold", "hotfix +1 (plays a sound)"},
			{"e", "edit hotfix counter"},
		}})
	}
	return ui.HelpOverlay("ambient · help", sections,
		"auto-screensaver after ~2 min idle · any key wakes")
}
