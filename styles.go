package main

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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

func renderEQ(frame, bars, height int, paused bool) string {
	levels := make([]float64, bars)
	for i := range levels {
		if paused {
			levels[i] = 0.08
			continue
		}
		f := float64(frame)
		v := 0.5 + 0.5*math.Sin(f*0.25+float64(i)*0.6)
		v *= 0.55 + 0.45*math.Abs(math.Sin(f*0.11+float64(i)*0.9))
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		levels[i] = v
	}

	rows := make([]string, height)
	for r := 0; r < height; r++ {
		cellFromBottom := height - r // 1 (bottom) .. height (top)
		color := eqRowColor(cellFromBottom, height)
		cellStyle := lipgloss.NewStyle().Foreground(color)

		var line strings.Builder
		for i := 0; i < bars; i++ {
			barH := levels[i] * float64(height) // in cells
			lower := float64(cellFromBottom - 1)

			ch := ' '
			if barH >= float64(cellFromBottom) {
				ch = '█'
			} else if barH > lower {
				frac := barH - lower // 0..1 within this cell
				idx := int(frac * float64(len(eqRunes)-1))
				if idx < 0 {
					idx = 0
				}
				if idx > len(eqRunes)-1 {
					idx = len(eqRunes) - 1
				}
				ch = eqRunes[idx]
			}
			line.WriteString(cellStyle.Render(string(ch)))
			line.WriteByte(' ')
		}
		rows[r] = strings.TrimRight(line.String(), " ")
	}
	return strings.Join(rows, "\n")
}
