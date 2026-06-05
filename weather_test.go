package main

import "testing"

// TestWeather prüft, dass Geolokalisierung + Open-Meteo eine Zeile liefern.
//
//	go test . -run TestWeather -v
func TestWeather(t *testing.T) {
	text, err := fetchWeather()
	if err != nil {
		t.Skipf("weather unavailable (offline?): %v", err)
	}
	t.Logf("weather line: %q", text)
	if text == "" {
		t.Errorf("expected a non-empty weather line")
	}
}
