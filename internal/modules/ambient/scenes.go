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
		&auroraScene{},
		&lavaScene{},
		&lissajousScene{},
		&swarmScene{},
		&cometsScene{},
		&bubblesScene{},
		&pongScene{},
		&tunnelScene{},
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
	if s.logo == "" {
		s.logo = "◆ radio ◆"
	}
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

// --- aurora ----------------------------------------------------------------
// Wavernde vertikale Lichtvorhänge (Nordlicht), Farbe nach Höhe.

type auroraScene struct{ t float64 }

func (s *auroraScene) name() string                     { return "aurora" }
func (s *auroraScene) advance(w, h int, rng *rand.Rand) { s.t += 0.05 }

func (s *auroraScene) draw(g *grid, rng *rand.Rand) {
	top := g.h / 6
	for x := 0; x < g.w; x++ {
		fx := float64(x)
		inten := (math.Sin(fx*0.15+s.t)+math.Sin(fx*0.06-s.t*0.7))/2 +
			0.35*math.Sin(fx*0.28+s.t*1.6)
		if inten <= 0 {
			continue
		}
		height := int(inten * float64(g.h) * 0.75)
		for y := top; y < top+height && y < g.h; y++ {
			f := 1 - float64(y-top)/float64(height+1) // hell oben -> dunkel unten
			var col lipgloss.Color
			switch {
			case f < 0.3:
				col = ui.ColFaint
			case f < 0.55:
				col = ui.ColTeal
			case f < 0.8:
				col = ui.ColGood
			default:
				col = ui.ColCream
			}
			g.set(x, y, rampChar(0.3+f*0.7), col)
		}
	}
}

// --- lava lamp (metaballs) -------------------------------------------------
// Weiche, verschmelzende Blobs über ein Schwellenwertfeld.

type blob struct{ x, y, vx, vy, r float64 }

type lavaScene struct {
	w, h  int
	blobs []blob
}

func (s *lavaScene) name() string { return "lava" }

func (s *lavaScene) ensure(w, h int, rng *rand.Rand) {
	if w == s.w && h == s.h && s.blobs != nil {
		return
	}
	s.w, s.h = w, h
	n := 4 + rng.Intn(2)
	s.blobs = make([]blob, n)
	for i := range s.blobs {
		s.blobs[i] = blob{
			x:  rng.Float64() * float64(w),
			y:  rng.Float64() * float64(h),
			vx: (rng.Float64()*2 - 1) * 0.5,
			vy: (rng.Float64()*2 - 1) * 0.3,
			r:  float64(h)/4 + rng.Float64()*float64(h)/6,
		}
	}
}

func (s *lavaScene) advance(w, h int, rng *rand.Rand) {
	s.ensure(w, h, rng)
	for i := range s.blobs {
		b := &s.blobs[i]
		b.x += b.vx
		b.y += b.vy
		if b.x < 0 || b.x >= float64(w) {
			b.vx = -b.vx
		}
		if b.y < 0 || b.y >= float64(h) {
			b.vy = -b.vy
		}
	}
}

func (s *lavaScene) draw(g *grid, rng *rand.Rand) {
	s.ensure(g.w, g.h, rng)
	for y := 0; y < g.h; y++ {
		for x := 0; x < g.w; x++ {
			var v float64
			for _, b := range s.blobs {
				dx := float64(x) - b.x
				dy := (float64(y) - b.y) * 2 // Zellen sind ~2:1 -> y strecken
				v += (b.r * b.r) / (dx*dx + dy*dy + 1)
			}
			if v < 0.7 {
				continue
			}
			n := math.Min(1, (v-0.7)/1.3)
			g.set(x, y, rampChar(n), gradColor(n))
		}
	}
}

// --- lissajous -------------------------------------------------------------
// Elegante parametrische Kurve, die langsam ihre Form ändert.

type lissajousScene struct{ t float64 }

func (s *lissajousScene) name() string                     { return "lissajous" }
func (s *lissajousScene) advance(w, h int, rng *rand.Rand) { s.t += 0.04 }

func (s *lissajousScene) draw(g *grid, rng *rand.Rand) {
	cx, cy := float64(g.w)/2, float64(g.h)/2
	ax, ay := cx-2, cy-1
	a := 3.0 + math.Sin(s.t*0.13)
	b := 2.0 + math.Cos(s.t*0.09)
	for u := 0.0; u < 2*math.Pi; u += 0.008 {
		x := int(cx + ax*math.Sin(a*u+s.t))
		y := int(cy + ay*math.Sin(b*u))
		g.set(x, y, '•', gradColor(u/(2*math.Pi)))
	}
}

// --- swarm (boids) ---------------------------------------------------------
// Schwarm aus Punkten mit leichtem Flocking, folgt einem wandernden Ziel.

type boid struct{ x, y, vx, vy float64 }

type swarmScene struct {
	w, h int
	bs   []boid
	t    float64
}

func (s *swarmScene) name() string { return "swarm" }

func (s *swarmScene) ensure(w, h int, rng *rand.Rand) {
	if w == s.w && h == s.h && s.bs != nil {
		return
	}
	s.w, s.h = w, h
	n := w * h / 60
	if n < 20 {
		n = 20
	}
	if n > 60 {
		n = 60
	}
	s.bs = make([]boid, n)
	for i := range s.bs {
		s.bs[i] = boid{rng.Float64() * float64(w), rng.Float64() * float64(h),
			rng.Float64()*2 - 1, rng.Float64()*2 - 1}
	}
}

func (s *swarmScene) advance(w, h int, rng *rand.Rand) {
	s.ensure(w, h, rng)
	s.t += 0.02
	// Wanderndes Ziel (Lissajous-Pfad).
	tx := float64(w)/2 + float64(w)/3*math.Sin(s.t)
	ty := float64(h)/2 + float64(h)/3*math.Sin(s.t*1.3)
	for i := range s.bs {
		b := &s.bs[i]
		// Kohäsion zum Ziel.
		b.vx += (tx - b.x) * 0.002
		b.vy += (ty - b.y) * 0.002
		// Separation von nahen Nachbarn.
		for j := range s.bs {
			if j == i {
				continue
			}
			dx, dy := b.x-s.bs[j].x, b.y-s.bs[j].y
			d2 := dx*dx + dy*dy
			if d2 > 0 && d2 < 9 {
				b.vx += dx / d2 * 0.35
				b.vy += dy / d2 * 0.35
			}
		}
		// Geschwindigkeit begrenzen.
		sp := math.Hypot(b.vx, b.vy)
		if sp > 1.2 {
			b.vx, b.vy = b.vx/sp*1.2, b.vy/sp*1.2
		}
		b.x += b.vx
		b.y += b.vy
	}
}

func (s *swarmScene) draw(g *grid, rng *rand.Rand) {
	s.ensure(g.w, g.h, rng)
	for _, b := range s.bs {
		sp := math.Hypot(b.vx, b.vy)
		ch, col := '·', ui.ColTeal
		switch {
		case sp > 0.9:
			ch, col = '●', ui.ColPeach
		case sp > 0.5:
			ch, col = '•', ui.ColMauve
		}
		g.set(int(b.x), int(b.y), ch, col)
	}
}

// --- comets (meteor shower) --------------------------------------------------
// Schräg fallende Meteore mit ausblendendem Schweif vor dünnem Sternhimmel.

type comet struct{ x, y, spd float64 }

type cometsScene struct {
	w, h   int
	cs     []comet
	stars  []int // Indizes fixer Hintergrundsterne
	starsN int
}

func (s *cometsScene) name() string { return "comets" }

func (s *cometsScene) ensure(w, h int, rng *rand.Rand) {
	if w == s.w && h == s.h && s.cs != nil {
		return
	}
	s.w, s.h = w, h
	n := w / 12
	if n < 4 {
		n = 4
	}
	s.cs = make([]comet, n)
	for i := range s.cs {
		s.cs[i] = comet{rng.Float64() * float64(w) * 1.5, rng.Float64() * float64(h), 0.6 + rng.Float64()*0.9}
	}
	s.starsN = w * h / 80
	s.stars = make([]int, s.starsN)
	for i := range s.stars {
		s.stars[i] = rng.Intn(w * h)
	}
}

func (s *cometsScene) advance(w, h int, rng *rand.Rand) {
	s.ensure(w, h, rng)
	for i := range s.cs {
		c := &s.cs[i]
		c.x -= c.spd * 1.6 // nach links unten
		c.y += c.spd * 0.5
		if c.x < -8 || c.y >= float64(h)+4 {
			c.x = float64(w) + rng.Float64()*float64(w)*0.5
			c.y = -rng.Float64() * float64(h) * 0.5
			c.spd = 0.6 + rng.Float64()*0.9
		}
	}
}

func (s *cometsScene) draw(g *grid, rng *rand.Rand) {
	s.ensure(g.w, g.h, rng)
	for _, idx := range s.stars {
		g.set(idx%g.w, idx/g.w, '·', ui.ColFaint)
	}
	for _, c := range s.cs {
		// Schweif entgegen der Flugrichtung (nach rechts oben).
		for t := 0; t < 7; t++ {
			x := int(c.x) + t*2
			y := int(c.y) - t/2
			ch, col := '─', ui.ColFaint
			switch {
			case t == 0:
				ch, col = '●', ui.ColCream
			case t < 3:
				ch, col = '━', ui.ColPeach
			case t < 5:
				ch, col = '─', ui.ColMauve
			}
			g.set(x, y, ch, col)
		}
	}
}

// --- bubbles -----------------------------------------------------------------
// Blasen steigen wackelnd auf und "ploppen" kurz vor der Oberfläche.

type bubble struct {
	x, y, spd, wob float64
	big            bool
}

type bubblesScene struct {
	w, h int
	bs   []bubble
}

func (s *bubblesScene) name() string { return "bubbles" }

func (s *bubblesScene) ensure(w, h int, rng *rand.Rand) {
	if w == s.w && h == s.h && s.bs != nil {
		return
	}
	s.w, s.h = w, h
	n := w * h / 55
	if n < 15 {
		n = 15
	}
	s.bs = make([]bubble, n)
	for i := range s.bs {
		s.bs[i] = bubble{
			x: rng.Float64() * float64(w), y: rng.Float64() * float64(h),
			spd: 0.15 + rng.Float64()*0.4, wob: rng.Float64() * 2 * math.Pi,
			big: rng.Float64() < 0.3,
		}
	}
}

func (s *bubblesScene) advance(w, h int, rng *rand.Rand) {
	s.ensure(w, h, rng)
	for i := range s.bs {
		b := &s.bs[i]
		b.y -= b.spd
		b.wob += 0.15
		b.x += math.Sin(b.wob) * 0.35
		if b.y < 1 {
			b.x, b.y = rng.Float64()*float64(w), float64(h)-1
			b.spd = 0.15 + rng.Float64()*0.4
			b.big = rng.Float64() < 0.3
		}
	}
}

func (s *bubblesScene) draw(g *grid, rng *rand.Rand) {
	s.ensure(g.w, g.h, rng)
	for _, b := range s.bs {
		ch, col := '∘', ui.ColDim
		switch {
		case b.y < 3: // kurz vor der Oberfläche: plopp
			ch, col = '*', ui.ColCream
		case b.big:
			ch, col = 'O', ui.ColTeal
		case b.spd > 0.35:
			ch, col = 'o', ui.ColTeal
		}
		g.set(int(b.x), int(b.y), ch, col)
	}
}

// --- pong (self-playing) -----------------------------------------------------
// Zwei KI-Paddles spielen endlos gegeneinander; Score oben.

type pongScene struct {
	w, h           int
	bx, by, vx, vy float64
	lp, rp         float64 // Paddle-Mitte links/rechts
	ls, rs         int     // Score
	initd          bool
}

func (s *pongScene) name() string { return "pong" }

const pongPad = 3 // halbe Paddle-Höhe

func (s *pongScene) reset(w, h int, dir float64) {
	s.bx, s.by = float64(w)/2, float64(h)/2
	s.vx, s.vy = 0.9*dir, 0.35
}

func (s *pongScene) advance(w, h int, rng *rand.Rand) {
	if !s.initd || w != s.w || h != s.h {
		s.w, s.h = w, h
		s.lp, s.rp = float64(h)/2, float64(h)/2
		s.reset(w, h, 1)
		s.initd = true
	}
	s.bx += s.vx
	s.by += s.vy

	// Wände oben/unten.
	if s.by <= 1 || s.by >= float64(h-1) {
		s.vy = -s.vy
	}

	// Paddles folgen dem Ball (leicht träge + Zittern -> verlieren manchmal).
	track := func(p float64, active bool) float64 {
		target := s.by
		if !active {
			target = float64(h) / 2
		}
		p += (target - p) * 0.08
		p += (rng.Float64() - 0.5) * 0.4
		if p < pongPad+1 {
			p = pongPad + 1
		}
		if p > float64(h)-pongPad-1 {
			p = float64(h) - pongPad - 1
		}
		return p
	}
	s.lp = track(s.lp, s.vx < 0)
	s.rp = track(s.rp, s.vx > 0)

	// Paddle-Kollision / Punkt.
	if s.bx <= 2 {
		if math.Abs(s.by-s.lp) <= pongPad+1 {
			s.vx = -s.vx * 1.03
			s.vy += (s.by - s.lp) * 0.12
		} else {
			s.rs++
			s.reset(w, h, 1)
		}
	}
	if s.bx >= float64(w-2) {
		if math.Abs(s.by-s.rp) <= pongPad+1 {
			s.vx = -s.vx * 1.03
			s.vy += (s.by - s.rp) * 0.12
		} else {
			s.ls++
			s.reset(w, h, -1)
		}
	}
	// Tempo deckeln.
	if math.Abs(s.vx) > 1.6 {
		s.vx = 1.6 * math.Copysign(1, s.vx)
	}
	if math.Abs(s.vy) > 0.9 {
		s.vy = 0.9 * math.Copysign(1, s.vy)
	}
}

func (s *pongScene) draw(g *grid, rng *rand.Rand) {
	if !s.initd {
		return
	}
	// Mittellinie + Score.
	for y := 0; y < g.h; y += 2 {
		g.set(g.w/2, y, '┊', ui.ColFaint)
	}
	score := fmtScore(s.ls) + " · " + fmtScore(s.rs)
	g.stampText(centerX(g.w, score), 1, score, ui.ColDim)

	// Paddles.
	for d := -pongPad; d <= pongPad; d++ {
		g.set(1, int(s.lp)+d, '█', ui.ColTeal)
		g.set(g.w-2, int(s.rp)+d, '█', ui.ColMauve)
	}
	g.set(int(s.bx), int(s.by), '●', ui.ColPeach)
}

func fmtScore(n int) string {
	if n > 99 {
		n = 99
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

// --- tunnel ------------------------------------------------------------------
// Flug durch konzentrische Ringe, die aus der Mitte heranzoomen.

type tunnelScene struct{ t float64 }

func (s *tunnelScene) name() string                     { return "tunnel" }
func (s *tunnelScene) advance(w, h int, rng *rand.Rand) { s.t += 0.06 }

func (s *tunnelScene) draw(g *grid, rng *rand.Rand) {
	cx, cy := float64(g.w)/2, float64(g.h)/2
	maxR := math.Max(cx, cy*2)
	const rings = 9
	for k := 0; k < rings; k++ {
		// Ringe wandern zyklisch von innen (0) nach außen (1), exponentiell
		// beschleunigt -> Zoom-Gefühl.
		f := math.Mod(float64(k)/rings+s.t*0.15, 1)
		r := math.Pow(f, 2.2) * maxR
		if r < 1 {
			continue
		}
		// Tunnel krümmt sich vor dem Betrachter: FERNE Ringe (f klein, in der
		// Mitte) driften seitlich, nahe (große) bleiben zentriert. Drift ∝ r
		// sah kaputt aus — die äußeren Ringe schwangen wild zur Seite.
		drift := math.Sin(s.t*0.7) * (1 - f) * float64(g.w) * 0.10
		col := gradColor(f)
		step := 0.35 / (r + 1) * 6
		if step > 0.3 {
			step = 0.3
		}
		for a := 0.0; a < 2*math.Pi; a += step {
			x := int(cx + drift + r*math.Cos(a))
			y := int(cy + r*math.Sin(a)*0.5)
			g.set(x, y, rampChar(0.2+f*0.8), col)
		}
	}
}

// --- blank -----------------------------------------------------------------

type blankScene struct{}

func (s *blankScene) name() string                     { return "blank" }
func (s *blankScene) advance(w, h int, rng *rand.Rand) {}
func (s *blankScene) draw(g *grid, rng *rand.Rand)     {}
