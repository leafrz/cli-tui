package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/charmbracelet/bubbles/list"
)

// Station implementiert list.Item
type station struct {
	Name      string `json:"name"`
	StreamURL string `json:"url_resolved"`
	Tags      string `json:"tags"` // <--- UMBENANNT von 'Description' zu 'Tags'
	Country   string `json:"country"`
}

// Interface Implementierung für die Liste
func (s station) Title() string { return s.Name }

// Hier gab es den Konflikt: Die Methode heißt Description, also darf das Feld oben nicht so heißen.
func (s station) Description() string {
	if s.Tags == "" { // <--- Hier greifen wir auf das umbenannte Feld zu
		return "Internet Radio (" + s.Country + ")"
	}
	return s.Tags // <--- Und hier auch
}

func (s station) FilterValue() string { return s.Name }

// SearchStations fragt die API ab
func SearchStations(query string) []list.Item {
	// URL Encoding ist wichtig für Leerzeichen etc.
	safeQuery := url.QueryEscape(query)

	// Suche nach Name, sortiert nach Votes, nur MP3
	apiURL := fmt.Sprintf("https://de1.api.radio-browser.info/json/stations/search?name=%s&order=clickcount&codec=mp3&limit=20", safeQuery)

	// Falls Query leer ist, holen wir einfach die Top-Charts
	if query == "" {
		apiURL = "https://de1.api.radio-browser.info/json/stations/search?countrycode=DE&order=clickcount&codec=mp3&limit=20"
	}

	client := http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return []list.Item{station{Name: "API Fehler", Tags: err.Error()}}
	}
	defer resp.Body.Close()

	var stations []station
	if err := json.NewDecoder(resp.Body).Decode(&stations); err != nil {
		return []list.Item{station{Name: "JSON Fehler", Tags: err.Error()}}
	}

	// Konvertieren in list.Item Slice
	items := make([]list.Item, len(stations))
	for i, s := range stations {
		items[i] = s
	}

	if len(items) == 0 {
		return []list.Item{station{Name: "Keine Ergebnisse", Tags: "Versuche einen anderen Suchbegriff"}}
	}

	return items
}
