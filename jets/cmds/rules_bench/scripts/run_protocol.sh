#!/usr/bin/env bash
# run_protocol.sh -- the run protocol every measured rules_bench run follows
# (rules_benchmark plan, AA.4 and section 5.5).
#
#   run_protocol.sh [options] <spec.json> [-- <extra rules_bench flags>]
#
# What it guarantees:
#   - it refuses to run unless every CPU it pins to uses the performance
#     governor and kernel.perf_event_paranoid is 1 or less;
#   - it pins every run with taskset: cores 0-5 by default, the performance
#     cores of the Core Ultra 7 265H; --production-shape pins to 0-3 with
#     GOMAXPROCS=4 and a pool of 10, the ECS task's 4 vCPU (F52);
#   - it interleaves the engines, A B A B, over N repetitions, one fresh process
#     per run by default, so that drift falls on both and each run loads its
#     engine as a pipeline does;
#   - it writes an environment record before the first run;
#   - it holds an exclusive flock for its whole duration, so two runs, from any
#     worktree, never overlap.
#
# Options:
#   --out DIR            results directory (default: ./rules_bench_out/<spec>_<UTC time>)
#   --bin PATH           the rules_bench binary (default: rules_bench on PATH)
#   --lib DIR            directory holding libjets.so, prepended to LD_LIBRARY_PATH
#   --engines LIST       comma-separated, in interleaving order (default: native,go)
#   --repetitions N      runs per engine (default: 10)
#   --warmup N           warmup runs inside each process, before its measured one (default: 0)
#   --cpus LIST          taskset CPU list (default: 0-5)
#   --pool N             pool size passed to rules_bench (default: the spec's)
#   --production-shape   --cpus 0-3, GOMAXPROCS=4, --pool 10
#   --in-process         one process runs every repetition, both engines loaded
#                        and interleaved inside it (rules_bench -engine LIST)
#   --wait               wait for the lock instead of refusing when it is held
#   --dry-run            check and print the commands, run nothing
#
# Environment: WORKSPACES_HOME and WORKSPACE name the compiled workspace, as for
# rules_bench itself. RULES_BENCH_LOCK overrides the lock file
# (default /tmp/rules_bench.lock).
set -euo pipefail

die() { echo "run_protocol: $*" >&2; exit 1; }

out=""
bin="rules_bench"
lib=""
engines="native,go"
reps=10
warmup=0
cpus="0-5"
pool=""
gomaxprocs=""
in_process=0
wait_lock=0
dry_run=0
spec=""
extra=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --out) out="$2"; shift 2 ;;
    --bin) bin="$2"; shift 2 ;;
    --lib) lib="$2"; shift 2 ;;
    --engines) engines="$2"; shift 2 ;;
    --repetitions) reps="$2"; shift 2 ;;
    --warmup) warmup="$2"; shift 2 ;;
    --cpus) cpus="$2"; shift 2 ;;
    --pool) pool="$2"; shift 2 ;;
    --production-shape) cpus="0-3"; gomaxprocs=4; pool=10; shift ;;
    --in-process) in_process=1; shift ;;
    --wait) wait_lock=1; shift ;;
    --dry-run) dry_run=1; shift ;;
    --) shift; extra=("$@"); break ;;
    -h|--help) sed -n '2,40p' "$0"; exit 0 ;;
    -*) die "unknown option $1" ;;
    *) [[ -z "$spec" ]] || die "one spec only"; spec="$1"; shift ;;
  esac
done
[[ -n "$spec" ]] || die "usage: run_protocol.sh [options] <spec.json> [-- <rules_bench flags>]"
[[ -f "$spec" ]] || die "no spec at $spec"
[[ -n "${WORKSPACES_HOME:-}" && -n "${WORKSPACE:-}" ]] || die "WORKSPACES_HOME and WORKSPACE must name the compiled workspace"
[[ "$reps" =~ ^[0-9]+$ && "$reps" -ge 1 ]] || die "--repetitions must be a positive integer"
[[ "$warmup" =~ ^[0-9]+$ ]] || die "--warmup must be an integer"
command -v taskset >/dev/null || die "taskset is not installed"
command -v flock >/dev/null || die "flock is not installed"
if [[ "$bin" != */* ]]; then
  bin="$(command -v "$bin")" || die "no rules_bench on PATH; pass --bin"
fi
[[ -x "$bin" ]] || die "$bin is not executable"
if [[ -n "$lib" ]]; then
  [[ -f "$lib/libjets.so" ]] || die "no libjets.so in $lib"
  export LD_LIBRARY_PATH="$lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
fi
spec="$(realpath "$spec")"
bin="$(realpath "$bin")"

# --- The lock: one protocol run at a time on this machine ----------------------
lock="${RULES_BENCH_LOCK:-/tmp/rules_bench.lock}"
exec 9>>"$lock"
if [[ "$wait_lock" -eq 1 ]]; then
  flock 9
elif ! flock -n 9; then
  die "another rules_bench protocol run holds $lock; wait for it, or pass --wait"
fi

# --- The tuning checks ---------------------------------------------------------
expand_cpus() {
  local part lo hi c
  IFS=',' read -ra parts <<<"$1"
  for part in "${parts[@]}"; do
    if [[ "$part" == *-* ]]; then
      lo="${part%-*}"; hi="${part#*-}"
      for ((c = lo; c <= hi; c++)); do echo "$c"; done
    else
      echo "$part"
    fi
  done
}
bad=()
for c in $(expand_cpus "$cpus"); do
  f="/sys/devices/system/cpu/cpu$c/cpufreq/scaling_governor"
  [[ -r "$f" ]] || die "CPU $c does not exist or has no cpufreq governor ($f)"
  g="$(<"$f")"
  [[ "$g" == performance ]] || bad+=("cpu$c=$g")
done
[[ ${#bad[@]} -eq 0 ]] || die "governor is not performance on: ${bad[*]} (a reboot resets it; it is Michel's to set, F46)"
paranoid="$(cat /proc/sys/kernel/perf_event_paranoid)"
[[ "$paranoid" -le 1 ]] || die "kernel.perf_event_paranoid is $paranoid; it must be 1 or less (F46)"
power="unknown"
for ac in /sys/class/power_supply/A{C,DP}*/online; do
  [[ -r "$ac" ]] && { [[ "$(<"$ac")" == 1 ]] && power="mains" || power="battery"; break; }
done
[[ "$power" != battery ]] || echo "run_protocol: warning: running on battery; section 5.5 asks for mains power" >&2

# --- The environment record ----------------------------------------------------
name="$(basename "$spec" .json)"
[[ -n "$out" ]] || out="./rules_bench_out/${name}_$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$out"
out="$(realpath "$out")"
{
  echo "recorded_at: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "host: $(hostname)"
  echo "kernel: $(uname -r)"
  echo "cpu_model: $(grep -m1 'model name' /proc/cpuinfo | cut -d: -f2- | sed 's/^ *//')"
  echo "cpus_pinned: $cpus"
  echo "gomaxprocs: ${gomaxprocs:-default}"
  echo "pool: ${pool:-spec}"
  echo "governor_pinned: performance"
  echo "perf_event_paranoid: $paranoid"
  echo "power: $power"
  echo "loadavg: $(cat /proc/loadavg)"
  echo "spec: $spec"
  echo "spec_sha256: $(sha256sum "$spec" | cut -d' ' -f1)"
  echo "bin: $bin"
  echo "bin_sha256: $(sha256sum "$bin" | cut -d' ' -f1)"
  libjets="$(LD_LIBRARY_PATH="${LD_LIBRARY_PATH:-}" ldd "$bin" | awk '/libjets.so/ {print $3}')"
  echo "libjets: ${libjets:-not linked}"
  [[ -n "$libjets" && -f "$libjets" ]] && echo "libjets_sha256: $(sha256sum "$libjets" | cut -d' ' -f1)"
  echo "workspace: $WORKSPACES_HOME/$WORKSPACE"
  echo "workspace_revision: $(git -C "$WORKSPACES_HOME/$WORKSPACE" rev-parse HEAD 2>/dev/null || echo none)"
  echo "engines: $engines"
  echo "repetitions: $reps"
  echo "warmup_per_process: $warmup"
  echo "in_process: $in_process"
  echo "extra_flags: ${extra[*]:-}"
  echo "GOGC: ${GOGC:-default} GOMEMLIMIT: ${GOMEMLIMIT:-default}"
} >"$out/protocol_env.txt"

# --- The runs ------------------------------------------------------------------
common=(-require_tuned)
[[ -n "$pool" ]] && common+=(-pool "$pool")
[[ "$warmup" -gt 0 ]] && common+=(-warmup "$warmup")
envs=()
[[ -n "$gomaxprocs" ]] && envs+=(GOMAXPROCS="$gomaxprocs")
# The secrets jets/user's init reads are not passed (rules_unit_test README).
strip=(-u JETS_ENCRYPTION_KEY -u JETS_ENCRYPTION_KEY_SECRET -u API_SECRET -u AWS_API_SECRET)

run() {
  local dir="$1"; shift
  local cmd=(env "${strip[@]}" "${envs[@]}" taskset -c "$cpus" "$bin" -out_dir "$dir" "${common[@]}" "$@" "${extra[@]}" "$spec")
  echo "run_protocol: ${cmd[*]}"
  [[ "$dry_run" -eq 1 ]] && return 0
  mkdir -p "$dir"
  "${cmd[@]}" >"$dir/stdout.log" 2>"$dir/stderr.log" || die "run failed: see $dir/stderr.log"
  grep '^rules_bench:' "$dir/stdout.log" || true
}

IFS=',' read -ra engine_list <<<"$engines"
if [[ "$in_process" -eq 1 ]]; then
  run "$out/in_process" -engine "$engines" -repetitions "$reps"
else
  n=0
  for ((rep = 1; rep <= reps; rep++)); do
    for e in "${engine_list[@]}"; do
      n=$((n + 1))
      run "$(printf '%s/run_%03d_%s' "$out" "$n" "$e")" -engine "$e" -repetitions 1
    done
  done
fi
echo "run_protocol: results in $out"
