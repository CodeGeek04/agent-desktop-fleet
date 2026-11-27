#!/bin/sh
# Install adf, and smolvm if it is missing.
#
#   curl -fsSL https://raw.githubusercontent.com/CodeGeek04/agent-desktop-fleet/main/install.sh | sh
#
# Options (environment variables):
#   ADF_VERSION      version to install, such as 0.1.0 (default: the latest release)
#   ADF_INSTALL_DIR  where to put adf (default: ~/.local/bin)
#   ADF_SKIP_SMOLVM  set to 1 to skip installing smolvm
#   ADF_BASE_URL     download from this base URL instead of GitHub Releases (for testing)
set -eu

repo="CodeGeek04/agent-desktop-fleet"
install_dir="${ADF_INSTALL_DIR:-$HOME/.local/bin}"

say() { printf '%s\n' "$*"; }
fail() { printf 'adf install: %s\n' "$*" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || fail "$1 is required"; }
need curl
need tar
need uname

case "$(uname -s)/$(uname -m)" in
  Darwin/arm64) platform="darwin_arm64" ;;
  Linux/x86_64 | Linux/amd64) platform="linux_amd64" ;;
  Linux/aarch64 | Linux/arm64) platform="linux_arm64" ;;
  Darwin/x86_64) fail "Intel Macs are not supported: smolvm needs Apple silicon on macOS" ;;
  *) fail "unsupported platform $(uname -s) $(uname -m); adf runs on Apple silicon Macs and Linux (x86_64, arm64)" ;;
esac

version="${ADF_VERSION:-}"
if [ -z "$version" ]; then
  # The newest release whose tag looks like vX.Y.Z (image releases use image-vN tags).
  version="$(curl -fsSL "https://api.github.com/repos/${repo}/releases?per_page=30" \
    | grep -o '"tag_name": *"v[0-9][^"]*"' | head -1 | sed 's/.*"v\([^"]*\)"/\1/')" \
    || fail "could not look up the latest release"
  [ -n "$version" ] || fail "no adf release found"
fi
version="${version#v}"

base="${ADF_BASE_URL:-https://github.com/${repo}/releases/download/v${version}}"
archive="adf_${version}_${platform}.tar.gz"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

say "Downloading adf ${version} for ${platform}"
curl -fsSL "${base}/${archive}" -o "${tmp}/${archive}" || fail "download failed: ${base}/${archive}"
curl -fsSL "${base}/checksums.txt" -o "${tmp}/checksums.txt" || fail "download failed: ${base}/checksums.txt"

expected="$(awk -v f="$archive" '$2 == f {print $1}' "${tmp}/checksums.txt")"
[ -n "$expected" ] || fail "no checksum published for ${archive}"
if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "${tmp}/${archive}" | awk '{print $1}')"
else
  actual="$(shasum -a 256 "${tmp}/${archive}" | awk '{print $1}')"
fi
[ "$actual" = "$expected" ] || fail "checksum mismatch for ${archive}"

tar -xzf "${tmp}/${archive}" -C "$tmp"
mkdir -p "$install_dir"
install -m 755 "${tmp}/adf_${version}_${platform}/adf" "${install_dir}/adf"
say "Installed adf to ${install_dir}/adf"

if [ "${ADF_SKIP_SMOLVM:-}" != "1" ] && ! command -v smolvm >/dev/null 2>&1 && [ ! -x "$HOME/.smolvm/smolvm" ]; then
  say ""
  say "Installing smolvm, which runs each desktop in a microVM (https://github.com/smol-machines/smolvm)"
  curl -fsSL https://smolmachines.com/install.sh | bash -s -- --no-modify-path \
    || fail "smolvm install failed; install it yourself, then run: adf doctor"
fi

say ""
case ":$PATH:" in
  *":${install_dir}:"*) adf="adf" ;;
  *)
    adf="${install_dir}/adf"
    say "Note: ${install_dir} is not on your PATH. Add it, or use the full path below."
    ;;
esac
say "Next steps:"
say "  ${adf} doctor     check this computer can run desktops"
say "  ${adf} init       build the desktop template (once, about a minute)"
say "  ${adf} console    watch every desktop live"
say "  claude mcp add adf -- ${install_dir}/adf mcp    give Claude Code the desktop tools"
