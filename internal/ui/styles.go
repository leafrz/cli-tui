package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// --- Help rendering --------------------------------------------------------

// HelpSection ist eine betitelte Gruppe von Tastenkürzeln.
type HelpSection struct {
	Title string
	Rows  [][2]string // {key, beschreibung}
}

// HelpOverlay rendert ein einheitlich formatiertes, zentriertes Hilfe-Panel.
// Die Key-Spalte wird über alle Zeilen hinweg bündig ausgerichtet.
func HelpOverlay(title string, sections []HelpSection, footer string) string {
	keyStyle := lipgloss.NewStyle().Foreground(ColPeach)
	descStyle := lipgloss.NewStyle().Foreground(ColCream)
	headStyle := lipgloss.NewStyle().Foreground(ColMauve).Bold(true)
	titleStyle := lipgloss.NewStyle().Foreground(ColPeach).Bold(true)

	// Breite der Key-Spalte über alle Zeilen bestimmen.
	maxK := 0
	for _, s := range sections {
		for _, r := range s.Rows {
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
		lines = append(lines, headStyle.Render(s.Title))
		for _, r := range s.Rows {
			key := keyStyle.Render(fmt.Sprintf("%-*s", maxK, r[0]))
			lines = append(lines, "  "+key+"   "+descStyle.Render(r[1]))
		}
	}

	card := CardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
	hint := HelpStyle.Render(footer)
	return lipgloss.JoinVertical(lipgloss.Center, card, "", hint)
}

// --- Lo-fi palette ---------------------------------------------------------
// Warm, dusty, low-key. Nothing saturated; everything sits slightly faded,
// like a worn cassette label.
var (
	ColCream  = lipgloss.Color("#e6ddcf") // primary text
	ColMauve  = lipgloss.Color("#c4a7b5") // accent / titles
	ColPurple = lipgloss.Color("#9a8c98") // borders / secondary accent
	ColTeal   = lipgloss.Color("#88a09e") // status / cool accent
	ColPeach  = lipgloss.Color("#dcae9a") // highlight / warm accent
	ColDim    = lipgloss.Color("#7a736b") // muted text / help
	ColFaint  = lipgloss.Color("#48433d") // empty track fill
	ColError  = lipgloss.Color("#c98a8a") // muted red
	ColGood   = lipgloss.Color("#a3b18a") // muted green
)

// --- Shared styles ---------------------------------------------------------
var (
	ClockStyle = lipgloss.NewStyle().Foreground(ColDim)
	HelpStyle  = lipgloss.NewStyle().Foreground(ColDim)
	DimStyle   = lipgloss.NewStyle().Foreground(ColDim)
	LabelStyle = lipgloss.NewStyle().Foreground(ColPurple)

	CardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColPurple).
			Padding(1, 4)

	StationNameStyle = lipgloss.NewStyle().Bold(true).Foreground(ColPeach)
	NowPlayingStyle  = lipgloss.NewStyle().Foreground(ColCream).Italic(true)

	// HeaderTextStyle wird vom Dashboard-Header genutzt; in rebuildStyles gesetzt.
	HeaderTextStyle = lipgloss.NewStyle().Bold(true).Foreground(ColMauve)

	ruleStyle = lipgloss.NewStyle().Foreground(ColFaint)
)

// HorizontalRule renders a faint full-width separator line.
func HorizontalRule(width int) string {
	if width < 1 {
		width = 1
	}
	return ruleStyle.Render(strings.Repeat("─", width))
}

// --- Volume bar ------------------------------------------------------------
// A smooth gradient bar using fine block characters.
func RenderVolumeBar(vol float64, width int) string {
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
				c = ColTeal
			case ratio < 0.75:
				c = ColMauve
			default:
				c = ColPeach
			}
			b.WriteString(lipgloss.NewStyle().Foreground(c).Render("█"))
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(ColFaint).Render("░"))
		}
	}
	return b.String()
}

// --- Animated equalizer ----------------------------------------------------
// We have no real FFT data, so we synthesize a plausible-looking spectrum
// from layered sine waves driven by an animation frame counter. Paused state
// shows a flat low baseline.
var eqRunes = []rune(" ▁▂▃▄▅▆▇█")

func EqRowColor(cellFromBottom, height int) lipgloss.Color {
	ratio := float64(cellFromBottom) / float64(height)
	switch {
	case ratio < 0.4:
		return ColTeal
	case ratio < 0.75:
		return ColMauve
	default:
		return ColPeach
	}
}

// BarRune liefert das passende Block-Zeichen für einen Balken der Höhe level
// (0..1) in der gegebenen Zelle (von unten gezählt).
func BarRune(level float64, cellFromBottom, height int) rune {
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

// RenderBars zeichnet kompakte Balken (ein Band = 1 Spalte + Lücke) über height
// Zeilen, eingefärbt nach Höhe. Für echte Spektrum-Pegel (0..1).
//
// WICHTIG: jede Zeile behält die volle Breite (kein TrimRight). Sonst wären die
// Zeilen unterschiedlich breit und würden beim Zentrieren versetzt ausgerichtet
// -> verrutschte Balken.
func RenderBars(levels []float64, height int) string {
	if height < 1 {
		height = 1
	}
	if len(levels) == 0 {
		return strings.Repeat("\n", height-1) // stabile Höhe, leer
	}
	rows := make([]string, height)
	for r := 0; r < height; r++ {
		cellFromBottom := height - r
		style := lipgloss.NewStyle().Foreground(EqRowColor(cellFromBottom, height))
		var line strings.Builder
		for i, lv := range levels {
			if i > 0 {
				line.WriteByte(' ') // Lücke zwischen Balken
			}
			if ch := BarRune(lv, cellFromBottom, height); ch == ' ' {
				line.WriteByte(' ')
			} else {
				line.WriteString(style.Render(string(ch)))
			}
		}
		rows[r] = line.String()
	}
	return strings.Join(rows, "\n")
}

// RenderSpectrum zeichnet ein bildschirmfüllendes Spektrum (ein Band = 2 Spalten).
func RenderSpectrum(levels []float64, width, height int) string {
	if len(levels) == 0 || width < 1 || height < 1 {
		return ""
	}
	out := make([]string, height)
	for r := 0; r < height; r++ {
		cellFromBottom := height - r
		style := lipgloss.NewStyle().Foreground(EqRowColor(cellFromBottom, height))
		var line strings.Builder
		col := 0
		for _, lv := range levels {
			if col >= width {
				break
			}
			if ch := BarRune(lv, cellFromBottom, height); ch == ' ' {
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
