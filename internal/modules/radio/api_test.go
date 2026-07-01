package radio

import "testing"

func TestIsStreamURL(t *testing.T) {
	cases := map[string]bool{
		"http://ice1.somafm.com/groovesalad-128-mp3": true,
		"https://stream.example.com/live":            true,
		"HTTP://UPPER.example.com/x":                 true, // Scheme wird normalisiert
		"  https://padded.example.com  ":             true, // Whitespace getrimmt
		"ftp://example.com/file.mp3":                 false,
		"techno":                                     false,
		"lofi hip hop":                               false,
		"http://":                                    false, // kein Host
		"":                                           false,
	}
	for in, want := range cases {
		if got := isStreamURL(in); got != want {
			t.Errorf("isStreamURL(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestCustomStationName(t *testing.T) {
	cases := map[string]string{
		"http://ice1.somafm.com/groovesalad": "ice1.somafm.com/groovesalad",
		"http://ice1.somafm.com/dronezone/":  "ice1.somafm.com/dronezone",
		"https://stream.example.com":         "stream.example.com",
	}
	for in, want := range cases {
		st := customStation(in)
		if st.Name != want {
			t.Errorf("customStation(%q).Name = %q, want %q", in, st.Name, want)
		}
		if st.StreamURL != in {
			t.Errorf("customStation(%q).StreamURL = %q", in, st.StreamURL)
		}
	}
}

func TestIsPlaylistURL(t *testing.T) {
	cases := map[string]bool{
		"https://somafm.com/groovesalad.pls":     true,
		"http://example.com/radio.m3u":           true,
		"http://example.com/hls/master.M3U8":     true, // case-insensitive
		"http://example.com/stream?fmt=pls":      false,
		"http://ice1.somafm.com/groovesalad-128": false,
		"http://example.com/song.mp3":            false,
	}
	for in, want := range cases {
		if got := isPlaylistURL(in); got != want {
			t.Errorf("isPlaylistURL(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestExtractStreamFromPlaylist(t *testing.T) {
	pls := "[playlist]\nnumberofentries=2\nFile1=http://ice1.somafm.com/groovesalad-128-mp3\nTitle1=SomaFM\nFile2=http://ice2.somafm.com/groovesalad-128-mp3\n"
	if got := extractStreamFromPlaylist(pls); got != "http://ice1.somafm.com/groovesalad-128-mp3" {
		t.Errorf("PLS: %q", got)
	}

	m3u := "#EXTM3U\n#EXTINF:-1,Some Station\nhttp://stream.example.com/live\n"
	if got := extractStreamFromPlaylist(m3u); got != "http://stream.example.com/live" {
		t.Errorf("M3U: %q", got)
	}

	if got := extractStreamFromPlaylist("#EXTM3U\n# nur kommentare\n"); got != "" {
		t.Errorf("leer erwartet, bekam %q", got)
	}
}
