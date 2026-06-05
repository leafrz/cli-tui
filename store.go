package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// persistedState ist der auf Platte gespeicherte Zustand.
type persistedState struct {
	Favorites   []station    `json:"favorites"`
	LastVolume  float64      `json:"last_volume"`
	LastStation *station     `json:"last_station,omitempty"`
	Header      headerConfig `json:"header"`
	Theme       string       `json:"theme"`
}

var storeMu sync.Mutex

// statePath liefert den Pfad zur state.json im OS-Konfigverzeichnis.
// Beispiel Windows: %AppData%\lofi-radio\state.json
func statePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "lofi-radio", "state.json"), nil
}

// loadState lädt den persistenten Zustand. Fehlt die Datei, gibt es einen
// sinnvollen Default zurück (kein Fehler).
func loadState() persistedState {
	def := persistedState{LastVolume: 1.0, Header: defaultHeaderConfig(), Theme: themes[0].name}

	path, err := statePath()
	if err != nil {
		return def
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return def // Datei existiert (noch) nicht
	}
	var s persistedState
	if err := json.Unmarshal(data, &s); err != nil {
		return def // korrupte Datei -> Default
	}
	if s.LastVolume <= 0 {
		s.LastVolume = 1.0
	}
	s.Header = s.Header.withDefaults()
	if s.Theme == "" {
		s.Theme = themes[0].name
	}
	return s
}

// saveState schreibt den Zustand atomar (temp + rename).
func saveState(s persistedState) error {
	storeMu.Lock()
	defer storeMu.Unlock()
	return saveStateUnlocked(s)
}

// saveStateUnlocked schreibt ohne Lock (Aufrufer muss storeMu halten).
func saveStateUnlocked(s persistedState) error {
	path, err := statePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// updateState lädt den aktuellen Zustand, wendet mut an und speichert wieder.
// So überschreibt z.B. das Speichern der Favoriten nicht die Header-Config
// (und umgekehrt). Läuft komplett unter storeMu.
func updateState(mut func(*persistedState)) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	s := loadState() // liest Datei + füllt Defaults (kein Lock im Innern)
	mut(&s)
	return saveStateUnlocked(s)
}
