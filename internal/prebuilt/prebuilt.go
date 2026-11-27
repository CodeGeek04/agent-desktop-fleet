// Package prebuilt downloads the prebuilt desktop image from a GitHub Release once, checks it
// against its published SHA-256, and keeps it in a local cache.
package prebuilt

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Repo hosts the image releases.
const Repo = "CodeGeek04/agent-desktop-fleet"

// Name is the release asset for an image version and CPU architecture (amd64 or arm64).
func Name(version, arch string) string {
	return fmt.Sprintf("adf-desktop-v%s-linux-%s.tar.gz", version, arch)
}

// URL is where that asset is published.
func URL(version, arch string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/image-v%s/%s", Repo, version, Name(version, arch))
}

// Fetcher downloads images into CacheDir.
type Fetcher struct {
	Client   *http.Client // nil means http.DefaultClient
	CacheDir string
}

func (f *Fetcher) client() *http.Client {
	if f.Client != nil {
		return f.Client
	}
	return http.DefaultClient
}

// Get returns the local path of the image at url, downloading it on first use. The download is
// checked against url+".sha256" before it enters the cache. progress may be nil.
func (f *Fetcher) Get(ctx context.Context, url string, progress func(string)) (string, error) {
	if progress == nil {
		progress = func(string) {}
	}
	name := path.Base(url)
	dest := filepath.Join(f.CacheDir, name)
	if _, err := os.Stat(dest); err == nil {
		return dest, nil
	}
	want, err := f.checksum(ctx, url+".sha256")
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(f.CacheDir, 0o755); err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(f.CacheDir, name+".part-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename

	resp, err := f.get(ctx, url)
	if err != nil {
		tmp.Close()
		return "", err
	}
	defer resp.Body.Close()
	h := sha256.New()
	counter := &progressWriter{total: resp.ContentLength, report: progress}
	_, err = io.Copy(io.MultiWriter(tmp, h, counter), resp.Body)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", fmt.Errorf("download %s: %w", name, err)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return "", fmt.Errorf("download %s: checksum %s does not match the published %s", name, got, want)
	}
	if err := os.Rename(tmp.Name(), dest); err != nil {
		return "", err
	}
	return dest, nil
}

func (f *Fetcher) get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.client().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	return resp, nil
}

func (f *Fetcher) checksum(ctx context.Context, url string) (string, error) {
	resp, err := f.get(ctx, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(b))
	if len(fields) == 0 || len(fields[0]) != 64 {
		return "", errors.New("published checksum is malformed")
	}
	return strings.ToLower(fields[0]), nil
}

// progressWriter reports every 10% of a download of known size, or every 50 MB otherwise.
type progressWriter struct {
	total, done, next int64
	report            func(string)
}

func (p *progressWriter) Write(b []byte) (int, error) {
	p.done += int64(len(b))
	if p.total > 0 {
		pct := p.done * 100 / p.total
		if pct >= p.next {
			p.report(fmt.Sprintf("downloading the desktop image: %d%% of %d MB", pct, p.total>>20))
			p.next = pct + 10
		}
	} else if p.done >= p.next {
		p.report(fmt.Sprintf("downloading the desktop image: %d MB", p.done>>20))
		p.next = p.done + 50<<20
	}
	return len(b), nil
}
