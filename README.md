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

- `reports/`: market and technical research (November 2026)
- `research_notes/`: raw notes behind the report
- `spike/`: runtime evaluation (smolvm vs microsandbox) with scripts and results

License: Apache-2.0 (to be added with the first code).
