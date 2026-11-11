---
title: Run a rules unit test from a shell
description: Build libjets.so and the rules_unit_test command, compile the workspace, and run one rules unit test against it.
kind: how-to
sidebar:
  order: 0
---

1. **Build `libjets.so`** with the CMake commands in the repository's `CLAUDE.md` (Release).
2. **Build the command against it.** `jets/bridge/bridge.go` hard-codes `#cgo` paths, so override
   them:

   ```bash
   LIB=<build>/jets                     # the directory holding libjets.so
   CGO_CFLAGS="-I$PWD/jets" CGO_LDFLAGS="-L$LIB" go build -o rules_unit_test ./jets/cmds/rules_unit_test
   LD_LIBRARY_PATH=$LIB ldd rules_unit_test | grep libjets    # check it resolves the build you meant
   ```

3. **Compile the workspace.** The test runs the compiled artefacts a pipeline would: the test's own
   `.jr` must be listed in `workspace_control.json`'s `rule_sets`, and the compile writes it into
   `build/` and `workspace.db`. In a workspace checkout, install the JetStore-owned assets first
   (`go run ./jets/cmds/install_workspace_assets -d <ws>`), then run `compile_workspace`.
   `compile_workspace` needs `JETS_WORKSPACE_VALIDATOR`, or `jets_validate_workspace.mjs` beside it,
   for its user-flow document check.
4. **Run.**

   ```bash
   cd $WORKSPACES_HOME/$WORKSPACE
   JETS_SECRETS_WITHHELD=1 LD_LIBRARY_PATH=$LIB rules_unit_test -out /tmp/r.json jet_rules/unit_tests/looping_test/looping_test1.tst.json
   ```

   It prints one line, for example `… fixed on native: pass`, and the details are in `/tmp/r.json`.
   Its log is standard error.

Do not pass the harness `JETS_ENCRYPTION_KEY`, `JETS_ENCRYPTION_KEY_SECRET`, `API_SECRET` or
`AWS_API_SECRET`. `JETS_SECRETS_WITHHELD=1` says they are absent on purpose, so the log does not
open with an error about the missing key, as the apiserver's own runs do not; see
[environment and secrets](/docs/developer/testing/rules-unit-tests/reference/#environment-and-secrets).
The flags, the test definition and the result file are in the
[reference](/docs/developer/testing/rules-unit-tests/reference/).
