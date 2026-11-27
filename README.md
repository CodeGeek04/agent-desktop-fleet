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
`screenshot`, `click`, `move_mouse`, `drag`, `scroll`, `type_text`, `press_key`, `cursor_position`
and `run_command`. Actions return a screenshot of the result by default.

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

## Repository layout

- `cmd/adf`, `internal/`: the `adf` CLI and MCP server
- `docs/superpowers/plans/`: implementation plans
- `reports/`: market and technical research (November 2026)
- `research_notes/`: raw notes behind the report
- `spike/`: runtime evaluation (smolvm vs microsandbox) with scripts and results

License: Apache-2.0 (see `LICENSE`).
