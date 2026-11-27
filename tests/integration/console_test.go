//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/activity"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/console"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/fleet"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/runner"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/smolvm"
)

func TestConsoleEndToEnd(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	m := fleet.NewManager(smolvm.New(smolvmBin(t), runner.Exec{}), os.Stderr)
	logPath := filepath.Join(t.TempDir(), "activity.jsonl")
	srv := console.New(m, console.Options{Token: "it", ActivityPath: logPath, Recorder: &activity.Log{Path: logPath}})
	go srv.RunCollector(ctx)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req := func(method, path, body string) (*http.Response, []byte) {
		t.Helper()
		r, err := http.NewRequestWithContext(ctx, method, ts.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Authorization", "Bearer it")
		res, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res, b
	}

	_ = m.Delete(ctx, "con-it")
	if res, b := req("POST", "/api/desktops", `{"name":"con-it"}`); res.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d %s", res.StatusCode, b)
	}
	t.Cleanup(func() { _ = m.Delete(context.Background(), "con-it") })

	res, before := req("GET", "/api/desktops/con-it/screen", "")
	if res.StatusCode != http.StatusOK || !bytes.HasPrefix(before, []byte{0xFF, 0xD8, 0xFF}) {
		t.Fatalf("screen: %d, %d bytes", res.StatusCode, len(before))
	}
	for _, in := range []string{`{"kind":"key","key":"l","ctrl":true}`, `{"kind":"type","text":"about:robots"}`, `{"kind":"key","key":"Enter"}`} {
		if res, b := req("POST", "/api/desktops/con-it/input", in); res.StatusCode != http.StatusNoContent {
			t.Fatalf("input %s: %d %s", in, res.StatusCode, b)
		}
	}
	time.Sleep(2 * time.Second)
	if _, after := req("GET", "/api/desktops/con-it/screen", ""); bytes.Equal(before, after) {
		t.Fatal("screen did not change after typing a URL")
	}

	deadline := time.Now().Add(15 * time.Second)
	for {
		_, b := req("GET", "/api/desktops", "")
		var out struct {
			Desktops []struct {
				Name  string
				Stats *struct {
					MemTotalBytes int64 `json:"mem_total_bytes"`
				}
			}
		}
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatal(err)
		}
		ok := false
		for _, d := range out.Desktops {
			if d.Name == "con-it" && d.Stats != nil && d.Stats.MemTotalBytes > 0 {
				ok = true
			}
		}
		if ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no stats for con-it after 15 s: %s", b)
		}
		time.Sleep(500 * time.Millisecond)
	}

	_, b := req("GET", "/api/activity", "")
	var feed struct{ Events []activity.Event }
	if err := json.Unmarshal(b, &feed); err != nil {
		t.Fatal(err)
	}
	if len(feed.Events) < 4 {
		t.Fatalf("want create plus three inputs in the feed, got %+v", feed.Events)
	}
}
