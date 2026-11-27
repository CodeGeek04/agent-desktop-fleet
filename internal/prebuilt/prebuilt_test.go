package prebuilt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

var ctx = context.Background()

func serve(t *testing.T, body []byte, checksum string) (*httptest.Server, *int32) {
	t.Helper()
	var hits int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		switch r.URL.Path {
		case "/img.tar.gz":
			_, _ = w.Write(body)
		case "/img.tar.gz.sha256":
			_, _ = w.Write([]byte(checksum + "  img.tar.gz\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)
	return ts, &hits
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func TestGetDownloadsVerifiesAndCaches(t *testing.T) {
	body := []byte(strings.Repeat("desktop image bytes ", 1000))
	ts, hits := serve(t, body, sum(body))
	f := &Fetcher{CacheDir: t.TempDir()}
	var msgs []string
	path, err := f.Get(ctx, ts.URL+"/img.tar.gz", func(m string) { msgs = append(msgs, m) })
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(body) || filepath.Base(path) != "img.tar.gz" {
		t.Fatalf("cached %q (%d bytes)", path, len(got))
	}
	if len(msgs) == 0 {
		t.Error("expected progress messages")
	}
	before := atomic.LoadInt32(hits)
	if again, err := f.Get(ctx, ts.URL+"/img.tar.gz", nil); err != nil || again != path {
		t.Fatalf("second get: %q %v", again, err)
	}
	if atomic.LoadInt32(hits) != before {
		t.Fatal("a cached image must not be downloaded again")
	}
}

func TestGetRejectsChecksumMismatch(t *testing.T) {
	body := []byte("tampered")
	ts, _ := serve(t, body, sum([]byte("original")))
	dir := t.TempDir()
	if _, err := (&Fetcher{CacheDir: dir}).Get(ctx, ts.URL+"/img.tar.gz", nil); err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("err = %v, want a checksum error", err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("nothing may stay in the cache after a failed download, found %d entries", len(entries))
	}
}

func TestGetFailsWhenUnavailable(t *testing.T) {
	ts, _ := serve(t, nil, "")
	if _, err := (&Fetcher{CacheDir: t.TempDir()}).Get(ctx, ts.URL+"/missing.tar.gz", nil); err == nil {
		t.Fatal("expected an error for a missing image")
	}
}

func TestURL(t *testing.T) {
	want := "https://github.com/CodeGeek04/agent-desktop-fleet/releases/download/image-v3/adf-desktop-v3-linux-arm64.tar.gz"
	if got := URL("3", "arm64"); got != want {
		t.Fatalf("URL = %q", got)
	}
}
