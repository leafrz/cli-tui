package main

import (
	"strings"

	"github.com/leafrz/dashboard/internal/config"
)

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
func headerText(h config.HeaderConfig, frame, width int, status string) string {
	switch h.Mode {
	case config.HeaderRotate:
		if len(h.Taglines) == 0 {
			return h.Text
		}
		idx := (frame / 15) % len(h.Taglines)
		return h.Taglines[idx]

	case config.HeaderMarquee:
		base := h.Text
		if len(h.Taglines) > 0 {
			base = h.Text + "   " + strings.Join(h.Taglines, "   •   ")
		}
		return marquee(base, width, frame)

	case config.HeaderContext:
		if status == "" {
			return h.Text
		}
		return marquee(status, width, frame)

	default: // static
		return h.Text
	}
}
