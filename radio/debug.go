package radio

import (
	"fmt"
	"os"
	"time"
)

// debugEnabled schaltet das Datei-Logging frei, wenn RADIO_DEBUG gesetzt ist.
// Wir loggen NICHT auf stdout, weil das die TUI-Darstellung zerstören würde.
var debugEnabled = os.Getenv("RADIO_DEBUG") != ""

// Debugf ist die exportierte Variante, damit auch package main loggen kann.
func Debugf(format string, args ...any) { debugf(format, args...) }

// debugf schreibt eine Zeile nach radio_debug.log.
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
