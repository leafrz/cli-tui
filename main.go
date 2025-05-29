package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

const streamURL = "https://streams.ilovemusic.de/iloveradio1.mp3"

type metadataMsg struct {
	metadata string
}

type statusMsg struct {
	playing bool
	paused  bool
	error   string
}

type model struct {
	playing     bool
	paused      bool
	ctrl        *beep.Ctrl
	volume      *effects.Volume
	volumeLevel float64
	metadata    string
	error       string
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	speakerInit bool
	httpResp    *http.Response
}

func (m *model) Init() tea.Cmd {
	m.volumeLevel = 0.5 // Start at 50% volume
	return nil
}

func (m *model) playStream() tea.Cmd {
	return func() tea.Msg {
		// Stop any existing stream
		m.stopStream()

		// Create new context for this stream
		m.ctx, m.cancel = context.WithCancel(context.Background())

		// Create HTTP request with proper headers for streaming
		req, err := http.NewRequestWithContext(m.ctx, "GET", streamURL, nil)
		if err != nil {
			return statusMsg{error: fmt.Sprintf("Failed to create request: %v", err)}
		}

		// Set headers for audio streaming
		req.Header.Set("User-Agent", "RadioPlayer/1.0")
		req.Header.Set("Accept", "audio/mpeg, */*")
		req.Header.Set("Connection", "keep-alive")

		// Use a client with no timeout for streaming
		client := &http.Client{
			Timeout: 0, // No timeout for streaming
		}

		resp, err := client.Do(req)
		if err != nil {
			return statusMsg{error: fmt.Sprintf("Failed to connect: %v", err)}
		}

		m.httpResp = resp

		// Create a wrapper that keeps the stream alive
		streamReader := &streamWrapper{
			ctx:    m.ctx,
			reader: resp.Body,
		}

		// Decode the MP3 stream
		streamer, format, err := mp3.Decode(streamReader)
		if err != nil {
			resp.Body.Close()
			return statusMsg{error: fmt.Sprintf("Failed to decode MP3: %v", err)}
		}

		// Initialize speaker if not already done
		if !m.speakerInit {
			err = speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
			if err != nil {
				streamer.Close()
				resp.Body.Close()
				return statusMsg{error: fmt.Sprintf("Failed to initialize audio: %v", err)}
			}
			m.speakerInit = true
		}

		// Create control wrapper with volume
		m.volume = &effects.Volume{
			Streamer: streamer,
			Base:     2,
			Volume:   m.volumeLevel,
			Silent:   false,
		}

		m.ctrl = &beep.Ctrl{
			Streamer: m.volume,
			Paused:   false,
		}

		// Clear any existing audio and start playing
		speaker.Clear()

		// Play the stream directly - no looping needed for infinite streams
		speaker.Play(m.ctrl)

		return statusMsg{playing: true, paused: false}
	}
}

// streamWrapper keeps the HTTP connection alive and handles context cancellation
type streamWrapper struct {
	ctx    context.Context
	reader io.ReadCloser
}

func (sw *streamWrapper) Read(p []byte) (n int, err error) {
	select {
	case <-sw.ctx.Done():
		return 0, sw.ctx.Err()
	default:
		return sw.reader.Read(p)
	}
}

func (sw *streamWrapper) Close() error {
	return sw.reader.Close()
}

func (m *model) stopStream() {
	if m.cancel != nil {
		m.cancel()
	}

	if m.speakerInit {
		speaker.Clear()
	}

	if m.httpResp != nil {
		m.httpResp.Body.Close()
		m.httpResp = nil
	}

	m.playing = false
	m.paused = false
	m.ctrl = nil
}

func (m *model) togglePause() {
	if m.ctrl != nil && m.speakerInit {
		speaker.Lock()
		m.ctrl.Paused = !m.ctrl.Paused
		m.paused = m.ctrl.Paused
		speaker.Unlock()
	}
}

func (m *model) adjustVolume(delta float64) {
	m.volumeLevel += delta
	if m.volumeLevel > 1.0 {
		m.volumeLevel = 1.0
	}
	if m.volumeLevel < 0.0 {
		m.volumeLevel = 0.0
	}

	if m.volume != nil {
		speaker.Lock()
		m.volume.Volume = m.volumeLevel
		speaker.Unlock()
	}

	if m.ctrl != nil && m.speakerInit {
		speaker.Lock()
		m.ctrl.Paused = !m.ctrl.Paused
		m.paused = m.ctrl.Paused
		speaker.Unlock()
	}
}

func (m *model) fetchMetadata() tea.Cmd {
	return func() tea.Msg {
		if !m.playing {
			return nil
		}

		// Create a separate request for metadata
		req, err := http.NewRequest("GET", streamURL, nil)
		if err != nil {
			return nil
		}

		req.Header.Set("Icy-MetaData", "1")
		req.Header.Set("User-Agent", "RadioPlayer/1.0")

		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil
		}
		defer resp.Body.Close()

		metaIntStr := resp.Header.Get("icy-metaint")
		if metaIntStr == "" {
			return nil
		}

		metaInt, err := strconv.Atoi(metaIntStr)
		if err != nil || metaInt <= 0 {
			return nil
		}

		reader := bufio.NewReader(resp.Body)

		// Skip audio data
		_, err = io.CopyN(io.Discard, reader, int64(metaInt))
		if err != nil {
			return nil
		}

		// Read metadata length
		metaLenByte, err := reader.ReadByte()
		if err != nil {
			return nil
		}

		metaLen := int(metaLenByte) * 16
		if metaLen == 0 {
			return nil
		}

		// Read metadata
		metaData := make([]byte, metaLen)
		_, err = io.ReadFull(reader, metaData)
		if err != nil {
			return nil
		}

		// Parse StreamTitle
		metaStr := strings.TrimRight(string(metaData), "\x00")
		if strings.Contains(metaStr, "StreamTitle=") {
			start := strings.Index(metaStr, "StreamTitle='") + 13
			if start < 13 {
				start = strings.Index(metaStr, "StreamTitle=\"") + 13
			}
			if start >= 13 {
				end := strings.IndexAny(metaStr[start:], "';\"")
				if end > 0 {
					title := strings.TrimSpace(metaStr[start : start+end])
					if title != "" {
						return metadataMsg{metadata: title}
					}
				}
			}
		}

		return nil
	}
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.stopStream()
			return m, tea.Quit
		case "p", " ":
			if !m.playing {
				m.error = ""
				return m, m.playStream()
			} else {
				m.togglePause()
			}
		case "s":
			m.stopStream()
			m.error = ""
			m.metadata = ""
		case "m":
			if m.playing {
				return m, m.fetchMetadata()
			}
		case "up", "+", "=":
			m.adjustVolume(0.1) // Increase volume by 10%
		case "down", "-":
			m.adjustVolume(-0.1) // Decrease volume by 10%
		}
	case statusMsg:
		m.playing = msg.playing
		m.paused = msg.paused
		m.error = msg.error
		if m.playing && m.error == "" {
			// Start fetching metadata periodically
			return m, tea.Tick(10*time.Second, func(t time.Time) tea.Msg {
				return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}}
			})
		}
	case metadataMsg:
		m.metadata = msg.metadata
	}
	return m, nil
}

func (m *model) View() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Padding(1, 2)

	statusStyle := lipgloss.NewStyle().
		Padding(0, 2).
		Foreground(lipgloss.Color("86"))

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Padding(0, 2)

	metaStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("111")).
		Italic(true).
		Padding(0, 2)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Padding(1, 2)

	title := titleStyle.Render("Internet Radio Player")

	var status string
	if m.error != "" {
		status = errorStyle.Render("Error: " + m.error)
	} else if m.playing {
		if m.paused {
			status = statusStyle.Render("Paused")
		} else {
			status = statusStyle.Render("Playing - " + streamURL)
		}
	} else {
		status = statusStyle.Render("Stopped")
	}

	var meta string
	if m.metadata != "" && m.playing {
		meta = "\n" + metaStyle.Render("♪ Now Playing: "+m.metadata)
	}

	help := helpStyle.Render(`
Controls:
  p/space = play/pause
  s       = stop  
  m       = refresh metadata
  +/up    = volume up
  -/down  = volume down
  q       = quit`)

	volumeInfo := fmt.Sprintf("\nVolume: %.0f%%", m.volumeLevel*100)

	return fmt.Sprintf("%s\n\n%s%s%s%s", title, status, volumeInfo, meta, help)
}

func main() {
	m := &model{}
	p := tea.NewProgram(m)

	if err := p.Start(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
