package dashboard

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TestHeaderTickAdvances prüft, dass der Header-Tick den Frame erhöht und sich
// neu armiert (die Basis-Animationsschleife).
func TestHeaderTickAdvances(t *testing.T) {
	r := NewRoot()
	r.Init() // richtet die Module ein (Listen etc.), wie Bubble Tea es tut
	r.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	before := r.headerFrame
	m, cmd := r.Update(headerTickMsg(time.Now()))
	rr := m.(*rootModel)
	if rr.headerFrame <= before {
		t.Errorf("headerFrame nicht erhöht: %d", rr.headerFrame)
	}
	if cmd == nil {
		t.Error("Header-Tick armiert nicht neu (nil cmd)")
	}
}

// TestModuleFocusStartsTicker öffnet ein Modul und prüft, dass der Focus den
// Ticker startet (das Modul gibt ein nicht-nil Tick-Cmd zurück).
func TestModuleFocusStartsTicker(t *testing.T) {
	r := NewRoot()
	r.Init() // richtet die Module ein (Listen etc.), wie Bubble Tea es tut
	r.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Cursor auf "system monitor" (Index 1) und öffnen.
	r.Update(tea.KeyMsg{Type: tea.KeyDown})
	m, cmd := r.Update(tea.KeyMsg{Type: tea.KeyEnter})
	rr := m.(*rootModel)
	if cmd == nil {
		t.Fatal("enter auf Modul gab nil cmd zurück (kein Focus)")
	}

	// Focus-Cmd ausführen -> FocusMsg, an Root füttern -> Modul startet Ticker.
	msg := cmd()
	_, cmd2 := rr.Update(msg)
	if cmd2 == nil {
		t.Fatal("Modul startet keinen Ticker bei Focus (nil cmd)")
	}
}
