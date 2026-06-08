package radio

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/charmbracelet/bubbles/list"

	"github.com/leafrz/dashboard/internal/config"
)

// SearchStations fragt die Radio-Browser-API ab.
func SearchStations(query string) []list.Item {
	safeQuery := url.QueryEscape(query)

	apiURL := fmt.Sprintf("https://de1.api.radio-browser.info/json/stations/search?name=%s&order=clickcount&codec=mp3&limit=20", safeQuery)
	if query == "" {
		apiURL = "https://de1.api.radio-browser.info/json/stations/search?countrycode=DE&order=clickcount&codec=mp3&limit=20"
	}

	client := http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return []list.Item{config.Station{Name: "API Fehler", Tags: err.Error()}}
	}
	defer resp.Body.Close()

	var stations []config.Station
	if err := json.NewDecoder(resp.Body).Decode(&stations); err != nil {
		return []list.Item{config.Station{Name: "JSON Fehler", Tags: err.Error()}}
	}

	items := make([]list.Item, len(stations))
	for i, s := range stations {
		items[i] = s
	}

	if len(items) == 0 {
		return []list.Item{config.Station{Name: "Keine Ergebnisse", Tags: "Versuche einen anderen Suchbegriff"}}
	}

	return items
}
