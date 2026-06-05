package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// weatherMsg trägt die formatierte Wetterzeile (leer = nicht verfügbar).
type weatherMsg struct{ text string }

func weatherCmd() tea.Cmd {
	return func() tea.Msg {
		text, err := fetchWeather()
		if err != nil {
			return weatherMsg{text: ""}
		}
		return weatherMsg{text: text}
	}
}

// fetchWeather: IP-Geolokalisierung (ip-api, ohne Key) + Open-Meteo (ohne Key).
func fetchWeather() (string, error) {
	lat, lon, city, err := geolocate()
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
	return fmt.Sprintf("%s  %.0f°C · %s · %s", icon, data.Current.Temp, desc, city), nil
}

func geolocate() (lat, lon float64, city string, err error) {
	client := http.Client{Timeout: 5 * time.Second}
	// ip-api: kostenloser Plan nur über http.
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
