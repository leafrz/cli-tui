package main

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/leafrz/dashboard/radio"
)

// vizTickMsg treibt die Visualizer-Animation (~50ms).
type vizTickMsg time.Time

func vizTick() tea.Cmd {
	return tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg { return vizTickMsg(t) })
}

// visualizerModule rendert ein Echtzeit-Frequenzspektrum des laufenden Radios.
type visualizerModule struct {
	player        *radio.Player
	width, height int
	levels        []float64 // geglättete Bandwerte (fallende Balken)
	showHelp      bool
	ticking       bool
}

func newVisualizerModule(p *radio.Player) *visualizerModule {
	return &visualizerModule{player: p}
}

func (m *visualizerModule) Name() string   { return "visualizer" }
func (m *visualizerModule) Status() string { return "" }
func (m *visualizerModule) Init() tea.Cmd  { return nil }

func (m *visualizerModule) Update(msg tea.Msg) (Module, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case focusMsg:
		if !m.ticking {
			m.ticking = true
			return m, vizTick()
		}

	case vizTickMsg:
		m.sample()
		return m, vizTick()

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
			return m, nil
		}
	}
	return m, nil
}

// bandsForWidth bestimmt die Anzahl Balken aus der Breite (ein Balken = 2 Spalten).
func (m *visualizerModule) bandsForWidth() int {
	b := m.width / 2
	if b < 8 {
		b = 8
	}
	if b > 96 {
		b = 96
	}
	return b
}

// sample holt ein frisches Spektrum und glättet es (schneller Anstieg,
// langsames Abklingen -> klassische fallende Balken).
func (m *visualizerModule) sample() {
	bands := m.bandsForWidth()
	if len(m.levels) != bands {
		m.levels = make([]float64, bands)
	}
	spec := m.player.Spectrum(bands)
	if spec == nil {
		// nichts spielt -> sanft auf 0 abklingen
		for i := range m.levels {
			m.levels[i] *= 0.8
		}
		return
	}
	for i := 0; i < bands; i++ {
		if spec[i] > m.levels[i] {
			m.levels[i] = spec[i] // Attack: sofort
		} else {
			m.levels[i] = m.levels[i]*0.82 + spec[i]*0.18 // Decay
		}
	}
}

func (m *visualizerModule) View(width, height int) string {
	m.width, m.height = width, height

	if m.showHelp {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, m.helpView())
	}

	playing, _, _ := m.player.GetStatus()
	if !playing {
		hint := lipgloss.JoinVertical(lipgloss.Center,
			dimStyle.Render("nothing playing"),
			helpStyle.Render("start a station in the radio module first"),
			"",
			helpStyle.Render("esc: dashboard"),
		)
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, hint)
	}

	help := helpStyle.Render("esc: dashboard   ·   ?: help")
	rows := height - 2 // Platz für die Hilfezeile
	if rows < 3 {
		rows = 3
	}
	spectrum := renderSpectrum(m.levels, width, rows)

	return lipgloss.JoinVertical(lipgloss.Left,
		spectrum,
		lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(help),
	)
}

func (m *visualizerModule) helpView() string {
	sections := []helpSection{
		{title: "visualizer", rows: [][2]string{
			{"esc / q", "back to dashboard"},
			{"?", "toggle this help"},
		}},
	}
	return helpOverlay("visualizer · help", sections,
		"? or esc to close   ·   reacts to the radio's live audio")
}

// renderSpectrum zeichnet vertikale Balken (ein Band = 2 Spalten: Balken + Lücke),
// von unten wachsend, mit Theme-Verlauf (teal -> mauve -> peach nach Höhe).
func renderSpectrum(levels []float64, width, height int) string {
	bars := len(levels)
	if bars == 0 || width < 1 || height < 1 {
		return ""
	}

	grid := make([][]rune, height) // [row][col]
	colors := make([][]lipgloss.Color, height)
	for r := 0; r < height; r++ {
		grid[r] = make([]rune, width)
		colors[r] = make([]lipgloss.Color, width)
		for c := 0; c < width; c++ {
			grid[r][c] = ' '
		}
	}

	for b := 0; b < bars; b++ {
		col := b * 2
		if col >= width {
			break
		}
		barH := levels[b] * float64(height)
		for r := 0; r < height; r++ {
			cellFromBottom := height - r // 1..height
			lower := float64(cellFromBottom - 1)
			var ch rune = ' '
			if barH >= float64(cellFromBottom) {
				ch = '█'
			} else if barH > lower {
				frac := barH - lower
				idx := int(frac * float64(len(eqRunes)-1))
				if idx < 0 {
					idx = 0
				}
				if idx > len(eqRunes)-1 {
					idx = len(eqRunes) - 1
				}
				ch = eqRunes[idx]
			}
			grid[r][col] = ch
			colors[r][col] = eqRowColor(cellFromBottom, height)
		}
	}

	// Zeilen mit gefärbten Runs zusammenbauen.
	out := make([]string, height)
	for r := 0; r < height; r++ {
		var line string
		for c := 0; c < width; c++ {
			if grid[r][c] == ' ' {
				line += " "
				continue
			}
			line += lipgloss.NewStyle().Foreground(colors[r][c]).Render(string(grid[r][c]))
		}
		out[r] = line
	}
	return lipgloss.JoinVertical(lipgloss.Left, out...)
}
