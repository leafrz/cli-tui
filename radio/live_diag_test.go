package radio

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
)

// TestLivePipeline checks the HTTP -> icyReader -> mp3.Decode -> samples chain
// WITHOUT touching the audio device. Run with:
//
//	go test ./radio -run TestLivePipeline -v
func TestLivePipeline(t *testing.T) {
	const streamURL = "http://ice1.somafm.com/groovesalad-128-mp3"

	ctx := context.Background()
	req, err := http.NewRequest("GET", streamURL, nil)
	if err != nil {
		t.Fatalf("request creation: %v", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (TobiRadio-Go)")
	req.Header.Set("Icy-MetaData", "1")
	req = req.WithContext(ctx)

	client := &http.Client{
		Transport: &http.Transport{
			DialContext:           (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
			ResponseHeaderTimeout: 5 * time.Second,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do failed: %v", err)
	}
	defer resp.Body.Close()

	t.Logf("status: %s", resp.Status)
	t.Logf("content-type: %q", resp.Header.Get("Content-Type"))
	t.Logf("icy-metaint: %q", resp.Header.Get("icy-metaint"))

	var src io.Reader = bufio.NewReader(resp.Body)
	if metaIntStr := resp.Header.Get("icy-metaint"); metaIntStr != "" {
		if metaInt, e := strconv.Atoi(metaIntStr); e == nil && metaInt > 0 {
			src = &icyReader{src: src, metaInt: metaInt, player: NewPlayer()}
			t.Logf("icyReader active, metaInt=%d", metaInt)
		}
	}

	streamer, format, err := mp3.Decode(io.NopCloser(src))
	if err != nil {
		t.Fatalf("mp3.Decode failed: %v", err)
	}
	t.Logf("decoded OK, stream sample rate = %d Hz", format.SampleRate)

	// Pull ~0.5s of samples and check we actually get audio.
	buf := make([][2]float64, 22050)
	n, ok := streamer.Stream(buf)
	t.Logf("streamer.Stream -> n=%d ok=%v", n, ok)
	if n == 0 {
		t.Fatalf("no samples decoded")
	}

	var nonZero int
	for i := 0; i < n; i++ {
		if buf[i][0] != 0 || buf[i][1] != 0 {
			nonZero++
		}
	}
	t.Logf("non-zero samples: %d / %d", nonZero, n)
	if nonZero == 0 {
		t.Fatalf("decoded only silence")
	}

	_ = beep.SampleRate(0) // keep beep import
}
