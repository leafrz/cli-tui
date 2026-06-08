// Package core enthält das Module-Interface und die modulübergreifenden
// Nachrichten. Es ist ein Blatt-Paket (importiert nur Bubble Tea), damit sowohl
// das Dashboard als auch die Module es nutzen können, ohne einen Import-Zyklus.
package core

import tea "github.com/charmbracelet/bubbletea"

// Module ist die Schnittstelle, die jedes Dashboard-Modul implementiert.
type Module interface {
	Name() string
	Init() tea.Cmd
	Update(msg tea.Msg) (Module, tea.Cmd)
	View(width, height int) string
	Status() string
}

// GoToLauncherMsg bringt das Dashboard zurück zum Startmenü.
type GoToLauncherMsg struct{}

func GoToLauncher() tea.Msg { return GoToLauncherMsg{} }

// SwitchModuleMsg wechselt direkt zu einem benannten Modul.
type SwitchModuleMsg struct{ Name string }

func SwitchTo(name string) tea.Cmd {
	return func() tea.Msg { return SwitchModuleMsg{name} }
}

// ReloadConfigMsg signalisiert, dass die persistierte Config geändert wurde.
type ReloadConfigMsg struct{}

func ReloadConfig() tea.Msg { return ReloadConfigMsg{} }

// ThemeChangedMsg fordert alle Module auf, ihre Komponenten-Styles neu zu setzen.
type ThemeChangedMsg struct{}

func ThemeChanged() tea.Msg { return ThemeChangedMsg{} }

// FocusMsg wird an ein Modul gesendet, sobald es aktiv wird (Ticker neu starten).
type FocusMsg struct{}

func FocusModule() tea.Msg { return FocusMsg{} }
