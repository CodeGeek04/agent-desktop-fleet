# Agent desktop fleet (working name)

Open-source tool to launch, manage, stop and watch many lightweight sandboxed Linux desktops
for AI agent computer use. Infra only: VM lifecycle, an MCP server, and a live console that
shows every machine. No agent loop.

Status: early development. macOS (Apple silicon) works; Linux KVM is next.

## Try it

Requirements: [smolvm](https://github.com/smol-machines/smolvm) 1.37 or newer and Go 1.25.

```sh
go build -o bin/adf ./cmd/adf
./bin/adf doctor            # checks smolvm and hardware virtualization
./bin/adf init              # builds the desktop template once (a few minutes)
./bin/adf create a b        # two running desktops, each a cheap copy of the template
./bin/adf key a ctrl+l && ./bin/adf type a "example.com" && ./bin/adf key a Return
./bin/adf screenshot a      # writes a.png
./bin/adf rm a b
```

Each desktop is a smolvm microVM running Xvfb, openbox and Firefox at 1280x800.
Desktops are branched from a running template, so they share its memory until they change it.

## Use with Claude Code, Cursor and other MCP clients

`adf mcp` serves the desktops as MCP tools: `create_desktop`, `list_desktops`, `delete_desktop`,
`start_desktop`, `pause_desktop`, `fleet_status`, `screenshot`, `click`, `move_mouse`, `drag`,
`scroll`, `type_text`, `press_key`, `cursor_position` and `run_command`. Actions return a
screenshot of the result by default, and an action on a paused or stopped desktop wakes it.

Claude Code:

```sh
claude mcp add adf -- "$(pwd)/bin/adf" mcp
```

Cursor (`~/.cursor/mcp.json`) and other clients that take a JSON config:

```json
{ "mcpServers": { "adf": { "command": "/absolute/path/to/bin/adf", "args": ["mcp"] } } }
```

Remote clients can use streamable HTTP with a bearer token:

```sh
ADF_MCP_TOKEN=change-me ./bin/adf mcp --http 127.0.0.1:7400   # endpoint: http://127.0.0.1:7400/mcp
```

The first `create_desktop` on a machine builds the desktop template (one to two minutes).

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

| Variable | Default | Meaning |
|---|---|---|
| `ADF_RESERVE_MB` | 2048 | memory always kept free for the computer itself |
| `ADF_DESKTOP_MB` | 512 | memory budgeted per desktop |
| `ADF_MAX_DESKTOPS` | none | hard cap on running desktops |
| `ADF_IDLE_PAUSE` | 10m | pause desktops idle this long; `0` turns it off |

## Watch and take over: the console

```sh
./bin/adf console
```

Opens a local page with every desktop live, its CPU and memory, and the latest agent action.
Click a desktop to watch it closely, follow its activity feed, or press "Take control" to use
your own mouse and keyboard on it. Agent actions from every `adf mcp` process (for example,
Claude Code sessions) show up as they happen. The page is served on 127.0.0.1 only, and the
link it prints contains a one-time token.

## Repository layout

- `cmd/adf`, `internal/`: the `adf` CLI, MCP server and web console
- `docs/superpowers/plans/`: implementation plans
- `reports/`: market and technical research (November 2026)
- `research_notes/`: raw notes behind the report
- `spike/`: runtime evaluation (smolvm vs microsandbox) with scripts and results

License: Apache-2.0 (see `LICENSE`).
