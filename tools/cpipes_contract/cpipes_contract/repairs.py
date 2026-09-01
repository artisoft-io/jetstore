"""The repair-case library: real configuration defects and the changes that fixed them.

**This is the third case shape and it is deliberately not the other two.** A
*mutation* case (``eval.Case``) is a working config with a hole cut in it and the
removed instance as ground truth; a *synthetic runtime* case is an input file and
the failure it should produce. A **repair** case is a config that was wrong in
production and the commit that fixed it. I-115 and I-136 settled that the three
stay apart, and P.1 made ``eval.Case`` more firmly the mutation type still, since
``Hole`` and ``Fill`` are mutation-specific. Hence ``provenance``, mandatory on
every record here.

**What the library is for, which is not what P.2 set out to build.** Measuring the
whole corpus found that **77% of real repairs change nothing the schema can see**:
the config was valid before the fix and valid after it, and the defect shipped
because every automated layer passed it. ``negative_suite.json`` already covers
what the schema *can* catch, under the rule *"a negative that validates is a
schema hole"*. This library is the complement — the defects that got through — and
its classification is the deliverable rather than any pass rate.

**Classification is by error-set delta, not by absolute validity, and the
difference is not a detail.** Judging a 2024 config against the 2026 schema puts
48 of 75 pairs in "invalid both before and after", because the contract itself
moved: W.1 added ``required`` fields in August 2026. That measures contract drift
and says nothing about the repair. Comparing the *set* of schema errors before
against after is robust to it — an unrelated pre-existing error appears on both
sides and cancels.

**The library stores coordinates and the changed fragments, never whole configs.**
A case is (workspace, sha, file) plus the JSON paths that differ, so the full
before and after are recoverable from the submodule's history while the record
stays small enough to read. That mirrors what ``Hole`` does for a mutation case:
locate the defect rather than carry the document around it.
"""

from __future__ import annotations

import json
import re
import subprocess
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Iterator

import jsonschema

# The four classes a repair falls into, by what the contract could see of it.
CONTRACT_BLIND = "contract-blind"
CONTRACT_VISIBLE = "contract-visible"
REGRESSION = "regression"
MIXED = "mixed"

CLASSES = (CONTRACT_BLIND, CONTRACT_VISIBLE, REGRESSION, MIXED)

PROVENANCE = "config-repair"


@dataclass(frozen=True)
class Site:
    """One JSON path at which the repair changed something."""

    path: list[str]
    before: Any
    after: Any


@dataclass
class Case:
    """One repair: a defect that reached a workspace, and the commit that fixed it."""

    name: str
    workspace: str
    sha: str
    files: list[str]
    cls: str
    diagnosis: str
    sites: list[Site] = field(default_factory=list)
    provenance: str = PROVENANCE

    def to_record(self) -> dict:
        return {
            "name": self.name,
            "provenance": self.provenance,
            "class": self.cls,
            "workspace": self.workspace,
            "sha": self.sha,
            "files": self.files,
            "diagnosis": self.diagnosis,
            "sites": [
                {"path": s.path, "before": s.before, "after": s.after} for s in self.sites
            ],
        }


def from_record(rec: dict) -> Case:
    return Case(
        name=rec["name"],
        workspace=rec["workspace"],
        sha=rec["sha"],
        files=list(rec["files"]),
        cls=rec["class"],
        diagnosis=rec.get("diagnosis", ""),
        sites=[Site(list(s["path"]), s.get("before"), s.get("after")) for s in rec.get("sites", [])],
        provenance=rec.get("provenance", ""),
    )


# --- the contract's view of a document ---------------------------------------


def error_set(doc: Any, schema: dict) -> set[tuple]:
    """The set of schema errors, keyed by where and which rule.

    **Keyed by (path, validator) rather than by message** so that a reworded
    error message does not read as a changed error set. The message is where
    jsonschema is least stable between versions.
    """
    v = jsonschema.Draft202012Validator(schema)
    return {
        (tuple(str(p) for p in e.absolute_path), str(e.validator)) for e in v.iter_errors(doc)
    }


def classify(before: Any, after: Any, schema: dict) -> str:
    """Which class a repair falls into, by what the fix did to the error set.

    Strict subset both ways, because a repair that both removes and adds errors
    is neither clean nor a regression and should be looked at by a human.
    """
    b, a = error_set(before, schema), error_set(after, schema)
    if b == a:
        return CONTRACT_BLIND
    if a < b:
        return CONTRACT_VISIBLE
    if b < a:
        return REGRESSION
    return MIXED


# --- reading the workspace history -------------------------------------------


def _git(ws_root: Path, *args: str) -> subprocess.CompletedProcess:
    return subprocess.run(["git", "-C", str(ws_root), *args], capture_output=True)


def blob(ws_root: Path, rev: str, path: str) -> Any | None:
    """The parsed JSON of one file at one revision, or None if it is unavailable.

    None covers three different situations on purpose — the revision does not
    exist, the path did not exist at it, or the content does not parse — because
    a caller can act on none of them differently: the case cannot be classified
    either way.
    """
    proc = _git(ws_root, "show", f"{rev}:{path}")
    if proc.returncode != 0:
        return None
    try:
        return json.loads(proc.stdout)
    except json.JSONDecodeError:
        return None


def diff_sites(before: Any, after: Any, path: list[str] | None = None) -> Iterator[Site]:
    """The JSON paths at which two documents differ, deepest common prefix first.

    **It stops descending as soon as the shapes stop matching**, so a replaced
    object is reported once at its own path rather than as a shower of leaf
    differences. That keeps a case readable: the site is the defect, and a reader
    should not have to reassemble it from fragments.
    """
    path = path or []
    if type(before) is not type(after):
        yield Site(path, before, after)
        return
    if isinstance(before, dict):
        for key in sorted(set(before) | set(after)):
            if key not in before or key not in after:
                yield Site(path + [key], before.get(key), after.get(key))
            else:
                yield from diff_sites(before[key], after[key], path + [key])
        return
    if isinstance(before, list):
        if len(before) != len(after):
            yield Site(path, before, after)
            return
        for i, (b, a) in enumerate(zip(before, after)):
            yield from diff_sites(b, a, path + [str(i)])
        return
    if before != after:
        yield Site(path, before, after)


# --- the falsifier ------------------------------------------------------------


@dataclass
class Finding:
    case: str
    message: str


def check(cases: list[Case], schema: dict, corpus_root: Path) -> list[Finding]:
    """Re-derive every case's class from the history and complain where it differs.

    **This is what makes the class a claim rather than a label**, and it is
    ``negative_suite.json``'s own rule applied one artefact over: there, every
    case marked invalid must fail validation, because a negative that validates
    is a schema hole. Here, a case marked ``contract-blind`` whose error set has
    started to move is a case the contract has *learned to see* — which is good
    news about the schema and a stale record, and either way something a person
    should look at rather than something to re-derive silently on every run.
    """
    findings: list[Finding] = []
    for case in cases:
        if case.provenance != PROVENANCE:
            findings.append(Finding(case.name, f"provenance is {case.provenance!r}, not {PROVENANCE!r}"))
        if case.cls not in CLASSES:
            findings.append(Finding(case.name, f"unknown class {case.cls!r}"))
            continue
        ws_root = corpus_root / "workspaces" / case.workspace
        if not ws_root.exists():
            findings.append(Finding(case.name, f"workspace not checked out: {ws_root}"))
            continue
        # **Aggregated over the case's files exactly as the extractor aggregates
        # them**, because a case is a commit. Checking per file against a class
        # that describes the commit would report a false mismatch on any
        # multi-file repair whose files disagree -- the case is `mixed` and each
        # file is not.
        classes = set()
        for path in case.files:
            before, after = blob(ws_root, case.sha + "^", path), blob(ws_root, case.sha, path)
            if before is None or after is None:
                findings.append(Finding(case.name, f"{path}: revision unavailable at {case.sha[:8]}"))
                continue
            classes.add(classify(before, after, schema))
        if not classes:
            continue
        actual = classes.pop() if len(classes) == 1 else MIXED
        if actual != case.cls:
            findings.append(Finding(case.name, f"recorded {case.cls}, now {actual}"))
    return findings


# --- the extractor ------------------------------------------------------------

# Words that mark a commit as claiming to fix something. Deliberately broad: this
# is a net for a human to sort through, not a classifier. Recall matters and
# precision does not, because every candidate is read before it becomes a case.
REPAIR_WORDS = (
    "fix", "fixed", "fixes", "bug", "error", "correct",
    "repair", "issue", "broken", "wrong", "revert",
)


# **A regex with word boundaries rather than a split on spaces**, because the
# first version of this function split on whitespace and so did not match
# "error-channel" -- which silently dropped the sweep commit that supplies most
# of the contract-visible cases. A hyphen is a word boundary and is not a space.
_REPAIR_RE = re.compile(r"\b(" + "|".join(REPAIR_WORDS) + r")\b", re.IGNORECASE)


def _looks_like_repair(subject: str) -> bool:
    return bool(_REPAIR_RE.search(subject))


def candidates(corpus_root: Path, workspace: str, schema: dict) -> Iterator[Case]:
    """Every commit in one workspace that modifies a live config and claims a fix.

    **An extractor, not a scraper**, and the distinction is R-17's *curation
    rather than extraction* with a fifth reason behind it: the ground truth of a
    repair is the diagnosis, the diagnosis is prose in the commit body, and no
    parser recovers it. What comes out of here is a candidate for a human to
    name, judge and keep or drop.
    """
    ws_root = corpus_root / "workspaces" / workspace
    log = _git(ws_root, "log", "--diff-filter=M", "--format=%H\t%s",
               "--", "pipes_config/*.pc.json").stdout.decode()
    for line in log.strip().splitlines():
        if "\t" not in line:
            continue
        sha, subject = line.split("\t", 1)
        if not _looks_like_repair(subject):
            continue
        files = _git(ws_root, "diff-tree", "--no-commit-id", "--name-only", "-r", sha,
                     "--", "pipes_config/*.pc.json").stdout.decode().split()
        body = _git(ws_root, "log", "-1", "--format=%B", sha).stdout.decode().strip()

        # **One case per commit, not per file.** A defect and its fix are one
        # thing; the sweep that removed an inert `sampling_max_count` touched
        # nine files in three workspaces and is one defect, not nine. Keying on
        # the file would have put it in the library nine times and inflated
        # every count drawn from it -- the denominator error this corpus keeps
        # producing.
        kept, classes, sites = [], set(), []
        for path in files:
            before, after = blob(ws_root, sha + "^", path), blob(ws_root, sha, path)
            if before is None or after is None:
                continue
            kept.append(path)
            classes.add(classify(before, after, schema))
            sites += list(diff_sites(before, after))
        if not kept:
            continue
        yield Case(
            name=f"{workspace}-{sha[:7]}",
            workspace=workspace,
            sha=sha,
            files=kept,
            cls=classes.pop() if len(classes) == 1 else MIXED,
            diagnosis=body,
            sites=sites,
        )


# --- rendering ----------------------------------------------------------------


def load(path: Path) -> list[Case]:
    if not path.exists():
        return []
    return [from_record(json.loads(l)) for l in path.read_text().splitlines() if l.strip()]


def to_jsonl(cases: list[Case]) -> str:
    return "".join(json.dumps(c.to_record(), sort_keys=True) + "\n" for c in cases)


def render_check(cases: list[Case], findings: list[Finding]) -> str:
    counts: dict[str, int] = {}
    for c in cases:
        counts[c.cls] = counts.get(c.cls, 0) + 1
    lines = [f"repair library: {len(cases)} case(s)"]
    for cls in CLASSES:
        if counts.get(cls):
            lines.append(f"  {cls:18} {counts[cls]}")
    if not findings:
        lines.append("every recorded class still holds against the current schema.")
    else:
        lines.append("")
        lines.append(f"{len(findings)} finding(s):")
        lines += [f"  {f.case}: {f.message}" for f in findings]
    return "\n".join(lines)
