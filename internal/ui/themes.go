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
	{
		name:  "dracula",
		cream: "#f8f8f2", mauve: "#bd93f9", purple: "#6272a4", teal: "#8be9fd",
		peach: "#ffb86c", dim: "#6272a4", faint: "#44475a", errorC: "#ff5555", good: "#50fa7b",
	},
	{
		name:  "gruvbox",
		cream: "#ebdbb2", mauve: "#d3869b", purple: "#665c54", teal: "#8ec07c",
		peach: "#fe8019", dim: "#928374", faint: "#3c3836", errorC: "#fb4934", good: "#b8bb26",
	},
	{
		name:  "tokyonight",
		cream: "#c0caf5", mauve: "#bb9af7", purple: "#565f89", teal: "#7dcfff",
		peach: "#ff9e64", dim: "#565f89", faint: "#292e42", errorC: "#f7768e", good: "#9ece6a",
	},
	{
		name:  "solarized",
		cream: "#93a1a1", mauve: "#6c71c4", purple: "#586e75", teal: "#2aa198",
		peach: "#cb4b16", dim: "#657b83", faint: "#073642", errorC: "#dc322f", good: "#859900",
	},
	{
		name:  "crt",
		cream: "#c8ffc8", mauve: "#66ff66", purple: "#2a7a2a", teal: "#55ff88",
		peach: "#99ff99", dim: "#3f7f3f", faint: "#123512", errorC: "#ff6b6b", good: "#66ff66",
	},
	{
		name:  "synthwave",
		cream: "#f2e9f7", mauve: "#ff7edb", purple: "#6d5a9c", teal: "#36f9f6",
		peach: "#fede5d", dim: "#8d80b0", faint: "#34294f", errorC: "#fe4450", good: "#72f1b8",
	},
	{
		name:  "sakura",
		cream: "#f5e6ea", mauve: "#e8a0bf", purple: "#9c7086", teal: "#a8c6c2",
		peach: "#f0c39a", dim: "#96788a", faint: "#3d2b34", errorC: "#d17b88", good: "#a9c49a",
	},
	{
		name:  "amber",
		cream: "#f5d7a3", mauve: "#ffb000", purple: "#8a6c33", teal: "#d9a441",
		peach: "#ffc46b", dim: "#a07f45", faint: "#3d2f14", errorC: "#e06c55", good: "#c8b458",
	},
	{
		name:  "ocean",
		cream: "#d8e6ee", mauve: "#8fb8d8", purple: "#4e6a80", teal: "#6fc3c9",
		peach: "#d9a97c", dim: "#6c8598", faint: "#1e3240", errorC: "#c97b7b", good: "#8fbf9a",
	},
	{
		name:  "cyberpunk",
		cream: "#f2f0e4", mauve: "#ff2a6d", purple: "#4a4a68", teal: "#00e5e5",
		peach: "#fcee0a", dim: "#8a8aa0", faint: "#2b2b3a", errorC: "#ff3f3f", good: "#39ff88",
	},
	{
		name:  "lava",
		cream: "#f2ded2", mauve: "#ff6b4a", purple: "#8a4a3c", teal: "#c98a6a",
		peach: "#ffa040", dim: "#9a7a6e", faint: "#3a201a", errorC: "#ff4530", good: "#b0b060",
	},
	{
		name:  "aurora",
		cream: "#e2f0ea", mauve: "#b48ce8", purple: "#4a6a5a", teal: "#4ee6b8",
		peach: "#eda0b0", dim: "#7a9a8e", faint: "#1e3a30", errorC: "#e07070", good: "#63e6a0",
	},
	{
		name:  "grape",
		cream: "#ece4f5", mauve: "#c890f0", purple: "#6a4a8a", teal: "#7ee0d0",
		peach: "#e0a860", dim: "#9a86b0", faint: "#32284a", errorC: "#e06888", good: "#98d080",
	},
	{
		name:  "citrus",
		cream: "#f2f0dc", mauve: "#ffb020", purple: "#6a7a3a", teal: "#50d0a0",
		peach: "#c8e838", dim: "#96a075", faint: "#333a1e", errorC: "#f05840", good: "#a0e040",
	},
	{
		name:  "flamingo",
		cream: "#faeae8", mauve: "#ff6f9c", purple: "#8a5068", teal: "#50c8c0",
		peach: "#ffb080", dim: "#a8808e", faint: "#402630", errorC: "#ff5060", good: "#80cc88",
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
