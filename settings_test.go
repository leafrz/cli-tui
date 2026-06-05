package main

import (
	"strings"
	"testing"
)

// TestSettingsValues prüft, dass jede Einstellung einen Wert rendert und das
// Umschalten/Zyklen jeder Zeile nicht paniken.
func TestSettingsValues(t *testing.T) {
	m := newSettingsModule()
	m.width, m.height = 80, 24

	for i := 0; i < settingsCount; i++ {
		if m.value(i) == "" {
			t.Errorf("setting %q has empty value", settingsLabels[i])
		}
		m.cursor = i
		_ = m.change(1)  // weiterschalten
		_ = m.change(-1) // zurück
	}

	out := m.View(80, 24)
	if !strings.Contains(out, "settings") {
		t.Errorf("expected settings title in view")
	}
	if len(strings.Split(out, "\n")) != 24 {
		t.Errorf("expected 24 lines, got %d", len(strings.Split(out, "\n")))
	}
}

func TestCycleList(t *testing.T) {
	list := []string{"a", "b", "c"}
	if got := cycleList(list, "a", 1); got != "b" {
		t.Errorf("forward: got %q", got)
	}
	if got := cycleList(list, "a", -1); got != "c" {
		t.Errorf("wrap back: got %q", got)
	}
	if got := cycleList(list, "x", 1); got != "b" {
		t.Errorf("unknown current: got %q", got)
	}
}
