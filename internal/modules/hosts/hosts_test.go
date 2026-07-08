package hosts

import (
	"testing"

	"github.com/leafrz/dashboard/internal/config"
)

func TestParseTarget(t *testing.T) {
	cases := []struct {
		in      string
		want    config.SSHHost
		wantErr bool
	}{
		{in: "pi@192.168.1.10", want: config.SSHHost{User: "pi", Host: "192.168.1.10", Port: 22}},
		{in: "pi@192.168.1.10:2222", want: config.SSHHost{User: "pi", Host: "192.168.1.10", Port: 2222}},
		{in: "  pi@raspi.local  ", want: config.SSHHost{User: "pi", Host: "raspi.local", Port: 22}},
		{in: "root@[::1]:2222", want: config.SSHHost{User: "root", Host: "::1", Port: 2222}},
		{in: "root@::1", want: config.SSHHost{User: "root", Host: "::1", Port: 22}},
		{in: "user@name@host", want: config.SSHHost{User: "user@name", Host: "host", Port: 22}},
		{in: "nohost", wantErr: true},
		{in: "@host", wantErr: true},
		{in: "user@", wantErr: true},
		{in: "", wantErr: true},
		{in: "pi@host:0", wantErr: true},
		{in: "pi@host:99999", wantErr: true},
		{in: "pi@host:abc", wantErr: true},
	}

	for _, c := range cases {
		got, err := parseTarget(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseTarget(%q): expected error, got %+v", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseTarget(%q): unexpected error: %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("parseTarget(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestHostTargetRoundtrip(t *testing.T) {
	// Target() muss wieder parsebar sein (Edit-Flow füllt das Feld damit vor).
	hosts := []config.SSHHost{
		{User: "pi", Host: "raspi.local", Port: 22},
		{User: "pi", Host: "raspi.local"}, // Port 0 => 22
		{User: "root", Host: "10.0.0.5", Port: 2222},
		{User: "root", Host: "::1", Port: 2222},
	}
	for _, h := range hosts {
		got, err := parseTarget(h.Target())
		if err != nil {
			t.Errorf("parseTarget(Target()=%q): %v", h.Target(), err)
			continue
		}
		if got.User != h.User || got.Host != h.Host || got.EffectivePort() != h.EffectivePort() {
			t.Errorf("roundtrip %q: got %+v, want %+v", h.Target(), got, h)
		}
	}
}

func TestHostAddr(t *testing.T) {
	if got := (config.SSHHost{Host: "raspi.local"}).Addr(); got != "raspi.local:22" {
		t.Errorf("Addr() = %q, want raspi.local:22", got)
	}
	if got := (config.SSHHost{Host: "::1", Port: 2222}).Addr(); got != "[::1]:2222" {
		t.Errorf("Addr() = %q, want [::1]:2222", got)
	}
}

func TestConnectCmdArgs(t *testing.T) {
	// Kein -p bei Default-Port; -p N sonst. (Indirekt über Target/EffectivePort
	// abgesichert; hier nur der Vertrag von EffectivePort.)
	if p := (config.SSHHost{}).EffectivePort(); p != 22 {
		t.Errorf("EffectivePort() zero value = %d, want 22", p)
	}
	if p := (config.SSHHost{Port: 2222}).EffectivePort(); p != 2222 {
		t.Errorf("EffectivePort() = %d, want 2222", p)
	}
}
