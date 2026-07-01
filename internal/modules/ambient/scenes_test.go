package ambient

import (
	"math/rand"
	"testing"
)

// TestScenesSmoke stellt sicher, dass jede Scene mit eindeutigem Namen
// registriert ist und advance/draw auf verschiedenen Gittergrößen nicht
// paniken (Grenzfälle: winzig, breit, hoch).
func TestScenesSmoke(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	sizes := [][2]int{{60, 20}, {3, 3}, {120, 8}, {8, 40}}

	seen := map[string]bool{}
	for _, sc := range buildScenes() {
		n := sc.name()
		if seen[n] {
			t.Fatalf("doppelter Scene-Name %q", n)
		}
		seen[n] = true

		for _, wh := range sizes {
			g := newGrid(wh[0], wh[1])
			for i := 0; i < 15; i++ {
				sc.advance(g.w, g.h, rng)
			}
			_ = sc.draw // draw rendert rein ins Grid
			sc.draw(g, rng)
			_ = g.render()
		}
	}

	if len(seen) < 21 {
		t.Fatalf("erwartete >=21 Scenes, bekam %d", len(seen))
	}
}
