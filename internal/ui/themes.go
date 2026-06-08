package ui

import "github.com/charmbracelet/lipgloss"

// themeDef ist eine benannte Farbpalette. Die 9 Rollen entsprechen den
// globalen col*-Variablen in styles.go.
type themeDef struct {
	name   string
	cream  lipgloss.Color // primary text
	mauve  lipgloss.Color // accent / titles
	purple lipgloss.Color // borders / secondary accent
	teal   lipgloss.Color // status / cool accent
	peach  lipgloss.Color // highlight / warm accent
	dim    lipgloss.Color // muted text / help
	faint  lipgloss.Color // empty fills
	errorC lipgloss.Color // error
	good   lipgloss.Color // success
}

// themes ist die Reihenfolge beim Durchschalten.
var themes = []themeDef{
	{
		name:  "lofi",
		cream: "#e6ddcf", mauve: "#c4a7b5", purple: "#9a8c98", teal: "#88a09e",
		peach: "#dcae9a", dim: "#7a736b", faint: "#48433d", errorC: "#c98a8a", good: "#a3b18a",
	},
	{
		name:  "midnight",
		cream: "#cdd6f4", mauve: "#b4befe", purple: "#585b70", teal: "#89dceb",
		peach: "#f5c2e7", dim: "#6c7086", faint: "#313244", errorC: "#f38ba8", good: "#a6e3a1",
	},
	{
		name:  "sepia",
		cream: "#efe3d0", mauve: "#c9a26b", purple: "#a3865f", teal: "#bd9a7a",
		peach: "#d08a4f", dim: "#9c8468", faint: "#43382c", errorC: "#b56b54", good: "#9a9a5e",
	},
	{
		name:  "forest",
		cream: "#e3e8d8", mauve: "#a7c080", purple: "#5c7a52", teal: "#83a598",
		peach: "#d3a86a", dim: "#728465", faint: "#2e3a2a", errorC: "#c0705e", good: "#8fb46a",
	},
	{
		name:  "rosepine",
		cream: "#e0def4", mauve: "#c4a7e7", purple: "#6e6a86", teal: "#9ccfd8",
		peach: "#f6c177", dim: "#908caa", faint: "#26233a", errorC: "#eb6f92", good: "#abc9a0",
	},
	{
		name:  "nord",
		cream: "#e5e9f0", mauve: "#b48ead", purple: "#4c566a", teal: "#88c0d0",
		peach: "#d08770", dim: "#7b88a1", faint: "#3b4252", errorC: "#bf616a", good: "#a3be8c",
	},
	{
		name:  "noir",
		cream: "#e6e6e6", mauve: "#c0c0c0", purple: "#6f6f6f", teal: "#9a9a9a",
		peach: "#cfcfcf", dim: "#808080", faint: "#353535", errorC: "#b86b6b", good: "#8f9f7f",
	},
}

// ThemeNames liefert alle Theme-Namen in Reihenfolge.
func ThemeNames() []string {
	out := make([]string, len(themes))
	for i, t := range themes {
		out[i] = t.name
	}
	return out
}

// ThemeByName liefert das Theme oder das erste (lofi) als Fallback.
func ThemeByName(name string) themeDef {
	for _, t := range themes {
		if t.name == name {
			return t
		}
	}
	return themes[0]
}

// NextThemeName liefert den Namen des nächsten Themes (zyklisch).
func NextThemeName(name string) string {
	cur := 0
	for i, t := range themes {
		if t.name == name {
			cur = i
			break
		}
	}
	return themes[(cur+1)%len(themes)].name
}

// ApplyTheme setzt die globale Palette und baut die Styles neu auf.
func ApplyTheme(t themeDef) {
	ColCream = t.cream
	ColMauve = t.mauve
	ColPurple = t.purple
	ColTeal = t.teal
	ColPeach = t.peach
	ColDim = t.dim
	ColFaint = t.faint
	ColError = t.errorC
	ColGood = t.good
	rebuildStyles()
}

// rebuildStyles baut alle vorgefertigten Styles aus der aktuellen Palette neu.
// Muss nach jeder Theme-Änderung laufen, da Styles ihre Farben beim Bauen
// einfrieren. View-Code liest diese Vars zur Render-Zeit -> Neuzuweisung wirkt.
func rebuildStyles() {
	ClockStyle = lipgloss.NewStyle().Foreground(ColDim)
	HelpStyle = lipgloss.NewStyle().Foreground(ColDim)
	DimStyle = lipgloss.NewStyle().Foreground(ColDim)
	LabelStyle = lipgloss.NewStyle().Foreground(ColPurple)

	CardStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColPurple).
		Padding(1, 4)

	StationNameStyle = lipgloss.NewStyle().Bold(true).Foreground(ColPeach)
	NowPlayingStyle = lipgloss.NewStyle().Foreground(ColCream).Italic(true)
	ruleStyle = lipgloss.NewStyle().Foreground(ColFaint)

	HeaderTextStyle = lipgloss.NewStyle().Bold(true).Foreground(ColMauve)
}
