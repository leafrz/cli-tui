package config

// Station ist ein Radiosender. Erfüllt strukturell list.Item (Title/Description/
// FilterValue), ohne dass config das bubbles/list-Paket importieren muss.
type Station struct {
	Name      string `json:"name"`
	StreamURL string `json:"url_resolved"`
	Tags      string `json:"tags"`
	Country   string `json:"country"`

	// Favorite ist ein transienter UI-Marker (nicht persistiert).
	Favorite bool `json:"-"`
}

func (s Station) Title() string {
	if s.Favorite {
		return "★ " + s.Name
	}
	return s.Name
}

func (s Station) Description() string {
	if s.Tags == "" {
		return "Internet Radio (" + s.Country + ")"
	}
	return s.Tags
}

func (s Station) FilterValue() string { return s.Name }
