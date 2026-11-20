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

## Name the corpus's workspaces

The matrix, the fragment and repair libraries and the templates name the corpus's
workspaces by neutral names, `org1_ws`, `org2_ws` and so on, and so do the class prefixes,
environment variables and file names drawn from them. The workspaces on disk keep their own
names. `CPIPES_CONTRACT_ALIASES` names a JSON file, kept outside this repository, that maps
each real token to its neutral one:

```json
{"acme_ws": "org1_ws", "acme": "org1", "ACME": "ORG1"}
```

```bash
export CPIPES_CONTRACT_ALIASES=/path/to/aliases.json
```

A token matches as a whole word only, so `_`, `:`, `-`, `.` and `/` separate it from its
neighbours, and the longer of two tokens wins. Everything read out of the corpus and recorded
or compared (fragments and their citations, repair cases, the configurations the template
tests reproduce) is read under the neutral names, and a recorded path is opened under the
real ones. Where two real tokens share a neutral one, the first listed is the one a path
resolves to. A repair case's diagnosis is commit text, so a ticket number in it is a token
like any other: list it before committing the case.

Without the file nothing resolves to a workspace on disk: `check --code` sets those citations
aside as it does for a workspace that is not checked out, `check --corpus` reports each
exemplar it cannot open, and the tests that read a named workspace's configuration skip.

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

A citation into `workspaces/` (generator evidence, which lives in a deployment's own code) resolves
against the folder holding `workspaces/`: `--corpus` when it is given, otherwise the folder holding
the `--code` root. Where that workspace is not in the checkout, the citation is not checked and
`check` prints how many it set aside, so a JetStore-only checkout reports the same problems as a
full one. The test suite runs the same rule over every citation into the JetStore repository, so a
change that moves a cited line fails `pytest` as well.

## Carry citations whose lines moved

```bash
.venv/bin/cpipes-contract relocate --code ../..             # what would move, and what cannot
.venv/bin/cpipes-contract relocate --code ../.. --apply     # write the carried citations
```

A line added above a cited declaration moves every citation below it. `relocate` takes each
citation `--code` rejects, finds the newest version of the cited file in git in which the line did
name what it cites, and carries that line through the diff to the file as it is now. It keeps the
result only when the carried line names it; a citation whose line itself changed is left for a
reader and makes the command exit non-zero. It never touches `review` or `reviewed_hash`, so a
reviewed row whose citation moved reads "changed since it was reviewed" until it is re-approved
with `stamp --restamp`.

## Measure the corpus

```bash
.venv/bin/cpipes-contract corpus --corpus ../../..           # recorded counts vs measured
.venv/bin/cpipes-contract corpus --corpus ../../.. --apply   # write the measured ones back
.venv/bin/cpipes-contract corpus --corpus ../../.. --unknown # keys no field row accounts for
```

`corpus` without `--apply` is the drift check and exits non-zero, so a corpus that moves under the
matrix is caught rather than assumed.

## Regenerate the fragment library

```bash
.venv/bin/cpipes-contract fragments --corpus ../../..
```

`fragments` writes `fragments/library.jsonl`, every distinct part of the live corpus keyed by its
`$defs` entry with its citations, and `fragments/library.curated.jsonl`, the selection that
authoring prompts and template expansion read. It exits non-zero when a union cannot be resolved,
when a type's count disagrees with `types.csv`, when the curation drops a part of a type with four
or fewer corpus instances, or when a historical source named in `recovered_sources.csv` cannot be
read.

**The library carries configuration and no free text.** Every property the schema describes as
free text for the reader (`comment`, which JetStore ignores) is removed from a configuration before
its parts are recorded, at every depth, so a workspace author's prose reaches neither the library,
nor the examples an authoring prompt shows, nor a configuration expanded from the library. Two parts
that differed only by a comment are one part. A key named `comment` inside data the schema does not
describe, such as a site operator's own `config`, is that data and is kept.

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
