package radio

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"

	"github.com/leafrz/dashboard/internal/config"
)

// isStreamURL erkennt eine direkt abspielbare Stream-URL (http/https).
func isStreamURL(s string) bool {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// customStation baut einen Sender aus einer benutzerdefinierten Stream-URL.
// Anzeigename ist host/pfad (unterscheidet mehrere Streams desselben Hosts,
// z.B. ice1.somafm.com/groovesalad vs. /dronezone).
func customStation(raw string) config.Station {
	raw = strings.TrimSpace(raw)
	name := raw
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		name = u.Host
		if p := strings.Trim(u.Path, "/"); p != "" {
			name += "/" + p
		}
	}
	return config.Station{
		Name:      name,
		StreamURL: raw,
		Tags:      "custom stream",
	}
}

// isPlaylistURL erkennt Playlist-Formate (.pls/.m3u/.m3u8), die keinen rohen
// Audio-Stream enthalten, sondern auf ihn verweisen.
func isPlaylistURL(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	switch strings.ToLower(path.Ext(u.Path)) {
	case ".pls", ".m3u", ".m3u8":
		return true
	}
	return false
}

// extractStreamFromPlaylist zieht die erste Stream-URL aus einem Playlist-
// Body (PLS: "FileN=URL"-Zeilen; M3U: erste Nicht-Kommentar-Zeile mit http).
func extractStreamFromPlaylist(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// PLS: File1=http://...
		if i := strings.Index(line, "="); i >= 0 &&
			strings.HasPrefix(strings.ToLower(line), "file") {
			line = strings.TrimSpace(line[i+1:])
		}
		if strings.HasPrefix(strings.ToLower(line), "http") {
			return line
		}
	}
	return ""
}

// httpClient baut einen HTTP-Client für Playlist-Fetches. insecure=true
// überspringt die TLS-Zertifikatsprüfung (Fallback, siehe fetchWithTLSFallback).
func httpClient(insecure bool) *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
			ResponseHeaderTimeout: 5 * time.Second,
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: insecure},
		},
	}
}

// isCertError erkennt TLS-Zertifikatsfehler (unbekannte CA, kaputte Kette,
// Hostname-Mismatch). Viele Radio-Server liefern unvollständige Ketten, die
// Browser tolerieren, Go aber ablehnt.
func isCertError(err error) bool {
	if err == nil {
		return false
	}
	var certErr *tls.CertificateVerificationError
	var unknownAuth x509.UnknownAuthorityError
	var hostErr x509.HostnameError
	return errors.As(err, &certErr) ||
		errors.As(err, &unknownAuth) ||
		errors.As(err, &hostErr)
}

// fetchWithTLSFallback holt eine URL und versucht bei einem TLS-Zertifikats-
// fehler EINMAL erneut ohne Verifikation (öffentliche Playlists, keine
// sensiblen Daten).
func fetchWithTLSFallback(rawURL string) (*http.Response, error) {
	resp, err := httpClient(false).Get(rawURL)
	if err != nil && isCertError(err) {
		resp, err = httpClient(true).Get(rawURL)
	}
	return resp, err
}

// resolveStreamURL löst Playlist-URLs zur eigentlichen Stream-URL auf
// (max. 3 Hops, falls eine Playlist auf die nächste zeigt). Direkte
// Stream-URLs gehen unverändert durch.
func resolveStreamURL(raw string) (string, error) {
	cur := strings.TrimSpace(raw)
	for hop := 0; hop < 3; hop++ {
		if !isPlaylistURL(cur) {
			return cur, nil
		}
		resp, err := fetchWithTLSFallback(cur)
		if err != nil {
			return "", fmt.Errorf("playlist fetch: %w", err)
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		resp.Body.Close()
		if err != nil {
			return "", fmt.Errorf("playlist read: %w", err)
		}
		next := extractStreamFromPlaylist(string(body))
		if next == "" {
			return "", fmt.Errorf("no stream url found in playlist")
		}
		cur = next
	}
	return "", fmt.Errorf("too many playlist redirects")
}

// SearchStations fragt die Radio-Browser-API ab.
func SearchStations(query string) []list.Item {
	safeQuery := url.QueryEscape(query)

	apiURL := fmt.Sprintf("https://de1.api.radio-browser.info/json/stations/search?name=%s&order=clickcount&codec=mp3&limit=20", safeQuery)
	if query == "" {
		apiURL = "https://de1.api.radio-browser.info/json/stations/search?countrycode=DE&order=clickcount&codec=mp3&limit=20"
	}

	resp, err := fetchWithTLSFallback(apiURL)
	if err != nil {
		return []list.Item{config.Station{Name: "API Fehler", Tags: err.Error()}}
	}
	defer resp.Body.Close()

	var stations []config.Station
	if err := json.NewDecoder(resp.Body).Decode(&stations); err != nil {
		return []list.Item{config.Station{Name: "JSON Fehler", Tags: err.Error()}}
	}

	items := make([]list.Item, len(stations))
	for i, s := range stations {
		items[i] = s
	}

	if len(items) == 0 {
		return []list.Item{config.Station{Name: "Keine Ergebnisse", Tags: "Versuche einen anderen Suchbegriff"}}
	}

	return items
}
