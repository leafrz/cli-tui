package ambient

import (
	"math/rand"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPetIndexByName(t *testing.T) {
	if got := petIndexByName(""); got != 0 {
		t.Errorf("leerer Name: got %d, want 0", got)
	}
	if got := petIndexByName("does-not-exist"); got != 0 {
		t.Errorf("unbekannter Name: got %d, want 0", got)
	}
	for i, p := range pets {
		if got := petIndexByName(p.name); got != i {
			t.Errorf("petIndexByName(%q) = %d, want %d", p.name, got, i)
		}
	}
}

// TestPetArtBounds: jedes Pet liefert in jedem Zustand ein kleines,
// nicht-leeres Art (Frames sind frame-abhängig, daher viele durchspielen).
func TestPetArtBounds(t *testing.T) {
	states := []petState{
		{playing: false, night: false},
		{playing: true, level: 0.9, night: true},
		{playing: true, level: 0.1, night: false},
	}
	for _, p := range pets[1:] { // pets[0] = none
		for _, base := range states {
			for frame := 0; frame < 400; frame++ {
				s := base
				s.frame = frame
				lines := p.art(s)
				if len(lines) < 3 || len(lines) > 5 {
					t.Fatalf("%s: %d Zeilen (frame %d)", p.name, len(lines), frame)
				}
				for _, ln := range lines {
					if n := len([]rune(ln)); n > 12 {
						t.Fatalf("%s: Zeile %q zu breit (%d, frame %d)", p.name, ln, n, frame)
					}
				}
			}
		}
	}
}

// TestDrawPetSmallGrid: auf winzigen Grids darf nichts panicen; das Pet
// bleibt dann einfach unsichtbar.
func TestDrawPetSmallGrid(t *testing.T) {
	for _, size := range [][2]int{{1, 1}, {5, 3}, {12, 6}, {80, 24}} {
		g := newGrid(size[0], size[1])
		for _, p := range pets {
			drawPet(g, p, petState{frame: 3, playing: true, level: 1}, petMotion{})
			drawPet(g, p, petState{frame: 3, moving: true}, petMotion{x: 2, init: true})
		}
	}
}

// TestStepPetStaysInBounds: egal wie lange die Pets herumlaufen, sie bleiben
// im sichtbaren Bereich (inkl. schmaler Terminals).
func TestStepPetStaysInBounds(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for _, width := range []int{20, 40, 80, 200} {
		for _, p := range pets[1:] {
			mo := petMotion{}
			s := petState{playing: true, level: 1, night: true}
			for frame := 0; frame < 5000; frame++ {
				s.frame = frame
				s.moving = petMoving(mo, p)
				stepPet(&mo, p, s, rng, width)
				maxX := float64(width - p.w - 2)
				if mo.init && (mo.x < 1 || mo.x > maxX) {
					t.Fatalf("%s (w=%d): x=%.1f außerhalb [1,%.0f] bei frame %d",
						p.name, width, mo.x, maxX, frame)
				}
			}
		}
	}
}

// TestCatNapsInPlace: ohne Musik bewegt sich die Katze nicht.
func TestCatNapsInPlace(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	cat := pets[petIndexByName("cat")]
	mo := petMotion{}
	stepPet(&mo, cat, petState{playing: false}, rng, 80) // init
	x := mo.x
	for frame := 1; frame < 1000; frame++ {
		stepPet(&mo, cat, petState{frame: frame, playing: false}, rng, 80)
		if mo.x != x {
			t.Fatalf("Katze läuft im Schlaf herum: x %.1f -> %.1f (frame %d)", x, mo.x, frame)
		}
	}
}

// TestGhostDrifts: der Ghost ist pausenlos unterwegs.
func TestGhostDrifts(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	ghost := pets[petIndexByName("ghost")]
	mo := petMotion{}
	stepPet(&mo, ghost, petState{}, rng, 80)
	x := mo.x
	for frame := 1; frame < 50; frame++ {
		stepPet(&mo, ghost, petState{frame: frame}, rng, 80)
	}
	if mo.x == x {
		t.Error("Ghost driftet nicht")
	}
	if !petMoving(mo, ghost) {
		t.Error("Ghost gilt nicht als moving")
	}
}

func TestIsNight(t *testing.T) {
	for hour, want := range map[int]bool{0: true, 5: true, 6: false, 12: false, 20: false, 21: true, 23: true} {
		if got := isNight(hour); got != want {
			t.Errorf("isNight(%d) = %v, want %v", hour, got, want)
		}
	}
}

// TestPetPickerFlow: p öffnet den Picker, Auswahl persistiert den Index,
// esc bricht ohne Änderung ab.
func TestPetPickerFlow(t *testing.T) {
	m := New(nil)
	m.width, m.height = 80, 24
	m.petIdx = 0 // New() lädt die echte User-Config -> explizit zurücksetzen

	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	if !m.pickingPet {
		t.Fatal("'p' öffnete den Picker nicht")
	}

	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.pickingPet {
		t.Fatal("enter schloss den Picker nicht")
	}
	if m.petIdx != 1 {
		t.Fatalf("petIdx = %d, want 1", m.petIdx)
	}
	if cmd == nil {
		t.Fatal("Auswahl gab keinen Persist-Cmd zurück")
	}

	// esc: Cursor-Bewegung ohne enter ändert nichts.
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.pickingPet || m.petIdx != 1 {
		t.Fatalf("esc: pickingPet=%v petIdx=%d, want false/1", m.pickingPet, m.petIdx)
	}
}

// TestPetVisibleInView: das gewählte Pet taucht im gerenderten Ambient-View auf.
func TestPetVisibleInView(t *testing.T) {
	m := New(nil)
	m.petIdx = petIndexByName("cat")

	out := m.View(80, 24)
	if !strings.Contains(out, `/\_/\`) {
		t.Error("Katze nicht im View gefunden")
	}
}
