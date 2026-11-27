# adf: a fleet of desktops for AI agents


https://github.com/user-attachments/assets/2ab670bf-780c-468c-a540-9e14d5c19c3b





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

## Use with your coding agent

adf works with any MCP client. Two pieces make an agent good at it: the **MCP tools** (`adf mcp`)
and the **adf skill**, which teaches the agent to split a task across desktops, stay within your
computer's limits, run desktops in parallel, hand you control for logins or payments, and delete
its desktops afterwards.

**Claude Code: install the plugin** (skill and MCP tools together):

```
/plugin install adf --marketplace CodeGeek04/agent-desktop-fleet
```

Then ask in plain words, for example: "Use adf desktops to compare a long weekend in Seattle, San
Diego or Las Vegas: flights, hotels and things to do." The plugin starts `adf mcp`, so `adf` must
be on your PATH. If you added the server with `claude mcp add` before, remove that copy with
`claude mcp remove adf`.

**Codex, Cursor, Gemini CLI, GitHub Copilot, OpenCode, Amp, Goose, Windsurf, Cline, Kiro:**

```sh
adf skill install            # copies the skill into every agent it finds (--dry-run to preview)
```

Or with the `skills` CLI: `npx skills add CodeGeek04/agent-desktop-fleet --skill adf-desktops`.
Then add the MCP server (`adf skill install` prints the right command for each agent it finds):

| Agent | Command |
|---|---|
| Claude Code | `claude mcp add --scope user adf -- "$(command -v adf)" mcp` |
| Codex | `codex mcp add adf -- "$(command -v adf)" mcp` |
| Gemini CLI | `gemini mcp add -s user adf "$(command -v adf)" mcp` |
| GitHub Copilot CLI | `copilot mcp add adf -- "$(command -v adf)" mcp` |
| OpenCode | `opencode mcp add adf --global -- "$(command -v adf)" mcp` |
| Cursor and other JSON configs | `{ "mcpServers": { "adf": { "command": "/absolute/path/to/adf", "args": ["mcp"] } } }` |

Without the skill, agents still get short guidance from the server's own MCP instructions.

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
scripts/demo.sh     # needs the Claude Code CLI; about $1 with Sonnet
```

Four Claude Code agents work at once, each on its own desktop, while the console shows them:

- `qa-problem` and `qa-error` test a demo shop ([saucedemo.com](https://www.saucedemo.com), made
  for test automation) under two test accounts and report the bugs they find.
- `office` looks up data on Wikipedia, then builds a table with a `SUM` formula and a chart in
  LibreOffice Calc, a native Linux app the script installs first.
- `shop` buys a backpack but stops at the final button. Take control of it in the console and
  press Finish yourself; the agent notices and confirms the order.

In a test run they found 5 and 6 real bugs, built the spreadsheet, and confirmed the human's
order, in 42 to 83 s each, for $0.97 in total.

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
- `skills/`: the adf skill (`adf skill install` embeds it); `plugin/`, `.claude-plugin/`: the Claude Code plugin
- `docs/superpowers/plans/`: implementation plans
- `reports/`, `research_notes/`, `spike/`: research and the runtime evaluation behind the design

License: Apache-2.0 (see `LICENSE`).
