package radio

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/leafrz/dashboard/internal/audio"
	"github.com/leafrz/dashboard/internal/core"
)

// TestRefocusRestartsVisualizer stellt nach: Radio spielt -> Modul wird inaktiv
// (animTicking bleibt veraltet true, weil der animMsg-Handler nie lief) -> beim
// erneuten Focus MUSS der Anim-Tick wieder mitgeschickt werden, sonst steht der
// Visualizer.
func TestRefocusRestartsVisualizer(t *testing.T) {
	m := New(audio.NewPlayer())
	m.Init()

	// Zustand "spielt im Player" mit veraltetem animTicking simulieren.
	m.state = statePlayer
	m.uiPlaying = true
	m.animTicking = true // wäre nach Inaktivität fälschlich true geblieben

	_, cmd := m.Update(core.FocusMsg{})
	if cmd == nil {
		t.Fatal("Focus gab nil cmd zurück")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatalf("erwartete BatchMsg, bekam %T", cmd())
	}
	// fetchMeta + doTick + animCmd = 3. Mit dem Bug (Guard) wären es nur 2.
	if len(batch) != 3 {
		t.Errorf("erwartete 3 cmds (meta, tick, anim), bekam %d -> Visualizer würde stehen", len(batch))
	}
}
