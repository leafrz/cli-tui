package config

import (
	"net"
	"strconv"
	"strings"
)

// SSHHost ist ein gespeicherter SSH-Zielhost für das Hosts-Modul.
type SSHHost struct {
	Name string `json:"name,omitempty"` // Anzeigename; leer => Target()
	User string `json:"user"`
	Host string `json:"host"`
	Port int    `json:"port,omitempty"` // <=0 => 22
}

// EffectivePort liefert den Port mit Default 22.
func (h SSHHost) EffectivePort() int {
	if h.Port <= 0 {
		return 22
	}
	return h.Port
}

// Addr liefert "host:port" für net.Dial (IPv6-sicher via JoinHostPort).
func (h SSHHost) Addr() string {
	return net.JoinHostPort(h.Host, strconv.Itoa(h.EffectivePort()))
}

// Target liefert "user@host[:port]" — Anzeige- und Editierformat.
// Der Port erscheint nur, wenn er vom Default abweicht; IPv6-Hosts werden
// dann geklammert, damit das Format wieder parsebar ist.
func (h SSHHost) Target() string {
	if p := h.EffectivePort(); p != 22 {
		host := h.Host
		if strings.Contains(host, ":") {
			host = "[" + host + "]"
		}
		return h.User + "@" + host + ":" + strconv.Itoa(p)
	}
	return h.User + "@" + h.Host
}

// DisplayName liefert den Namen, sonst das Target als Fallback.
func (h SSHHost) DisplayName() string {
	if h.Name != "" {
		return h.Name
	}
	return h.Target()
}
