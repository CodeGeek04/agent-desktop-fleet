package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/CodeGeek04/agent-desktop-fleet/internal/image"
	"github.com/CodeGeek04/agent-desktop-fleet/internal/prebuilt"
)

// prebuiltImage picks where a new template's image comes from. By default it is the release
// asset for this image version and CPU, downloaded once into the user cache. ADF_IMAGE
// overrides it: "build" always builds from Debian, an http(s) URL downloads that file, and
// anything else is used as a local image file.
func prebuiltImage() func(context.Context, func(string)) (string, error) {
	override := os.Getenv("ADF_IMAGE")
	switch {
	case override == "build":
		return nil
	case override != "" && !strings.HasPrefix(override, "http://") && !strings.HasPrefix(override, "https://"):
		return func(context.Context, func(string)) (string, error) { return override, nil }
	}
	url := prebuilt.URL(image.Version, runtime.GOARCH)
	if override != "" {
		url = override
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil
	}
	f := &prebuilt.Fetcher{CacheDir: filepath.Join(cache, "adf", "images")}
	return func(ctx context.Context, progress func(string)) (string, error) {
		return f.Get(ctx, url, progress)
	}
}
