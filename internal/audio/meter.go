package audio

import (
	"math"
	"sync"

	"github.com/faiface/beep"
)

// meter sitzt in der Streamer-Kette und kopiert die zuletzt durchlaufenden
// Mono-Samples in einen Ring-Puffer. Die UI liest daraus (per FFT) ein
// Echtzeit-Spektrum für den Visualizer. Eigener Mutex -> keine Kopplung an
// p.mu, da Stream() aus der Speaker-Callback-Goroutine läuft.
type meter struct {
	inner beep.Streamer
	mu    sync.Mutex
	ring  []float64
	pos   int
	size  int
}

func newMeter(inner beep.Streamer, size int) *meter {
	return &meter{inner: inner, ring: make([]float64, size), size: size}
}

func (m *meter) Stream(samples [][2]float64) (int, bool) {
	n, ok := m.inner.Stream(samples)
	m.mu.Lock()
	for i := 0; i < n; i++ {
		m.ring[m.pos] = (samples[i][0] + samples[i][1]) * 0.5
		m.pos = (m.pos + 1) % m.size
	}
	m.mu.Unlock()
	return n, ok
}

func (m *meter) Err() error { return m.inner.Err() }

// snapshot liefert die Ring-Samples in chronologischer Reihenfolge.
func (m *meter) snapshot() []float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]float64, m.size)
	for i := 0; i < m.size; i++ {
		out[i] = m.ring[(m.pos+i)%m.size]
	}
	return out
}

// spectrum berechnet ein normalisiertes Band-Spektrum (0..1) aus den letzten
// Samples: Hann-Fenster -> FFT -> log-spaced Bänder -> log-Magnitude.
func (m *meter) spectrum(bands int) []float64 {
	if bands < 1 {
		bands = 1
	}
	buf := m.snapshot()
	n := len(buf) // Zweierpotenz

	re := make([]float64, n)
	im := make([]float64, n)
	for i := 0; i < n; i++ {
		w := 0.5 - 0.5*math.Cos(2*math.Pi*float64(i)/float64(n-1)) // Hann
		re[i] = buf[i] * w
	}
	fft(re, im)

	half := n / 2
	mags := make([]float64, half)
	for i := 0; i < half; i++ {
		mags[i] = math.Hypot(re[i], im[i])
	}

	out := make([]float64, bands)
	for b := 0; b < bands; b++ {
		lo := int(math.Pow(float64(half), float64(b)/float64(bands)))
		hi := int(math.Pow(float64(half), float64(b+1)/float64(bands)))
		if hi <= lo {
			hi = lo + 1
		}
		if hi > half {
			hi = half
		}
		sum, cnt := 0.0, 0
		for k := lo; k < hi; k++ {
			sum += mags[k]
			cnt++
		}
		v := 0.0
		if cnt > 0 {
			v = sum / float64(cnt)
		}
		out[b] = math.Log1p(v * 8) // weiche, hörbare Skala
	}

	max := 1e-4
	for _, v := range out {
		if v > max {
			max = v
		}
	}
	for i := range out {
		out[i] /= max
	}
	return out
}

// fft ist eine iterative In-Place Radix-2 FFT (len muss Zweierpotenz sein).
func fft(re, im []float64) {
	n := len(re)
	// Bit-Reversal-Permutation
	for i, j := 1, 0; i < n; i++ {
		bit := n >> 1
		for ; j&bit != 0; bit >>= 1 {
			j ^= bit
		}
		j ^= bit
		if i < j {
			re[i], re[j] = re[j], re[i]
			im[i], im[j] = im[j], im[i]
		}
	}
	for length := 2; length <= n; length <<= 1 {
		ang := -2 * math.Pi / float64(length)
		wr, wi := math.Cos(ang), math.Sin(ang)
		for i := 0; i < n; i += length {
			cr, ci := 1.0, 0.0
			for k := 0; k < length/2; k++ {
				a := i + k
				b := i + k + length/2
				vr := re[b]*cr - im[b]*ci
				vi := re[b]*ci + im[b]*cr
				re[b] = re[a] - vr
				im[b] = im[a] - vi
				re[a] += vr
				im[a] += vi
				cr, ci = cr*wr-ci*wi, cr*wi+ci*wr
			}
		}
	}
}
