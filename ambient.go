package main

import (
	"math/rand"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// --- cell grid (für Overlay von Uhr über Animation) ------------------------

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

// --- messages --------------------------------------------------------------

type ambientTickMsg time.Time

func ambientTick() tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(t time.Time) tea.Msg { return ambientTickMsg(t) })
}

// --- module ----------------------------------------------------------------

const (
	saverStarfield = iota
	saverMatrix
	saverBlank
	saverCount
)

var saverNames = []string{"starfield", "matrix", "blank"}

type star struct{ x, y, z float64 }

type ambientModule struct {
	width, height int
	frame         int
	style         int
	showClock     bool
	showHelp      bool
	ticking       bool

	rng *rand.Rand

	// starfield
	stars        []star
	lastW, lastH int

	// matrix
	drops []float64 // Kopf-Position je Spalte
}

func newAmbientModule() *ambientModule {
	return &ambientModule{
		rng:       rand.New(rand.NewSource(time.Now().UnixNano())),
		showClock: true,
	}
}

func (m *ambientModule) Name() string   { return "ambient" }
func (m *ambientModule) Status() string { return "" }
func (m *ambientModule) Init() tea.Cmd  { return nil }

func (m *ambientModule) Update(msg tea.Msg) (Module, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case focusMsg:
		if !m.ticking {
			m.ticking = true
			return m, ambientTick()
		}

	case ambientTickMsg:
		m.frame++
		m.advance()
		return m, ambientTick()

	case tea.KeyMsg:
		k := msg.String()
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
			m.style = (m.style + 1) % saverCount
		case "c":
			m.showClock = !m.showClock
		}
	}
	return m, nil
}

func (m *ambientModule) View(width, height int) string {
	m.width, m.height = width, height
	if m.showHelp {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, m.helpView())
	}
	if width < 4 || height < 4 {
		return ""
	}

	g := newGrid(width, height)

	switch m.style {
	case saverStarfield:
		m.drawStars(g)
	case saverMatrix:
		m.drawMatrix(g)
	}

	if m.showClock {
		m.drawClock(g)
	}

	// dezente Hilfezeile unten.
	hint := "space: next scene · c: clock · ?: help · esc: dashboard   (" + saverNames[m.style] + ")"
	g.stampText(centerX(width, hint), height-1, hint, colDim)

	return g.render()
}

// advance schreitet die Animation genau einmal pro Tick voran.
func (m *ambientModule) advance() {
	switch m.style {
	case saverStarfield:
		m.advanceStars()
	case saverMatrix:
		m.advanceMatrix()
	}
}

// centerX liefert die linke Startspalte, um s (nach Runen) zu zentrieren.
func centerX(width int, s string) int {
	x := (width - len([]rune(s))) / 2
	if x < 0 {
		x = 0
	}
	return x
}

// --- starfield -------------------------------------------------------------

func (m *ambientModule) ensureStars() {
	if m.width == m.lastW && m.height == m.lastH && m.stars != nil {
		return
	}
	m.lastW, m.lastH = m.width, m.height
	n := m.width * m.height / 14
	if n < 30 {
		n = 30
	}
	m.stars = make([]star, n)
	for i := range m.stars {
		m.stars[i] = star{m.rng.Float64()*2 - 1, m.rng.Float64()*2 - 1, m.rng.Float64()*0.9 + 0.1}
	}
}

func (m *ambientModule) advanceStars() {
	m.ensureStars()
	for i := range m.stars {
		s := &m.stars[i]
		s.z -= 0.012
		if s.z <= 0.02 {
			s.x = m.rng.Float64()*2 - 1
			s.y = m.rng.Float64()*2 - 1
			s.z = 1
		}
	}
}

func (m *ambientModule) drawStars(g *grid) {
	m.ensureStars()
	cx, cy := float64(m.width)/2, float64(m.height)/2
	for i := range m.stars {
		s := m.stars[i]
		sx := int(s.x/s.z*cx + cx)
		sy := int(s.y/s.z*cy + cy)
		var ch rune
		var col lipgloss.Color
		switch {
		case s.z < 0.35:
			ch, col = '@', colCream
		case s.z < 0.7:
			ch, col = '*', colMauve
		default:
			ch, col = '.', colDim
		}
		g.set(sx, sy, ch, col)
	}
}

// --- matrix rain -----------------------------------------------------------

var matrixRunes = []rune("ABCDEFGHJKLMNPQRSTUVWXYZ0123456789@#$%&*+=<>?/")

func (m *ambientModule) ensureDrops() {
	if len(m.drops) != m.width {
		m.drops = make([]float64, m.width)
		for x := range m.drops {
			m.drops[x] = m.rng.Float64() * float64(m.height)
		}
	}
}

func (m *ambientModule) advanceMatrix() {
	m.ensureDrops()
	const trail = 10
	for x := 0; x < m.width; x++ {
		m.drops[x] += 0.5 + float64(x%3)*0.25
		if int(m.drops[x])-trail > m.height {
			m.drops[x] = 0
		}
	}
}

func (m *ambientModule) drawMatrix(g *grid) {
	m.ensureDrops()
	const trail = 10
	for x := 0; x < m.width && x < len(m.drops); x++ {
		head := int(m.drops[x])
		for t := 0; t < trail; t++ {
			y := head - t
			if y < 0 || y >= m.height {
				continue
			}
			r := matrixRunes[m.rng.Intn(len(matrixRunes))]
			var col lipgloss.Color
			switch {
			case t == 0:
				col = colCream // heller Kopf
			case t < 3:
				col = colGood
			case t < 6:
				col = colTeal
			default:
				col = colFaint
			}
			g.set(x, y, r, col)
		}
	}
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

	const gw, gh = 4, 5 // Glyph-Basisgröße
	const gap = 1
	const sx, sy = 2, 2 // Skalierung (x, y)

	// Gesamtbreite in Basis-Spalten.
	baseW := len(text)*(gw+gap) - gap
	totalW := baseW * sx
	totalH := gh * sy

	ox := (m.width - totalW) / 2
	oy := (m.height-totalH)/2 - 1

	col := colPeach
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
				// skaliert stampen
				for dy := 0; dy < sy; dy++ {
					for dx := 0; dx < sx; dx++ {
						g.set(ox+(baseX+c)*sx+dx, oy+r*sy+dy, '█', col)
					}
				}
			}
		}
	}

	// Datum/Sekunden darunter, normaler Text.
	sub := now.Format("Mon 02 Jan · 15:04:05")
	g.stampText(centerX(m.width, sub), oy+totalH+1, sub, colDim)
}

func (m *ambientModule) helpView() string {
	sections := []helpSection{
		{title: "ambient", rows: [][2]string{
			{"space / s", "next scene (starfield / matrix / blank)"},
			{"c", "toggle clock"},
			{"esc / q", "back to dashboard"},
			{"?", "toggle this help"},
		}},
	}
	return helpOverlay("ambient · help", sections,
		"? or esc to close   ·   global commands on the dashboard")
}
