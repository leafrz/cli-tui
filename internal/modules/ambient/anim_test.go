package ambient

import (
	"testing"
	"time"

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
		if _, cmd := am.Update(ambientTickMsg(time.Now())); cmd == nil {
			t.Fatal("Tick armiert nicht neu")
		}
	}
	if am.frame <= before {
		t.Errorf("frame nicht erhöht: %d -> %d", before, am.frame)
	}
}
