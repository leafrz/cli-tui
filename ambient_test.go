package main

import (
	"strings"
	"testing"
)

// TestAmbientRender prüft, dass ein Frame die richtige Höhe hat und die Uhr malt.
func TestAmbientRender(t *testing.T) {
	const w, h = 80, 24
	m := newAmbientModule()
	m.width, m.height = w, h

	for _, style := range []int{saverStarfield, saverMatrix, saverBlank} {
		m.style = style
		for i := 0; i < 5; i++ {
			m.advance()
		}
		out := m.View(w, h)
		lines := strings.Split(out, "\n")
		if len(lines) != h {
			t.Errorf("style %d: expected %d lines, got %d", style, h, len(lines))
		}
		if !strings.ContainsRune(out, '█') {
			t.Errorf("style %d: expected clock blocks in output", style)
		}
	}
}
