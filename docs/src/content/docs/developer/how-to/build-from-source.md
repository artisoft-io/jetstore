---
title: Build JetStore from source
description: Build the C++ rule engine, the Go modules and binaries, the web app and the documentation site on a workstation, in the order they depend on each other.
kind: how-to
sidebar:
  order: 1
---

JetStore is four builds: the C++ rule engine (`libjets.so`), the Go modules, the web app
(`jetsclient_ide/`) and this documentation site (`docs/`). Build the engine first, because the Go
packages that run rules link it.

Every command below runs from the repository root unless it says otherwise. They were run on
2026-11-11 at Release 4.1.6, on Ubuntu 24.04 with GCC 13.3, CMake 4.4, Go 1.27.4 and Node 24, and
each step says what it gave there.

## What you need

| For | Tools and libraries |
|---|---|
| The engine | a C++20 compiler, CMake, and the development packages of abseil (as a CMake package), glog, gflags, Boost `date_time` and SQLite 3; googletest for its tests |
| The Go modules | Go at the version `go.mod` declares (`go 1.27.4`), and a C compiler for cgo |
| The web app and the site | Node 24 and npm |

The images install the same set on Amazon Linux 2023 (`dockerfiles/Dockerfile.cpipes_base_builder`):
Boost, SQLite and gflags from the distribution, and abseil `20260107.1` and glog `0.6.0` built from
their release archives. That Dockerfile is the recipe to follow when a distribution's own packages are
too old.

## Build the rule engine

```bash
cmake -S . -B build -DCMAKE_BUILD_TYPE=Release
cmake --build build --target jets jets_test -j "$(nproc)"
(cd build/jets && ctest)
```

`build/jets/libjets.so` is the shared library the Go bridge links, `libjets_static.a` (target
`jets_static`) the static one, and `jets_test` the engine's Google Test suite. Here the build took
1 minute 45 seconds on 16 cores, and `ctest` ran 277 tests, all passing, in under a second.

**Run `ctest` from `build/jets`, not from `build`.** The tests are registered in
`jets/CMakeLists.txt` (`enable_testing`, `gtest_discover_tests`), so `ctest` in the top of the build
tree reports `No tests were found!!!` and exits 0. Running `build/jets/jets_test` directly works from
anywhere. Running the Go tests, those that need a PostgreSQL or the engine among them, is in
[Run the Go and C++ tests](/docs/developer/testing/go-and-cpp/).

The version stamped into the library is the `JETS_VERSION` environment variable, or `-DJETS_VERSION=`
on the configure line; without either, the configure prints `Building, no version specified`
(`CMakeLists.txt`). The images build `jets` alone with link-time optimisation on (`-DJETS_LTO=ON`,
`dockerfiles/Dockerfile.cpipes_builder`). The other build options, the sanitizer build among them, are
described at the top of `jets/CMakeLists.txt`.

## Build the Go modules

```bash
go build ./...
```

The repository's `go.work` composes three modules: the root module (`github.com/artisoft-io/jetstore`,
which holds `jets/`, the stack in `cdk/jetstore_one` and its Lambdas) and the two CDK apps
`cdk/bootstrap_aws` and `cdk/vpc_peering`. Go uses the first `go.work` it finds walking up from the
working directory, so run Go commands from inside the repository. A checkout nested in another
repository that has a `go.work` of its own resolves differently from above this one.

**The packages that link the engine look for it in `build/jets` first.** The cgo directives in
`jets/bridge/bridge.go` put `-L${SRCDIR}/../../build/jets` ahead of the image paths (`/usr/local/go/build/jets`,
`/app/jets`), so a release build at `build/` is found with no flag. Built anywhere else, say where:

```bash
CGO_LDFLAGS="-L/path/to/cmake-build/jets" go build ./...
```

**When `build/jets` has no library, the linker falls back to its default path, and an old
`libjets.so` there is linked without a word until a newer function is missing.** That is what the
first run here did: with the engine built outside `build/`, `go build ./...` failed in the five
packages that link it (`jets/cmds/rules_bench`, `jets/cmds/rules_unit_test`,
`jets/cmds/cpipes_native_server`, `jets/compute_pipes/local_test_driver` and
`cdk/jetstore_one/lambdas/compute_pipes/cp_node_native`) with `undefined reference to
'get_vertex_observations'`, from a `libjets.so` on the linker's default path that had been built from an
older checkout. With
`CGO_LDFLAGS` pointing at the new build, every package built. Check which library a binary will load
with `ldd`, and run it with `LD_LIBRARY_PATH` set to the directory you linked against.

To build one binary, name its package:

```bash
go build -o /tmp/jets-bin/apiserver ./jets/apiserver
go build -o /tmp/jets-bin/update_db ./jets/update_db
```

`docs/screenshots/stack/build-bin.sh` builds the set a local stack runs (the apiserver, `update_db`,
`run_reports`, `compile_workspace`, `install_workspace_assets`, the engine and the rules unit test
harness, the workspace validator) into a directory of your choosing, with the files they read beside
them. It is the working recipe for a local installation; `JETS_BIN_DIR` names the directory, and it
refuses `/usr/local/bin`.

## Build the web app

```bash
cd jetsclient_ide
npm ci
npm test
npm run build
```

`npm test` runs the Vitest suite once: here 96 files and 1,862 tests, 1,836 passing and 26 skipped,
in 22 seconds. `npm run build` type-checks, writes the app bundle to `dist/`, then the workspace
validator to `dist-validator/jets_validate_workspace.mjs`; it took 18 seconds. Both directories are
git-ignored. **`npm run build` is the real check**, because the suite does not exercise every type.
Running the app against a local apiserver is in
[Develop and test the web app](/docs/developer/testing/web-app/develop-and-test/).

## Build the documentation

```bash
cd docs
npm ci
npm run build
```

The build runs the reference generators first (`npm run gen`, `docs/scripts/gen.mjs`), which read the
contract matrix, `jets/jets_schema.json` and the compiler's operator list, then writes the site to
`docs/dist/` and checks every internal link and anchor. A broken link fails it.
[Writing the documentation](/docs/developer/contributing/documentation/) has the preview commands.

## What the images do instead

The deployed binaries are not built on a workstation. `dockerfiles/Dockerfile.cpipes_builder`
compiles the engine and cross-compiles the Go binaries for `linux/amd64` in one image, from the
toolchain image `Dockerfile.cpipes_base_builder`, and the runtime images copy what they need out of
it. Use a workstation build to develop and test, and the images to ship. Each image is listed in
[Images](/docs/install/reference/images/).

## See also

- [Components](/docs/developer/architecture/components/): what each part you build does.
