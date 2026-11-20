---
title: Run the Go and C++ tests
description: Build and run the C++ rule engine's tests with CMake and ctest, and run the Go tests, including those that need a PostgreSQL, the C++ engine, or a workspace outside the Go module.
kind: how-to
---

JetStore has two test suites: the C++ rule engine's, written with Google Test and built by CMake, and
the Go packages', run by `go test`. The commands run from the repository root. Build the C++ engine
first if you will run Go packages that link it.

## Run the C++ engine's tests

The tests are the `*_test.cc` files under `jets/rdf/` and `jets/rete/`, built into one binary,
`jets_test`, by `jets/CMakeLists.txt`.

1. **Configure and build** the library and the test binary. Any build directory works; `build/` is
   the one the cgo defaults in `jets/bridge/bridge.go` look in.

   ```bash
   cmake -S . -B build -DCMAKE_BUILD_TYPE=Release
   make -C build jets jets_test -j8
   ```

   It needs a C++ compiler, CMake, SQLite, Abseil (found with `find_package(absl)`) and Google Test.
2. **Run them with `ctest`, in `build/jets`.** The tests are registered there, by
   `gtest_discover_tests` in `jets/CMakeLists.txt`; `ctest` in `build/` itself finds no test
   configuration.

   ```bash
   ctest --test-dir build/jets
   ```

   It ends with `100% tests passed out of <n>`. When `go` is on the `PATH` at configure time, `ctest`
   also runs the Go tests that link `libjets.so` and are behind a build tag, so that no plain
   `go test` runs them. Their label is `go`, and `jets/CMakeLists.txt` lists them after the
   engine's tests. They use the `libjets.so` of the build directory, so build `jets` before running
   `ctest`. `ctest --test-dir build/jets -LE go` leaves them out, and `-DJETS_GO_ENGINE_TESTS=OFF`
   does not register them.
3. **To run the binary directly, run it from its own directory.** The build copies
   `jets/rete/test_data` beside `jets_test`, and some tests open it by a relative path, so they fail
   when the binary runs from anywhere else.

   ```bash
   (cd build/jets && ./jets_test --gtest_filter='LookupRowIdTest.*')
   ```

The sanitizer build, `-DJETS_SANITIZE=ON`, is described at the head of `jets/CMakeLists.txt`, and the
build variants for measuring the engine in
[Build and run the benchmark](/docs/developer/testing/rules-benchmark/build-and-run/).

## Run the Go tests

The repository's `go.work` composes three modules: the root, `cdk/bootstrap_aws` and `cdk/vpc_peering`.
From the root:

```bash
go test ./jets/datatable/...                    # a package tree
go test -run TestRegisterFileKeys ./jets/datatable/   # one test
```

Most packages need nothing else. Three kinds of test do, and each kind skips, or fails to build,
without it.

### Tests that need a PostgreSQL

A test that needs a database reads its address from `JETS_TEST_DSN` and **skips when it is not set**,
so a run without it passes and tests less. These tests create the tables they need, in the `jetsapi`
schema from `jets/jets_schema.json` or in a schema of their own, and drop them again: **point
`JETS_TEST_DSN` at a scratch database**, never at an installation's. A container is enough:

```bash
docker run --rm -d --name jets-test-pg -e POSTGRES_PASSWORD=test \
  -e POSTGRES_DB=jetstest -p 55432:5432 postgres:16-alpine

export JETS_TEST_DSN='postgres://postgres:test@localhost:55432/jetstest'
go test -count=1 ./jets/datatable/... ./jets/schema/... ./jets/update_db/... ./jets/apiserver/
```

The packages with such tests include `jets/datatable`, `jets/schema`, `jets/update_db`,
`jets/apiserver`, `jets/compute_pipes`, `jets/workspace`, `jets/purge_database/delegate`,
`jets/sessionpurge` and several under `jets/agentic/`; `grep -rl JETS_TEST_DSN --include='*_test.go' jets`
lists them. Each test file's header says what it creates. Run with `go test -v` to see which tests
skipped and why. Stop the container afterwards with `docker stop jets-test-pg`.

### Tests that link the C++ engine

A package that imports `jets/bridge` links `libjets.so` through cgo, and does not build without it.
`jets/compute_pipes` is one; `go list -test -deps <package> | grep jets/bridge` says whether another
is. `jets/bridge/bridge.go` hard-codes its `#cgo` paths, so point them at the build from the first
section, as [Run a rules unit test from a shell](/docs/developer/testing/rules-unit-tests/run-from-a-shell/)
does for the harness:

```bash
LIB=$PWD/build/jets       # the directory holding libjets.so
CGO_CFLAGS="-I$PWD/jets" CGO_LDFLAGS="-L$LIB" LD_LIBRARY_PATH=$LIB \
  go test -count=1 ./jets/compute_pipes/
```

The rule tests in that package compile a copy of `jets/jetrules/test_ws` and run it in a child process,
so they need no workspace of yours.

`utils/libjets_env.sh` does both steps: it builds `libjets.so` from the checkout it is in and prints
the three variables, ready to `eval`. Give it `-B <dir>` to build somewhere other than `build/`, and
`--no-build` to print the variables for a build you already have:

```bash
eval "$(utils/libjets_env.sh -B bld)"     # bld/ is ignored, like build/
go test -count=1 ./jets/bridge/ ./jets/compute_pipes/
```

#### When the link fails with `undefined reference`

A failure like this one means the test linked a `libjets.so` older than the code that calls it:

```text
/tmp/go-build/bridge.cgo2.c:1357:(.text+0xf3a): undefined reference to `get_vertex_observations'
collect2: error: ld returned 1 exit status
FAIL    github.com/artisoft-io/jetstore/jets/bridge [build failed]
```

The `#cgo` directives in `jets/bridge/bridge.go` look for the library in `build/jets` of the
checkout, then in `/usr/local/go/build/jets` and `/app/jets` (the second is where the builder image
puts it, `dockerfiles/Dockerfile.cpipes_builder`), and then the
linker looks in its default path (`ldconfig -p | grep libjets` shows what is there). **A checkout with
no `build/jets` of its own, such as a git worktree, falls through to the last of these**, and links
whatever `libjets.so` another checkout left there. The failure names the first function the old
library lacks, not the library, so it reads like a bug in the code. Build the library in this
checkout and set the variables above, or run `utils/libjets_env.sh`.

**The run needs `LD_LIBRARY_PATH` as well, even when the link succeeds.** The test binary loads
`libjets.so` at run time from the loader's path, not from the directory it was linked against, so
with `CGO_LDFLAGS` alone it can link the new library and load the old one. That fails at the first
call the old library lacks, as `symbol lookup error: … undefined symbol: …`, or, worse, runs the old
engine. `ldd` on a test binary built with `go test -c` shows which `libjets.so` it will load.

### Tests that read a workspace

Some tests check a claim against a workspace's own files, which are in a separate repository and not
in a checkout of this one. Each **skips unless a variable names the folder** to read:

| Variable | Read by | What it names |
|---|---|---|
| `JETS_PC_CORPUS_DIR` | `jets/compute_pipes`, `jets/agentic/briefing/briefingtmpl` | a folder walked for `*.pc.json` pipeline configurations |
| `JETS_SQL_CORPUS_DIR` | `jets/sqlscript` | a folder walked for SQL scripts |
| `JETS_REPORTS_CORPUS_DIR` | `jets/run_reports/delegate` | a folder of workspaces whose `reports/*.sql` are parsed |
| `JETS_COMPILE_TEST_WORKSPACE` | `jets/workspace` | one workspace directory to compile |
| `WORKSPACES_HOME`, `WORKSPACE` | tests in `jets/compute_pipes/jetrules_native_adaptor` | the folder of workspaces and the active one |

**Run every such test with `-count=1`.** The workspace is outside the Go module, and the test cache does
not reliably notice when its files change, so a second run can replay an earlier `ok` from the cache;
it looks exactly like a pass. With the example workspace `jets_ws` checked out beside this repository:

```bash
JETS_PC_CORPUS_DIR=../jets_ws go test -count=1 -run TestCorpus ./jets/compute_pipes/   # with the cgo variables above
JETS_PC_CORPUS_DIR=../jets_ws go test -count=1 ./jets/agentic/briefing/briefingtmpl/
JETS_SQL_CORPUS_DIR=../jets_ws go test -count=1 -run TestCorpusLexesClean ./jets/sqlscript/
```

The same caution applies to a test against `JETS_TEST_DSN`: the cache cannot see the database either.

## When a package fails before your change

A package can carry failing tests that have nothing to do with your change. Run it on the commit you
started from as well, and compare the names of the failing tests, rather than expecting a clean run.

## See also

- [Build JetStore from source](/docs/developer/how-to/build-from-source/): the full build, of which the first section is the engine.
