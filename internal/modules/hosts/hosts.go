// Package hosts ist das SSH-Hosts-Modul: gespeicherte Zielhosts mit
// Erreichbarkeits-Check (TCP-Dial) und Verbinden per tea.ExecProcess.
// Während einer SSH-Session läuft der Audio-Player weiter.
package hosts

import (
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/leafrz/dashboard/internal/config"
	"github.com/leafrz/dashboard/internal/core"
	"github.com/leafrz/dashboard/internal/ui"
)

const (
	pingInterval = 30 * time.Second
	pingTimeout  = 3 * time.Second
)

// --- Messages / commands ---------------------------------------------------

// pingTickMsg trägt die Generation des Ticker-Loops; veraltete Loops
// (nach erneutem Focus) laufen so nicht doppelt weiter.
type pingTickMsg struct{ gen int }

type pingResultMsg struct {
	key     string // SSHHost.Target() als stabile Identität (Index kann sich ändern)
	ok      bool
	latency time.Duration
}

type sshDoneMsg struct {
	name string
	err  error
}

func pingTick(gen int) tea.Cmd {
	return tea.Tick(pingInterval, func(time.Time) tea.Msg { return pingTickMsg{gen} })
}

// pingCmd prüft Erreichbarkeit per TCP-Dial auf den SSH-Port.
func pingCmd(h config.SSHHost) tea.Cmd {
	addr, key := h.Addr(), h.Target()
	return func() tea.Msg {
		start := time.Now()
		conn, err := net.DialTimeout("tcp", addr, pingTimeout)
		if err != nil {
			return pingResultMsg{key: key}
		}
		_ = conn.Close()
		return pingResultMsg{key: key, ok: true, latency: time.Since(start)}
	}
}

// connectCmd übergibt das Terminal an ssh; das TUI (und der Player) laufen
// im Hintergrund weiter, bis die Session endet.
func connectCmd(h config.SSHHost) tea.Cmd {
	args := make([]string, 0, 3)
	if p := h.EffectivePort(); p != 22 {
		args = append(args, "-p", strconv.Itoa(p))
	}
	args = append(args, h.User+"@"+h.Host)
	name := h.DisplayName()
	return tea.ExecProcess(exec.Command("ssh", args...), func(err error) tea.Msg {
		return sshDoneMsg{name: name, err: err}
	})
}

// parseTarget zerlegt "user@host[:port]" in einen SSHHost.
// IPv6 mit Port braucht Klammern ("user@[::1]:2222"), ohne Port geht es bar.
func parseTarget(s string) (config.SSHHost, error) {
	s = strings.TrimSpace(s)
	at := strings.LastIndex(s, "@")
	if at <= 0 || at == len(s)-1 {
		return config.SSHHost{}, errors.New("format: user@host[:port]")
	}
	user, rest := s[:at], s[at+1:]

	host, port := rest, 22
	if hp, pp, err := net.SplitHostPort(rest); err == nil {
		p, perr := strconv.Atoi(pp)
		if perr != nil || p < 1 || p > 65535 {
			return config.SSHHost{}, errors.New("invalid port: " + pp)
		}
		host, port = hp, p
	}
	if host == "" {
		return config.SSHHost{}, errors.New("format: user@host[:port]")
	}
	return config.SSHHost{User: user, Host: host, Port: port}, nil
}

// --- core.Module ------------------------------------------------------------

// inputStage: der Add/Edit-Dialog fragt erst das Target, dann den Namen ab.
type inputStage int

const (
	stageNone inputStage = iota
	stageTarget
	stageName
)

type hostStatus struct {
	known   bool
	ok      bool
	latency time.Duration
}

type hostsModule struct {
	width, height int

	items  []config.SSHHost
	cursor int
	status map[string]hostStatus // key = SSHHost.Target()

	stage   inputStage
	editIdx int             // -1 = neuer Host
	pending config.SSHHost  // geparste Target-Eingabe, wartet auf Namen
	input   textinput.Model

	note    string // letzte Meldung (Session beendet, Parse-Fehler, ...)
	noteErr bool

	gen int // Ticker-Generation (siehe pingTickMsg)
}

func New() *hostsModule {
	in := textinput.New()
	in.Prompt = "› "
	in.CharLimit = 120
	in.Width = 40

	m := &hostsModule{
		items:   config.Load().Hosts,
		status:  map[string]hostStatus{},
		editIdx: -1,
		input:   in,
	}
	m.restyle()
	return m
}

func (m *hostsModule) Name() string  { return "hosts" }
func (m *hostsModule) Init() tea.Cmd { return nil } // Pings starten erst bei core.FocusMsg

func (m *hostsModule) Status() string {
	up, known := 0, 0
	for _, h := range m.items {
		if st, exists := m.status[h.Target()]; exists && st.known {
			known++
			if st.ok {
				up++
			}
		}
	}
	if known == 0 {
		return ""
	}
	return fmt.Sprintf("hosts %d/%d up", up, len(m.items))
}

func (m *hostsModule) restyle() {
	m.input.PromptStyle = lipgloss.NewStyle().Foreground(ui.ColTeal)
	m.input.TextStyle = lipgloss.NewStyle().Foreground(ui.ColCream)
	m.input.PlaceholderStyle = lipgloss.NewStyle().Foreground(ui.ColFaint)
	m.input.Cursor.Style = lipgloss.NewStyle().Foreground(ui.ColPeach)
}

func (m *hostsModule) save() tea.Cmd {
	items := append([]config.SSHHost(nil), m.items...)
	return func() tea.Msg {
		_ = config.Update(func(s *config.State) { s.Hosts = items })
		return nil
	}
}

// pingAll stößt einen Check für jeden Host an.
func (m *hostsModule) pingAll() tea.Cmd {
	if len(m.items) == 0 {
		return nil
	}
	cmds := make([]tea.Cmd, 0, len(m.items))
	for _, h := range m.items {
		cmds = append(cmds, pingCmd(h))
	}
	return tea.Batch(cmds...)
}

// startInput öffnet den Add/Edit-Dialog (Stufe 1: Target).
func (m *hostsModule) startInput(idx int) {
	m.stage = stageTarget
	m.editIdx = idx
	m.note = ""
	m.input.Placeholder = "pi@192.168.1.10:22"
	if idx >= 0 {
		m.input.SetValue(m.items[idx].Target())
	} else {
		m.input.SetValue("")
	}
	m.input.CursorEnd()
	m.input.Focus()
}

func (m *hostsModule) closeInput() {
	m.stage = stageNone
	m.editIdx = -1
	m.pending = config.SSHHost{}
	m.input.SetValue("")
	m.input.Blur()
}

func (m *hostsModule) Update(msg tea.Msg) (core.Module, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case core.FocusMsg:
		// Config könnte extern geändert sein; Pings + neuen Ticker-Loop starten.
		m.items = config.Load().Hosts
		if m.cursor >= len(m.items) && m.cursor > 0 {
			m.cursor = len(m.items) - 1
		}
		m.gen++
		return m, tea.Batch(m.pingAll(), pingTick(m.gen))

	case core.ThemeChangedMsg:
		m.restyle()

	case pingTickMsg:
		if msg.gen != m.gen {
			return m, nil // veralteter Loop nach erneutem Focus
		}
		return m, tea.Batch(m.pingAll(), pingTick(m.gen))

	case pingResultMsg:
		m.status[msg.key] = hostStatus{known: true, ok: msg.ok, latency: msg.latency}
		return m, nil

	case sshDoneMsg:
		if msg.err != nil {
			m.note, m.noteErr = fmt.Sprintf("%s: %v", msg.name, msg.err), true
		} else {
			m.note, m.noteErr = fmt.Sprintf("session to %s ended", msg.name), false
		}
		// Nach der Session direkt neu prüfen (Host könnte z.B. runtergefahren sein).
		return m, m.pingAll()

	case tea.KeyMsg:
		if m.stage != stageNone {
			return m.updateInput(msg)
		}

		switch msg.String() {
		case "esc", "q", "backspace":
			return m, core.GoToLauncher
		case "a":
			m.startInput(-1)
			return m, textinput.Blink
		case "e":
			if m.cursor < len(m.items) {
				m.startInput(m.cursor)
				return m, textinput.Blink
			}
		case "d":
			if m.cursor < len(m.items) {
				delete(m.status, m.items[m.cursor].Target())
				m.items = append(m.items[:m.cursor], m.items[m.cursor+1:]...)
				if m.cursor >= len(m.items) && m.cursor > 0 {
					m.cursor--
				}
				return m, m.save()
			}
		case "r":
			return m, m.pingAll()
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "enter":
			if m.cursor < len(m.items) {
				m.note = ""
				return m, connectCmd(m.items[m.cursor])
			}
		}
	}
	return m, nil
}

// updateInput behandelt Tasten, solange der Add/Edit-Dialog offen ist.
func (m *hostsModule) updateInput(msg tea.KeyMsg) (core.Module, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closeInput()
		return m, nil

	case "enter":
		val := strings.TrimSpace(m.input.Value())

		if m.stage == stageTarget {
			h, err := parseTarget(val)
			if err != nil {
				m.note, m.noteErr = err.Error(), true
				return m, nil
			}
			m.note = ""
			m.pending = h
			// Stufe 2: Name (optional, leer => Target als Anzeige).
			m.stage = stageName
			m.input.Placeholder = "name (optional)"
			if m.editIdx >= 0 {
				m.input.SetValue(m.items[m.editIdx].Name)
			} else {
				m.input.SetValue("")
			}
			m.input.CursorEnd()
			return m, nil
		}

		// stageName: Host übernehmen.
		m.pending.Name = val
		if m.editIdx >= 0 {
			delete(m.status, m.items[m.editIdx].Target())
			m.items[m.editIdx] = m.pending
		} else {
			m.items = append(m.items, m.pending)
			m.cursor = len(m.items) - 1
		}
		added := m.pending
		m.closeInput()
		return m, tea.Batch(m.save(), pingCmd(added))
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m *hostsModule) View(width, height int) string {
	m.width, m.height = width, height

	nameStyle := lipgloss.NewStyle().Foreground(ui.ColCream)
	targetStyle := ui.DimStyle
	cursorStyle := lipgloss.NewStyle().Foreground(ui.ColPeach)
	upStyle := lipgloss.NewStyle().Foreground(ui.ColGood)
	downStyle := lipgloss.NewStyle().Foreground(ui.ColError)
	unknownStyle := lipgloss.NewStyle().Foreground(ui.ColFaint)

	rows := []string{ui.LabelStyle.Render("ssh hosts"), ""}
	if len(m.items) == 0 {
		rows = append(rows, ui.DimStyle.Render("no hosts yet — press 'a' to add"))
	}
	for i, h := range m.items {
		cursor := "  "
		if i == m.cursor && m.stage == stageNone {
			cursor = cursorStyle.Render("▸ ")
		}

		dot, latency := unknownStyle.Render("●"), ""
		if st, exists := m.status[h.Target()]; exists && st.known {
			if st.ok {
				dot = upStyle.Render("●")
				latency = ui.DimStyle.Render(fmt.Sprintf("  %dms", st.latency.Milliseconds()))
			} else {
				dot = downStyle.Render("●")
			}
		}

		name := nameStyle.Render(fmt.Sprintf("%-20s", truncate(h.DisplayName(), 20)))
		rows = append(rows, cursor+dot+" "+name+" "+targetStyle.Render(h.Target())+latency)
	}

	if m.stage != stageNone {
		label := "target: "
		if m.stage == stageName {
			label = "name: "
		}
		rows = append(rows, "", ui.LabelStyle.Render(label)+m.input.View())
	}

	if m.note != "" {
		style := ui.DimStyle
		if m.noteErr {
			style = lipgloss.NewStyle().Foreground(ui.ColError)
		}
		rows = append(rows, "", style.Render(m.note))
	}

	card := ui.CardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
	help := ui.HelpStyle.Render("enter connect · a add · e edit · d delete · r refresh · esc back")
	body := lipgloss.JoinVertical(lipgloss.Center, card, "", help)
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, body)
}

// truncate kürzt s (nach Runen) auf max Zeichen mit "…".
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}
