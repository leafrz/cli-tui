package ambient

import (
	"math"
	"math/rand"

	"github.com/charmbracelet/lipgloss"
	"github.com/leafrz/dashboard/internal/ui"
)

// scene ist eine austauschbare Hintergrund-Animation für das Ambient-Modul.
// advance() schreitet den Zustand pro Tick voran, draw() rendert (rein) ins Grid.
type scene interface {
	name() string
	advance(w, h int, rng *rand.Rand)
	draw(g *grid, rng *rand.Rand)
}

func buildScenes() []scene {
	return []scene{
		&starfieldScene{},
		&matrixScene{},
		&rainScene{},
		&snowScene{},
		&plasmaScene{},
		&lifeScene{},
		&fireworksScene{},
		&dvdScene{},
		&wavesScene{},
		&fireScene{},
		&ripplesScene{},
		&spiralScene{},
		&blankScene{},
	}
}

// --- shared helpers --------------------------------------------------------

var rampRunes = []rune(" .:-=+*#%@")

func rampChar(t float64) rune {
	i := int(t * float64(len(rampRunes)-1))
	if i < 0 {
		i = 0
	}
	if i > len(rampRunes)-1 {
		i = len(rampRunes) - 1
	}
	return rampRunes[i]
}

// gradColor bildet 0..1 auf den Theme-Verlauf ab (kühl -> warm).
func gradColor(t float64) lipgloss.Color {
	switch {
	case t < 0.25:
		return ui.ColFaint
	case t < 0.45:
		return ui.ColTeal
	case t < 0.7:
		return ui.ColMauve
	case t < 0.9:
		return ui.ColPeach
	default:
		return ui.ColCream
	}
}

var themeCycle = []lipgloss.Color{ui.ColTeal, ui.ColMauve, ui.ColPeach, ui.ColGood, ui.ColCream}

// --- starfield -------------------------------------------------------------

type starfieldScene struct {
	w, h  int
	stars []star
}

func (s *starfieldScene) name() string { return "starfield" }

func (s *starfieldScene) ensure(w, h int, rng *rand.Rand) {
	if w == s.w && h == s.h && s.stars != nil {
		return
	}
	s.w, s.h = w, h
	n := w * h / 14
	if n < 30 {
		n = 30
	}
	s.stars = make([]star, n)
	for i := range s.stars {
		s.stars[i] = star{rng.Float64()*2 - 1, rng.Float64()*2 - 1, rng.Float64()*0.9 + 0.1}
	}
}

func (s *starfieldScene) advance(w, h int, rng *rand.Rand) {
	s.ensure(w, h, rng)
	for i := range s.stars {
		st := &s.stars[i]
		st.z -= 0.012
		if st.z <= 0.02 {
			st.x, st.y, st.z = rng.Float64()*2-1, rng.Float64()*2-1, 1
		}
	}
}

func (s *starfieldScene) draw(g *grid, rng *rand.Rand) {
	s.ensure(g.w, g.h, rng)
	cx, cy := float64(g.w)/2, float64(g.h)/2
	for _, st := range s.stars {
		sx := int(st.x/st.z*cx + cx)
		sy := int(st.y/st.z*cy + cy)
		switch {
		case st.z < 0.35:
			g.set(sx, sy, '@', ui.ColCream)
		case st.z < 0.7:
			g.set(sx, sy, '*', ui.ColMauve)
		default:
			g.set(sx, sy, '.', ui.ColDim)
		}
	}
}

// --- matrix rain -----------------------------------------------------------

var matrixRunes = []rune("ABCDEFGHJKLMNPQRSTUVWXYZ0123456789@#$%&*+=<>?/")

type matrixScene struct {
	w, h  int
	drops []float64
}

func (s *matrixScene) name() string { return "matrix" }

func (s *matrixScene) ensure(w, h int, rng *rand.Rand) {
	if w == s.w && h == s.h && s.drops != nil {
		return
	}
	s.w, s.h = w, h
	s.drops = make([]float64, w)
	for x := range s.drops {
		s.drops[x] = rng.Float64() * float64(h)
	}
}

func (s *matrixScene) advance(w, h int, rng *rand.Rand) {
	s.ensure(w, h, rng)
	const trail = 10
	for x := range s.drops {
		s.drops[x] += 0.5 + float64(x%3)*0.25
		if int(s.drops[x])-trail > h {
			s.drops[x] = 0
		}
	}
}

func (s *matrixScene) draw(g *grid, rng *rand.Rand) {
	s.ensure(g.w, g.h, rng)
	const trail = 10
	for x := 0; x < g.w && x < len(s.drops); x++ {
		head := int(s.drops[x])
		for t := 0; t < trail; t++ {
			y := head - t
			if y < 0 || y >= g.h {
				continue
			}
			r := matrixRunes[rng.Intn(len(matrixRunes))]
			switch {
			case t == 0:
				g.set(x, y, r, ui.ColCream)
			case t < 3:
				g.set(x, y, r, ui.ColGood)
			case t < 6:
				g.set(x, y, r, ui.ColTeal)
			default:
				g.set(x, y, r, ui.ColFaint)
			}
		}
	}
}

// --- rain ------------------------------------------------------------------

type rdrop struct{ x, y, spd float64 }

type rainScene struct {
	w, h  int
	drops []rdrop
}

func (s *rainScene) name() string { return "rain" }

func (s *rainScene) ensure(w, h int, rng *rand.Rand) {
	if w == s.w && h == s.h && s.drops != nil {
		return
	}
	s.w, s.h = w, h
	n := w * h / 30
	if n < 20 {
		n = 20
	}
	s.drops = make([]rdrop, n)
	for i := range s.drops {
		s.drops[i] = rdrop{rng.Float64() * float64(w), rng.Float64() * float64(h), 0.8 + rng.Float64()}
	}
}

func (s *rainScene) advance(w, h int, rng *rand.Rand) {
	s.ensure(w, h, rng)
	for i := range s.drops {
		d := &s.drops[i]
		d.y += d.spd
		d.x += 0.4
		if d.y >= float64(h) || d.x >= float64(w) {
			d.x, d.y, d.spd = rng.Float64()*float64(w), 0, 0.8+rng.Float64()
		}
	}
}

func (s *rainScene) draw(g *grid, rng *rand.Rand) {
	s.ensure(g.w, g.h, rng)
	for _, d := range s.drops {
		x, y := int(d.x), int(d.y)
		g.set(x, y, '/', ui.ColTeal)
		g.set(x-1, y-1, '/', ui.ColFaint)
	}
}

// --- snow ------------------------------------------------------------------

type sflake struct{ x, y, spd float64 }

type snowScene struct {
	w, h   int
	flakes []sflake
}

func (s *snowScene) name() string { return "snow" }

func (s *snowScene) ensure(w, h int, rng *rand.Rand) {
	if w == s.w && h == s.h && s.flakes != nil {
		return
	}
	s.w, s.h = w, h
	n := w * h / 45
	if n < 20 {
		n = 20
	}
	s.flakes = make([]sflake, n)
	for i := range s.flakes {
		s.flakes[i] = sflake{rng.Float64() * float64(w), rng.Float64() * float64(h), 0.15 + rng.Float64()*0.35}
	}
}

func (s *snowScene) advance(w, h int, rng *rand.Rand) {
	s.ensure(w, h, rng)
	for i := range s.flakes {
		f := &s.flakes[i]
		f.y += f.spd
		f.x += math.Sin(f.y*0.12) * 0.3
		if f.y >= float64(h) {
			f.x, f.y = rng.Float64()*float64(w), 0
		}
	}
}

func (s *snowScene) draw(g *grid, rng *rand.Rand) {
	s.ensure(g.w, g.h, rng)
	for _, f := range s.flakes {
		ch := '·'
		col := ui.ColDim
		if f.spd > 0.35 {
			ch, col = '*', ui.ColCream
		}
		g.set(int(f.x), int(f.y), ch, col)
	}
}

// --- plasma ----------------------------------------------------------------

type plasmaScene struct{ t float64 }

func (s *plasmaScene) name() string                     { return "plasma" }
func (s *plasmaScene) advance(w, h int, rng *rand.Rand) { s.t += 0.08 }

func (s *plasmaScene) draw(g *grid, rng *rand.Rand) {
	for y := 0; y < g.h; y++ {
		for x := 0; x < g.w; x++ {
			fx, fy := float64(x), float64(y)
			v := math.Sin(fx/8+s.t) + math.Sin(fy/6-s.t) +
				math.Sin((fx+fy)/10+s.t*1.3) + math.Sin(math.Hypot(fx-float64(g.w)/2, fy-float64(g.h)/2)/8-s.t)
			n := (v + 4) / 8
			if n < 0.15 {
				continue
			}
			g.set(x, y, rampChar(n), gradColor(n))
		}
	}
}

// --- game of life ----------------------------------------------------------

type lifeScene struct {
	w, h       int
	cells      []bool
	gens, pop  int
	stableSeen int
}

func (s *lifeScene) name() string { return "life" }

func (s *lifeScene) seed(rng *rand.Rand) {
	for i := range s.cells {
		s.cells[i] = rng.Float64() < 0.28
	}
	s.gens, s.stableSeen = 0, 0
}

func (s *lifeScene) ensure(w, h int, rng *rand.Rand) {
	if w == s.w && h == s.h && s.cells != nil {
		return
	}
	s.w, s.h = w, h
	s.cells = make([]bool, w*h)
	s.seed(rng)
}

func (s *lifeScene) advance(w, h int, rng *rand.Rand) {
	s.ensure(w, h, rng)
	next := make([]bool, len(s.cells))
	pop := 0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			n := 0
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					nx, ny := x+dx, y+dy
					if nx >= 0 && nx < w && ny >= 0 && ny < h && s.cells[ny*w+nx] {
						n++
					}
				}
			}
			alive := s.cells[y*w+x]
			if (alive && (n == 2 || n == 3)) || (!alive && n == 3) {
				next[y*w+x] = true
				pop++
			}
		}
	}
	s.cells = next
	s.gens++
	if pop == s.pop {
		s.stableSeen++
	} else {
		s.stableSeen = 0
	}
	s.pop = pop
	// Reseed bei Stillstand, niedriger Population oder hohem Alter.
	if s.stableSeen > 12 || pop < w*h/60 || s.gens > 400 {
		s.seed(rng)
	}
}

func (s *lifeScene) draw(g *grid, rng *rand.Rand) {
	s.ensure(g.w, g.h, rng)
	for y := 0; y < g.h; y++ {
		for x := 0; x < g.w; x++ {
			if s.cells[y*g.w+x] {
				g.set(x, y, '●', ui.ColGood)
			}
		}
	}
}

// --- fireworks -------------------------------------------------------------

type particle struct {
	x, y, vx, vy, life float64
	col                lipgloss.Color
}

type fireworksScene struct {
	w, h int
	ps   []particle
	t    int
}

func (s *fireworksScene) name() string { return "fireworks" }

func (s *fireworksScene) advance(w, h int, rng *rand.Rand) {
	s.w, s.h = w, h
	s.t++
	if s.t%6 == 0 {
		cx := rng.Float64() * float64(w)
		cy := rng.Float64()*float64(h)*0.5 + float64(h)*0.1
		col := themeCycle[rng.Intn(len(themeCycle))]
		k := 24 + rng.Intn(18)
		for i := 0; i < k; i++ {
			a := rng.Float64() * 2 * math.Pi
			sp := 0.4 + rng.Float64()*1.1
			s.ps = append(s.ps, particle{cx, cy, math.Cos(a) * sp, math.Sin(a) * sp * 0.6, 1, col})
		}
	}
	out := s.ps[:0]
	for _, p := range s.ps {
		p.x += p.vx
		p.y += p.vy
		p.vy += 0.03
		p.life -= 0.025
		if p.life > 0 && p.y < float64(h) {
			out = append(out, p)
		}
	}
	s.ps = out
}

func (s *fireworksScene) draw(g *grid, rng *rand.Rand) {
	for _, p := range s.ps {
		ch := '*'
		if p.life < 0.4 {
			ch = '.'
		} else if p.life < 0.7 {
			ch = '+'
		}
		col := p.col
		if p.life < 0.35 {
			col = ui.ColFaint
		}
		g.set(int(p.x), int(p.y), ch, col)
	}
}

// --- bouncing logo (DVD) ---------------------------------------------------

type dvdScene struct {
	w, h         int
	x, y, vx, vy float64
	ci           int
	logo         string
	initd        bool
}

func (s *dvdScene) name() string { return "dvd" }

func (s *dvdScene) advance(w, h int, rng *rand.Rand) {
	s.w, s.h = w, h
	s.logo = "◆ lofi ◆"
	if !s.initd {
		s.x, s.y = float64(w)/2, float64(h)/2
		s.vx, s.vy = 0.7, 0.4
		s.initd = true
	}
	lw := float64(len([]rune(s.logo)))
	s.x += s.vx
	s.y += s.vy
	if s.x <= 0 {
		s.x, s.vx, s.ci = 0, -s.vx, s.ci+1
	}
	if s.x+lw >= float64(w) {
		s.x, s.vx, s.ci = float64(w)-lw, -s.vx, s.ci+1
	}
	if s.y <= 0 {
		s.y, s.vy, s.ci = 0, -s.vy, s.ci+1
	}
	if s.y >= float64(h-1) {
		s.y, s.vy, s.ci = float64(h-1), -s.vy, s.ci+1
	}
}

func (s *dvdScene) draw(g *grid, rng *rand.Rand) {
	if s.logo == "" {
		return
	}
	g.stampText(int(s.x), int(s.y), s.logo, themeCycle[s.ci%len(themeCycle)])
}

// --- waves -----------------------------------------------------------------

type wavesScene struct{ t float64 }

func (s *wavesScene) name() string                     { return "waves" }
func (s *wavesScene) advance(w, h int, rng *rand.Rand) { s.t += 0.15 }

func (s *wavesScene) draw(g *grid, rng *rand.Rand) {
	cols := []lipgloss.Color{ui.ColTeal, ui.ColMauve, ui.ColPeach}
	mid := float64(g.h) / 2
	amp := float64(g.h) / 4
	for k := 0; k < 3; k++ {
		freq := 0.06 + float64(k)*0.03
		for x := 0; x < g.w; x++ {
			y := int(mid + amp*math.Sin(float64(x)*freq+s.t+float64(k)*1.7))
			g.set(x, y, '~', cols[k])
		}
	}
}

// --- fire ------------------------------------------------------------------

type fireScene struct {
	w, h int
	heat []float64
}

func (s *fireScene) name() string { return "fire" }

func (s *fireScene) ensure(w, h int) {
	if w == s.w && h == s.h && s.heat != nil {
		return
	}
	s.w, s.h = w, h
	s.heat = make([]float64, w*h)
}

func (s *fireScene) advance(w, h int, rng *rand.Rand) {
	s.ensure(w, h)
	for x := 0; x < w; x++ {
		s.heat[(h-1)*w+x] = 0.6 + rng.Float64()*0.4
	}
	for y := 0; y < h-1; y++ {
		for x := 0; x < w; x++ {
			below := (y+1)*w + x
			l := below
			r := below
			if x > 0 {
				l = below - 1
			}
			if x < w-1 {
				r = below + 1
			}
			v := (s.heat[below] + s.heat[l] + s.heat[r] + s.heat[below]) / 4.05
			s.heat[y*w+x] = v
		}
	}
}

func (s *fireScene) draw(g *grid, rng *rand.Rand) {
	s.ensure(g.w, g.h)
	for y := 0; y < g.h; y++ {
		for x := 0; x < g.w; x++ {
			t := s.heat[y*g.w+x]
			if t < 0.12 {
				continue
			}
			var col lipgloss.Color
			switch {
			case t < 0.3:
				col = ui.ColFaint
			case t < 0.55:
				col = ui.ColError
			case t < 0.8:
				col = ui.ColPeach
			default:
				col = ui.ColCream
			}
			g.set(x, y, rampChar(t), col)
		}
	}
}

// --- ripples ---------------------------------------------------------------

type ripple struct{ cx, cy, r, maxR float64 }

type ripplesScene struct {
	w, h int
	rs   []ripple
	t    int
}

func (s *ripplesScene) name() string { return "ripples" }

func (s *ripplesScene) advance(w, h int, rng *rand.Rand) {
	s.w, s.h = w, h
	s.t++
	if s.t%9 == 0 {
		s.rs = append(s.rs, ripple{
			rng.Float64() * float64(w), rng.Float64() * float64(h),
			0, 6 + rng.Float64()*float64(h)/2,
		})
	}
	out := s.rs[:0]
	for _, rp := range s.rs {
		rp.r += 0.6
		if rp.r < rp.maxR {
			out = append(out, rp)
		}
	}
	s.rs = out
}

func (s *ripplesScene) draw(g *grid, rng *rand.Rand) {
	for _, rp := range s.rs {
		fade := 1 - rp.r/rp.maxR
		col := gradColor(fade)
		for a := 0.0; a < 2*math.Pi; a += 0.18 {
			x := int(rp.cx + rp.r*math.Cos(a))
			y := int(rp.cy + rp.r*math.Sin(a)*0.5)
			g.set(x, y, '○', col)
		}
	}
}

// --- spiral ----------------------------------------------------------------

type spiralScene struct{ phase float64 }

func (s *spiralScene) name() string                     { return "spiral" }
func (s *spiralScene) advance(w, h int, rng *rand.Rand) { s.phase += 0.1 }

func (s *spiralScene) draw(g *grid, rng *rand.Rand) {
	cx, cy := float64(g.w)/2, float64(g.h)/2
	maxT := math.Min(float64(g.w), float64(g.h)*2) * 0.9
	for t := 0.0; t < maxT; t += 0.12 {
		r := t * 0.7
		for arm := 0; arm < 2; arm++ {
			theta := t*0.5 + s.phase + float64(arm)*math.Pi
			x := int(cx + r*math.Cos(theta))
			y := int(cy + r*math.Sin(theta)*0.5)
			g.set(x, y, '*', gradColor(t/maxT))
		}
	}
}

// --- blank -----------------------------------------------------------------

type blankScene struct{}

func (s *blankScene) name() string                     { return "blank" }
func (s *blankScene) advance(w, h int, rng *rand.Rand) {}
func (s *blankScene) draw(g *grid, rng *rand.Rand)     {}
