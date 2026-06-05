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
	"sync/atomic"
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
	muted       bool

	// ended wird gesetzt, wenn der Stream von selbst endet (Drop/EOF), damit
	// die UI automatisch neu verbinden kann. Atomic, weil es aus der
	// Speaker-Callback-Goroutine gesetzt wird.
	ended atomic.Bool

	// gateThreshold steuert das Noise Gate (0 = deaktiviert).
	gateThreshold float64

	// metadata hält den zuletzt inline gelesenen Songtitel (ICY).
	// Eigener Mutex! Der icyReader schreibt aus der Speaker-Callback-Goroutine
	// (die bereits den Speaker-Lock hält). Würde das p.mu nehmen, entstünde
	// eine Lock-Order-Inversion mit applyVolumeInternal (p.mu -> speaker).
	metaMu   sync.RWMutex
	metadata string

	ctx      context.Context
	cancel   context.CancelFunc
	httpResp *http.Response
	buffered *bufferedStreamer
	meter    *meter

	// Auto-Reconnect: der Player verbindet sich selbst neu, wenn der Stream
	// endet ODER stehenbleibt (halb-tote Verbindung) — unabhängig davon, welches
	// Modul gerade aktiv ist.
	streamURL string
	stopped   bool          // true = absichtlich gestoppt (kein Reconnect)
	lastData  atomic.Int64  // UnixNano des letzten echten Audio-Reads
	reconnect chan struct{} // gepuffert (1), entprellt Reconnect-Signale
}

func NewPlayer() *Player {
	p := &Player{
		volumeLevel:   1.0,
		gateThreshold: 0.0, // Gate standardmäßig aus -> kein Eingriff ins Audio
		reconnect:     make(chan struct{}, 1),
	}
	go p.supervise()
	return p
}

// triggerReconnect meldet (nicht-blockierend) einen Reconnect-Wunsch.
func (p *Player) triggerReconnect() {
	select {
	case p.reconnect <- struct{}{}:
	default:
	}
}

// supervise verbindet bei einem Reconnect-Signal mit Backoff neu, solange der
// Player nicht absichtlich gestoppt (oder pausiert) wurde.
func (p *Player) supervise() {
	for range p.reconnect {
		p.mu.RLock()
		url, stopped, paused := p.streamURL, p.stopped, p.isPaused
		p.mu.RUnlock()
		if stopped || paused || url == "" {
			continue
		}
		backoff := time.Second
		for attempt := 0; attempt < 6; attempt++ {
			p.mu.RLock()
			abort := p.stopped || p.streamURL != url
			p.mu.RUnlock()
			if abort {
				break
			}
			time.Sleep(backoff)
			debugf("reconnect: attempt %d -> %s", attempt+1, url)
			if err := p.Play(url); err == nil {
				break
			}
			backoff *= 2
			if backoff > 15*time.Second {
				backoff = 15 * time.Second
			}
		}
	}
}

// GetStatus gibt thread-safe den aktuellen Status zurück
func (p *Player) GetStatus() (playing bool, paused bool, volume float64) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.isPlaying, p.isPaused, p.volumeLevel
}

// IsMuted gibt zurück, ob die Wiedergabe stummgeschaltet ist.
func (p *Player) IsMuted() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.muted
}

// ToggleMute schaltet stumm/laut und gibt den neuen Zustand zurück.
func (p *Player) ToggleMute() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.muted = !p.muted
	p.applyVolumeInternal()
	return p.muted
}

// SetVolume setzt die Lautstärke absolut (0..1), z.B. zum Wiederherstellen.
func (p *Player) SetVolume(level float64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if level < 0 {
		level = 0
	}
	if level > 1 {
		level = 1
	}
	p.volumeLevel = level
	p.applyVolumeInternal()
}

// Ended meldet, ob der Stream seit dem letzten Play von selbst geendet ist
// (Verbindungsabbruch). Die UI nutzt das für Auto-Reconnect.
func (p *Player) Ended() bool {
	return p.ended.Load()
}

// Spectrum liefert ein normalisiertes Frequenzspektrum (bands Werte, 0..1) aus
// dem aktuell laufenden Audio. nil, wenn nichts spielt.
func (p *Player) Spectrum(bands int) []float64 {
	p.mu.RLock()
	m := p.meter
	playing := p.isPlaying && !p.isPaused
	p.mu.RUnlock()
	if m == nil || !playing {
		return nil
	}
	return m.spectrum(bands)
}

// GetMetadata gibt den zuletzt inline gelesenen Titel zurück (kein Netzwerk).
func (p *Player) GetMetadata() string {
	p.metaMu.RLock()
	defer p.metaMu.RUnlock()
	return p.metadata
}

// setMetadata wird vom icyReader (Speaker-Goroutine) aufgerufen.
// Nutzt bewusst metaMu, NICHT p.mu (siehe Kommentar am Feld).
func (p *Player) setMetadata(title string) {
	p.metaMu.Lock()
	p.metadata = title
	p.metaMu.Unlock()
}

func (p *Player) Play(streamURL string) error {
	// WICHTIG: Während der blockierenden Arbeit (HTTP-Connect + mp3.Decode)
	// halten wir KEINEN p.mu Lock. Sonst blockiert GetStatus()/GetMetadata()
	// aus der UI-Update-Schleife -> die komplette TUI friert ein (Tasten tot).
	// Der Lock wird nur kurz gehalten, um den gemeinsamen Zustand zu setzen.

	// 1. Vorherigen Stream sauber beenden (eigener kurzer Lock).
	p.Stop()

	// 2. Neuen Context anlegen (kurzer Lock).
	p.ended.Store(false)
	p.lastData.Store(time.Now().UnixNano())
	p.mu.Lock()
	p.ctx, p.cancel = context.WithCancel(context.Background())
	ctx := p.ctx
	localCancel := p.cancel
	threshold := p.gateThreshold
	p.streamURL = streamURL
	p.stopped = false
	p.mu.Unlock()

	// 3. Blockierende Arbeit OHNE Lock --------------------------------------
	req, err := http.NewRequest("GET", streamURL, nil)
	if err != nil {
		return fmt.Errorf("request creation failed: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (TobiRadio-Go)")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Accept", "audio/mpeg, */*")
	req.Header.Set("Icy-MetaData", "1")
	// Body wird über ctx gesteuert (Abbruch via Stop()); Connect-/Header-
	// Timeout läuft über den Transport, damit Streaming nicht abgewürgt wird.
	req = req.WithContext(ctx)

	client := &http.Client{
		Transport: &http.Transport{
			DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
			ResponseHeaderTimeout: 5 * time.Second,
		},
	}

	debugf("Play: requesting %s", streamURL)
	resp, err := client.Do(req)
	if err != nil {
		debugf("Play: client.Do error: %v", err)
		return fmt.Errorf("connection failed (timeout oder ablehnung): %v", err)
	}

	contentType := resp.Header.Get("Content-Type")
	debugf("Play: status=%s content-type=%q icy-metaint=%q",
		resp.Status, contentType, resp.Header.Get("icy-metaint"))
	if !strings.Contains(contentType, "audio") &&
		!strings.Contains(contentType, "mpeg") &&
		!strings.Contains(contentType, "ogg") {
		resp.Body.Close()
		debugf("Play: rejected content-type %q", contentType)
		return fmt.Errorf("falsches Format: %s (Player kann nur MP3, keine M3U/PLS)", contentType)
	}

	var src io.Reader = bufio.NewReader(resp.Body)

	// ICY-Metadaten inline aus dem laufenden Stream lesen.
	if metaIntStr := resp.Header.Get("icy-metaint"); metaIntStr != "" {
		if metaInt, e := strconv.Atoi(metaIntStr); e == nil && metaInt > 0 {
			src = &icyReader{src: src, metaInt: metaInt, player: p}
		}
	}

	// Decode-Watchdog: mp3.Decode blockiert, wenn der Stream KEIN echtes MP3
	// ist (z.B. AAC/AAC+). Wir brechen den Body nach 10s ab -> Decode kehrt
	// mit Fehler zurück. Bei Erfolg beendet decodeDone den Watchdog sofort.
	decodeDone := make(chan struct{})
	go func() {
		select {
		case <-decodeDone:
			return
		case <-time.After(10 * time.Second):
			debugf("Play: decode watchdog fired (kein MP3-Stream?) -> abort")
			localCancel()
		}
	}()

	streamer, format, err := mp3.Decode(io.NopCloser(src))
	close(decodeDone)
	if err != nil {
		resp.Body.Close()
		debugf("Play: mp3.Decode error: %v", err)
		return fmt.Errorf("mp3 decode failed (kein Audio-Stream oder kein MP3?): %v", err)
	}
	debugf("Play: decoded OK, stream rate=%d Hz (speaker=%d Hz)", format.SampleRate, SampleRate)

	// Netzwerk/Decode vom Audio-Callback entkoppeln (siehe streamer.go).
	// 2 Sekunden Vorauspuffer in der Quell-Sample-Rate. Bei natürlichem Ende
	// (Stream-Drop) setzen wir das ended-Flag für den Auto-Reconnect.
	buffered := newBufferedStreamer(streamer, int(format.SampleRate.N(2*time.Second)),
		func() { // onEnd: Stream zu Ende -> Reconnect anstoßen
			p.ended.Store(true)
			p.triggerReconnect()
		},
		func() { // onData: echte Audiodaten gelesen -> Liveness-Zeitstempel
			p.lastData.Store(time.Now().UnixNano())
		})

	// Auf die feste Speaker-Rate resampeln, falls nötig.
	var audioStream beep.Streamer = buffered
	if format.SampleRate != SampleRate {
		debugf("Play: resampling %d -> %d", format.SampleRate, SampleRate)
		audioStream = beep.Resample(4, format.SampleRate, SampleRate, buffered)
	}

	// Meter für den Echtzeit-Visualizer (tappt das Quell-Audio vor Volume,
	// damit es unabhängig von Lautstärke/Mute reagiert). 1024 = Zweierpotenz
	// für die FFT.
	mtr := newMeter(audioStream, 1024)

	gate := &NoiseGate{
		Streamer:    mtr,
		Threshold:   threshold,
		holdSamples: SampleRate.N(200 * time.Millisecond),
	}

	// 4. Zustand unter kurzem Lock setzen -----------------------------------
	p.mu.Lock()
	// Wurden wir zwischenzeitlich gestoppt / neuer Play gestartet?
	if ctx.Err() != nil {
		p.mu.Unlock()
		buffered.Close()
		resp.Body.Close()
		debugf("Play: aborted before start (ctx done)")
		return fmt.Errorf("wiedergabe abgebrochen")
	}
	p.httpResp = resp
	p.buffered = buffered
	p.meter = mtr
	p.volume = &effects.Volume{Streamer: gate, Base: 2, Volume: 0, Silent: false}
	p.applyVolumeInternal()
	p.ctrl = &beep.Ctrl{Streamer: p.volume, Paused: false}
	ctrl := p.ctrl
	p.isPlaying = true
	p.isPaused = false
	p.mu.Unlock()

	// Clear() und Play() nehmen INTERN den Speaker-Lock -> NICHT wrappen.
	speaker.Clear()
	speaker.Play(ctrl)
	debugf("Play: speaker.Play issued, isPlaying=true")

	// Stall-Watchdog: erkennt halb-tote Verbindungen (kein EOF, aber keine
	// Daten mehr) und stößt einen Reconnect an.
	go func() {
		t := time.NewTicker(2 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				// Pausiert? Dann liest der Puffer absichtlich nicht -> kein Stall.
				// lastData frisch halten, damit es beim Fortsetzen keinen
				// Fehlalarm gibt.
				p.mu.RLock()
				paused := p.isPaused
				p.mu.RUnlock()
				if paused {
					p.lastData.Store(time.Now().UnixNano())
					continue
				}
				last := time.Unix(0, p.lastData.Load())
				if time.Since(last) > 10*time.Second {
					debugf("stall watchdog: no audio data for >10s -> reconnect")
					p.triggerReconnect()
					return
				}
			}
		}
	}()

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
	// WICHTIG: speaker.Clear() nimmt INTERN den Speaker-Lock. NICHT in
	// speaker.Lock()/Unlock() wrappen -> sonst Selbst-Deadlock (mu nicht
	// reentrant).
	speaker.Clear()

	if p.buffered != nil {
		p.buffered.Close()
		p.buffered = nil
	}
	p.meter = nil
	if p.httpResp != nil {
		p.httpResp.Body.Close()
		p.httpResp = nil
	}
	p.isPlaying = false
	p.isPaused = false
	p.ctrl = nil
	p.volume = nil
	// Als "absichtlich gestoppt" markieren -> kein Auto-Reconnect. Play() setzt
	// das danach wieder auf false.
	p.stopped = true
	// metadata über metaMu leeren (eigener Lock, siehe Feldkommentar).
	p.setMetadata("")
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

		if p.muted || p.volumeLevel <= 0.01 {
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
