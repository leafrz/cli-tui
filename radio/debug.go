package radio

import (
	"fmt"
	"os"
	"time"
)

// debugEnabled schaltet das Datei-Logging frei, wenn RADIO_DEBUG gesetzt ist.
// Wir loggen NICHT auf stdout, weil das die TUI-Darstellung zerstören würde.
var debugEnabled = os.Getenv("RADIO_DEBUG") != ""

// debugf schreibt eine Zeile nach radio_debug.log (nur wenn RADIO_DEBUG gesetzt).
func debugf(format string, args ...any) {
	if !debugEnabled {
		return
	}
	f, err := os.OpenFile("radio_debug.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, time.Now().Format("15:04:05.000")+" "+format+"\n", args...)
}
