package radio

import (
	"github.com/charmbracelet/bubbles/list"

	"github.com/leafrz/dashboard/internal/config"
)

// favFlash liefert die Toast-Nachricht für das Favoriten-Umschalten.
func favFlash(added bool) string {
	if added {
		return "★ added to favorites"
	}
	return "☆ removed from favorites"
}

// isFavorite prüft anhand der Stream-URL, ob ein Sender Favorit ist.
func isFavorite(favs []config.Station, url string) bool {
	for _, f := range favs {
		if f.StreamURL == url {
			return true
		}
	}
	return false
}

// markFavorites setzt das Favorite-Flag auf allen station-Items.
func markFavorites(items []list.Item, favs []config.Station) {
	for i, it := range items {
		if s, ok := it.(config.Station); ok {
			s.Favorite = isFavorite(favs, s.StreamURL)
			items[i] = s
		}
	}
}

// toggleFavorite fügt einen Sender hinzu oder entfernt ihn. Gibt die neue
// Favoritenliste und den neuen Zustand zurück.
func toggleFavorite(favs []config.Station, s config.Station) (out []config.Station, nowFav bool) {
	for i, f := range favs {
		if f.StreamURL == s.StreamURL {
			// entfernen
			return append(favs[:i:i], favs[i+1:]...), false
		}
	}
	// hinzufügen (ohne transientes Flag)
	s.Favorite = false
	return append(favs, s), true
}

// favoritesAsItems wandelt die Favoritenliste in markierte list.Items um.
func favoritesAsItems(favs []config.Station) []list.Item {
	if len(favs) == 0 {
		return []list.Item{config.Station{Name: "noch keine favoriten", Tags: "mit 'f' in der liste hinzufügen"}}
	}
	items := make([]list.Item, len(favs))
	for i, f := range favs {
		f.Favorite = true
		items[i] = f
	}
	return items
}
