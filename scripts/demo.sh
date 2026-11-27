#!/bin/sh
# Live demo: several real Claude Code agents work in parallel, each on its own adf desktop,
# while the console shows all of them. Good for a screen recording.
#
# Usage: scripts/demo.sh [--keep] [--agents N] [--no-open]
#   --keep       leave the demo desktops running afterwards
#   --no-open    do not open the console in a browser
#   --agents N   how many agents to run, 1 to 5 (default 4)
#
# Needs adf and the Claude Code CLI (claude). Each agent costs roughly $0.10 to $0.20 with Sonnet.
set -eu

keep=0
agents=4
open_flag=""
while [ $# -gt 0 ]; do
  case "$1" in
    --keep) keep=1 ;;
    --no-open) open_flag="--no-open" ;;
    --agents) shift; agents="$1" ;;
    *) echo "usage: scripts/demo.sh [--keep] [--agents N] [--no-open]" >&2; exit 2 ;;
  esac
  shift
done
case "$agents" in 1 | 2 | 3 | 4 | 5) ;; *) echo "--agents must be 1 to 5" >&2; exit 2 ;; esac

adf="$(command -v adf || true)"
[ -n "$adf" ] || adf="$(cd "$(dirname "$0")/.." && pwd)/bin/adf"
[ -x "$adf" ] || { echo "adf not found: install it or build bin/adf first" >&2; exit 1; }
command -v claude >/dev/null 2>&1 || { echo "the Claude Code CLI (claude) is required" >&2; exit 1; }

work="$(mktemp -d)"
cat > "$work/mcp.json" <<EOF
{"mcpServers":{"adf":{"command":"$adf","args":["mcp"]}}}
EOF

task() {
  case "$1" in
    1) echo "Create a desktop named demo-news. In its Firefox open news.ycombinator.com, open the comments of the top story, and report the story title and the first comment in one sentence each." ;;
    2) echo "Create a desktop named demo-wiki. In its Firefox open en.wikipedia.org, open today's featured article, scroll through it, and summarize it in two sentences." ;;
    3) echo "Create a desktop named demo-form. In its Firefox open https://httpbin.org/forms/post and order a large pizza with bacon and extra cheese for Ada Lovelace (phone 555-0100, email ada@example.com, delivery 19:30, note 'ring twice'). Submit it and report what the server echoed back." ;;
    4) echo "Create a desktop named demo-github. In its Firefox open github.com/trending and report the top three repositories with one line on what each does." ;;
    5) echo "Create a desktop named demo-maps. In its Firefox open www.openstreetmap.org, search for the Eiffel Tower, zoom in a few steps, and describe what the map shows around it." ;;
  esac
}

echo "Starting the console (it opens in your browser)..."
"$adf" console $open_flag > "$work/console.log" 2>&1 &
console_pid=$!
sleep 2
head -1 "$work/console.log"

echo "Launching $agents agents in parallel. Watch the console."
i=1
while [ "$i" -le "$agents" ]; do
  prompt="You have tools for sandboxed Linux desktops (the adf MCP server). $(task "$i") Leave the desktop running. Work efficiently."
  claude -p "$prompt" --mcp-config "$work/mcp.json" --strict-mcp-config \
    --allowedTools "mcp__adf__*" \
    --disallowedTools "Bash,Edit,Write,Read,WebFetch,WebSearch,Glob,Grep,Task,Agent" \
    --model sonnet --output-format json > "$work/agent-$i.json" 2> "$work/agent-$i.err" &
  echo "  agent $i started"
  i=$((i + 1))
  sleep 1
done
wait_for_agents() {
  while pgrep -f "$work/mcp.json" >/dev/null 2>&1; do sleep 2; done
}
wait_for_agents

echo
i=1
while [ "$i" -le "$agents" ]; do
  echo "== agent $i"
  python3 -c 'import json,sys
d=json.load(open(sys.argv[1]))
print(d.get("result","").strip())
print("(%d turns, %.0f s, $%.2f)" % (d.get("num_turns",0), d.get("duration_ms",0)/1000, d.get("total_cost_usd",0)))' "$work/agent-$i.json" 2>/dev/null \
    || { echo "(no result; see $work/agent-$i.err)"; }
  i=$((i + 1))
done

if [ "$keep" = 0 ]; then
  echo
  echo "Removing the demo desktops (use --keep to leave them)..."
  for d in demo-news demo-wiki demo-form demo-github demo-maps; do "$adf" rm "$d" >/dev/null 2>&1 || true; done
fi
echo "The console is still running (pid $console_pid). Stop it with: kill $console_pid"
