package main

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leafrz/dashboard/internal/core"
	"github.com/leafrz/dashboard/internal/ui"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

// --- Messages / commands ---------------------------------------------------

type sysmonTickMsg time.Time

type sysmonDataMsg struct {
	perCPU            []float64
	memUsed, memTotal uint64
	memPct            float64
	diskUsed          uint64
	diskTotal         uint64
	diskPct           float64
	netSent, netRecv  uint64
	t                 time.Time
}

func sysmonTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return sysmonTickMsg(t) })
}

// rootPath liefert den zu überwachenden Datenträger-Mountpoint.
func rootPath() string {
	if runtime.GOOS == "windows" {
		return "C:\\"
	}
	return "/"
}

// sampleCmd sammelt die Systemwerte (schnell, non-blocking) in einer Goroutine.
func sampleCmd() tea.Cmd {
	return func() tea.Msg {
		d := sysmonDataMsg{t: time.Now()}

		if per, err := cpu.Percent(0, true); err == nil {
			d.perCPU = per
		}
		if vm, err := mem.VirtualMemory(); err == nil {
			d.memUsed, d.memTotal, d.memPct = vm.Used, vm.Total, vm.UsedPercent
		}
		if du, err := disk.Usage(rootPath()); err == nil {
			d.diskUsed, d.diskTotal, d.diskPct = du.Used, du.Total, du.UsedPercent
		}
		if io, err := net.IOCounters(false); err == nil && len(io) > 0 {
			d.netSent, d.netRecv = io[0].BytesSent, io[0].BytesRecv
		}
		return d
	}
}

// --- core.Module ----------------------------------------------------------------

type sysmonModule struct {
	width, height int
	showHelp      bool

	cpu               float64
	perCPU            []float64
	memUsed, memTotal uint64
	memPct            float64
	diskUsed          uint64
	diskTotal         uint64
	diskPct           float64
	netUp, netDown    float64

	prevSent, prevRecv uint64
	prevTime           time.Time
	havePrev           bool

	cpuHist, upHist, downHist []float64
}

func newSysmonModule() *sysmonModule { return &sysmonModule{} }

func (m *sysmonModule) Name() string { return "system" }

func (m *sysmonModule) Status() string {
	if !m.havePrev {
		return ""
	}
	return fmt.Sprintf("cpu %.0f%% · mem %.0f%%", m.cpu, m.memPct)
}

func (m *sysmonModule) Init() tea.Cmd { return nil } // Start erst bei core.FocusMsg

func (m *sysmonModule) Update(msg tea.Msg) (core.Module, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case core.FocusMsg:
		// (Neu) starten: sofort messen + Ticker anwerfen.
		return m, tea.Batch(sampleCmd(), sysmonTick())

	case sysmonTickMsg:
		return m, tea.Batch(sampleCmd(), sysmonTick())

	case sysmonDataMsg:
		m.applyData(msg)
		return m, nil

	case tea.KeyMsg:
		k := msg.String()
		if m.showHelp {
			if k == "esc" || k == "?" || k == "q" {
				m.showHelp = false
			}
			return m, nil
		}
		switch k {
		case "esc", "q", "backspace":
			return m, core.GoToLauncher
		case "?":
			m.showHelp = true
			return m, nil
		}
	}
	return m, nil
}

func (m *sysmonModule) applyData(d sysmonDataMsg) {
	m.perCPU = d.perCPU
	m.cpu = avg(d.perCPU)
	m.memUsed, m.memTotal, m.memPct = d.memUsed, d.memTotal, d.memPct
	m.diskUsed, m.diskTotal, m.diskPct = d.diskUsed, d.diskTotal, d.diskPct

	if m.havePrev {
		dt := d.t.Sub(m.prevTime).Seconds()
		if dt > 0 {
			m.netDown = rate(d.netRecv, m.prevRecv, dt)
			m.netUp = rate(d.netSent, m.prevSent, dt)
		}
	}
	m.prevSent, m.prevRecv, m.prevTime = d.netSent, d.netRecv, d.t
	m.havePrev = true

	m.cpuHist = pushHist(m.cpuHist, m.cpu, 48)
	m.downHist = pushHist(m.downHist, m.netDown, 48)
	m.upHist = pushHist(m.upHist, m.netUp, 48)
}

// --- View ------------------------------------------------------------------

func (m *sysmonModule) View(width, height int) string {
	m.width, m.height = width, height

	if m.showHelp {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, m.helpView())
	}
	if !m.havePrev {
		return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center,
			ui.DimStyle.Render("gathering system stats…"))
	}

	const inner = 46
	label := lipgloss.NewStyle().Foreground(ui.ColPurple)
	val := lipgloss.NewStyle().Foreground(ui.ColCream)

	// CPU
	cpuRows := []string{
		rowLabelValue("cpu", fmt.Sprintf("%.0f%%", m.cpu), inner, label, val),
		ui.RenderVolumeBar(m.cpu/100, inner),
		renderSparkline(m.cpuHist, inner, ui.ColTeal),
	}
	if len(m.perCPU) > 0 {
		cpuRows = append(cpuRows, label.Render("cores ")+renderCores(m.perCPU))
	}
	cpuCard := ui.CardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, cpuRows...))

	// Memory
	memCard := ui.CardStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		rowLabelValue("memory",
			fmt.Sprintf("%s / %s  %.0f%%", humanBytes(m.memUsed), humanBytes(m.memTotal), m.memPct),
			inner, label, val),
		ui.RenderVolumeBar(m.memPct/100, inner),
	))

	// Disk
	diskCard := ui.CardStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		rowLabelValue("disk "+rootPath(),
			fmt.Sprintf("%s / %s  %.0f%%", humanBytes(m.diskUsed), humanBytes(m.diskTotal), m.diskPct),
			inner, label, val),
		ui.RenderVolumeBar(m.diskPct/100, inner),
	))

	// Network
	netCard := ui.CardStyle.Render(lipgloss.JoinVertical(lipgloss.Left,
		rowLabelValue("network",
			fmt.Sprintf("↓ %s   ↑ %s", humanRate(m.netDown), humanRate(m.netUp)),
			inner, label, val),
		label.Render("↓ ")+renderSparkline(m.downHist, inner-2, ui.ColTeal),
		label.Render("↑ ")+renderSparkline(m.upHist, inner-2, ui.ColPeach),
	))

	stack := lipgloss.JoinVertical(lipgloss.Left, cpuCard, memCard, diskCard, netCard)
	help := ui.HelpStyle.Render("esc: dashboard   ·   ?: help")
	body := lipgloss.JoinVertical(lipgloss.Center, stack, "", help)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, body)
}

func (m *sysmonModule) helpView() string {
	sections := []ui.HelpSection{
		{Title: "system monitor", Rows: [][2]string{
			{"esc / q", "back to dashboard"},
			{"?", "toggle this help"},
		}},
	}
	return ui.HelpOverlay("system · help", sections, "? or esc to close   ·   global commands on the dashboard")
}

// --- helpers ---------------------------------------------------------------

var sparkRunes = []rune("▁▂▃▄▅▆▇█")

// renderSparkline zeichnet eine Mini-Verlaufskurve (letzte width Werte).
func renderSparkline(vals []float64, width int, color lipgloss.Color) string {
	if width < 1 {
		width = 1
	}
	start := 0
	if len(vals) > width {
		start = len(vals) - width
	}
	window := vals[start:]

	max := 0.0
	for _, v := range window {
		if v > max {
			max = v
		}
	}
	if max <= 0 {
		max = 1
	}

	style := lipgloss.NewStyle().Foreground(color)
	var b strings.Builder
	for i := 0; i < width-len(window); i++ {
		b.WriteByte(' ')
	}
	for _, v := range window {
		idx := int((v / max) * float64(len(sparkRunes)-1))
		if idx < 0 {
			idx = 0
		}
		if idx > len(sparkRunes)-1 {
			idx = len(sparkRunes) - 1
		}
		b.WriteString(style.Render(string(sparkRunes[idx])))
	}
	return b.String()
}

// renderCores zeichnet pro Kern einen kleinen, last-gefärbten Balken.
func renderCores(per []float64) string {
	var b strings.Builder
	for _, p := range per {
		idx := int((p / 100) * float64(len(sparkRunes)-1))
		if idx < 0 {
			idx = 0
		}
		if idx > len(sparkRunes)-1 {
			idx = len(sparkRunes) - 1
		}
		c := ui.ColTeal
		switch {
		case p >= 85:
			c = ui.ColError
		case p >= 50:
			c = ui.ColPeach
		}
		b.WriteString(lipgloss.NewStyle().Foreground(c).Render(string(sparkRunes[idx])))
	}
	return b.String()
}

// rowLabelValue rendert "label .... value" über die volle Breite.
func rowLabelValue(lbl, value string, width int, label, val lipgloss.Style) string {
	l := label.Render(lbl)
	v := val.Render(value)
	spacer := width - lipgloss.Width(l) - lipgloss.Width(v)
	if spacer < 1 {
		spacer = 1
	}
	return lipgloss.JoinHorizontal(lipgloss.Bottom,
		l, lipgloss.NewStyle().Width(spacer).Render(""), v)
}

func avg(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}

func rate(cur, prev uint64, dt float64) float64 {
	if cur < prev { // Counter-Reset
		return 0
	}
	return float64(cur-prev) / dt
}

func pushHist(h []float64, v float64, max int) []float64 {
	h = append(h, v)
	if len(h) > max {
		h = h[len(h)-max:]
	}
	return h
}

func humanBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f%cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func humanRate(bps float64) string {
	if bps < 1 {
		return "0 B/s"
	}
	const unit = 1024.0
	units := []string{"B/s", "KB/s", "MB/s", "GB/s"}
	i := 0
	for bps >= unit && i < len(units)-1 {
		bps /= unit
		i++
	}
	return fmt.Sprintf("%.1f %s", bps, units[i])
}
