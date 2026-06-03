package main

import (
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
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

// searchResultMsg liefert die Ergebnisse einer asynchronen Suche.
type searchResultMsg struct {
	items []list.Item
	title string
}

// animMsg treibt die Equalizer-Animation an.
type animMsg time.Time

func animCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg {
		return animMsg(t)
	})
}

// searchCmd führt die Suche asynchron aus (blockiert die UI nicht).
func (m *model) searchCmd(query string) tea.Cmd {
	favs := m.favorites
	return func() tea.Msg {
		items := SearchStations(query)
		markFavorites(items, favs)
		title := "ergebnisse: " + query
		if query == "" {
			title = "top sender DE"
		}
		return searchResultMsg{items: items, title: title}
	}
}

// saveStateCmd persistiert den Zustand asynchron (Fehler werden ignoriert).
func saveStateCmd(s persistedState) tea.Cmd {
	return func() tea.Msg {
		_ = saveState(s)
		return nil
	}
}

// --- MODEL ---
type model struct {
	radioPlayer *radio.Player

	// UI Komponenten
	list      list.Model
	textInput textinput.Model
	spinner   spinner.Model

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
	uiMuted    bool

	// Animation
	animFrame   int
	animTicking bool

	// QoL
	connecting  bool      // verbindet gerade -> Spinner statt EQ
	searching   bool      // Suche läuft -> Spinner
	showHelp    bool      // Hilfe-Overlay
	favorites   []station // persistente Favoriten
	lastStation *station  // zuletzt gespielter Sender (für Resume)
	sleepUntil  time.Time // Sleep-Timer Ziel (zero = aus)
	sleepStep   int       // 0=aus, 1=15m, 2=30m, 3=60m
}

// sleepMinutes ordnet sleepStep eine Dauer zu.
var sleepMinutes = []int{0, 15, 30, 60}

// cycleSleep schaltet den Sleep-Timer weiter (aus -> 15 -> 30 -> 60 -> aus).
func (m *model) cycleSleep() {
	m.sleepStep = (m.sleepStep + 1) % len(sleepMinutes)
	min := sleepMinutes[m.sleepStep]
	if min == 0 {
		m.sleepUntil = time.Time{}
		return
	}
	m.sleepUntil = time.Now().Add(time.Duration(min) * time.Minute)
}

// refreshFavMarks markiert die aktuellen Listen-Items neu (Selektion bleibt).
func (m *model) refreshFavMarks() {
	items := m.list.Items()
	markFavorites(items, m.favorites)
	idx := m.list.Index()
	m.list.SetItems(items)
	m.list.Select(idx)
}

func (m *model) Init() tea.Cmd {
	m.radioPlayer = radio.NewPlayer()

	// Persistenten Zustand laden (Favoriten, letzte Lautstärke, letzter Sender).
	st := loadState()
	m.favorites = st.Favorites
	m.lastStation = st.LastStation
	m.radioPlayer.SetVolume(st.LastVolume)
	m.updateUIState()

	// Spinner (Verbinden/Suchen)
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colTeal)
	m.spinner = sp

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
	m.uiMuted = m.radioPlayer.IsMuted()
}

// startPlay startet die Wiedergabe der currentURL inkl. Connecting-Spinner.
func (m *model) startPlay() tea.Cmd {
	m.state = statePlayer
	m.err = nil
	m.connecting = true
	m.metadata = ""
	return tea.Batch(m.playCmd(), m.spinner.Tick)
}

// volume / sleep helpers
func (m *model) persistCmd() tea.Cmd {
	return saveStateCmd(persistedState{
		Favorites:   m.favorites,
		LastVolume:  m.uiVolume,
		LastStation: m.lastStation,
	})
}

// --- COMMANDS ---
func (m *model) playCmd() tea.Cmd {
	return func() tea.Msg {
		if err := m.radioPlayer.Play(m.currentURL); err != nil {
			return errorMsg{err: err}
		}
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
		key := msg.String()
		if key == "ctrl+c" {
			m.radioPlayer.Stop()
			return m, tea.Sequence(m.persistCmd(), tea.Quit)
		}
		// Hilfe-Overlay schluckt alle Tasten, solange es offen ist.
		if m.showHelp {
			if key == "esc" || key == "?" || key == "q" {
				m.showHelp = false
			}
			return m, nil
		}
		// '?' öffnet die Hilfe (nicht im Such-Eingabefeld, dort ist es Text).
		if key == "?" && m.state != stateSearch {
			m.showHelp = true
			return m, nil
		}

	case spinner.TickMsg:
		if m.connecting || m.searching {
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case searchResultMsg:
		m.searching = false
		m.list.SetItems(msg.items)
		m.list.Title = msg.title
		m.state = stateList
		return m, nil

	case playSuccessMsg:
		// Wiedergabe läuft -> Metadaten-Polling + Animation starten.
		m.connecting = false
		m.uiPlaying = true
		m.err = nil
		m.metadata = ""
		batch := []tea.Cmd{m.fetchMetaCmd(), doTick()}
		if !m.animTicking {
			m.animTicking = true
			batch = append(batch, animCmd())
		}
		return m, tea.Batch(batch...)

	case animMsg:
		m.animFrame++
		if m.uiPlaying && m.state == statePlayer {
			return m, animCmd()
		}
		m.animTicking = false
		return m, nil

	case tickMsg:
		// Sleep-Timer abgelaufen?
		if !m.sleepUntil.IsZero() && time.Now().After(m.sleepUntil) {
			m.sleepUntil = time.Time{}
			m.sleepStep = 0
			m.radioPlayer.Stop()
			m.uiPlaying = false
			m.metadata = ""
			m.state = stateList
			return m, nil
		}
		// Auto-Reconnect bei Stream-Abbruch.
		if m.uiPlaying && !m.uiPaused && m.state == statePlayer && m.radioPlayer.Ended() {
			m.connecting = true
			m.metadata = ""
			return m, tea.Batch(m.playCmd(), m.spinner.Tick, doTick())
		}
		if m.uiPlaying && m.state == statePlayer {
			return m, tea.Batch(m.fetchMetaCmd(), doTick())
		}
		return m, nil

	case metadataMsg:
		if msg.metadata != "" {
			m.metadata = msg.metadata
		}

	case errorMsg:
		m.connecting = false
		m.err = msg.err
		m.uiPlaying = false
		m.radioPlayer.Stop()
		m.metadata = ""
		m.uiPaused = true
	}

	// --- STATE MACHINE ---
	switch m.state {

	// 1. SUCH-MODUS
	case stateSearch:
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "enter":
				m.searching = true
				return m, tea.Batch(m.searchCmd(m.textInput.Value()), m.spinner.Tick)
			case "ctrl+f":
				m.list.SetItems(favoritesAsItems(m.favorites))
				m.list.Title = "★ favoriten"
				m.state = stateList
				return m, nil
			case "ctrl+r":
				if m.lastStation != nil {
					m.currentURL = m.lastStation.StreamURL
					return m, m.startPlay()
				}
			}
		}
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)

	// 2. LISTEN-MODUS
	case stateList:
		if km, ok := msg.(tea.KeyMsg); ok && !m.list.SettingFilter() {
			switch km.String() {
			case "esc":
				m.state = stateSearch
				m.textInput.Focus()
				return m, textinput.Blink

			case "f":
				if it := m.list.SelectedItem(); it != nil {
					if s, ok := it.(station); ok && s.StreamURL != "" {
						m.favorites, _ = toggleFavorite(m.favorites, s)
						m.refreshFavMarks()
						return m, m.persistCmd()
					}
				}

			case "ctrl+f":
				m.list.SetItems(favoritesAsItems(m.favorites))
				m.list.Title = "★ favoriten"
				return m, nil

			case "enter":
				if it := m.list.SelectedItem(); it != nil {
					if s, ok := it.(station); ok && s.StreamURL != "" {
						m.currentURL = s.StreamURL
						ls := s
						ls.Favorite = false
						m.lastStation = &ls
						return m, tea.Batch(m.startPlay(), m.persistCmd())
					}
				}
			}
		}
		m.list, cmd = m.list.Update(msg)
		cmds = append(cmds, cmd)

	// 3. PLAYER-MODUS
	case statePlayer:
		if km, ok := msg.(tea.KeyMsg); ok {
			switch km.String() {
			case "esc", "q", "backspace":
				m.radioPlayer.Stop()
				m.uiPlaying = false
				m.connecting = false
				m.metadata = ""
				m.state = stateList
				return m, nil

			case "p", " ":
				if m.uiPlaying {
					m.radioPlayer.TogglePause()
				} else {
					return m, m.startPlay()
				}

			case "s":
				m.radioPlayer.Stop()
				m.uiPlaying = false
				m.metadata = ""

			case "m":
				m.radioPlayer.ToggleMute()

			case "t":
				m.cycleSleep()

			case "f":
				if m.lastStation != nil {
					m.favorites, _ = toggleFavorite(m.favorites, *m.lastStation)
					return m, m.persistCmd()
				}

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

	if m.showHelp {
		content = lipgloss.Place(m.width, contentHeight,
			lipgloss.Center, lipgloss.Center, m.helpView())

	} else if m.state == stateSearch {
		// Such-Ansicht: zentrierte Karte
		var card string
		if m.searching {
			card = cardStyle.Render(m.spinner.View() + " " + labelStyle.Render("searching…"))
		} else {
			prompt := labelStyle.Render("find a station or a mood")
			input := lipgloss.NewStyle().Width(40).Render(m.textInput.View())
			card = cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, prompt, "", input))
		}

		help := helpStyle.Render("enter search   ·   ⌃f favorites   ·   ? help   ·   esc quit")
		parts := []string{card, "", help}
		if m.lastStation != nil {
			resume := dimStyle.Render("⌃r resume ") + labelStyle.Render(m.lastStation.Name)
			parts = append(parts, resume)
		}
		searchContent := lipgloss.JoinVertical(lipgloss.Center, parts...)

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

	center := lipgloss.NewStyle().Width(cardInner).Align(lipgloss.Center)

	// Statuszeile mit kleinem Indikator
	var status string
	switch {
	case m.err != nil:
		status = lipgloss.NewStyle().Foreground(colError).Render("✕ " + m.err.Error())
	case m.connecting:
		status = lipgloss.NewStyle().Foreground(colTeal).Render(m.spinner.View() + " connecting…")
	case m.uiPaused:
		status = lipgloss.NewStyle().Foreground(colPeach).Render("❚❚ paused")
	default:
		status = lipgloss.NewStyle().Foreground(colTeal).Render("▶ streaming")
	}

	// Mitte: Spinner beim Verbinden, sonst Equalizer.
	var midBlock string
	if m.connecting {
		midBlock = center.Render(lipgloss.NewStyle().Foreground(colMauve).Render(
			m.spinner.View() + " " + m.spinner.View() + " " + m.spinner.View()))
		// Höhe an EQ angleichen
		midBlock = lipgloss.NewStyle().Height(5).Render(midBlock)
	} else {
		eq := renderEQ(m.animFrame, 16, 5, m.uiPaused || m.err != nil)
		midBlock = center.Render(eq)
	}

	// Now Playing
	nowPlaying := dimStyle.Render("— stille —")
	if m.metadata != "" {
		nowPlaying = labelStyle.Render("♫ ") + nowPlayingStyle.Render(m.metadata)
	}
	nowPlaying = center.Render(nowPlaying)

	// Volume (oder MUTED)
	var volLine string
	if m.uiMuted {
		volLine = center.Render(lipgloss.NewStyle().Foreground(colPeach).Render("🔇 muted"))
	} else {
		bar := renderVolumeBar(m.uiVolume, 24)
		volLine = center.Render(lipgloss.JoinHorizontal(lipgloss.Left,
			labelStyle.Render("vol "),
			bar,
			dimStyle.Render(fmt.Sprintf(" %3.0f%%", m.uiVolume*100)),
		))
	}

	title := center.Render(stationNameStyle.Render(stationName))
	statusLine := center.Render(status)

	rows := []string{title, statusLine, "", midBlock, "", nowPlaying, "", volLine}

	// Sleep-Timer Hinweis
	if !m.sleepUntil.IsZero() {
		rem := time.Until(m.sleepUntil).Round(time.Minute)
		rows = append(rows, "", center.Render(
			lipgloss.NewStyle().Foreground(colPurple).Render(
				fmt.Sprintf("☾ sleep in %dm", int(rem.Minutes())))))
	}

	card := cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))

	help := helpStyle.Render("space play/pause · +/− vol · m mute · t sleep · f fav · ? help · esc back")

	return lipgloss.JoinVertical(lipgloss.Center, card, "", help)
}

// helpView rendert ein Tasten-Cheatsheet als zentrierte Karte.
func (m *model) helpView() string {
	key := lipgloss.NewStyle().Foreground(colPeach).Bold(true)
	desc := lipgloss.NewStyle().Foreground(colCream)
	head := lipgloss.NewStyle().Foreground(colMauve).Bold(true)

	row := func(k, d string) string {
		return lipgloss.JoinHorizontal(lipgloss.Left,
			key.Width(12).Render(k), desc.Render(d))
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		head.Render("search"),
		row("enter", "search (empty = top DE)"),
		row("⌃f", "show favorites"),
		row("⌃r", "resume last station"),
		"",
		head.Render("list"),
		row("↑/↓", "navigate"),
		row("/", "filter"),
		row("f", "toggle favorite"),
		row("enter", "play station"),
		row("esc", "back to search"),
		"",
		head.Render("player"),
		row("space", "play / pause"),
		row("+ / −", "volume"),
		row("m", "mute"),
		row("t", "sleep timer (15/30/60)"),
		row("f", "favorite this station"),
		row("esc / q", "back to list"),
		"",
		head.Render("global"),
		row("?", "toggle this help"),
		row("⌃c", "quit"),
	)

	card := cardStyle.Render(body)
	hint := helpStyle.Render("press ? or esc to close")
	return lipgloss.JoinVertical(lipgloss.Center, card, "", hint)
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

	// 2. Volume Bar (kompakt) bzw. Muted-Hinweis
	var volumeInfo string
	if m.uiMuted {
		volumeInfo = lipgloss.NewStyle().Foreground(colPeach).Render("🔇 muted")
	} else {
		bar := renderVolumeBar(m.uiVolume, 12)
		volumeInfo = lipgloss.JoinHorizontal(lipgloss.Left,
			dimStyle.Render("vol "),
			bar,
			dimStyle.Render(fmt.Sprintf(" %3.0f%%", m.uiVolume*100)),
		)
	}

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
