package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// weatherMsg trägt die formatierte Wetterzeile (leer = nicht verfügbar/aus).
type weatherMsg struct{ text string }

func weatherCmd(cfg weatherConfig) tea.Cmd {
	return func() tea.Msg {
		if cfg.Mode == "off" {
			return weatherMsg{text: ""}
		}
		text, err := fetchWeather(cfg)
		if err != nil {
			return weatherMsg{text: ""}
		}
		return weatherMsg{text: text}
	}
}

// fetchWeather ermittelt den Standort gemäß cfg und holt das aktuelle Wetter
// von Open-Meteo (immer ohne API-Key).
func fetchWeather(cfg weatherConfig) (string, error) {
	var lat, lon float64
	var city string
	var err error

	switch cfg.Mode {
	case "manual":
		if cfg.City != "" {
			// Geokodierung über Open-Meteo selbst -> kein ip-api nötig.
			lat, lon, city, err = geocodeCity(cfg.City)
		} else {
			lat, lon = cfg.Lat, cfg.Lon // feste Koordinaten, ohne Ortsnamen
		}
	default: // "auto"
		lat, lon, city, err = geolocate()
	}
	if err != nil {
		return "", err
	}

	url := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&current=temperature_2m,weather_code",
		lat, lon)

	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var data struct {
		Current struct {
			Temp float64 `json:"temperature_2m"`
			Code int     `json:"weather_code"`
		} `json:"current"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	icon, desc := weatherCodeInfo(data.Current.Code)
	if city != "" {
		return fmt.Sprintf("%s  %.0f°C · %s · %s", icon, data.Current.Temp, desc, city), nil
	}
	return fmt.Sprintf("%s  %.0f°C · %s", icon, data.Current.Temp, desc), nil
}

// geolocate ermittelt den Standort grob über die öffentliche IP (ip-api, ohne Key).
func geolocate() (lat, lon float64, city string, err error) {
	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://ip-api.com/json/?fields=status,lat,lon,city")
	if err != nil {
		return 0, 0, "", err
	}
	defer resp.Body.Close()

	var data struct {
		Status string  `json:"status"`
		Lat    float64 `json:"lat"`
		Lon    float64 `json:"lon"`
		City   string  `json:"city"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, 0, "", err
	}
	if data.Status != "success" {
		return 0, 0, "", fmt.Errorf("geolocation failed")
	}
	return data.Lat, data.Lon, data.City, nil
}

// geocodeCity wandelt einen Ortsnamen über Open-Meteo in Koordinaten um.
func geocodeCity(name string) (lat, lon float64, city string, err error) {
	u := "https://geocoding-api.open-meteo.com/v1/search?count=1&language=en&name=" +
		url.QueryEscape(name)

	client := http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(u)
	if err != nil {
		return 0, 0, "", err
	}
	defer resp.Body.Close()

	var data struct {
		Results []struct {
			Lat     float64 `json:"latitude"`
			Lon     float64 `json:"longitude"`
			Name    string  `json:"name"`
			Country string  `json:"country_code"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, 0, "", err
	}
	if len(data.Results) == 0 {
		return 0, 0, "", fmt.Errorf("city %q not found", name)
	}
	r := data.Results[0]
	return r.Lat, r.Lon, r.Name, nil
}

// weatherCodeInfo bildet WMO-Codes auf Icon + Kurzbeschreibung ab.
func weatherCodeInfo(code int) (icon, desc string) {
	switch {
	case code == 0:
		return "☀", "clear"
	case code <= 2:
		return "⛅", "partly cloudy"
	case code == 3:
		return "☁", "overcast"
	case code <= 48:
		return "🌫", "fog"
	case code <= 57:
		return "🌦", "drizzle"
	case code <= 67:
		return "🌧", "rain"
	case code <= 77:
		return "❄", "snow"
	case code <= 82:
		return "🌧", "showers"
	case code <= 86:
		return "🌨", "snow showers"
	default:
		return "⛈", "thunderstorm"
	}
}
