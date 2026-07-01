package radio

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/leafrz/dashboard/internal/config"
	"github.com/leafrz/dashboard/internal/core"
	"github.com/leafrz/dashboard/internal/ui"

	"github.com/leafrz/dashboard/internal/audio"
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

// customStreamMsg liefert das Ergebnis der Custom-URL-Auflösung (Playlists
// werden async per HTTP zur eigentlichen Stream-URL aufgelöst).
type customStreamMsg struct {
	st  config.Station
	err error
}

// resolveCustomCmd löst eine benutzerdefinierte Stream-/Playlist-URL auf.
func resolveCustomCmd(raw string) tea.Cmd {
	return func() tea.Msg {
		st := customStation(raw) // Name aus der Original-URL (host/pfad)
		streamURL, err := resolveStreamURL(raw)
		if err != nil {
			return customStreamMsg{err: err}
		}
		st.StreamURL = streamURL
		return customStreamMsg{st: st}
	}
}

// animMsg treibt die Equalizer-Animation an.
type animMsg time.Time

func animCmd() tea.Cmd {
	// ~12.5 fps – schnell genug für einen flüssigen Visualizer, ruhig genug
	// für die TUI.
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
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
	radioPlayer *audio.Player

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

	// Visualizer (echtes Spektrum, geglättet)
	animTicking bool
	cardLevels  []float64 // 16 Bänder für die Player-Karte
	fullLevels  []float64 // breitenabhängig für Vollbild
	vizFull     bool      // Vollbild-Visualizer

	// QoL
	connecting  bool             // verbindet gerade -> Spinner statt EQ
	searching   bool             // Suche läuft -> Spinner
	showHelp    bool             // Hilfe-Overlay
	favorites   []config.Station // persistente Favoriten
	lastStation *config.Station  // zuletzt gespielter Sender (für Resume)
	sleepUntil  time.Time        // Sleep-Timer Ziel (zero = aus)
	sleepStep   int              // 0=aus, 1=15m, 2=30m, 3=60m

	flash      string    // transiente Toast-Nachricht (z.B. "★ added")
	flashUntil time.Time // Ablaufzeitpunkt der Toast-Nachricht
}

func New(p *audio.Player) *radioModule { return &radioModule{radioPlayer: p} }

// Name erfüllt core.Module.
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
	// radioPlayer wird vom Root injiziert (geteilt mit dem Visualizer).

	// Persistenten Zustand laden (Favoriten, letzte Lautstärke, letzter Sender).
	st := config.Load()
	m.favorites = st.Favorites
	m.lastStation = st.LastStation
	m.radioPlayer.SetVolume(st.LastVolume)
	m.updateUIState()

	// Spinner (Verbinden/Suchen)
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	m.spinner = sp

	// 1. Text Input konfigurieren (Struktur; Farben via restyle)
	ti := textinput.New()
	ti.Placeholder = "techno, rock, jazz… or paste a stream URL"
	ti.Prompt = "› "
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 40
	m.textInput = ti

	// 2. Leere Liste initialisieren
	m.list = list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	m.list.Title = "Suchergebnisse"

	// Eigene Tasten in die Hilfe-Leiste der Liste einblenden.
	m.list.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "play")),
			key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "favorite")),
			key.NewBinding(key.WithKeys("ctrl+f"), key.WithHelp("ctrl+f", "favorites")),
		}
	}

	m.restyle() // Farben der Komponenten an das aktuelle Theme anpassen

	m.state = stateSearch // Wir starten im Suchmodus

	return textinput.Blink
}

// restyle wendet die aktuelle Palette auf die Bubbles-Komponenten an
// (Spinner, Eingabe, Liste). Wird bei Init und bei Theme-Wechsel aufgerufen.
func (m *radioModule) restyle() {
	m.spinner.Style = lipgloss.NewStyle().Foreground(ui.ColTeal)

	m.textInput.PromptStyle = lipgloss.NewStyle().Foreground(ui.ColTeal)
	m.textInput.TextStyle = lipgloss.NewStyle().Foreground(ui.ColCream)
	m.textInput.PlaceholderStyle = lipgloss.NewStyle().Foreground(ui.ColFaint)
	m.textInput.Cursor.Style = lipgloss.NewStyle().Foreground(ui.ColPeach)

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(ui.ColPeach).BorderForeground(ui.ColMauve)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(ui.ColMauve).BorderForeground(ui.ColMauve)
	delegate.Styles.NormalTitle = delegate.Styles.NormalTitle.Foreground(ui.ColCream)
	delegate.Styles.NormalDesc = delegate.Styles.NormalDesc.Foreground(ui.ColDim)
	delegate.Styles.DimmedTitle = delegate.Styles.DimmedTitle.Foreground(ui.ColDim)
	delegate.Styles.DimmedDesc = delegate.Styles.DimmedDesc.Foreground(ui.ColFaint)
	m.list.SetDelegate(delegate)

	m.list.Styles.Title = lipgloss.NewStyle().
		Padding(0, 1).Background(ui.ColPurple).Foreground(ui.ColCream).Bold(true)
	m.list.Styles.FilterPrompt = lipgloss.NewStyle().Foreground(ui.ColTeal)
	m.list.Styles.FilterCursor = lipgloss.NewStyle().Foreground(ui.ColPeach)
}

func (m *radioModule) updateUIState() {
	m.uiPlaying, m.uiPaused, m.uiVolume = m.radioPlayer.GetStatus()
	m.uiMuted = m.radioPlayer.IsMuted()
}

// bandsForWidth bestimmt die Balkenzahl für das Vollbild (ein Band = 2 Spalten).
func (m *radioModule) bandsForWidth() int {
	b := m.width / 2
	if b < 8 {
		b = 8
	}
	if b > 96 {
		b = 96
	}
	return b
}

// sampleSpectrum holt das echte Spektrum vom Player und glättet es
// (Attack sofort, Decay langsam -> fallende Balken). Bei Stille/Pause -> abklingen.
func (m *radioModule) sampleSpectrum() {
	apply := func(buf []float64, spec []float64) []float64 {
		if spec == nil {
			for i := range buf {
				buf[i] *= 0.8
			}
			return buf
		}
		for i := range buf {
			if spec[i] > buf[i] {
				buf[i] = spec[i]
			} else {
				buf[i] = buf[i]*0.82 + spec[i]*0.18
			}
		}
		return buf
	}

	if m.vizFull {
		bands := m.bandsForWidth()
		if len(m.fullLevels) != bands {
			m.fullLevels = make([]float64, bands)
		}
		m.fullLevels = apply(m.fullLevels, m.radioPlayer.Spectrum(bands))
	} else {
		if len(m.cardLevels) != 16 {
			m.cardLevels = make([]float64, 16)
		}
		m.cardLevels = apply(m.cardLevels, m.radioPlayer.Spectrum(16))
	}
}

// startPlay startet die Wiedergabe der currentURL inkl. Connecting-Spinner.
func (m *radioModule) startPlay() tea.Cmd {
	m.state = statePlayer
	m.err = nil
	m.connecting = true
	m.metadata = ""
	if m.lastStation != nil {
		m.radioPlayer.SetStationName(m.lastStation.Name) // für globalen Footer
	}
	return tea.Batch(m.playCmd(), m.spinner.Tick)
}

// persistCmd speichert Favoriten/Lautstärke/letzten Sender (merge, ohne die
// Header-Config zu überschreiben).
func (m *radioModule) persistCmd() tea.Cmd {
	favs := m.favorites
	vol := m.uiVolume
	last := m.lastStation
	return func() tea.Msg {
		_ = config.Update(func(s *config.State) {
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
func (m *radioModule) Update(msg tea.Msg) (core.Module, tea.Cmd) {
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

	case customStreamMsg:
		m.searching = false
		if msg.err != nil {
			return m, m.setFlash("✗ " + msg.err.Error())
		}
		st := msg.st
		m.currentURL = st.StreamURL
		m.lastStation = &st
		return m, tea.Batch(m.startPlay(), m.persistCmd())

	case clearFlashMsg:
		if time.Now().After(m.flashUntil) {
			m.flash = ""
		}
		return m, nil

	case core.ThemeChangedMsg:
		m.restyle()
		return m, nil

	case core.FocusMsg:
		// Beim (Wieder-)Öffnen die Ticker neu starten — sie sterben, während
		// das Modul inaktiv ist.
		var cmds []tea.Cmd
		if m.state == stateSearch {
			cmds = append(cmds, textinput.Blink)
		}
		if m.uiPlaying && m.state == statePlayer {
			// Die Ticker sind beim Inaktiv-Sein gestorben (animMsg ging ans dann
			// aktive Modul). animTicking kann veraltet true sein -> bedingungslos
			// neu starten, sonst bleibt der Visualizer stehen.
			m.animTicking = true
			cmds = append(cmds, m.fetchMetaCmd(), doTick(), animCmd())
		}
		return m, tea.Batch(cmds...)

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
		if m.uiPlaying && m.state == statePlayer {
			m.sampleSpectrum()
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
		// Auto-Reconnect passiert jetzt im Player selbst (auch wenn dieses Modul
		// inaktiv ist). Hier nur noch Metadaten-Polling.
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
				return m, core.GoToLauncher
			case "enter":
				// Direkte Stream-URL? -> auflösen (Playlists) und abspielen.
				// Async, weil .pls/.m3u erst per HTTP geholt werden müssen.
				if isStreamURL(m.textInput.Value()) {
					m.searching = true
					return m, tea.Batch(resolveCustomCmd(m.textInput.Value()), m.spinner.Tick)
				}
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
					if s, ok := it.(config.Station); ok && s.StreamURL != "" {
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
					if s, ok := it.(config.Station); ok && s.StreamURL != "" {
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
				if m.vizFull {
					// Erst Vollbild verlassen, Wiedergabe läuft weiter.
					m.vizFull = false
					return m, nil
				}
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

			case "v":
				m.vizFull = !m.vizFull

			case "a":
				// In den Ambient-Modus wechseln; Audio läuft weiter.
				return m, core.SwitchTo("ambient")

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

	// Now-Playing/Volume zeigt der globale Footer (Root). Hier nur noch ein
	// kurzer Flash-Toast (Favorit hinzugefügt/entfernt) bei Bedarf.
	var footer string
	if m.flash != "" {
		footer = lipgloss.NewStyle().Width(width).Align(lipgloss.Center).
			Foreground(ui.ColPeach).Bold(true).Render(m.flash)
	}
	footerH := 0
	if footer != "" {
		footerH = lipgloss.Height(footer)
	}
	contentStyle := lipgloss.NewStyle().Margin(1, 2)

	contentHeight := height - footerH
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
			card = ui.CardStyle.Render(m.spinner.View() + " " + ui.LabelStyle.Render("searching…"))
		} else {
			prompt := ui.LabelStyle.Render("find a station, a mood, or paste a URL")
			input := lipgloss.NewStyle().Width(40).Render(m.textInput.View())
			card = ui.CardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, prompt, "", input))
		}

		help := ui.HelpStyle.Render("enter: search / play URL   ·   ctrl+f: favorites   ·   esc: dashboard")
		parts := []string{card, "", help}
		if m.lastStation != nil {
			resume := ui.DimStyle.Render("ctrl+r: resume ") + ui.LabelStyle.Render(m.lastStation.Name)
			parts = append(parts, resume)
		}
		searchContent := lipgloss.JoinVertical(lipgloss.Center, parts...)

		content = lipgloss.Place(width, contentHeight,
			lipgloss.Center, lipgloss.Center, searchContent)

	} else if m.state == stateList {
		h, v := contentStyle.GetFrameSize()
		m.list.SetSize(width-h, contentHeight-v)
		content = contentStyle.Render(m.list.View())

	} else if m.vizFull && m.uiPlaying && !m.connecting {
		content = m.fullVizView(width, contentHeight)
	} else {
		playerRendered := m.playerViewRender()
		content = lipgloss.Place(width, contentHeight,
			lipgloss.Center, lipgloss.Center, playerRendered)
	}

	if footer != "" {
		return lipgloss.JoinVertical(lipgloss.Left, content, footer)
	}
	return content
}

func (m *radioModule) playerViewRender() string {
	const cardInner = 40 // Innenbreite der Karte

	stationName := "Radio"
	if m.lastStation != nil {
		stationName = m.lastStation.Name
	} else if item := m.list.SelectedItem(); item != nil {
		stationName = item.(config.Station).Name
	}

	center := lipgloss.NewStyle().Width(cardInner).Align(lipgloss.Center)

	// Statuszeile mit kleinem Indikator
	var status string
	switch {
	case m.err != nil:
		status = lipgloss.NewStyle().Foreground(ui.ColError).Render("✕ " + m.err.Error())
	case m.connecting:
		status = lipgloss.NewStyle().Foreground(ui.ColTeal).Render(m.spinner.View() + " connecting…")
	case m.uiPaused:
		status = lipgloss.NewStyle().Foreground(ui.ColPeach).Render("❚❚ paused")
	default:
		status = lipgloss.NewStyle().Foreground(ui.ColTeal).Render("▶ streaming")
	}

	// Mitte: Spinner beim Verbinden, sonst Equalizer.
	var midBlock string
	if m.connecting {
		midBlock = center.Render(lipgloss.NewStyle().Foreground(ui.ColMauve).Render(
			m.spinner.View() + " " + m.spinner.View() + " " + m.spinner.View()))
		midBlock = lipgloss.NewStyle().Height(5).Render(midBlock)
	} else {
		// Echtes Spektrum (geglättet) aus dem laufenden Audio.
		midBlock = center.Render(ui.RenderBars(m.cardLevels, 5))
	}

	// Now Playing
	nowPlaying := ui.DimStyle.Render("— stille —")
	if m.metadata != "" {
		nowPlaying = ui.LabelStyle.Render("♫ ") + ui.NowPlayingStyle.Render(m.metadata)
	}
	nowPlaying = center.Render(nowPlaying)

	// Volume (oder MUTED)
	var volLine string
	if m.uiMuted {
		volLine = center.Render(lipgloss.NewStyle().Foreground(ui.ColPeach).Render("🔇 muted"))
	} else {
		bar := ui.RenderVolumeBar(m.uiVolume, 24)
		volLine = center.Render(lipgloss.JoinHorizontal(lipgloss.Left,
			ui.LabelStyle.Render("vol "),
			bar,
			ui.DimStyle.Render(fmt.Sprintf(" %3.0f%%", m.uiVolume*100)),
		))
	}

	titleText := ui.StationNameStyle.Render(stationName)
	if isFavorite(m.favorites, m.currentURL) {
		titleText = lipgloss.NewStyle().Foreground(ui.ColPeach).Render("★ ") + titleText
	}
	title := center.Render(titleText)
	statusLine := center.Render(status)

	rows := []string{title, statusLine, "", midBlock, "", nowPlaying, "", volLine}

	// Sleep-Timer Hinweis
	if !m.sleepUntil.IsZero() {
		rem := time.Until(m.sleepUntil).Round(time.Minute)
		rows = append(rows, "", center.Render(
			lipgloss.NewStyle().Foreground(ui.ColPurple).Render(
				fmt.Sprintf("☾ sleep in %dm", int(rem.Minutes())))))
	}

	card := ui.CardStyle.Render(lipgloss.JoinVertical(lipgloss.Left, rows...))

	help := ui.HelpStyle.Render("space · +/− vol · m mute · v viz · a ambient · t sleep · f fav · ? help · esc")

	return lipgloss.JoinVertical(lipgloss.Center, card, "", help)
}

// fullVizView rendert den bildschirmfüllenden Visualizer (echtes Spektrum).
func (m *radioModule) fullVizView(width, height int) string {
	footerH := 2
	rows := height - footerH
	if rows < 3 {
		rows = 3
	}
	spectrum := ui.RenderSpectrum(m.fullLevels, width, rows)

	track := ""
	if m.metadata != "" {
		track = ui.NowPlayingStyle.Render("♫ " + m.metadata)
	}
	help := ui.HelpStyle.Render("v: exit fullscreen   ·   space: pause   ·   esc: back")
	bottom := lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(
		lipgloss.JoinVertical(lipgloss.Center, track, help))

	return lipgloss.JoinVertical(lipgloss.Left, spectrum, bottom)
}

// helpView rendert die MODUL-spezifische Hilfe (nur Radio-Tasten).
// Globale Befehle stehen auf der Dashboard-Hilfe (esc -> ?).
func (m *radioModule) helpView() string {
	sections := []ui.HelpSection{
		{Title: "search", Rows: [][2]string{
			{"enter", "search (empty = top DE)"},
			{"enter", "paste http(s):// URL = play custom stream"},
			{"ctrl+f", "show favorites"},
			{"ctrl+r", "resume last station"},
			{"esc", "back to dashboard"},
		}},
		{Title: "list", Rows: [][2]string{
			{"↑/↓", "navigate"},
			{"/", "filter"},
			{"f", "toggle favorite"},
			{"enter", "play station"},
			{"esc", "back to search"},
		}},
		{Title: "player", Rows: [][2]string{
			{"space", "play / pause"},
			{"+ / −", "volume"},
			{"m", "mute"},
			{"v", "fullscreen visualizer"},
			{"a", "ambient mode (keeps playing)"},
			{"t", "sleep timer (15/30/60)"},
			{"f", "favorite station"},
			{"esc / q", "back to list"},
		}},
	}
	return ui.HelpOverlay("radio · help", sections, "? or esc to close   ·   global commands on the dashboard")
}
