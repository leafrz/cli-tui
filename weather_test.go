package main

import (
	"testing"

	"github.com/leafrz/dashboard/internal/config"
)

// TestWeather prüft, dass Geolokalisierung + Open-Meteo eine Zeile liefern.
//
//	go test . -run TestWeather -v
func TestWeather(t *testing.T) {
	if testing.Short() {
		t.Skip("network test; skipped with -short")
	}
	// auto (IP-based)
	if text, err := fetchWeather(config.WeatherConfig{Mode: "auto"}); err != nil {
		t.Skipf("weather unavailable (offline?): %v", err)
	} else {
		t.Logf("auto: %q", text)
	}

	// manual city -> geocoded via Open-Meteo (no ip-api)
	text, err := fetchWeather(config.WeatherConfig{Mode: "manual", City: "Vienna"})
	if err != nil {
		t.Skipf("manual weather unavailable: %v", err)
	}
	t.Logf("manual(Vienna): %q", text)
	if text == "" {
		t.Errorf("expected a non-empty weather line for manual city")
	}
}
