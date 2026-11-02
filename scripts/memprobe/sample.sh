#!/usr/bin/env bash
set -uo pipefail

app_pid=$1
marks=$2
out_dir=$3
timeout_sec=${4:-3600}
interval=${SAMPLE_INTERVAL_SEC:-3}
footprint_every=${FOOTPRINT_EVERY:-5}

csv="$out_dir/samples.csv"
mkdir -p "$out_dir/screens"
echo "ts,phase,process,pid,rss_kb,footprint" >"$csv"

footprint() {
  vmmap --summary "$1" 2>/dev/null | awk '/^Physical footprint:/ {print $3; exit}'
}

current_phase() {
  if [[ -s "$marks" ]]; then
    tail -n 1 "$marks" | cut -f 2
  else
    echo "startup"
  fi
}

start=$(date +%s)
tick=0
last_phase=""
while kill -0 "$app_pid" 2>/dev/null; do
  now=$(date +%s)
  phase=$(current_phase)
  if [[ "$phase" != "$last_phase" ]]; then
    screencapture -x "$out_dir/screens/$(printf '%05d' $((now - start)))-${phase// /_}.png" 2>/dev/null || true
    last_phase=$phase
  fi
  pids=("$app_pid" $(pgrep -f 'com\.apple\.WebKit\.' || true))
  for pid in "${pids[@]}"; do
    rss=$(ps -o rss= -p "$pid" 2>/dev/null | tr -d ' ')
    [[ -z "$rss" ]] && continue
    if [[ "$pid" == "$app_pid" ]]; then
      name="PlainShelf"
    else
      name=$(basename "$(ps -o comm= -p "$pid")")
    fi
    fp=""
    if ((tick % footprint_every == 0)); then
      fp=$(footprint "$pid")
    fi
    echo "$now,$phase,$name,$pid,$rss,$fp" >>"$csv"
  done
  if [[ "$phase" == "done" ]]; then
    break
  fi
  if ((now - start > timeout_sec)); then
    echo "sampler timed out after ${timeout_sec}s in phase '$phase'" >&2
    exit 1
  fi
  tick=$((tick + 1))
  sleep "$interval"
done

if [[ "$(current_phase)" != "done" ]]; then
  echo "app exited before the scenario finished (last phase '$(current_phase)')" >&2
  exit 1
fi
