package main

import (
	"strings"
	"testing"

	"github.com/leafrz/dashboard/radio"
)

// TestAmbientRender prüft, dass JEDE Szene einen Frame korrekter Höhe liefert
// und die Uhr malt (kein Out-of-bounds o.ä.).
func TestAmbientRender(t *testing.T) {
	const w, h = 80, 24
	m := newAmbientModule(radio.NewPlayer())
	m.width, m.height = w, h

	for style := 0; style < len(m.scenes); style++ {
		m.style = style
		for i := 0; i < 6; i++ {
			m.advance()
		}
		out := m.View(w, h)
		lines := strings.Split(out, "\n")
		if len(lines) != h {
			t.Errorf("scene %q: expected %d lines, got %d", m.scenes[style].name(), h, len(lines))
		}
		if !strings.ContainsRune(out, '█') {
			t.Errorf("scene %q: expected clock blocks in output", m.scenes[style].name())
		}
	}
}
