package main

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

// clearFlashMsg löscht die transiente Toast-Nachricht.
type clearFlashMsg struct{}

// animMsg treibt die Equalizer-Animation an.
type animMsg time.Time

func animCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(t time.Time) tea.Msg {
		return animMsg(t)
	})
}

func doTick() tea.Cmd {
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// --- RADIO MODULE ---
type radioModule struct {
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

	flash      string    // transiente Toast-Nachricht (z.B. "★ added")
	flashUntil time.Time // Ablaufzeitpunkt der Toast-Nachricht
}

func newRadioModule() *radioModule { return &radioModule{} }

// Name erfüllt Module.
func (m *radioModule) Name() string { return "radio" }

// Status liefert den Live-Status für den Context-Header.
func (m *radioModule) Status() string {
	if m.uiPlaying && m.metadata != "" {
		return "♫ " + m.metadata
	}
	return ""
}

// setFlash zeigt eine kurze Toast-Nachricht (~2s) und gibt das Lösch-Cmd zurück.
func (m *radioModule) setFlash(s string) tea.Cmd {
	m.flash = s
	m.flashUntil = time.Now().Add(2 * time.Second)
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return clearFlashMsg{} })
}

// sleepMinutes ordnet sleepStep eine Dauer zu.
var sleepMinutes = []int{0, 15, 30, 60}

// cycleSleep schaltet den Sleep-Timer weiter (aus -> 15 -> 30 -> 60 -> aus).
func (m *radioModule) cycleSleep() {
	m.sleepStep = (m.sleepStep + 1) % len(sleepMinutes)
	min := sleepMinutes[m.sleepStep]
	if min == 0 {
		m.sleepUntil = time.Time{}
		return
	}
	m.sleepUntil = time.Now().Add(time.Duration(min) * time.Minute)
}

// refreshFavMarks markiert die aktuellen Listen-Items neu (Selektion bleibt).
func (m *radioModule) refreshFavMarks() {
	items := m.list.Items()
	markFavorites(items, m.favorites)
	idx := m.list.Index()
	m.list.SetItems(items)
	m.list.Select(idx)
}

func (m *radioModule) Init() tea.Cmd {
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

	// Eigene Tasten in die Hilfe-Leiste der Liste einblenden.
	m.list.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "play")),
			key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "favorite")),
			key.NewBinding(key.WithKeys("ctrl+f"), key.WithHelp("ctrl+f", "favorites")),
		}
	}

	m.state = stateSearch // Wir starten im Suchmodus

	return textinput.Blink
}

func (m *radioModule) updateUIState() {
	m.uiPlaying, m.uiPaused, m.uiVolume = m.radioPlayer.GetStatus()
	m.uiMuted = m.radioPlayer.IsMuted()
}

// startPlay startet die Wiedergabe der currentURL inkl. Connecting-Spinner.
func (m *radioModule) startPlay() tea.Cmd {
	m.state = statePlayer
	m.err = nil
	m.connecting = true
	m.metadata = ""
	return tea.Batch(m.playCmd(), m.spinner.Tick)
}

// persistCmd speichert Favoriten/Lautstärke/letzten Sender (merge, ohne die
// Header-Config zu überschreiben).
func (m *radioModule) persistCmd() tea.Cmd {
	favs := m.favorites
	vol := m.uiVolume
	last := m.lastStation
	return func() tea.Msg {
		_ = updateState(func(s *persistedState) {
			s.Favorites = favs
			s.LastVolume = vol
			s.LastStation = last
		})
		return nil
	}
}

func (m *radioModule) playCmd() tea.Cmd {
	return func() tea.Msg {
		if err := m.radioPlayer.Play(m.currentURL); err != nil {
			return errorMsg{err: err}
		}
		return playSuccessMsg{}
	}
}

func (m *radioModule) fetchMetaCmd() tea.Cmd {
	return func() tea.Msg {
		if !m.uiPlaying {
			return nil
		}
		// Metadaten werden inline aus dem laufenden Stream gelesen.
		return metadataMsg{metadata: m.radioPlayer.GetMetadata()}
	}
}

// --- UPDATE ---
func (m *radioModule) Update(msg tea.Msg) (Module, tea.Cmd) {
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

	case clearFlashMsg:
		if time.Now().After(m.flashUntil) {
			m.flash = ""
		}
		return m, nil

	case playSuccessMsg:
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
			case "esc":
				// Zurück zum Dashboard-Startmenü.
				return m, goToLauncher
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
						var nowFav bool
						m.favorites, nowFav = toggleFavorite(m.favorites, s)
						m.refreshFavMarks()
						return m, tea.Batch(m.persistCmd(), m.setFlash(favFlash(nowFav)))
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
					var nowFav bool
					m.favorites, nowFav = toggleFavorite(m.favorites, *m.lastStation)
					return m, tea.Batch(m.persistCmd(), m.setFlash(favFlash(nowFav)))
				}

			case "+", "=":
				m.radioPlayer.AdjustVolume(0.1)
				m.updateUIState()
				return m, m.persistCmd()

			case "-", "_":
				m.radioPlayer.AdjustVolume(-0.1)
				m.updateUIState()
				return m, m.persistCmd()
			}
		}
	}

	m.updateUIState()
	return m, tea.Batch(cmds...)
}

// searchCmd führt die Suche asynchron aus (blockiert die UI nicht).
func (m *radioModule) searchCmd(query string) tea.Cmd {
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

// --- VIEW ---
// View rendert NUR den Modul-Inhalt + eigenen Footer (kein globaler Header).
func (m *radioModule) View(width, height int) string {
	m.width = width
	m.height = height

	footer := m.footerView()
	contentStyle := lipgloss.NewStyle().Margin(1, 2)

	contentHeight := height - lipgloss.Height(footer)
	if contentHeight < 1 {
		contentHeight = 1
	}

	var content string
	if m.showHelp {
		content = lipgloss.Place(width, contentHeight,
			lipgloss.Center, lipgloss.Center, m.helpView())

	} else if m.state == stateSearch {
		var card string
		if m.searching {
			card = cardStyle.Render(m.spinner.View() + " " + labelStyle.Render("searching…"))
		} else {
			prompt := labelStyle.Render("find a station or a mood")
			input := lipgloss.NewStyle().Width(40).Render(m.textInput.View())
			card = cardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, prompt, "", input))
		}

		help := helpStyle.Render("enter: search   ·   ctrl+f: favorites   ·   esc: dashboard")
		parts := []string{card, "", help}
		if m.lastStation != nil {
			resume := dimStyle.Render("ctrl+r: resume ") + labelStyle.Render(m.lastStation.Name)
			parts = append(parts, resume)
		}
		searchContent := lipgloss.JoinVertical(lipgloss.Center, parts...)

		content = lipgloss.Place(width, contentHeight,
			lipgloss.Center, lipgloss.Center, searchContent)

	} else if m.state == stateList {
		h, v := contentStyle.GetFrameSize()
		m.list.SetSize(width-h, contentHeight-v)
		content = contentStyle.Render(m.list.View())

	} else {
		playerRendered := m.playerViewRender()
		content = lipgloss.Place(width, contentHeight,
			lipgloss.Center, lipgloss.Center, playerRendered)
	}

	return lipgloss.JoinVertical(lipgloss.Left, content, footer)
}

func (m *radioModule) playerViewRender() string {
	const cardInner = 40 // Innenbreite der Karte

	stationName := "Radio"
	if m.lastStation != nil {
		stationName = m.lastStation.Name
	} else if item := m.list.SelectedItem(); item != nil {
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

	titleText := stationNameStyle.Render(stationName)
	if isFavorite(m.favorites, m.currentURL) {
		titleText = lipgloss.NewStyle().Foreground(colPeach).Render("★ ") + titleText
	}
	title := center.Render(titleText)
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

// helpView rendert die MODUL-spezifische Hilfe (nur Radio-Tasten).
// Globale Befehle stehen auf der Dashboard-Hilfe (esc -> ?).
func (m *radioModule) helpView() string {
	sections := []helpSection{
		{title: "search", rows: [][2]string{
			{"enter", "search (empty = top DE)"},
			{"ctrl+f", "show favorites"},
			{"ctrl+r", "resume last station"},
			{"esc", "back to dashboard"},
		}},
		{title: "list", rows: [][2]string{
			{"↑/↓", "navigate"},
			{"/", "filter"},
			{"f", "toggle favorite"},
			{"enter", "play station"},
			{"esc", "back to search"},
		}},
		{title: "player", rows: [][2]string{
			{"space", "play / pause"},
			{"+ / −", "volume"},
			{"m", "mute"},
			{"t", "sleep timer (15/30/60)"},
			{"f", "favorite station"},
			{"esc / q", "back to list"},
		}},
	}
	return helpOverlay("radio · help", sections, "? or esc to close   ·   global commands on the dashboard")
}

// footerView zeigt Status (links) und Lautstärke (rechts).
func (m *radioModule) footerView() string {
	var statusText string
	switch {
	case m.flash != "":
		statusText = lipgloss.NewStyle().Foreground(colPeach).Bold(true).Render(m.flash)
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
