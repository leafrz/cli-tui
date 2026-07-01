package audio

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/wav"
)

// PlayRandomSFX spielt eine zufällige .mp3/.wav aus dir über den laufenden
// Speaker-Mixer ab (mischt sich über den Radio-Stream). Fehlt der Ordner oder
// ist er leer, ist das kein harter Fehler-Fall für den Aufrufer gedacht —
// der Fehler wird zurückgegeben, Playback läuft ungestört weiter.
func PlayRandomSFX(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		switch strings.ToLower(filepath.Ext(e.Name())) {
		case ".mp3", ".wav":
			files = append(files, e.Name())
		}
	}
	if len(files) == 0 {
		return fmt.Errorf("no .mp3/.wav files in %s", dir)
	}
	return PlaySFX(filepath.Join(dir, files[rand.Intn(len(files))]))
}

// PlaySFX dekodiert eine einzelne Sound-Datei und mischt sie in den Speaker.
// Der Speaker muss initialisiert sein (passiert in main).
func PlaySFX(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	var (
		str    beep.StreamSeekCloser
		format beep.Format
	)
	switch strings.ToLower(filepath.Ext(path)) {
	case ".mp3":
		str, format, err = mp3.Decode(f)
	case ".wav":
		str, format, err = wav.Decode(f)
	default:
		f.Close()
		return fmt.Errorf("unsupported sound format: %s", path)
	}
	if err != nil {
		f.Close()
		return err
	}

	var s beep.Streamer = str
	if format.SampleRate != SampleRate {
		s = beep.Resample(3, format.SampleRate, SampleRate, str)
	}
	speaker.Play(beep.Seq(s, beep.Callback(func() { str.Close() })))
	return nil
}
