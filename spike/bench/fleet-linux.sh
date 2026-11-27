#!/bin/bash
# Fleet benchmark for smolvm on a Linux KVM host.
# Branches N live desktops from a running, branchable source machine that has the
# host display enabled, gives each its own host VNC port, and records timing, memory and CPU.
# Usage: fleet-linux.sh SOURCE COUNT [BASE_PORT]
set -u
SRC="$1"; COUNT="$2"; BASE_PORT="${3:-6000}"
export PATH="$HOME/.smolvm:$PATH"
VNCDO="${VNCDO:-/tmp/v/bin/vncdo}"
now() { date +%s.%N; }
used_mb() { free -m | awk '/^Mem:/ {print $3}'; }

echo "host: $(nproc) vCPU, $(free -g | awk '/^Mem:/ {print $2}') GiB"
echo "used before: $(used_mb) MB"
START_ALL=$(now)
for i in $(seq 1 "$COUNT"); do
  name="${SRC}-b$i"; port=$((BASE_PORT + i))
  t0=$(now)
  SMOLVM_DISPLAY=1280x800 SMOLVM_VNC=$port smolvm machine branch --from "$SRC" --name "$name" >/dev/null 2>&1 \
    || { echo "branch $name failed"; continue; }
  t1=$(now)
  printf "branch %-12s port %s: %.2fs, used now %s MB\n" "$name" "$port" "$(echo "$t1-$t0" | bc)" "$(used_mb)"
done
printf "all %s branches: %.1fs\n" "$COUNT" "$(echo "$(now)-$START_ALL" | bc)"

sleep 10
echo "used after settle: $(used_mb) MB"
echo "per-VM memory (smaps_rollup, MB): name pss private_dirty"
for pid in $(pgrep -f "_boot-vm"); do
  name=$(tr '\0' ' ' < /proc/$pid/cmdline | grep -o "${SRC}[^ /\"]*" | head -1)
  # VM processes are not dumpable, so reading smaps_rollup needs root.
  sudo awk -v n="${name:-pid$pid}" '/^Pss:/ {p=$2} /^Private_Dirty:/ {d=$2} END {printf "  %-14s %6.0f %6.0f\n", n, p/1024, d/1024}' /proc/$pid/smaps_rollup
done

echo "idle CPU per VM process (%, 5 s sample):"
top -b -d 5 -n 2 | awk '/^ *PID/ {blk++} blk==2 && /smolvm/ {printf "  pid %s cpu %s\n", $1, $9}' | head -40

echo "screenshot from every branch via its own host VNC port:"
mkdir -p /tmp/fleet
for i in $(seq 1 "$COUNT"); do
  port=$((BASE_PORT + i)); t0=$(now)
  rm -f "/tmp/fleet/b$i.png"
  # vncdo can exit 0 without writing a file, so check the file itself.
  timeout 20 "$VNCDO" -s "127.0.0.1::$port" capture "/tmp/fleet/b$i.png" 2>/dev/null
  if [ -s "/tmp/fleet/b$i.png" ]; then
    printf "  b%s: %.2fs %s bytes\n" "$i" "$(echo "$(now)-$t0" | bc)" "$(stat -c %s /tmp/fleet/b$i.png)"
  else
    echo "  b$i: capture failed"
  fi
done
