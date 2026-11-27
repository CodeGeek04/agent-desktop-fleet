#!/bin/sh
# Live demo: four Claude Code agents work at once, each on its own adf desktop, while the
# console shows all of them.
#
#   qa-problem, qa-error  QA testers: each tests a demo shop under a different test account
#                          and reports the bugs it finds
#   office                 researches on the web, then builds a spreadsheet with a formula
#                          and a chart in LibreOffice Calc (a native Linux app)
#   shop                   buys something, but stops at the final button for a human to
#                          approve: take control of it in the console and press Finish
#
# Usage: scripts/demo.sh [--keep] [--no-open]
#   --keep     leave the demo desktops running afterwards
#   --no-open  do not open the console in a browser
#
# Needs adf and the Claude Code CLI (claude). Costs roughly $1 to $2 with Sonnet.
# The shop scene uses www.saucedemo.com, a public demo store made for test automation.
set -eu

keep=0
open_flag=""
while [ $# -gt 0 ]; do
  case "$1" in
    --keep) keep=1 ;;
    --no-open) open_flag="--no-open" ;;
    *) echo "usage: scripts/demo.sh [--keep] [--no-open]" >&2; exit 2 ;;
  esac
  shift
done

adf="$(command -v adf || true)"
[ -n "$adf" ] || adf="$(cd "$(dirname "$0")/.." && pwd)/bin/adf"
[ -x "$adf" ] || { echo "adf not found: install it or build bin/adf first" >&2; exit 1; }
command -v claude >/dev/null 2>&1 || { echo "the Claude Code CLI (claude) is required" >&2; exit 1; }

desktops="qa-problem qa-error office shop"
work="$(mktemp -d)"
cat > "$work/mcp.json" <<EOF
{"mcpServers":{"adf":{"command":"$adf","args":["mcp"]}}}
EOF

name() { echo "$desktops" | cut -d' ' -f"$1"; }

task() {
  case "$1" in
    1 | 2)
      user=problem_user
      [ "$1" = 2 ] && user=error_user
      echo "You are a QA tester. Create a desktop named $(name "$1"). In its Firefox open https://www.saucedemo.com and log in as $user with password secret_sauce. Test the shop like a real customer: look at the product list and images, try sorting, open a product page, add items to the cart and go all the way through checkout (any name and postal code). Then list every bug you found, one line each, most serious first. Spend no more than about 25 actions." ;;
    3)
      echo "Use the existing desktop named office, where Firefox and LibreOffice Calc are open (switch windows with the taskbar at the bottom). In Firefox, find the five most populous countries on Wikipedia's 'List of countries and dependencies by population' page. In LibreOffice Calc, enter them as a table with the headers Country and Population, add a Total row that uses a SUM formula, make both columns wide enough to show every value in full, insert a bar chart of the five populations to the right of the table (starting around cell D2, not covering it), and save the file as /root/top5.ods (keep the format). Report the total." ;;
    4)
      echo "Create a desktop named shop. In its Firefox open https://www.saucedemo.com, log in as standard_user with password secret_sauce, add the Sauce Labs Backpack to the cart, check out as Ada Lovelace with postal code 94103, and continue to the overview page. Do not press Finish: placing the order needs a human's approval, and a human will press Finish themselves from the adf console. Wait for that by calling screenshot with wait_seconds 20, again and again, for up to 5 minutes, until the page says 'Thank you for your order'. Then report the order total and that a human approved it, or that nobody approved it in time." ;;
  esac
}

# Check the Claude Code login once before starting anything. This also refreshes an expired
# login token up front, so the four agents do not all try to refresh it at the same moment.
echo "Checking the Claude Code login..."
if ! claude -p "Reply with OK" --model haiku > "$work/login.txt" 2>&1; then
  cat "$work/login.txt" >&2
  echo "Claude Code could not run. Run 'claude', type /login, then try the demo again." >&2
  exit 1
fi

# Desktops left over from an earlier run (for example with --keep) would get in the way.
leftover=""
for d in $desktops; do
  "$adf" ls | awk 'NR > 1 {print $1}' | grep -qx "$d" && leftover="$leftover $d"
done
if [ -n "$leftover" ]; then
  echo "Removing demo desktops left from an earlier run:$leftover"
  "$adf" rm $leftover >/dev/null
fi

console_pid=""
if curl -fs -o /dev/null --max-time 2 http://127.0.0.1:7300/ 2>/dev/null; then
  echo "A console is already running at http://127.0.0.1:7300/: watch the demo there."
else
  echo "Starting the console..."
  "$adf" console $open_flag > "$work/console.log" 2>&1 &
  console_pid=$!
  sleep 2
  head -1 "$work/console.log"
fi

echo "Preparing the office desktop: installing LibreOffice Calc (about 20 s)..."
"$adf" create office >/dev/null
"$adf" exec office -- sh -c 'export DEBIAN_FRONTEND=noninteractive; apt-get update -qq && apt-get install -y -qq --no-install-recommends libreoffice-calc libreoffice-gtk3 >/dev/null 2>&1'
"$adf" exec office -- sh -c 'cd /root && DISPLAY=:1 nohup localc --nologo >/dev/null 2>&1 &'
sleep 6

echo "Launching 4 agents. Watch the console."
i=1
while [ "$i" -le 4 ]; do
  prompt="You have tools for sandboxed Linux desktops (the adf MCP server). $(task "$i") Leave the desktop running. Work efficiently."
  claude -p "$prompt" --mcp-config "$work/mcp.json" --strict-mcp-config \
    --allowedTools "mcp__adf__*" \
    --disallowedTools "Bash,Edit,Write,Read,WebFetch,WebSearch,Glob,Grep,Task,Agent" \
    --model sonnet --output-format json > "$work/agent-$i.json" 2> "$work/agent-$i.err" &
  echo "  $(name "$i") started"
  i=$((i + 1))
  sleep 1
done
echo
echo "When 'shop' reaches the checkout overview it waits for you: click it in the console,"
echo "press Take control, then click Finish."
echo
while pgrep -f "$work/mcp.json" >/dev/null 2>&1; do sleep 2; done

i=1
while [ "$i" -le 4 ]; do
  echo "== $(name "$i")"
  python3 -c 'import json,sys
d=json.load(open(sys.argv[1]))
print(d.get("result","").strip())
print("(%d turns, %.0f s, $%.2f)" % (d.get("num_turns",0), d.get("duration_ms",0)/1000, d.get("total_cost_usd",0)))' "$work/agent-$i.json" 2>/dev/null \
    || echo "(no result; see $work/agent-$i.err)"
  echo
  i=$((i + 1))
done
if grep -qs "Failed to authenticate" "$work"/agent-*.json; then
  echo "The agents could not log in to Claude. Run 'claude', type /login, then run the demo again."
  echo
fi
python3 -c 'import json,sys
t=0
for p in sys.argv[1:]:
    try: t+=json.load(open(p)).get("total_cost_usd",0)
    except Exception: pass
print("Total cost: $%.2f" % t)' "$work"/agent-*.json

if [ "$keep" = 0 ]; then
  echo "Removing the demo desktops (use --keep to leave them)..."
  for d in $desktops; do "$adf" rm "$d" >/dev/null 2>&1 || true; done
fi
[ -z "$console_pid" ] || echo "The console is still running (pid $console_pid). Stop it with: kill $console_pid"
