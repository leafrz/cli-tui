package main

import "testing"

// TestSysmonSample prüft, dass gopsutil auf dieser Maschine echte Werte liefert.
//
//	go test . -run TestSysmonSample -v
func TestSysmonSample(t *testing.T) {
	msg := sampleCmd()()
	d, ok := msg.(sysmonDataMsg)
	if !ok {
		t.Fatalf("expected sysmonDataMsg, got %T", msg)
	}

	t.Logf("cores: %d (avg %.1f%%)", len(d.perCPU), avg(d.perCPU))
	t.Logf("mem: %s / %s (%.1f%%)", humanBytes(d.memUsed), humanBytes(d.memTotal), d.memPct)
	t.Logf("disk %s: %s / %s (%.1f%%)", rootPath(), humanBytes(d.diskUsed), humanBytes(d.diskTotal), d.diskPct)
	t.Logf("net: sent=%s recv=%s", humanBytes(d.netSent), humanBytes(d.netRecv))

	if d.memTotal == 0 {
		t.Errorf("memTotal is 0 — gopsutil mem not working")
	}
	if d.diskTotal == 0 {
		t.Errorf("diskTotal is 0 — gopsutil disk not working for %s", rootPath())
	}
	if len(d.perCPU) == 0 {
		t.Errorf("no per-CPU data")
	}
}
