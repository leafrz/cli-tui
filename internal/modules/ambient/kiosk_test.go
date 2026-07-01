package ambient

import (
	"math"
	"testing"

	"github.com/leafrz/dashboard/internal/audio"
	"github.com/leafrz/dashboard/internal/core"
)

// TestKioskVolumeCycle prüft den Enter-Buzzer: 20%-Schritte, Wrap nach 100%,
// krumme Werte rasten auf den nächsten Schritt ein.
func TestKioskVolumeCycle(t *testing.T) {
	m := New(audio.NewPlayer())
	m.Update(core.AutostartMsg{})
	if !m.kiosk {
		t.Fatal("AutostartMsg setzte kiosk nicht")
	}

	vol := func() float64 { _, _, v := m.player.GetStatus(); return v }
	step := func() float64 { m.cycleVolume(); return vol() }

	m.player.SetVolume(0)
	want := []float64{0.2, 0.4, 0.6, 0.8, 1.0, 0} // voller Zyklus inkl. Wrap
	for i, w := range want {
		if got := step(); math.Abs(got-w) > 1e-9 {
			t.Fatalf("Schritt %d: vol = %v, want %v", i+1, got, w)
		}
	}

	// Krummer Wert (55%) rastet auf 60% ein.
	m.player.SetVolume(0.55)
	if got := step(); math.Abs(got-0.6) > 1e-9 {
		t.Fatalf("von 0.55: vol = %v, want 0.6", got)
	}

	// Flash-Fenster wurde gesetzt.
	if m.volFlashUntil.IsZero() {
		t.Fatal("cycleVolume setzte volFlashUntil nicht")
	}
}
