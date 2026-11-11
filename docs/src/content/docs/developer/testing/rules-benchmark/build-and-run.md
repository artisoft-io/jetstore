---
title: Build and run the benchmark
description: Build libjets.so and its variants, build rules_bench against it, compile the workspace, run a measurement through the run protocol, profile it, and check the harness.
kind: how-to
sidebar:
  order: 0
---

## Build the command

`libjets.so` must be a build you made and can name. Never use the copies under `/usr/local`, which were
stale on the benchmark workstation.
`jets/bridge/bridge.go` hard-codes `#cgo` paths, so override them, as for `rules_unit_test`:

```bash
LIB=<cmake build dir>/jets     # the directory holding libjets.so, built Release
CGO_CFLAGS="-I$PWD/jets" CGO_LDFLAGS="-L$LIB" go build -o rules_bench ./jets/cmds/rules_bench
LD_LIBRARY_PATH=$LIB ldd rules_bench | grep libjets    # check it resolves the build you meant
```

### Build variants of `libjets.so`: `JETS_MARCH` and `JETS_LTO`

**`jets/CMakeLists.txt` has two options for measuring the engine built differently.** **Both are off
by default**: a configure line that does not set them compiles and links with exactly production's
flags (`dockerfiles/Dockerfile.cpipes_builder`).

| Option | Effect |
|---|---|
| `-DJETS_MARCH=<isa>` | adds `-march=<isa>` to every C and C++ compile of `jets/` (`objlib`, so `libjets.so` and `libjets_static.a`, and `jets_test`). Empty, the default, adds nothing |
| `-DJETS_LTO=ON` | link-time optimisation (`INTERPROCEDURAL_OPTIMIZATION`) on `objlib`, `jets`, `jets_static` and `jets_test`, after CMake's `check_ipo_supported`; configuring fails if the toolchain has none |

```bash
cmake -S . -B build-v3    -DCMAKE_BUILD_TYPE=Release -DJETS_MARCH=x86-64-v3
cmake -S . -B build-lto   -DCMAKE_BUILD_TYPE=Release -DJETS_LTO=ON
cmake -S . -B build-v3lto -DCMAKE_BUILD_TYPE=Release -DJETS_MARCH=x86-64-v3 -DJETS_LTO=ON
make -C build-v3lto jets jets_test -j8 && ctest --test-dir build-v3lto/jets
```

- **A `-march` build runs only on a CPU with that instruction set.** `x86-64-v3` is AVX2 and its
  companions.
- **With LTO, `libjets.so` links the prebuilt static abseil and the system SQLite as before**: they
  are ordinary objects, and only `jets/`'s own code is optimised across files. **`libjets_static.a`
  then holds LTO objects only**, which only an LTO-aware link reads; `jets_test` is one, and Go links
  the shared library, not the archive.
- **Compare the variants by configure line, commit and size, not by hash**: the library embeds its
  build directory's paths and the build time (`version.h`).

**Every result records the library that actually ran.** It is read from `/proc/self/maps`, with its
sha256 and mtime. The result also records the jetstore commit the binary was built from, from Go's
build information (`jetstore_modified` is true when the tree had changes).

## Compile the workspace

**Compile the workspace at the commit you mean to measure**: compiled files older than the rule sets
are a real hazard. For a workspace `my_ws`, in a checkout of your own:

```bash
go run ./jets/cmds/install_workspace_assets -d $WORKSPACES_HOME/my_ws
JETS_WORKSPACE_VALIDATOR=<path to jets_validate_workspace.mjs> go run ./jets/cmds/compile_workspace -w my_ws -v bench
```

## Run through the protocol

**Every measured run goes through `scripts/run_protocol.sh`:**

```bash
scripts/run_protocol.sh --bin ./rules_bench --lib $LIB --repetitions 10 --out results/w1_small spec.json
scripts/run_protocol.sh --production-shape --bin ./rules_bench --lib $LIB spec.json   # 4 cores, GOMAXPROCS=4, pool 10
```

**It refuses to run**:

- unless every CPU it pins to uses the `performance` governor and `kernel.perf_event_paranoid` is 1
  or less. A reboot resets both;
- on battery power, unless `--allow-battery` is given: measurements are taken on mains power.

**How it runs:**

- **Pinning.** It pins with `taskset -c 0-5`, the performance cores of the benchmark workstation.
  `--production-shape` pins to `0-3` with `GOMAXPROCS=4` and a pool of 10, the ECS task's shape.
- **Interleaving.** It interleaves the engines it is given, A, B, A, B, over the repetitions (since
  the Go engine's retirement there is one, `native`), one fresh process per run, so each run builds
  its engine as a pipeline does and holds only that engine's memory. `--in-process` runs everything in
  one process instead.
- **The record.** It writes `protocol_env.txt` before the first run: the CPUs, governor,
  `perf_event_paranoid`, power, load average, and the spec's, binary's and `libjets.so`'s sha256.
- **The lock.** It holds an exclusive `flock` on `/tmp/rules_bench.lock`, or `$RULES_BENCH_LOCK`, for
  its whole duration. A second run refuses at once, or waits with `--wait`. Two runs from different
  checkouts can therefore never overlap.

**`--preload <lib>` runs `rules_bench` with a shared library preloaded**, an allocator for instance,
so that an allocator variant is an arm like any other:

```bash
scripts/run_protocol.sh --bin ./rules_bench --lib $LIB --preload /usr/lib/x86_64-linux-gnu/libjemalloc.so.2 spec.json
```

- **It refuses a path that is not a file, or not an ELF file**, and resolves symbolic links, so the
  record names the file that was loaded.
- **`LD_PRELOAD` is set for the `rules_bench` process only**, by an `env` that `taskset` runs;
  `taskset` and the protocol's own commands do not get it. An `LD_PRELOAD` inherited from the caller is
  dropped, so nothing is preloaded unless `--preload` says so.
- **It is recorded twice.** `protocol_env.txt` gets `preload` (the path, or `none`), `preload_sha256`
  and `preload_package` (the Debian package and version, when `dpkg` knows the file). Every result's
  `env` gets `ld_preload`, verbatim, and `preloaded`: each library with the file the loader actually
  mapped, from `/proc/self/maps`, with its sha256.
- **`rules_bench` refuses to run when a library in `LD_PRELOAD` was not mapped.** The loader ignores
  a library it cannot preload, with a message on standard error, and the run would otherwise be
  labelled with an allocator it did not use.
- **A preloaded allocator replaces the C library's `malloc`, which C++ and cgo use. It does not touch
  the Go heap**: the Go runtime allocates its own memory with `mmap`. So a preload changes the C++
  engine's every `new` and `delete`, and only the bridge's and cgo's few allocations on the Go side.

**`--dry-run` writes nothing.** It makes every check, then prints the environment record and the
commands to standard output. It creates neither `--out` nor the lock file, and says whether another
run holds the lock now. Anything after `--` is passed to `rules_bench`.

## Profile a run

Read the times from unprofiled runs; profiles are for attribution (see
[How the benchmark measures](/docs/developer/testing/rules-benchmark/measurement/#profiling)).

**Go profiles:**

```bash
rules_bench -out_dir /tmp/p -repetitions 3 -cpuprofile cpu.pprof spec.json
go tool pprof -top rules_bench cpu.pprof
go tool pprof -http=: rules_bench cpu.pprof     # graph view; needs graphviz
go tool trace trace.out
```

Take `-trace` in a run of its own.

**C++: `perf` and `heaptrack`, against a `libjets.so` with symbols.** Build it as Release plus `-g`:

```bash
cmake -DCMAKE_BUILD_TYPE=Release -DCMAKE_CXX_FLAGS=-g -DCMAKE_C_FLAGS=-g
```

Then profile the native engine only, in a process of its own:

```bash
LD_LIBRARY_PATH=$LIBG perf record --call-graph dwarf -F 999 -o perf.data -- \
  taskset -c 0-5 ./rules_bench -out_dir /tmp/p -engine native -repetitions 3 spec.json
perf report -i perf.data --no-children --sort dso,symbol
LD_LIBRARY_PATH=$LIBG heaptrack -o heaptrack.native ./rules_bench -out_dir /tmp/h -engine native spec.json
heaptrack_print heaptrack.native.zst | head -60
```

`perf` needs `perf_event_paranoid` ≤ 1, as the protocol checks. **`perf` sees the whole process, load
included.** Limit it to the runs with `perf report --time`, or profile enough repetitions that the
load is small beside them.

## Check the harness

```bash
go test -count=1 ./jets/rulesbench/ ./jets/compute_pipes/ -run 'TestRun|TestParseSpec|TestSessions|TestNewDumper|TestRunRuleSetsTiming|TestCastTextRowAndExtraction'
```

- **`jets/rulesbench`'s tests** run the C++ engine end to end over a compiled copy of
  `jets/jetrules/test_ws`, in a child process: warmup and repetitions, `-warmup 0`, a pool of 2, the
  caps, and the dump hook, whose time must stay out of every phase. They need the cgo environment
  above.
- **`jetrules_run_rule_sets_timing_test.go`** pins that a timing sink changes no outcome of
  `RunRuleSets`, and that a timer's callback time is in no phase. It also covers `CastTextRow` and a
  `SessionExtractor` serving two sessions.
- **A smoke run** on a workspace's four-member unit test exercised the command on both engines while
  there were two: both completed all four members with the same loops, extracted rows and triples per
  member (2026-10-20).
- **Run all of them with `-count=1`**: they read a workspace outside the Go module, which the test
  cache does not see.
