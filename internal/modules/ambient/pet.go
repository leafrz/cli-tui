package ambient

import (
	"math/rand"

	"github.com/charmbracelet/lipgloss"
	"github.com/leafrz/dashboard/internal/ui"
)

// --- desk pets ---------------------------------------------------------------
// Kleine ASCII-Begleiter, die unten in der Ambient-Ansicht leben, herumlaufen
// und auf vorhandene Signale reagieren (Musik, Spektrum, Tageszeit). Bewusst
// zwecklos.

// petState bündelt die Signale, auf die Pets reagieren.
type petState struct {
	frame   int     // Ambient-Frame (90ms-Takt)
	playing bool    // Radio läuft
	level   float64 // Ø-Spektrum 0..1 (nur sinnvoll wenn playing)
	night   bool    // 21:00–06:00
	moving  bool    // gerade unterwegs (Lauf-/Hüpf-Frames)
}

// moveStyle beschreibt, wie sich ein Pet über den Boden bewegt.
type moveStyle struct {
	speed              float64 // Spalten pro Tick während einer Bewegung
	burstMin, burstMax int     // Dauer einer Bewegung (Ticks)
	waitMin, waitMax   int     // Pause zwischen Bewegungen (Ticks)
	drift              bool    // pausenlos unterwegs (Ghost)
	bob                bool    // vertikales Schweben (Ghost)
	lift               bool    // eine Zeile hoch während der Bewegung (Frosch)
	can                func(s petState) bool // nil = darf immer
}

// petMotion ist der Bewegungszustand (lebt im Modul, nicht im Pet).
type petMotion struct {
	x     float64
	dir   int
	moveT int // verbleibende Ticks der aktuellen Bewegung
	waitT int // Ticks bis zur nächsten Bewegungsentscheidung
	init  bool
}

type pet struct {
	name string
	desc string
	w    int // maximale Art-Breite (für Bewegungs-Clamping)
	move moveStyle
	// col zur Render-Zeit auflösen: ApplyTheme weist die ui.Col*-Vars neu zu.
	col func() lipgloss.Color
	art func(s petState) []string // nil = "none"
}

// blinkPhase: true für die ersten closed-Ticks eines period-Zyklus.
// offset entzerrt die Pets, damit nicht alle synchron blinzeln.
func blinkPhase(frame, period, closed, offset int) bool {
	return (frame+offset)%period < closed
}

var pets = []pet{
	{name: "none", desc: "no companion"},
	{
		name: "cat", desc: "strolls around, naps when the music stops",
		w: 7,
		move: moveStyle{
			speed: 0.3, burstMin: 20, burstMax: 50, waitMin: 40, waitMax: 160,
			can: func(s petState) bool { return s.playing }, // beim Nickerchen kein Herumlaufen
		},
		col: func() lipgloss.Color { return ui.ColPeach },
		art: func(s petState) []string {
			if !s.playing {
				// Nickerchen: geschlossene Augen + wanderndes z.
				z := []string{"  z  ", "   z ", "    z"}[(s.frame/9)%3]
				return []string{z, " /\\_/\\", "( -.- )", " > ^ <"}
			}
			eyes := "( o.o )"
			if blinkPhase(s.frame, 44, 3, 0) {
				eyes = "( -.- )"
			}
			paws := " > ^ <"
			if s.moving && (s.frame/3)%2 == 0 {
				paws = " >^ ^<"
			}
			return []string{" /\\_/\\", eyes, paws}
		},
	},
	{
		name: "ghost", desc: "floats around, vibes to the beat",
		w: 5,
		move: moveStyle{
			speed: 0.12, drift: true, bob: true,
		},
		col: func() lipgloss.Color { return ui.ColMauve },
		art: func(s petState) []string {
			eyes := "(o.o)"
			if blinkPhase(s.frame, 52, 3, 17) {
				eyes = "(-.-)"
			}
			mouth := "|   |"
			if s.playing && s.level > 0.35 {
				mouth = "| O |"
			}
			return []string{" .-.", eyes, mouth, "'~~~'"}
		},
	},
	{
		name: "owl", desc: "only wakes at night, hops between perches",
		w: 7,
		move: moveStyle{
			speed: 0.9, burstMin: 4, burstMax: 7, waitMin: 100, waitMax: 250,
			can: func(s petState) bool { return s.night },
		},
		col: func() lipgloss.Color { return ui.ColPurple },
		art: func(s petState) []string {
			if !s.night {
				z := []string{" z  ", "  z ", "   z"}[(s.frame/11)%3]
				return []string{z, " ,___,", " (-,-)", " /)_)", "  \" \""}
			}
			eyes := " (O,O)"
			if blinkPhase(s.frame, 60, 4, 31) {
				eyes = " (-,-)"
			}
			if s.moving {
				flap := "^(O,O)^"
				if (s.frame/2)%2 == 0 {
					flap = "v(O,O)v"
				}
				return []string{" ,___,", flap, " /)_)"}
			}
			return []string{" ,___,", eyes, " /)_)", "  \" \""}
		},
	},
	{
		name: "frog", desc: "hops across the screen",
		w: 6,
		move: moveStyle{
			speed: 0.8, burstMin: 5, burstMax: 5, waitMin: 30, waitMax: 140,
			lift: true,
		},
		col: func() lipgloss.Color { return ui.ColGood },
		art: func(s petState) []string {
			if s.moving {
				return []string{" @..@", "(----)", "/>  <\\"}
			}
			eyes := " @..@"
			if blinkPhase(s.frame, 48, 3, 7) {
				eyes = " -..-"
			}
			return []string{eyes, "(----)", "(>__<)"}
		},
	},
}

// isNight: Eulen-Zeit (21:00–06:00).
func isNight(hour int) bool { return hour >= 21 || hour < 6 }

// avgLevels liefert den Mittelwert des Spektrums (0 bei leer).
func avgLevels(levels []float64) float64 {
	if len(levels) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range levels {
		sum += v
	}
	return sum / float64(len(levels))
}

// petIndexByName liefert den Index in pets; ""/unbekannt => 0 (none).
func petIndexByName(name string) int {
	for i, p := range pets {
		if p.name == name {
			return i
		}
	}
	return 0
}

// stepPet bewegt das Pet um einen Tick weiter. Startposition ist rechts unten
// (wie die statische Variante); an den Rändern wird umgedreht.
func stepPet(mo *petMotion, p pet, s petState, rng *rand.Rand, gridW int) {
	if p.art == nil {
		return
	}
	maxX := gridW - p.w - 2
	if maxX < 1 {
		return // Terminal zu schmal, Position einfrieren
	}
	if !mo.init {
		mo.x, mo.dir, mo.init = float64(maxX), -1, true
	}

	allowed := p.move.can == nil || p.move.can(s)
	switch {
	case !allowed:
		mo.moveT = 0 // Bewegung abbrechen (z.B. Katze schläft ein)
	case p.move.drift:
		mo.x += p.move.speed * float64(mo.dir)
	case mo.moveT > 0:
		mo.x += p.move.speed * float64(mo.dir)
		mo.moveT--
		if mo.moveT == 0 {
			mo.waitT = p.move.waitMin + rng.Intn(p.move.waitMax-p.move.waitMin+1)
		}
	case mo.waitT > 0:
		mo.waitT--
	default:
		mo.dir = 1
		if rng.Intn(2) == 0 {
			mo.dir = -1
		}
		mo.moveT = p.move.burstMin + rng.Intn(p.move.burstMax-p.move.burstMin+1)
	}

	if mo.x < 1 {
		mo.x, mo.dir = 1, 1
	}
	if mo.x > float64(maxX) {
		mo.x, mo.dir = float64(maxX), -1
	}
}

// bobOffset: sanfte Schwebe-Welle für den Ghost (Dreieckswelle, kein float).
func bobOffset(frame int) int {
	return []int{0, 0, -1, -1, 0, 0, 1, 1}[(frame/6)%8]
}

// petMoving: läuft gerade eine Bewegung (für Lauf-/Hüpf-Frames)?
func petMoving(mo petMotion, p pet) bool {
	return p.move.drift || mo.moveT > 0
}

// drawPet stempelt das Pet an seiner aktuellen Position aufs Grid
// (über der Hint-Zeile).
func drawPet(g *grid, p pet, s petState, mo petMotion) {
	if p.art == nil {
		return
	}
	lines := p.art(s)
	x := int(mo.x)
	if !mo.init {
		x = g.w - p.w - 2 // noch nie bewegt -> rechte Ecke wie gehabt
	}
	y := g.h - len(lines) - 2
	if s.moving && p.move.lift {
		y--
	}
	if p.move.bob {
		y += bobOffset(s.frame)
	}
	if x < 0 || y < 0 {
		return // Terminal zu klein -> Pet lässt sich nicht blicken
	}
	col := p.col()
	for i, ln := range lines {
		g.stampText(x, y+i, ln, col)
	}
}

// petPickerView rendert die Begleiter-Auswahl (Overlay wie Help/Editor).
// Die Arts nutzen den Live-State, damit die Vorschau schon animiert.
func petPickerView(cursor int, s petState) string {
	nameSel := lipgloss.NewStyle().Bold(true).Foreground(ui.ColPeach)
	nameDef := lipgloss.NewStyle().Foreground(ui.ColCream)

	rows := []string{ui.LabelStyle.Render("choose your companion"), ""}
	for i, p := range pets {
		cur, name := "  ", nameDef.Render(p.name)
		if i == cursor {
			cur = nameSel.Render("▸ ")
			name = nameSel.Render(p.name)
		}
		label := lipgloss.JoinVertical(lipgloss.Left, name, ui.DimStyle.Render(p.desc))

		var block string
		if p.art == nil {
			block = lipgloss.JoinHorizontal(lipgloss.Center, cur, label)
		} else {
			art := lipgloss.NewStyle().Foreground(p.col()).Width(10).
				Render(lipgloss.JoinVertical(lipgloss.Left, p.art(s)...))
			block = lipgloss.JoinHorizontal(lipgloss.Center, cur, art, "  ", label)
		}
		rows = append(rows, block, "")
	}

	card := ui.CardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
	help := ui.HelpStyle.Render("↑/↓: select   ·   enter: adopt   ·   esc: cancel")
	return lipgloss.JoinVertical(lipgloss.Center, card, "", help)
}
