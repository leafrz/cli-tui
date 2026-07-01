package ui

import "testing"

// TestThemesUniqueAndCycle prüft eindeutige Namen, Mindestanzahl und dass
// NextThemeName nach einem vollen Durchlauf wieder beim Start landet.
func TestThemesUniqueAndCycle(t *testing.T) {
	names := ThemeNames()
	if len(names) < 12 {
		t.Fatalf("erwartete >=12 Themes, bekam %d", len(names))
	}

	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			t.Fatalf("doppelter Theme-Name %q", n)
		}
		seen[n] = true
	}

	cur := names[0]
	for range names {
		cur = NextThemeName(cur)
	}
	if cur != names[0] {
		t.Fatalf("Zyklus endet nicht am Start: %q != %q", cur, names[0])
	}

	if ThemeByName("does-not-exist").name != names[0] {
		t.Fatalf("Fallback ist nicht das erste Theme, sondern %q", ThemeByName("does-not-exist").name)
	}
}
