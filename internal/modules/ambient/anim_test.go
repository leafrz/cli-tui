package ambient

import (
	"testing"

	"github.com/leafrz/dashboard/internal/core"
)

// TestAmbientAnimates prüft, dass nach Focus die Tick-Kette den Frame über
// mehrere Ticks hinweg erhöht (Animation läuft).
func TestAmbientAnimates(t *testing.T) {
	am := New(nil)
	am.width, am.height = 80, 24

	// Focus startet den Ticker.
	if _, cmd := am.Update(core.FocusMsg{}); cmd == nil {
		t.Fatal("Focus startete keinen Ticker")
	}

	before := am.frame
	for i := 0; i < 5; i++ {
		if _, cmd := am.Update(ambientTickMsg{gen: am.gen}); cmd == nil {
			t.Fatal("Tick armiert nicht neu")
		}
	}
	if am.frame <= before {
		t.Errorf("frame nicht erhöht: %d -> %d", before, am.frame)
	}
}

// TestAmbientRefocusRestartsStaleTick prüft die Regression: ein Fokus-Verlust
// ohne den "esc"-Key (z.B. Idle-Wakeup, der core.FocusMsg direkt weiterreicht,
// ohne ambient.Update mit dem Wake-Key aufzurufen) darf die Tick-Kette beim
// nächsten Focus nicht für immer eingefroren lassen.
func TestAmbientRefocusRestartsStaleTick(t *testing.T) {
	am := New(nil)
	am.width, am.height = 80, 24

	am.Update(core.FocusMsg{})
	staleGen := am.gen

	// Modul wird deaktiviert, OHNE dass "esc"/"q"/"backspace" je ambient.Update
	// erreicht (z.B. weil das Dashboard beim Idle-Wakeup direkt umschaltet).
	// Erneuter Focus (zweiter Aktivierungszyklus) muss trotzdem wieder ticken.
	if _, cmd := am.Update(core.FocusMsg{}); cmd == nil {
		t.Fatal("erneuter Focus startete keinen Ticker")
	}
	if am.gen == staleGen {
		t.Fatal("Generation wurde beim erneuten Focus nicht erhöht")
	}

	// Ein Tick aus dem alten (veralteten) Loop darf nichts mehr bewirken.
	before := am.frame
	am.Update(ambientTickMsg{gen: staleGen})
	if am.frame != before {
		t.Errorf("veralteter Tick hat frame trotzdem erhöht: %d -> %d", before, am.frame)
	}

	// Ein Tick aus dem aktuellen Loop muss weiter animieren.
	if _, cmd := am.Update(ambientTickMsg{gen: am.gen}); cmd == nil {
		t.Fatal("aktueller Tick armierte nicht neu")
	}
	if am.frame <= before {
		t.Errorf("frame nach aktuellem Tick nicht erhöht: %d -> %d", before, am.frame)
	}
}
