#!/bin/sh
# Build release archives for every supported platform into dist/.
# Usage: scripts/build-release.sh VERSION   (for example 0.1.0, without the v)
set -eu
version="$1"
root="$(cd "$(dirname "$0")/.." && pwd)"
dist="$root/dist"
rm -rf "$dist" && mkdir -p "$dist"
# Intel Macs are left out: smolvm runs on Apple silicon only on macOS.
for target in darwin/arm64 linux/amd64 linux/arm64; do
  os="${target%/*}"
  arch="${target#*/}"
  name="adf_${version}_${os}_${arch}"
  mkdir -p "$dist/$name"
  (cd "$root" && CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
    -ldflags "-s -w -X main.version=${version}" -o "$dist/$name/adf" ./cmd/adf)
  cp "$root/LICENSE" "$root/README.md" "$dist/$name/"
  tar -C "$dist" -czf "$dist/$name.tar.gz" "$name"
  rm -rf "$dist/$name"
done
(cd "$dist" && (command -v sha256sum >/dev/null && sha256sum ./*.tar.gz || shasum -a 256 ./*.tar.gz) | sed 's# \./# #' > checksums.txt)
cat "$dist/checksums.txt"
