package ambient

import (
	"testing"
	"time"
)

// mo liefert einen Montag + Offset-Tage als Referenzpunkt.
// 2026-06-29 ist ein Montag.
func mo(weekOffset, days int) time.Time {
	base := time.Date(2026, 6, 29, 12, 0, 0, 0, time.Local)
	return base.AddDate(0, 0, weekOffset*7+days)
}

func TestHotfixStreak(t *testing.T) {
	now := mo(0, 2) // Mittwoch der aktuellen Woche

	cases := []struct {
		name string
		log  []time.Time
		want int
	}{
		{"leer", nil, 0},
		{"nur diese Woche", []time.Time{mo(0, 0)}, 1},
		{"drei Wochen in Folge", []time.Time{mo(-2, 1), mo(-1, 3), mo(0, 0)}, 3},
		{"laufende Woche leer, Vorwoche zaehlt", []time.Time{mo(-2, 0), mo(-1, 4)}, 2},
		{"Luecke reisst den Streak", []time.Time{mo(-3, 0), mo(-2, 0)}, 0},
		{"mehrere pro Woche zaehlen einfach", []time.Time{mo(0, 0), mo(0, 1), mo(-1, 2)}, 2},
		{"Sonntag gehoert zur selben Woche", []time.Time{mo(-1, 6), mo(0, 0)}, 2},
	}
	for _, c := range cases {
		if got := hotfixStreakWeeks(c.log, now); got != c.want {
			t.Errorf("%s: streak = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestDaysSince(t *testing.T) {
	now := mo(0, 2)
	if d := daysSince(mo(0, 2), now); d != 0 {
		t.Errorf("heute: %d, want 0", d)
	}
	if d := daysSince(mo(0, 0), now); d != 2 {
		t.Errorf("vor 2 Tagen: %d, want 2", d)
	}
}

func TestHotfixStatusLines(t *testing.T) {
	now := mo(0, 2)

	// Ohne Log: nur die Counter-Zeile.
	lines := hotfixStatusLines(7, nil, now)
	if len(lines) != 1 {
		t.Fatalf("ohne Log: %d Zeilen, want 1", len(lines))
	}

	// Mit Streak: drei Zeilen, Streak lebt.
	lines = hotfixStatusLines(42, []time.Time{mo(-1, 0), mo(0, 0)}, now)
	if len(lines) != 3 {
		t.Fatalf("mit Log: %d Zeilen, want 3", len(lines))
	}
	if lines[1] != "streak: 2 weeks ↑" {
		t.Errorf("Streak-Zeile: %q", lines[1])
	}
	if lines[2] != "2 days since last" {
		t.Errorf("Days-Zeile: %q", lines[2])
	}

	// Gerissener Streak.
	lines = hotfixStatusLines(42, []time.Time{mo(-3, 0)}, now)
	if lines[1] != "streak: broken ✗" {
		t.Errorf("Broken-Zeile: %q", lines[1])
	}
}
