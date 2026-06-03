package main

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/faiface/beep/speaker"

	// HIER MUSS DEIN KORREKTER MODUL-PFAD STEHEN
	"github.com/leafrz/dashboard/radio"
)

// UI States
type sessionState int

const (
	stateSearch sessionState = iota // Startet mit Suche
	stateList
	statePlayer
)

// Messages
type tickMsg time.Time

type metadataMsg struct {
	metadata string
}

type errorMsg struct {
	err error
}
type playSuccessMsg struct{}

// animMsg treibt die Equalizer-Animation an.
type animMsg time.Time

func animCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg {
		return animMsg(t)
	})
}

// --- MODEL ---
type model struct {
	radioPlayer *radio.Player

	// UI Komponenten
	list      list.Model
	textInput textinput.Model

	state  sessionState
	width  int
	height int

	// Player State
	currentURL string
	metadata   string
	err        error
	uiPlaying  bool
	uiPaused   bool
	uiVolume   float64

	// Animation
	animFrame   int
	animTicking bool
}

func (m *model) Init() tea.Cmd {
	m.radioPlayer = radio.NewPlayer()
	m.updateUIState()

	// 1. Text Input konfigurieren
	ti := textinput.New()
	ti.Placeholder = "techno, rock, jazz… or leave empty"
	ti.Prompt = "› "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(colTeal)
	ti.TextStyle = lipgloss.NewStyle().Foreground(colCream)
	ti.PlaceholderStyle = lipgloss.NewStyle().Foreground(colFaint)
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(colPeach)
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 40
	m.textInput = ti

	// 2. Leere Liste initialisieren (lo-fi getönt)
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(colPeach).BorderForeground(colMauve)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(colMauve).BorderForeground(colMauve)
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.Foreground(colCream)
	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.Foreground(colDim)
	delegate.Styles.DimmedTitle = delegate.Styles.DimmedTitle.Foreground(colDim)
	delegate.Styles.DimmedDesc = delegate.Styles.DimmedDesc.Foreground(colFaint)

	m.list = list.New([]list.Item{}, delegate, 0, 0)
	m.list.Title = "Suchergebnisse"
	m.list.Styles.Title = m.list.Styles.Title.
		Background(colPurple).Foreground(colCream).Bold(true)
	m.list.Styles.FilterPrompt = m.list.Styles.FilterPrompt.Foreground(colTeal)
	m.list.Styles.FilterCursor = m.list.Styles.FilterCursor.Foreground(colPeach)

	m.state = stateSearch // Wir starten im Suchmodus

	return textinput.Blink
}

func (m *model) updateUIState() {
	m.uiPlaying, m.uiPaused, m.uiVolume = m.radioPlayer.GetStatus()
}

// --- COMMANDS ---
func (m *model) playCmd() tea.Cmd {
	return func() tea.Msg {
		err := m.radioPlayer.Play(m.currentURL)
		if err != nil {
			// Sende Fehler an die Update-Funktion
			return errorMsg{err: err}
		}
		// Sende Erfolg an die Update-Funktion
		return playSuccessMsg{}
	}
}

func (m *model) fetchMetaCmd() tea.Cmd {
	return func() tea.Msg {
		if !m.uiPlaying {
			return nil
		}

		// Metadaten werden jetzt inline aus dem laufenden Stream gelesen.
		// Wir holen hier nur den zuletzt bekannten Titel (kein Netzwerk).
		return metadataMsg{metadata: m.radioPlayer.GetMetadata()}
	}
}

func doTick() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// --- UPDATE ---
func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		h, v := lipgloss.NewStyle().Margin(1, 2).GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.radioPlayer.Stop()
			return m, tea.Quit
		}

	case tickMsg:
		if m.uiPlaying && m.state == statePlayer {
			return m, tea.Batch(m.fetchMetaCmd(), doTick())
		}
		return m, nil

	case playSuccessMsg:
		// Wiedergabe läuft -> Metadaten-Polling + Animation starten.
		m.uiPlaying = true
		m.err = nil
		cmds := []tea.Cmd{m.fetchMetaCmd(), doTick()}
		if !m.animTicking {
			m.animTicking = true
			cmds = append(cmds, animCmd())
		}
		return m, tea.Batch(cmds...)

	case animMsg:
		m.animFrame++
		if m.uiPlaying && m.state == statePlayer {
			return m, animCmd()
		}
		m.animTicking = false
		return m, nil

	case metadataMsg:
		// Wir überschreiben den Metadaten-String.
		// Wenn title aus fetchMetaCmd leer war (z.B. bei Werbung), wird m.metadata geleert.
		if msg.metadata != "" {
			m.metadata = msg.metadata
		} else {
			// Optionale Logik: Wenn leer, zeige "Werbung" oder "..." an, anstatt den letzten Songnamen zu behalten.
			// Fürs Erste: Wir lassen den alten Titel stehen, damit bei Werbung nicht ständig "..." blinkt.
			// Wenn du den Titel auf jeden Fall löschen willst, nutze: m.metadata = ""
		}

	case errorMsg:
		m.err = msg.err
		m.uiPlaying = false

		// Sicherstellen, dass der Player gestoppt ist
		m.radioPlayer.Stop()
		m.metadata = ""

		// 🛑 DIESE ZEILE LÖSCHEN ODER AUSKOMMENTIEREN:
		// m.state = stateList

		// Stattdessen nur Pausieren, damit der rote Fehlertext sichtbar bleibt:
		m.uiPaused = true
	}
	// --- STATE MACHINE ---

	switch m.state {

	// 1. SUCH-MODUS
	case stateSearch:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "enter":
				query := m.textInput.Value()

				// Blockiert kurz, um API-Ergebnisse zu holen
				items := SearchStations(query)

				m.list.SetItems(items)
				m.list.Title = "Ergebnisse für: " + query
				if query == "" {
					m.list.Title = "Top Sender DE"
				}

				m.state = stateList
				return m, nil
			}
		}
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)

	// 2. LISTEN-MODUS
	case stateList:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc":
				m.state = stateSearch
				m.textInput.Focus()
				return m, textinput.Blink

			case "enter":
				selectedItem := m.list.SelectedItem()
				if selectedItem != nil {
					s := selectedItem.(station)
					m.currentURL = s.StreamURL // <-- Korrekt: Großgeschrieben

					m.state = statePlayer
					m.err = nil
					m.metadata = "Lade Puffer..."
					m.uiPlaying = true
					return m, m.playCmd()
				}
			}
		}
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)

	// 3. PLAYER-MODUS
	case statePlayer:
		switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
			case "esc", "q", "backspace":
				m.radioPlayer.Stop()
				m.uiPlaying = false
				m.metadata = ""
				m.state = stateList
				return m, nil

			case "p", " ":
				if m.uiPlaying {
					m.radioPlayer.TogglePause()
				} else {
					m.err = nil
					m.uiPlaying = true
					return m, m.playCmd()
				}
			case "s":
				m.radioPlayer.Stop()
				m.metadata = ""
			case "+", "=":
				m.radioPlayer.AdjustVolume(0.1)
			case "-", "_":
				m.radioPlayer.AdjustVolume(-0.1)
			}
		}
	}

	m.updateUIState()
	return m, tea.Batch(cmds...)
}

// --- VIEW ---
func (m *model) View() string {

	// 1. Header (fixiert)
	header := m.headerView()

	// 2. Inhalt (dynamisch, basierend auf State)
	var content string
	contentStyle := lipgloss.NewStyle().Margin(1, 2)

	// Verfügbare Höhe für den Haupt-Content berechnen
	// 4 = Puffer/Margins + 2 = Die ungefähre Höhe von Header/Footer ohne Content-Margin
	contentHeight := m.height - lipgloss.Height(m.headerView()) - lipgloss.Height(m.footerView()) - 4

	if m.state == stateSearch {
		// Such-Ansicht: zentrierte Karte
		prompt := labelStyle.Render("find a station or a mood")
		input := lipgloss.NewStyle().Width(40).Render(m.textInput.View())
		help := helpStyle.Render("enter search   ·   leave empty for top DE   ·   esc quit")

		searchCard := cardStyle.Render(
			lipgloss.JoinVertical(lipgloss.Left, prompt, "", input),
		)
		searchContent := lipgloss.JoinVertical(lipgloss.Center, searchCard, "", help)

		content = lipgloss.Place(m.width, contentHeight,
			lipgloss.Center, lipgloss.Center, searchContent)

	} else if m.state == stateList {
		// Listen-Ansicht
		// Höhe der Liste an den verfügbaren Platz anpassen
		h, v := contentStyle.GetFrameSize()
		m.list.SetSize(m.width-h, contentHeight-v)
		content = contentStyle.Render(m.list.View())

	} else {
		// Player-Ansicht: Karte mittig platzieren
		playerRendered := m.playerViewRender()
		content = lipgloss.Place(m.width, contentHeight,
			lipgloss.Center, lipgloss.Center, playerRendered)
	}

	// 3. Footer (fixiert)
	footer := m.footerView()

	// 4. Alles zusammenfügen (vertikal stapeln)
	return lipgloss.JoinVertical(lipgloss.Left,
		header, // <--- Komma nicht nötig, da nächste Zeile ein Funktionsaufruf ist
		content,
		footer, // <--- KEIN Komma am Ende
	)
}

func (m *model) playerViewRender() string {
	const cardInner = 40 // Innenbreite der Karte

	stationName := "Radio"
	if item := m.list.SelectedItem(); item != nil {
		stationName = item.(station).Name
	}

	// Statuszeile mit kleinem Indikator
	var status string
	switch {
	case m.err != nil:
		status = lipgloss.NewStyle().Foreground(colError).Render("✕ " + m.err.Error())
	case m.uiPaused:
		status = lipgloss.NewStyle().Foreground(colPeach).Render("❚❚ paused")
	default:
		status = lipgloss.NewStyle().Foreground(colTeal).Render("▶ streaming")
	}

	// Equalizer (mittig)
	eq := renderEQ(m.animFrame, 16, 5, m.uiPaused || m.err != nil)
	eqBlock := lipgloss.NewStyle().Width(cardInner).Align(lipgloss.Center).Render(eq)

	// Now Playing
	nowPlaying := dimStyle.Render("— stille —")
	if m.metadata != "" {
		track := m.metadata
		nowPlaying = labelStyle.Render("♫ ") + nowPlayingStyle.Render(track)
	}
	nowPlaying = lipgloss.NewStyle().Width(cardInner).Align(lipgloss.Center).Render(nowPlaying)

	// Volume
	volBar := renderVolumeBar(m.uiVolume, 24)
	volLine := lipgloss.JoinHorizontal(lipgloss.Left,
		labelStyle.Render("vol "),
		volBar,
		dimStyle.Render(fmt.Sprintf(" %3.0f%%", m.uiVolume*100)),
	)
	volLine = lipgloss.NewStyle().Width(cardInner).Align(lipgloss.Center).Render(volLine)

	// Titelzeile in der Karte
	title := lipgloss.NewStyle().Width(cardInner).Align(lipgloss.Center).Render(
		stationNameStyle.Render(stationName),
	)
	statusLine := lipgloss.NewStyle().Width(cardInner).Align(lipgloss.Center).Render(status)

	body := lipgloss.JoinVertical(lipgloss.Left,
		title,
		statusLine,
		"",
		eqBlock,
		"",
		nowPlaying,
		"",
		volLine,
	)

	card := cardStyle.Render(body)

	help := helpStyle.Render("space play/pause   ·   +/− volume   ·   esc back")

	return lipgloss.JoinVertical(lipgloss.Center, card, "", help)
}

// Zeigt den fixen Header an (Uhrzeit, App-Titel)
func (m *model) headerView() string {
	title := appTitleStyle.Render("◖ lofi.radio ◗")
	tag := tagStyle.Render("  ˗ˏˋ warm static & late nights ˎˊ˗")
	left := lipgloss.JoinHorizontal(lipgloss.Bottom, title, tag)

	currentTime := clockStyle.Render(time.Now().Format("15:04"))

	spacerW := m.width - lipgloss.Width(left) - lipgloss.Width(currentTime) - 2
	if spacerW < 1 {
		spacerW = 1
	}
	bar := lipgloss.JoinHorizontal(lipgloss.Bottom,
		left,
		lipgloss.NewStyle().Width(spacerW).Render(""),
		currentTime,
	)

	rule := horizontalRule(m.width - 2)
	return lipgloss.NewStyle().Padding(0, 1).Render(
		lipgloss.JoinVertical(lipgloss.Left, bar, rule),
	)
}

// Zeigt den fixen Footer an (Lautstärke, Status)
func (m *model) footerView() string {

	// 1. Status/Metadaten
	var statusText string
	switch {
	case m.state == stateSearch:
		statusText = dimStyle.Render("ready · enter for top DE charts")
	case m.state == stateList:
		statusText = lipgloss.NewStyle().Foreground(colTeal).Render(m.list.Title)
	case m.uiPaused:
		statusText = lipgloss.NewStyle().Foreground(colPeach).Render("❚❚ paused")
	case m.err != nil:
		statusText = lipgloss.NewStyle().Foreground(colError).Render("✕ " + m.err.Error())
	case m.uiPlaying && m.metadata != "":
		statusText = lipgloss.NewStyle().Foreground(colMauve).Render("♫ " + m.metadata)
	default:
		statusText = dimStyle.Render("radio ready")
	}

	// 2. Volume Bar (kompakt)
	bar := renderVolumeBar(m.uiVolume, 12)
	volumeInfo := lipgloss.JoinHorizontal(lipgloss.Left,
		dimStyle.Render("vol "),
		bar,
		dimStyle.Render(fmt.Sprintf(" %3.0f%%", m.uiVolume*100)),
	)

	rule := horizontalRule(m.width - 2)

	spacerW := m.width - lipgloss.Width(statusText) - lipgloss.Width(volumeInfo) - 4
	if spacerW < 1 {
		spacerW = 1
	}
	row := lipgloss.JoinHorizontal(lipgloss.Bottom,
		statusText,
		lipgloss.NewStyle().Width(spacerW).Render(""),
		volumeInfo,
	)

	return lipgloss.NewStyle().Padding(0, 1).Render(
		lipgloss.JoinVertical(lipgloss.Left, rule, row),
	)
}

func main() {
	m := &model{}
	p := tea.NewProgram(m, tea.WithAltScreen())

	// Speaker EINMALIG mit fester Rate initialisieren und offen halten.
	// Streams mit abweichender Rate werden im Player resampled.
	err := speaker.Init(radio.SampleRate, radio.SampleRate.N(time.Second/10))
	if err != nil {
		fmt.Printf("FATALER FEHLER: Audio-Gerät konnte nicht initialisiert werden: %v\n", err)
		return // Programm beenden, da Audio nicht geht
	}
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
