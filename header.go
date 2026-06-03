package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Header-Modi
const (
	headerStatic  = "static"  // fester eigener Text
	headerRotate  = "rotate"  // rotiert durch Taglines
	headerMarquee = "marquee" // Lauftext (Ticker)
	headerContext = "context" // Live-Status des aktiven Moduls (scrollt)
)

// headerModes definiert die Reihenfolge beim Durchschalten.
var headerModes = []string{headerStatic, headerRotate, headerMarquee, headerContext}

// headerConfig ist der persistente Header-Zustand.
type headerConfig struct {
	Mode     string   `json:"mode"`
	Text     string   `json:"text"`
	Taglines []string `json:"taglines"`
}

func defaultHeaderConfig() headerConfig {
	return headerConfig{
		Mode: headerStatic,
		Text: "lofi.radio",
		Taglines: []string{
			"˗ˏˋ warm static & late nights ˎˊ˗",
			"˗ˏˋ tapes, rain & neon ˎˊ˗",
			"˗ˏˋ 3am study sessions ˎˊ˗",
			"˗ˏˋ slow mornings ˎˊ˗",
		},
	}
}

// withDefaults füllt fehlende Felder mit sinnvollen Defaults auf.
func (h headerConfig) withDefaults() headerConfig {
	d := defaultHeaderConfig()
	if h.Mode == "" {
		h.Mode = d.Mode
	}
	if h.Text == "" {
		h.Text = d.Text
	}
	if len(h.Taglines) == 0 {
		h.Taglines = d.Taglines
	}
	return h
}

// next schaltet den Modus weiter (static -> rotate -> marquee -> context -> …).
func (h headerConfig) next() headerConfig {
	cur := 0
	for i, m := range headerModes {
		if m == h.Mode {
			cur = i
			break
		}
	}
	h.Mode = headerModes[(cur+1)%len(headerModes)]
	return h
}

// animated meldet, ob der aktuelle Modus eine laufende Animation braucht
// (und damit der Header-Tick laufen muss).
func (h headerConfig) animated() bool {
	return h.Mode == headerRotate || h.Mode == headerMarquee || h.Mode == headerContext
}

// marquee scrollt text innerhalb von width Zeichen. frame treibt die Position.
// Bei kurzem Text wird einfach links ausgerichtet (kein Scrollen nötig).
func marquee(text string, width, frame int) string {
	if width < 1 {
		width = 1
	}
	r := []rune(text)
	if len(r) <= width {
		return text
	}
	// Endlos-Scroll mit Trenner.
	sep := []rune("   •   ")
	loop := append(append([]rune{}, r...), sep...)
	off := frame % len(loop)

	out := make([]rune, 0, width)
	for i := 0; i < width; i++ {
		out = append(out, loop[(off+i)%len(loop)])
	}
	return string(out)
}

// headerText liefert den anzuzeigenden Header-Text je nach Modus.
// frame = Header-Animationszähler, status = Live-Status des aktiven Moduls.
func headerText(h headerConfig, frame, width int, status string) string {
	switch h.Mode {
	case headerRotate:
		if len(h.Taglines) == 0 {
			return h.Text
		}
		// alle ~3s wechseln (Header-Tick ~200ms -> 15 Frames)
		idx := (frame / 15) % len(h.Taglines)
		return h.Taglines[idx]

	case headerMarquee:
		base := h.Text
		if len(h.Taglines) > 0 {
			base = h.Text + "   " + strings.Join(h.Taglines, "   •   ")
		}
		return marquee(base, width, frame)

	case headerContext:
		if status == "" {
			return h.Text
		}
		return marquee(status, width, frame)

	default: // static
		return h.Text
	}
}

// headerTitleStyle/headerTagStyle werden im Root verwendet.
var (
	headerTextStyle = lipgloss.NewStyle().Bold(true).Foreground(colMauve)
)
