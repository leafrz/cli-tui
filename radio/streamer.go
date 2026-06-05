package radio

import (
	"sync"

	"github.com/faiface/beep"
)

// bufferedStreamer entkoppelt das (blockierende) Netzwerk-/Decode-Lesen vom
// Audio-Callback des Speakers.
//
// Problem: beep ruft Stream() aus der Audio-Callback-Goroutine auf und hält
// dabei den Speaker-Lock. Liest unsere Decoder-Kette direkt vom Netzwerk, dann
// blockiert der Callback bei einem leeren Socket WÄHREND er den Speaker-Lock
// hält -> jede UI-Aktion, die den Speaker-Lock braucht (Pause, Volume, Stop),
// friert ein -> die ganze TUI hängt.
//
// Lösung: Eine Hintergrund-Goroutine liest im Voraus aus dem inneren Streamer
// (Netzwerk) in einen Ring-Puffer. Stream() liefert nur aus dem Puffer und gibt
// bei Unterlauf Stille zurück - es blockiert NIE. Damit hält der Audio-Callback
// den Speaker-Lock immer nur kurz.
type bufferedStreamer struct {
	inner beep.Streamer

	mu      sync.Mutex
	cond    *sync.Cond
	buf     [][2]float64 // Ring-Puffer
	size    int
	head    int // Leseposition
	count   int // gefüllte Samples
	eof     bool
	closed  bool
	err     error
	onEnd   func() // einmalig bei natürlichem Stream-Ende
	onData  func() // bei jedem erfolgreichen Quell-Read (Liveness)
	endOnce sync.Once
}

// newBufferedStreamer startet sofort die Vorausles-Goroutine.
// capacity = Puffergröße in Samples (z.B. SampleRate.N(2*time.Second)).
// onEnd wird einmal aufgerufen, wenn der Stream von selbst endet (Drop/EOF).
// onData wird bei jedem erfolgreichen Quell-Read aufgerufen (für Stall-Erkennung).
func newBufferedStreamer(inner beep.Streamer, capacity int, onEnd, onData func()) *bufferedStreamer {
	if capacity < 1 {
		capacity = 1
	}
	b := &bufferedStreamer{
		inner:  inner,
		buf:    make([][2]float64, capacity),
		size:   capacity,
		onEnd:  onEnd,
		onData: onData,
	}
	b.cond = sync.NewCond(&b.mu)
	go b.fill()
	return b
}

// signalEnd ruft onEnd genau einmal auf (außerhalb von Locks bedenkenlos).
func (b *bufferedStreamer) signalEnd() {
	if b.onEnd != nil {
		b.endOnce.Do(b.onEnd)
	}
}

// fill läuft im Hintergrund und darf blockieren (es ist NICHT der Audio-Thread).
func (b *bufferedStreamer) fill() {
	tmp := make([][2]float64, 1024)
	for {
		// Warten, bis Platz im Puffer ist (Backpressure).
		b.mu.Lock()
		for b.count > b.size-len(tmp) && !b.closed {
			b.cond.Wait()
		}
		if b.closed {
			b.mu.Unlock()
			return
		}
		b.mu.Unlock()

		// Blockierender Read aus dem Netzwerk/Decoder - ohne Lock.
		n, ok := b.inner.Stream(tmp)
		if n > 0 && b.onData != nil {
			b.onData() // Liveness-Signal (Stall-Erkennung)
		}

		b.mu.Lock()
		for i := 0; i < n; i++ {
			tail := (b.head + b.count) % b.size
			b.buf[tail] = tmp[i]
			b.count++
		}
		if !ok {
			b.eof = true
			b.err = b.inner.Err()
			b.cond.Broadcast()
			b.mu.Unlock()
			return
		}
		b.cond.Broadcast()
		b.mu.Unlock()
	}
}

// Stream liefert aus dem Puffer und blockiert nie. Bei Unterlauf -> Stille.
func (b *bufferedStreamer) Stream(samples [][2]float64) (n int, ok bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i := range samples {
		if b.count > 0 {
			samples[i] = b.buf[b.head]
			b.head = (b.head + 1) % b.size
			b.count--
		} else if b.eof {
			// Quelle zu Ende und Puffer leer -> Wiedergabe beenden.
			b.cond.Broadcast()
			b.signalEnd()
			return i, i > 0
		} else {
			// Unterlauf: Stille ausgeben statt zu blockieren.
			samples[i] = [2]float64{0, 0}
		}
	}
	b.cond.Broadcast() // Platz frei -> fill() aufwecken
	return len(samples), true
}

func (b *bufferedStreamer) Err() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.err
}

// Close stoppt die Hintergrund-Goroutine.
func (b *bufferedStreamer) Close() {
	b.mu.Lock()
	b.closed = true
	b.cond.Broadcast()
	b.mu.Unlock()
}
