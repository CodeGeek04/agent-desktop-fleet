# Runtime spike: microsandbox vs smolvm (2026-11-27)

Goal: pick the VM runtime for an open-source, easy-to-install fleet of agent desktops.
Both candidates are built on libkrun, Apache-2.0, and claim Linux, macOS and Windows hosts.

## Test host

- Apple M5, 10 cores, 24 GB RAM, macOS 27.2.1 (Apple Silicon, Hypervisor.framework)
- Guest: `debian:trixie-slim` (arm64), 2 vCPU, 2 GiB
- Desktop stack (same script for both, see `images/`): Xvfb 1280x800, openbox, tint2, xdotool, x11vnc, scrot, Firefox ESR
- Not tested yet: Linux host, Windows host, fleets larger than 4 machines

## Versions

- smolvm 1.37.7 (2026-11-26)
- microsandbox (msb) 0.8.3 (2026-11-26, self-labelled beta)

## Install

| | smolvm | microsandbox |
|---|---|---|
| Homebrew tap | Failed: formula wants Xcode 27 (host has 26.2) | Not reached (same brew run) |
| Install script | Worked, checksum verified, `--no-modify-path` honoured | Worked, checksum verified |
| Sudo / daemon | None | None |
| Notarized | No ("signed but not notarized") | Not reported |

Lesson for our product: ship prebuilt, notarized binaries; never depend on the user's Xcode version.

## Measurements

| Metric | smolvm | microsandbox |
|---|---|---|
| Alpine boot, run, exit (cached image) | 1.3 s | 0.37 s |
| First boot of new machine | 12.4 s | 4.3 s |
| Desktop package install (same script) | 20 s | 95 s |
| Cold boot to Firefox window visible | 0.91 s | 1.06 s |
| Screenshot round trip (exec + scrot) | 40 to 60 ms | 20 to 40 ms |
| Host memory, single cold-booted desktop | 1.5 GB phys_footprint | 1.2 to 2.7 GB |
| Copy a running desktop | Live branch, source keeps running: 0.47 to 1.57 s | Full snapshot 1.55 s (2.0 GB on disk), then restore with `--cow-mem`: 0.31 s (first 1.07 s) |
| Host memory per idle copy | 241 to 247 MB | 195 to 210 MB |
| Host memory per copy after loading Wikipedia | not measured | 415 MB |

Memory is macOS `phys_footprint` (private memory charged to the VM process), not RSS.
RSS over-counts because shared snapshot pages are charged to every process.

## Host-side display (smolvm only)

`SMOLVM_DISPLAY=1280x800 SMOLVM_VNC=<port>` gives the guest a virtio-gpu screen and serves it
from the host as RFB, a browser client and a WebSocket. Results:

- Screenshot from the host over VNC with no VNC server or screenshot tool in the guest: works.
- Keyboard input from the host into the guest: works, after the image creates `/dev/input/event*`
  nodes and runs udev with `/sys` remounted read-write so libinput sees tagged devices.
- Guest compositor: sway with `WLR_RENDERER=pixman` and `LIBSEAT_BACKEND=noop`.
- Firefox (native Wayland) passes get.webgl.org with software rendering, no host GPU.
- Cost: a continuously animating WebGL page uses about 1.3 host cores; static pages are cheap.
- The host VNC port has no authentication, so it must stay on loopback behind our own proxy.

Why this matters: screenshots, input, the live grid and human takeover can all come from the
host for any Linux image with a compositor. The guest needs no agent software.

## Gotchas found

- microsandbox: a full snapshot fails if a host folder is mounted and a file moves out of it on
  the host. Desktop machines should not use host mounts; move files through an API instead.
- microsandbox: snapshots record mounts; restoring without them needs `--allow-missing-resources`.
- smolvm: input devices exist in the guest but the slim image has no device manager, so the image
  must create the nodes and run udev at boot.
- Firefox first run opens a privacy-notice tab; the image needs Firefox policies to suppress it.

## Recommendation (first pass, Mac only; revisited after the Linux run below)

Build on **smolvm**, behind our own small runtime interface so a second backend can be added later.

1. Host-side display and input is the deciding feature. It makes the computer API, the live grid
   and takeover guest-agnostic and removes VNC and screenshot tooling from every guest.
2. Live branching without pausing the source fits fork and reset for evals and RL.
3. Much faster provisioning (20 s vs 95 s) and slightly faster cold boot to desktop.
4. Bundled GPU path on macOS (virglrenderer + MoltenVK) for WebGL-heavy work later.
5. More mature release line (1.37 vs 0.8 beta with breaking changes in the latest release).

Costs we accept: about 20 percent more memory per idle copy, slower copy creation than
microsandbox's warm restore, and an unauthenticated VNC port we must wrap.

## Linux host results (AWS, same day)

Host: AWS c8i.4xlarge in ap-south-1 (16 vCPU, 30 GiB, Ubuntu 24.04, kernel 7.0), launched with
`--cpu-options NestedVirtualization=enabled` (needs a newer AWS CLI than 2.27; latest v1 via `uvx` works).
Price: $0.75/hour on demand. smolvm 1.37.7, same Wayland desktop (`images/wayland-setup.sh`).

| Metric | Result |
|---|---|
| Alpine boot, run, exit (cached) | 1.8 to 2.0 s (Mac: 1.3 s) |
| Desktop provisioning | 60 s |
| Machine start without display | 0.54 s |
| Machine start with `SMOLVM_DISPLAY` | 5.8 s, every time |
| Cold boot to Firefox window (with display) | 7.7 to 8.3 s |
| Host-side keyboard input and capture | Works (Ctrl+L, type URL, Enter, capture) |
| One capture with the `vncdo` CLI | 0.64 s (CLI startup dominates) |
| Branch 10 live desktops from one source | 0.43 to 2.20 s each, 13.4 s total |
| Host memory for 10 branches | +2.55 GB total (about 255 MB each) |
| Per-branch PSS / private dirty | 218 to 262 MB / 188 to 223 MB |

The 5.3 s display penalty on Linux matches smolvm's documented 5 second wait for a host
`/dev/dri` render node; this host has no GPU. Workaround or upstream fix needed.

## Blocker: branchable mode and host display do not work together

On smolvm 1.37.7, both on macOS and Linux:

- A machine started with `--branchable` and `SMOLVM_DISPLAY` opens its VNC port and completes the
  RFB handshake, but never sends a frame. Captures hang. This applies to the source and to every branch.
- The same machine started without `--branchable` captures normally.
- Forcing a redraw (pointer move, `swaymsg reload`, output power cycle, restarting sway) does not help.
- `machine checkpoint` of a branchable source failed with "invalid checkpoint stream".

So today we can have cheap shared-memory copies (about 250 MB each) or host-side display and input,
not both. A cold-booted desktop with host display costs roughly 1.2 to 1.5 GB instead.

## Options for the capture path

1. In-guest capture with cheap copies (works today, any runtime): a tiny guest agent takes
   screenshots and injects input (Xvfb + xdotool, or grim + wtype on Wayland). Grid thumbnails reuse
   those screenshots; a VNC server starts in the guest only when a human opens a machine.
2. Host display without copies: guest-agnostic, but each desktop cold boots and costs about 5x the memory.
3. Fix smolvm upstream so branchable machines keep their display, then move to host capture later.

## Next steps

1. Pick the capture path from the options above.
2. Bake the image as an OCI image in CI (Firefox policies, session-restore off).
3. Native capture and input client in our daemon; measure latency without CLI startup.
4. CPU per idle desktop at fleet scale (the top sampling in `bench/fleet-linux.sh` needs a fix).
