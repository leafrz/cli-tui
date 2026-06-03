package main

import tea "github.com/charmbracelet/bubbletea"

// Module ist die Schnittstelle, die jedes Dashboard-Modul implementiert.
// Das Root-Dashboard rendert den globalen Header und delegiert den restlichen
// Platz (Content + ggf. eigener Footer) an das aktive Modul.
type Module interface {
	// Name liefert den Anzeigenamen (z.B. "radio").
	Name() string

	// Init wird einmal beim Start aufgerufen und kann ein Start-Cmd liefern.
	Init() tea.Cmd

	// Update verarbeitet eine Nachricht und gibt das (ggf. aktualisierte)
	// Modul plus ein optionales Cmd zurück.
	Update(msg tea.Msg) (Module, tea.Cmd)

	// View rendert den Modul-Inhalt in der gegebenen Größe (ohne globalen
	// Header).
	View(width, height int) string

	// Status liefert eine kurze Live-Statuszeile für den Context-Header
	// (z.B. den aktuell laufenden Titel). Leerer String = nichts.
	Status() string
}
