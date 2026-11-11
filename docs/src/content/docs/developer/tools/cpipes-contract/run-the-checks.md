---
title: Run the contract checks
description: Install the compute pipes contract tooling and run its coherence, citation, corpus and harness checks, the review stamp, and its test suite.
kind: how-to
sidebar:
  order: 7
---

The commands run from `tools/cpipes_contract`. The relative paths below assume the `jetstore`
checkout sits beside a `workspaces/` folder holding the workspaces the corpus is drawn from: `../..`
is the JetStore repository root (`--code`), and `../../..` is the folder holding `workspaces/`
(`--corpus`).

## Install

```bash
python3 -m venv .venv && .venv/bin/pip install -e .
```

## Check the matrix

```bash
.venv/bin/cpipes-contract check                        # coherence, while the matrix is partial
.venv/bin/cpipes-contract check --code ../..           # every citation resolves and names its field
.venv/bin/cpipes-contract check --corpus ../../..      # every exemplar resolves, and is a live config
.venv/bin/cpipes-contract check --strict --code ../.. --corpus ../../..
```

`--strict` adds the checks that only hold once extraction is complete, and its failures are the
worklist: every `ref_struct` with no `types.csv` row is a type the extraction has yet to reach.

**`--code` requires the cited line itself to name the field**, by its Go name or its json key. It
exists because line numbers inferred from the shape of a file, rather than read, were mostly a line
or two off, pointing at the neighbouring field's declaration, which a reader would take at face value.
Compute a citation by locating the field in the source rather than typing it. A citation that a
program cannot follow is one nobody will follow either.

## Measure the corpus

```bash
.venv/bin/cpipes-contract corpus --corpus ../../..           # recorded counts vs measured
.venv/bin/cpipes-contract corpus --corpus ../../.. --apply   # write the measured ones back
.venv/bin/cpipes-contract corpus --corpus ../../.. --unknown # keys no field row accounts for
```

`corpus` without `--apply` is the drift check and exits non-zero, so a corpus that moves under the
matrix is caught rather than assumed.

## Run the harness

```bash
.venv/bin/cpipes-contract harness --code ../..               # run the harness; findings on stderr
.venv/bin/cpipes-contract harness --code ../.. --apply       # write results onto the harness columns
.venv/bin/cpipes-contract harness --code ../.. --dump DIR    # also keep each synthesized config
```

`harness` needs a Go toolchain: the runner is `go run ./tools/cpipes_contract/harness` from the
`--code` root. Without `--apply` it exits non-zero when it has findings, which makes it the same kind
of drift check as `corpus`. What it does is described in
[the harness](/docs/developer/tools/cpipes-contract/harness/).

## Certify reviewed rows

```bash
.venv/bin/cpipes-contract stamp                              # certify reviewed rows; clear withdrawn ones
.venv/bin/cpipes-contract stamp --restamp                    # re-approve rows changed since review
```

Mark a row `reviewed` by hand, then run `stamp`. See `review` and `reviewed_hash` in
[the applicability matrix](/docs/developer/tools/cpipes-contract/matrix/#fieldscsv).

## Run the test suite

The test suite is the `tests_*.py` files; `pyproject.toml` declares that pattern, without which
`pytest` collects nothing and exits 0.

```bash
uv run --extra dev pytest -q
```

Each check is one command with one exit code: there is no CI service to host them.
