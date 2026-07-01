package ambient

import (
	"math"
	"testing"
	"time"

	"github.com/leafrz/dashboard/internal/audio"
	"github.com/leafrz/dashboard/internal/core"
)

// vol liest die aktuelle Player-Lautstärke.
func vol(m *ambientModule) float64 { _, _, v := m.player.GetStatus(); return v }

func newKiosk(t *testing.T) *ambientModule {
	t.Helper()
	m := New(audio.NewPlayer())
	m.Update(core.AutostartMsg{})
	if !m.kiosk {
		t.Fatal("AutostartMsg setzte kiosk nicht")
	}
	return m
}

// TestBuzzerSinglePressVolumeUp: 1x Enter -> nach Ruhefenster +10%.
func TestBuzzerSinglePressVolumeUp(t *testing.T) {
	m := newKiosk(t)
	m.player.SetVolume(0.5)

	t0 := time.Now()
	if cmd := m.handleEnter(t0); cmd == nil {
		t.Fatal("handleEnter lieferte keinen Resolve-Tick")
	}
	m.resolveEnter(t0.Add(700 * time.Millisecond))

	if got := vol(m); math.Abs(got-0.6) > 1e-9 {
		t.Fatalf("vol = %v, want 0.6", got)
	}
}

// TestBuzzerDoublePressVolumeDown: 2x Enter im Fenster -> -10%.
func TestBuzzerDoublePressVolumeDown(t *testing.T) {
	m := newKiosk(t)
	m.player.SetVolume(0.5)

	t0 := time.Now()
	m.handleEnter(t0)
	m.handleEnter(t0.Add(300 * time.Millisecond))  // zweiter Druck (>150ms = kein Repeat)
	m.resolveEnter(t0.Add(500 * time.Millisecond)) // zu früh -> darf nichts tun
	if got := vol(m); math.Abs(got-0.5) > 1e-9 {
		t.Fatalf("zu früher Resolve änderte vol: %v", got)
	}
	m.resolveEnter(t0.Add(1100 * time.Millisecond))

	if got := vol(m); math.Abs(got-0.4) > 1e-9 {
		t.Fatalf("vol = %v, want 0.4", got)
	}
}

// TestBuzzerLongPressHotfix: Key-Repeat-Burst -> Hotfix +1 (genau einmal),
// keine Volume-Änderung.
func TestBuzzerLongPressHotfix(t *testing.T) {
	m := newKiosk(t)
	m.player.SetVolume(0.5)
	m.hotfixCount = 41

	// Gehaltener Buzzer: Initial-Druck, ~500ms Repeat-Verzögerung, dann 30ms-Takt.
	t0 := time.Now()
	m.handleEnter(t0)
	for i := 0; i < 8; i++ {
		m.handleEnter(t0.Add(500*time.Millisecond + time.Duration(i)*30*time.Millisecond))
	}
	if m.hotfixCount != 42 {
		t.Fatalf("Long-Press: hotfixCount = %d, want 42 (genau +1)", m.hotfixCount)
	}
	if !m.longFired {
		t.Fatal("longFired nicht gesetzt")
	}

	// Burst-Ende: Resolve darf die Lautstärke NICHT anfassen.
	m.resolveEnter(t0.Add(2 * time.Second))
	if got := vol(m); math.Abs(got-0.5) > 1e-9 {
		t.Fatalf("Long-Press änderte vol: %v, want 0.5", got)
	}
	if m.longFired {
		t.Fatal("longFired wurde nach Resolve nicht zurückgesetzt")
	}
}

// TestBuzzerAfterLongPressSingleWorks: nach einem Burst + Pause funktioniert
// ein normaler Einzeldruck wieder.
func TestBuzzerAfterLongPressSingleWorks(t *testing.T) {
	m := newKiosk(t)
	m.player.SetVolume(0.5)

	t0 := time.Now()
	m.handleEnter(t0)
	for i := 0; i < 5; i++ {
		m.handleEnter(t0.Add(500*time.Millisecond + time.Duration(i)*30*time.Millisecond))
	}
	m.resolveEnter(t0.Add(2 * time.Second))

	// 3s später: normaler Einzeldruck.
	t1 := t0.Add(3 * time.Second)
	m.handleEnter(t1)
	m.resolveEnter(t1.Add(700 * time.Millisecond))
	if got := vol(m); math.Abs(got-0.6) > 1e-9 {
		t.Fatalf("Einzeldruck nach Burst: vol = %v, want 0.6", got)
	}
}
