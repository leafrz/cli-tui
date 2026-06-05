package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// --- Help rendering --------------------------------------------------------

// helpSection ist eine betitelte Gruppe von Tastenkürzeln.
type helpSection struct {
	title string
	rows  [][2]string // {key, beschreibung}
}

// helpOverlay rendert ein einheitlich formatiertes, zentriertes Hilfe-Panel.
// Die Key-Spalte wird über alle Zeilen hinweg bündig ausgerichtet.
func helpOverlay(title string, sections []helpSection, footer string) string {
	keyStyle := lipgloss.NewStyle().Foreground(colPeach)
	descStyle := lipgloss.NewStyle().Foreground(colCream)
	headStyle := lipgloss.NewStyle().Foreground(colMauve).Bold(true)
	titleStyle := lipgloss.NewStyle().Foreground(colPeach).Bold(true)

	// Breite der Key-Spalte über alle Zeilen bestimmen.
	maxK := 0
	for _, s := range sections {
		for _, r := range s.rows {
			if w := lipgloss.Width(r[0]); w > maxK {
				maxK = w
			}
		}
	}

	lines := []string{titleStyle.Render(title), ""}
	for i, s := range sections {
		if i > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, headStyle.Render(s.title))
		for _, r := range s.rows {
			key := keyStyle.Render(fmt.Sprintf("%-*s", maxK, r[0]))
			lines = append(lines, "  "+key+"   "+descStyle.Render(r[1]))
		}
	}

	card := cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	hint := helpStyle.Render(footer)
	return lipgloss.JoinVertical(lipgloss.Center, card, "", hint)
}

// --- Lo-fi palette ---------------------------------------------------------
// Warm, dusty, low-key. Nothing saturated; everything sits slightly faded,
// like a worn cassette label.
var (
	colCream  = lipgloss.Color("#e6ddcf") // primary text
	colMauve  = lipgloss.Color("#c4a7b5") // accent / titles
	colPurple = lipgloss.Color("#9a8c98") // borders / secondary accent
	colTeal   = lipgloss.Color("#88a09e") // status / cool accent
	colPeach  = lipgloss.Color("#dcae9a") // highlight / warm accent
	colDim    = lipgloss.Color("#7a736b") // muted text / help
	colFaint  = lipgloss.Color("#48433d") // empty track fill
	colError  = lipgloss.Color("#c98a8a") // muted red
	colGood   = lipgloss.Color("#a3b18a") // muted green
)

// --- Shared styles ---------------------------------------------------------
var (
	clockStyle = lipgloss.NewStyle().Foreground(colDim)
	helpStyle  = lipgloss.NewStyle().Foreground(colDim)
	dimStyle   = lipgloss.NewStyle().Foreground(colDim)
	labelStyle = lipgloss.NewStyle().Foreground(colPurple)

	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colPurple).
			Padding(1, 4)

	stationNameStyle = lipgloss.NewStyle().Bold(true).Foreground(colPeach)
	nowPlayingStyle  = lipgloss.NewStyle().Foreground(colCream).Italic(true)

	ruleStyle = lipgloss.NewStyle().Foreground(colFaint)
)

// horizontalRule renders a faint full-width separator line.
func horizontalRule(width int) string {
	if width < 1 {
		width = 1
	}
	return ruleStyle.Render(strings.Repeat("─", width))
}

// --- Volume bar ------------------------------------------------------------
// A smooth gradient bar using fine block characters.
func renderVolumeBar(vol float64, width int) string {
	if width < 1 {
		width = 1
	}
	if vol < 0 {
		vol = 0
	}
	if vol > 1 {
		vol = 1
	}

	filled := int(vol * float64(width))
	if filled > width {
		filled = width
	}

	var b strings.Builder
	for i := 0; i < width; i++ {
		if i < filled {
			// warm gradient: teal -> mauve -> peach as the bar fills
			ratio := float64(i) / float64(width)
			var c lipgloss.Color
			switch {
			case ratio < 0.4:
				c = colTeal
			case ratio < 0.75:
				c = colMauve
			default:
				c = colPeach
			}
			b.WriteString(lipgloss.NewStyle().Foreground(c).Render("█"))
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(colFaint).Render("░"))
		}
	}
	return b.String()
}

// --- Animated equalizer ----------------------------------------------------
// We have no real FFT data, so we synthesize a plausible-looking spectrum
// from layered sine waves driven by an animation frame counter. Paused state
// shows a flat low baseline.
var eqRunes = []rune(" ▁▂▃▄▅▆▇█")

func eqRowColor(cellFromBottom, height int) lipgloss.Color {
	ratio := float64(cellFromBottom) / float64(height)
	switch {
	case ratio < 0.4:
		return colTeal
	case ratio < 0.75:
		return colMauve
	default:
		return colPeach
	}
}

// barRune liefert das passende Block-Zeichen für einen Balken der Höhe level
// (0..1) in der gegebenen Zelle (von unten gezählt).
func barRune(level float64, cellFromBottom, height int) rune {
	barH := level * float64(height)
	lower := float64(cellFromBottom - 1)
	if barH >= float64(cellFromBottom) {
		return '█'
	}
	if barH > lower {
		idx := int((barH - lower) * float64(len(eqRunes)-1))
		if idx < 0 {
			idx = 0
		}
		if idx > len(eqRunes)-1 {
			idx = len(eqRunes) - 1
		}
		return eqRunes[idx]
	}
	return ' '
}

// renderBars zeichnet kompakte Balken (ein Band = 1 Spalte + Lücke) über height
// Zeilen, eingefärbt nach Höhe. Für echte Spektrum-Pegel (0..1).
func renderBars(levels []float64, height int) string {
	if height < 1 {
		height = 1
	}
	if len(levels) == 0 {
		return strings.Repeat("\n", height-1) // stabile Höhe, leer
	}
	rows := make([]string, height)
	for r := 0; r < height; r++ {
		cellFromBottom := height - r
		style := lipgloss.NewStyle().Foreground(eqRowColor(cellFromBottom, height))
		var line strings.Builder
		for _, lv := range levels {
			if ch := barRune(lv, cellFromBottom, height); ch == ' ' {
				line.WriteString("  ")
			} else {
				line.WriteString(style.Render(string(ch)) + " ")
			}
		}
		rows[r] = strings.TrimRight(line.String(), " ")
	}
	return strings.Join(rows, "\n")
}

// renderSpectrum zeichnet ein bildschirmfüllendes Spektrum (ein Band = 2 Spalten).
func renderSpectrum(levels []float64, width, height int) string {
	if len(levels) == 0 || width < 1 || height < 1 {
		return ""
	}
	out := make([]string, height)
	for r := 0; r < height; r++ {
		cellFromBottom := height - r
		style := lipgloss.NewStyle().Foreground(eqRowColor(cellFromBottom, height))
		var line strings.Builder
		col := 0
		for _, lv := range levels {
			if col >= width {
				break
			}
			if ch := barRune(lv, cellFromBottom, height); ch == ' ' {
				line.WriteString("  ")
			} else {
				line.WriteString(style.Render(string(ch)) + " ")
			}
			col += 2
		}
		out[r] = line.String()
	}
	return strings.Join(out, "\n")
}
