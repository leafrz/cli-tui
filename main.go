package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/faiface/beep/speaker"

	"github.com/leafrz/dashboard/radio"
)

func main() {
	// Speaker EINMALIG mit fester Rate initialisieren und offen halten.
	// Streams mit abweichender Rate werden im Player resampled.
	if err := speaker.Init(radio.SampleRate, radio.SampleRate.N(time.Second/10)); err != nil {
		fmt.Printf("FATALER FEHLER: Audio-Gerät konnte nicht initialisiert werden: %v\n", err)
		return
	}

	p := tea.NewProgram(newRoot(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
