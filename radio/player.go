// radio/player.go
package radio

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

// SampleRate ist die feste Rate, mit der der Speaker EINMALIG initialisiert wird.
// Streams mit abweichender Rate werden zur Laufzeit resampled. Das vermeidet
// das fragile Re-Init des Audio-Geräts (vor allem unter Windows/WASAPI).
const SampleRate beep.SampleRate = 44100

// Player steuert den Audio-Stream und den Status
type Player struct {
	mu          sync.RWMutex
	ctrl        *beep.Ctrl
	volume      *effects.Volume
	volumeLevel float64
	isPlaying   bool
	isPaused    bool

	// gateThreshold steuert das Noise Gate (0 = deaktiviert).
	gateThreshold float64

	// metadata hält den zuletzt inline gelesenen Songtitel (ICY).
	metadata string

	ctx      context.Context
	cancel   context.CancelFunc
	httpResp *http.Response
}

func NewPlayer() *Player {
	return &Player{
		volumeLevel:   1.0,
		gateThreshold: 0.0, // Gate standardmäßig aus -> kein Eingriff ins Audio
	}
}

// GetStatus gibt thread-safe den aktuellen Status zurück
func (p *Player) GetStatus() (playing bool, paused bool, volume float64) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.isPlaying, p.isPaused, p.volumeLevel
}

// GetMetadata gibt den zuletzt inline gelesenen Titel zurück (kein Netzwerk).
func (p *Player) GetMetadata() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.metadata
}

// setMetadata wird vom icyReader (Speaker-Goroutine) aufgerufen.
func (p *Player) setMetadata(title string) {
	p.mu.Lock()
	p.metadata = title
	p.mu.Unlock()
}

func (p *Player) Play(streamURL string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Vorherigen Stream sicher beenden
	p.stopInternal()

	p.ctx, p.cancel = context.WithCancel(context.Background())

	req, err := http.NewRequest("GET", streamURL, nil)
	if err != nil {
		return fmt.Errorf("request creation failed: %v", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (TobiRadio-Go)")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Accept", "audio/mpeg, */*")
	// ICY-Metadaten direkt im Audio-Stream anfordern.
	req.Header.Set("Icy-MetaData", "1")

	// Wichtig: Der Body wird über p.ctx gesteuert (Abbruch via Stop()),
	// NICHT über einen kurzlebigen Timeout-Context. Der Verbindungs-/Header-
	// Timeout läuft stattdessen über den Transport, damit Streaming nicht
	// nach wenigen Sekunden vom Context-Cancel abgewürgt wird.
	req = req.WithContext(p.ctx)

	client := &http.Client{
		Transport: &http.Transport{
			DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
			ResponseHeaderTimeout: 5 * time.Second,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed (timeout oder ablehnung): %v", err)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "audio") &&
		!strings.Contains(contentType, "mpeg") &&
		!strings.Contains(contentType, "ogg") {
		resp.Body.Close()
		return fmt.Errorf("falsches Format: %s (Player kann nur MP3, keine M3U/PLS)", contentType)
	}

	p.httpResp = resp
	p.metadata = ""

	var src io.Reader = bufio.NewReader(resp.Body)

	// ICY-Metadaten inline aus dem laufenden Stream lesen (keine zweite
	// HTTP-Verbindung mehr). Nur wenn der Server icy-metaint anbietet.
	if metaIntStr := resp.Header.Get("icy-metaint"); metaIntStr != "" {
		if metaInt, e := strconv.Atoi(metaIntStr); e == nil && metaInt > 0 {
			src = &icyReader{src: src, metaInt: metaInt, player: p}
		}
	}

	streamer, format, err := mp3.Decode(io.NopCloser(src))
	if err != nil {
		resp.Body.Close()
		return fmt.Errorf("mp3 decode failed (kein Audio-Stream?): %v", err)
	}

	// Auf die feste Speaker-Rate resampeln, falls nötig.
	var audioStream beep.Streamer = streamer
	if format.SampleRate != SampleRate {
		audioStream = beep.Resample(4, format.SampleRate, SampleRate, streamer)
	}

	// Noise Gate -> Volume -> Ctrl
	gate := &NoiseGate{
		Streamer:    audioStream,
		Threshold:   p.gateThreshold,
		holdSamples: SampleRate.N(200 * time.Millisecond),
	}

	p.volume = &effects.Volume{
		Streamer: gate,
		Base:     2,
		Volume:   0,
		Silent:   false,
	}
	p.applyVolumeInternal()

	p.ctrl = &beep.Ctrl{Streamer: p.volume, Paused: false}

	speaker.Lock()
	speaker.Clear()
	speaker.Play(p.ctrl)
	speaker.Unlock()

	p.isPlaying = true
	p.isPaused = false

	return nil
}

func (p *Player) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopInternal()
}

// stopInternal beendet den Stream ohne Mutex-Lock (von Play und Stop genutzt).
func (p *Player) stopInternal() {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	// Speaker ist global initialisiert -> wir leeren nur die Wiedergabe.
	speaker.Lock()
	speaker.Clear()
	speaker.Unlock()

	if p.httpResp != nil {
		p.httpResp.Body.Close()
		p.httpResp = nil
	}
	p.isPlaying = false
	p.isPaused = false
	p.metadata = ""
	p.ctrl = nil
	p.volume = nil
}

func (p *Player) TogglePause() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.ctrl != nil {
		speaker.Lock()
		p.ctrl.Paused = !p.ctrl.Paused
		p.isPaused = p.ctrl.Paused
		speaker.Unlock()
		return p.isPaused
	}
	return false
}

// AdjustVolume ändert die Lautstärke relativ (delta z.B. 0.1 oder -0.1)
func (p *Player) AdjustVolume(delta float64) float64 {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.volumeLevel += delta
	if p.volumeLevel > 1.0 {
		p.volumeLevel = 1.0
	}
	if p.volumeLevel < 0.0 {
		p.volumeLevel = 0.0
	}

	p.applyVolumeInternal()
	return p.volumeLevel
}

// SetGateThreshold setzt die Noise-Gate-Schwelle (0 = aus). Wirkt sofort auf
// einen laufenden Stream.
func (p *Player) SetGateThreshold(t float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if t < 0 {
		t = 0
	}
	p.gateThreshold = t
}

// applyVolumeInternal berechnet die logarithmische Lautstärke.
// Muss innerhalb eines p.mu Locks aufgerufen werden.
func (p *Player) applyVolumeInternal() {
	if p.volume != nil {
		speaker.Lock()
		defer speaker.Unlock()

		if p.volumeLevel <= 0.01 {
			p.volume.Silent = true
		} else {
			p.volume.Silent = false
			// Logarithmische Skalierung (perzeptiv linear).
			p.volume.Volume = math.Log2(p.volumeLevel)
		}
	}
}

// --- ICY Metadaten Reader ---

// icyReader sitzt zwischen HTTP-Body und mp3-Decoder. Er liefert reine
// Audio-Bytes an den Decoder und fängt an den ICY-Intervallgrenzen die
// Metadaten-Blöcke ab.
type icyReader struct {
	src     io.Reader
	metaInt int
	counter int // gelesene Audio-Bytes seit dem letzten Metadaten-Block
	player  *Player
}

func (r *icyReader) Read(p []byte) (int, error) {
	if r.metaInt <= 0 {
		return r.src.Read(p)
	}

	// Steht der nächste Metadaten-Block an?
	if r.counter >= r.metaInt {
		if err := r.readMeta(); err != nil {
			return 0, err
		}
		r.counter = 0
	}

	// Lesevorgang auf die verbleibenden Audio-Bytes bis zur nächsten
	// Metadaten-Grenze begrenzen.
	remaining := r.metaInt - r.counter
	if len(p) > remaining {
		p = p[:remaining]
	}

	n, err := r.src.Read(p)
	r.counter += n
	return n, err
}

func (r *icyReader) readMeta() error {
	var lenByte [1]byte
	if _, err := io.ReadFull(r.src, lenByte[:]); err != nil {
		return err
	}

	metaLen := int(lenByte[0]) * 16
	if metaLen == 0 {
		return nil // leerer Block -> kein Titel-Update
	}

	buf := make([]byte, metaLen)
	if _, err := io.ReadFull(r.src, buf); err != nil {
		return err
	}

	if title := parseICYTitle(string(buf)); title != "" {
		r.player.setMetadata(title)
	}
	return nil
}

// parseICYTitle extrahiert den Songtitel aus einem ICY-Metadaten-Block.
// Format: StreamTitle='Song Name';
func parseICYTitle(meta string) string {
	const tag = "StreamTitle='"
	start := strings.Index(meta, tag)
	if start == -1 {
		return ""
	}
	start += len(tag)

	end := strings.Index(meta[start:], "';")
	if end == -1 {
		// Manche Streams beenden nur mit einfachem '.
		end = strings.Index(meta[start:], "'")
		if end == -1 {
			return ""
		}
	}
	return strings.TrimSpace(meta[start : start+end])
}

// --- Noise Gate ---

// NoiseGate dämpft Samples unterhalb einer Amplituden-Schwelle auf Stille.
// Mit holdSamples wird ein kurzes "Nachlaufen" nach dem letzten Signal
// erlaubt, damit das Gate bei kurzen Pausen nicht hörbar "atmet".
// Threshold == 0 deaktiviert das Gate vollständig.
type NoiseGate struct {
	Streamer    beep.Streamer
	Threshold   float64
	holdSamples int
	held        int
}

func (ng *NoiseGate) Stream(samples [][2]float64) (n int, ok bool) {
	n, ok = ng.Streamer.Stream(samples)
	if ng.Threshold <= 0 {
		return n, ok
	}

	for i := range samples[:n] {
		l := math.Abs(samples[i][0])
		r := math.Abs(samples[i][1])

		if l < ng.Threshold && r < ng.Threshold {
			if ng.held <= 0 {
				samples[i] = [2]float64{0, 0} // stummschalten
			} else {
				ng.held--
			}
		} else {
			ng.held = ng.holdSamples // Signal da -> Hold zurücksetzen
		}
	}
	return n, ok
}

func (ng *NoiseGate) Err() error {
	return ng.Streamer.Err()
}
