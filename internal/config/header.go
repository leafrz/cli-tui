package config

// Header-Modi
const (
	HeaderStatic  = "static"  // fester eigener Text
	HeaderRotate  = "rotate"  // rotiert durch Taglines
	HeaderMarquee = "marquee" // Lauftext (Ticker)
	HeaderContext = "context" // Live-Status des aktiven Moduls (scrollt)
)

// HeaderModes definiert die Reihenfolge beim Durchschalten.
var HeaderModes = []string{HeaderStatic, HeaderRotate, HeaderMarquee, HeaderContext}

// HeaderConfig ist der persistente Header-Zustand.
type HeaderConfig struct {
	Mode     string   `json:"mode"`
	Text     string   `json:"text"`
	Taglines []string `json:"taglines"`
}

func DefaultHeaderConfig() HeaderConfig {
	return HeaderConfig{
		Mode: HeaderStatic,
		Text: "lofi.radio",
		Taglines: []string{
			"˗ˏˋ warm static & late nights ˎˊ˗",
			"˗ˏˋ tapes, rain & neon ˎˊ˗",
			"˗ˏˋ 3am study sessions ˎˊ˗",
			"˗ˏˋ slow mornings ˎˊ˗",
		},
	}
}

// WithDefaults füllt fehlende Felder mit sinnvollen Defaults auf.
func (h HeaderConfig) WithDefaults() HeaderConfig {
	d := DefaultHeaderConfig()
	if h.Mode == "" {
		h.Mode = d.Mode
	}
	if h.Text == "" {
		h.Text = d.Text
	}
	if len(h.Taglines) == 0 {
		h.Taglines = d.Taglines
	}
	return h
}

// Next schaltet den Modus weiter (static -> rotate -> marquee -> context -> …).
func (h HeaderConfig) Next() HeaderConfig {
	cur := 0
	for i, m := range HeaderModes {
		if m == h.Mode {
			cur = i
			break
		}
	}
	h.Mode = HeaderModes[(cur+1)%len(HeaderModes)]
	return h
}

// Animated meldet, ob der aktuelle Modus eine laufende Animation braucht.
func (h HeaderConfig) Animated() bool {
	return h.Mode == HeaderRotate || h.Mode == HeaderMarquee || h.Mode == HeaderContext
}
