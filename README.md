# adf: a fleet of desktops for AI agents

Run many lightweight, sandboxed Linux desktops on your own computer, give any agent the tools to
use them over MCP, and watch every one of them live. adf is infrastructure only: desktop
lifecycle, computer-use tools and a console. Bring your own agent.

Each desktop is a [smolvm](https://github.com/smol-machines/smolvm) microVM running Xvfb, openbox
and Firefox at 1280x800. Desktops are copies of a running template, so they start in about a
second and share its memory until they change it.

Status: early development. Tested on macOS (Apple silicon) and Linux (KVM).

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/CodeGeek04/agent-desktop-fleet/main/install.sh | sh
```

The script installs `adf` to `~/.local/bin` (it checks the published SHA-256) and installs smolvm
if it is missing. Or with Homebrew:

```sh
brew tap CodeGeek04/adf https://github.com/CodeGeek04/agent-desktop-fleet
brew install CodeGeek04/adf/adf
```

Supported: Apple silicon Macs, and Linux on x86_64 or arm64 with KVM (bare metal, or a cloud VM
with nested virtualization). The binaries are not notarized; installs through the script or
Homebrew are not quarantined, but if you download an archive in a browser, clear the quarantine
flag with `xattr -d com.apple.quarantine adf`.

## Quick start

```sh
adf doctor          # checks smolvm and hardware virtualization
adf init            # gets the desktop template ready (downloads a prebuilt image once)
adf create a b      # two running desktops
adf key a ctrl+l && adf type a "example.com" && adf key a Return
adf screenshot a    # writes a.png
adf console         # watch them live in your browser
adf rm a b
```

## Use with Claude Code, Cursor and other MCP clients

```sh
claude mcp add adf -- "$(command -v adf)" mcp
```

Cursor (`~/.cursor/mcp.json`) and other clients that take a JSON config:

```json
{ "mcpServers": { "adf": { "command": "/absolute/path/to/adf", "args": ["mcp"] } } }
```

`adf mcp` serves these tools: `create_desktop`, `list_desktops`, `delete_desktop`,
`start_desktop`, `pause_desktop`, `fleet_status`, `screenshot`, `click`, `move_mouse`, `drag`,
`scroll`, `type_text`, `press_key`, `cursor_position` and `run_command`. Actions return a
screenshot of the result by default, and an action on a paused or stopped desktop wakes it.
Remote clients can use streamable HTTP with a bearer token:

```sh
ADF_MCP_TOKEN=change-me adf mcp --http 127.0.0.1:7400   # endpoint: http://127.0.0.1:7400/mcp
```

## Watch and take over: the console

```sh
adf console
```

Opens a local page with every desktop live, its CPU and memory, and the latest agent action. A
desktop's border turns amber while an agent acts on it. Click a desktop to watch it closely,
follow its activity feed, or press "Take control" to use your own mouse and keyboard on it.
Agent actions from every `adf mcp` process (for example, each Claude Code session) show up as
they happen. The page is served on 127.0.0.1 only, and the link it prints carries a one-time
token.

## Demo

```sh
scripts/demo.sh            # 4 Claude Code agents work in parallel while the console shows them
scripts/demo.sh --agents 5 --keep
```

Each agent gets its own desktop and a task (Hacker News, Wikipedia, a web form, GitHub
trending, OpenStreetMap). In a test run all four finished correctly in 17 to 31 s, for about
$0.54 in total with Sonnet. Needs the Claude Code CLI.

## Room and limits

`adf status` shows free memory, desktops by state, the limits and how many more desktops fit.
Agents get the same from the `fleet_status` tool.

Measured on an Apple M5 with 24 GB:

| | CPU | Host memory | Disk |
|---|---|---|---|
| Idle desktop | 0.1% of one core | 200 to 450 MB | about 10 MB |
| Paused desktop | none | none | about 450 MB |

A desktop is created in 0.5 to 1 s, pauses in about 6 s and resumes in 2 to 3 s with its windows
intact. Desktops with no activity for 10 minutes are paused automatically; the next action
resumes them. A new or waking desktop is refused, with advice, when it would leave the computer
short of memory.

On Linux (an AWS c8i.4xlarge with nested virtualization) desktops share even more memory, about
72 MB of host memory each. smolvm 1.37 cannot resume a paused desktop on Linux yet
([smolvm#2477](https://github.com/smol-machines/smolvm/issues/2477)), so there pausing stops the
desktop instead (files kept, windows closed) and idle desktops keep running.

| Variable | Default | Meaning |
|---|---|---|
| `ADF_RESERVE_MB` | 2048 | memory always kept free for the computer itself |
| `ADF_DESKTOP_MB` | 512 | memory budgeted per desktop |
| `ADF_MAX_DESKTOPS` | none | hard cap on running desktops |
| `ADF_IDLE_PAUSE` | 10m | pause desktops idle this long; `0` turns it off |
| `ADF_PAUSE` | unset | set to `1` to pause on Linux once smolvm can resume there |
| `ADF_IMAGE` | prebuilt | `build` builds the template from Debian; a URL or file uses that image |

## Build from source

Requires Go 1.25 and smolvm 1.37 or newer.

```sh
go build -o bin/adf ./cmd/adf
go test ./...                                                      # unit tests
go test -tags integration -timeout 20m ./tests/integration/ -v    # needs smolvm and KVM or Apple silicon
```

Releases: push a `vX.Y.Z` tag to build archives, publish them and update `Formula/adf.rb`. Push an
`image-vN` tag (matching `image.Version`) to publish the prebuilt desktop image.

## Repository layout

- `cmd/adf`, `internal/`: the `adf` CLI, MCP server and web console
- `install.sh`, `scripts/`, `Formula/`: installer, release build and Homebrew formula
- `docs/superpowers/plans/`: implementation plans
- `reports/`, `research_notes/`, `spike/`: research and the runtime evaluation behind the design

License: Apache-2.0 (see `LICENSE`).
