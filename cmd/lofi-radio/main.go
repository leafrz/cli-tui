package main

import (
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/faiface/beep/speaker"

	"github.com/leafrz/dashboard/internal/audio"
	"github.com/leafrz/dashboard/internal/dashboard"
)

// version wird beim Release-Build per -ldflags aus dem Git-Tag gesetzt.
var version = "dev"

func main() {
	autostart := false
	for _, a := range os.Args[1:] {
		switch a {
		case "--version", "-v":
			fmt.Println("lofi-radio", version)
			return
		case "--autostart", "--kiosk":
			autostart = true
		}
	}

	// Speaker EINMALIG mit fester Rate initialisieren und offen halten.
	// Streams mit abweichender Rate werden im Player resampled.
	if err := speaker.Init(audio.SampleRate, audio.SampleRate.N(time.Second/10)); err != nil {
		fmt.Printf("FATALER FEHLER: Audio-Gerät konnte nicht initialisiert werden: %v\n", err)
		return
	}

	p := tea.NewProgram(dashboard.NewRoot(autostart), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
