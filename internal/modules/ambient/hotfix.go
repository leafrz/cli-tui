package ambient

import (
	"fmt"
	"time"
)

// weekStart liefert Montag 00:00 der Woche von t (lokale Zeit).
func weekStart(t time.Time) time.Time {
	wd := int(t.Weekday())
	if wd == 0 {
		wd = 7 // Sonntag ans Wochenende
	}
	y, m, d := t.AddDate(0, 0, -(wd - 1)).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// hotfixStreakWeeks zählt aufeinanderfolgende Wochen mit >=1 Hotfix, rückwärts
// ab der aktuellen Woche. Die laufende Woche darf noch leer sein, solange die
// Vorwoche getroffen wurde (Streak "lebt noch"); danach bricht er.
func hotfixStreakWeeks(log []time.Time, now time.Time) int {
	if len(log) == 0 {
		return 0
	}
	weeks := make(map[time.Time]bool, len(log))
	for _, t := range log {
		weeks[weekStart(t)] = true
	}

	w := weekStart(now)
	if !weeks[w] {
		w = w.AddDate(0, 0, -7) // laufende Woche noch leer -> Vorwoche zählt
		if !weeks[w] {
			return 0 // Lücke -> Streak gerissen
		}
	}
	streak := 0
	for weeks[w] {
		streak++
		w = w.AddDate(0, 0, -7)
	}
	return streak
}

// daysSince liefert volle Tage zwischen last und now (>=0).
func daysSince(last, now time.Time) int {
	d := int(now.Sub(last).Hours() / 24)
	if d < 0 {
		d = 0
	}
	return d
}

// hotfixStatusLines baut die Anzeigezeilen für die Kiosk-Box.
func hotfixStatusLines(count int, log []time.Time, now time.Time) []string {
	lines := []string{fmt.Sprintf("⚑ hotfix #%d", count)}
	if len(log) == 0 {
		return lines
	}
	// (keine Breit-Emojis hier: doppelt breite Runen verschieben die Box-Ränder)
	if s := hotfixStreakWeeks(log, now); s > 0 {
		unit := "weeks"
		if s == 1 {
			unit = "week"
		}
		lines = append(lines, fmt.Sprintf("streak: %d %s ↑", s, unit))
	} else {
		lines = append(lines, "streak: broken ✗")
	}
	d := daysSince(log[len(log)-1], now)
	unit := "days"
	if d == 1 {
		unit = "day"
	}
	lines = append(lines, fmt.Sprintf("%d %s since last", d, unit))
	return lines
}
