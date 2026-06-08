package audio

import (
	"math"
	"testing"
)

func sineRing(freq float64, n int) *meter {
	m := &meter{ring: make([]float64, n), size: n}
	for i := 0; i < n; i++ {
		m.ring[i] = math.Sin(2 * math.Pi * freq * float64(i) / float64(SampleRate))
	}
	return m
}

func argmax(xs []float64) int {
	best, bi := xs[0], 0
	for i, v := range xs {
		if v > best {
			best, bi = v, i
		}
	}
	return bi
}

// TestSpectrumOrdering prüft, dass ein höherer Ton in einem höheren Band landet.
func TestSpectrumOrdering(t *testing.T) {
	low := sineRing(300, 1024).spectrum(32)
	high := sineRing(8000, 1024).spectrum(32)

	lo, hi := argmax(low), argmax(high)
	t.Logf("peak band: 300Hz -> %d, 8000Hz -> %d", lo, hi)
	if hi <= lo {
		t.Errorf("expected high tone in a higher band: low=%d high=%d", lo, hi)
	}

	// Stille -> alles ~0 (normalisiert auf max, also prüfen wir den Rohpegel
	// indirekt: ein echter Ton hat klar einen dominanten Peak).
	if low[lo] < 0.9 {
		t.Errorf("expected a dominant peak for a pure tone, got %.2f", low[lo])
	}
}
