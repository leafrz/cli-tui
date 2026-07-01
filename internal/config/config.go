package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// State ist der auf Platte gespeicherte Zustand.
type State struct {
	Favorites   []Station     `json:"favorites"`
	LastVolume  float64       `json:"last_volume"`
	LastStation *Station      `json:"last_station,omitempty"`
	Header      HeaderConfig  `json:"header"`
	Theme       string        `json:"theme"`
	Weather     WeatherConfig `json:"weather"`
	Ambient     AmbientConfig `json:"ambient"`
	Todos       []TodoItem    `json:"todos"`
	Hotfixes    int           `json:"hotfix_count"`         // Kiosk: Meme-Hotfix-Counter
	HotfixLog   []time.Time   `json:"hotfix_log,omitempty"` // Zeitstempel je Hotfix (für Streak)
}

// AmbientConfig merkt sich Ambient-Vorlieben. Bools sind so gewählt, dass der
// Zero-Value die sinnvollen Defaults ergibt (Uhr an, 24h, kein Auto-Rotate,
// Idle-Screensaver an mit 120s).
type AmbientConfig struct {
	Scene     string `json:"scene"`
	HideClock bool   `json:"hide_clock"`
	Clock12   bool   `json:"clock12"`
	Rotate    bool   `json:"rotate"`
	IdleOff   bool   `json:"idle_off"`
	IdleSecs  int    `json:"idle_secs"` // <=0 -> 120
}

// IdleTimeout liefert die Inaktivitätsdauer bis zum Auto-Screensaver (0 = aus).
func (c AmbientConfig) IdleTimeout() time.Duration {
	if c.IdleOff {
		return 0
	}
	s := c.IdleSecs
	if s <= 0 {
		s = 120
	}
	return time.Duration(s) * time.Second
}

// WeatherConfig steuert die Standortquelle für das Wetter.
type WeatherConfig struct {
	Mode string  `json:"mode"` // "auto" (IP), "manual" (City/Lat+Lon), "off"
	City string  `json:"city"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
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

// Load lädt den persistenten Zustand. Fehlt die Datei, gibt es einen
// sinnvollen Default zurück (kein Fehler).
func Load() State {
	def := State{
		LastVolume: 1.0,
		Header:     DefaultHeaderConfig(),
		Theme:      "lofi", // = themes[0]; hier hartkodiert, damit config nicht von ui abhängt
		Weather:    WeatherConfig{Mode: "auto"},
	}

	path, err := statePath()
	if err != nil {
		return def
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return def // Datei existiert (noch) nicht
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return def // korrupte Datei -> Default
	}
	if s.LastVolume <= 0 {
		s.LastVolume = 1.0
	}
	s.Header = s.Header.WithDefaults()
	if s.Theme == "" {
		s.Theme = "lofi"
	}
	if s.Weather.Mode == "" {
		s.Weather.Mode = "auto"
	}
	return s
}

// Save schreibt den Zustand atomar (temp + rename).
func Save(s State) error {
	storeMu.Lock()
	defer storeMu.Unlock()
	return saveUnlocked(s)
}

// saveUnlocked schreibt ohne Lock (Aufrufer muss storeMu halten).
func saveUnlocked(s State) error {
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

// Update lädt den aktuellen Zustand, wendet mut an und speichert wieder.
// So überschreibt z.B. das Speichern der Favoriten nicht die Header-Config
// (und umgekehrt). Läuft komplett unter storeMu.
func Update(mut func(*State)) error {
	storeMu.Lock()
	defer storeMu.Unlock()

	s := Load() // liest Datei + füllt Defaults (kein Lock im Innern)
	mut(&s)
	return saveUnlocked(s)
}
