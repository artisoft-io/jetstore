"""The token diff: what JetStore's engine can run against what this node can.

**This is `R10`, and it is the cheap half of `P9-I02` rather than its answer.**
`P9-I02` asks for an instrument because *"JetStore adds a built-in; the Python
node does not have it"*. Both sides already enumerate their tokens in one place
each -- JetStore in `jets/compute_pipes/cpipes_contract_data.go`, this node in
`scope._REGISTRY` -- so the difference is a set subtraction and the subtraction
is affordable today, before either engine is deployed anywhere.

**Three sets, and the useful one is JetStore-only.** A token JetStore declares
and this node does not is what a `use_python_node_when` expression must never
route to Python: the step would reach `scope.classify`, be refused as out of
scope, and abort the run at startup. That is the loud direction and it is by
design (X6), but it is loud *at run time* and this instrument is what makes it
visible at review time.

**The posture is baseline-pinned, and neither of the two obvious postures would
work.** A checker that failed on any difference would be red on its first run
over a gap of sixteen that `D-206` makes deliberate -- `P9-I17` reproduced
exactly, *a gate nobody can run green*. A checker that only reports never sees
the risk `P9-I02` actually names, which is not the gap but the gap *growing*: a
twentieth JetStore token lands and nothing says so. So the difference is pinned
to a declared baseline and `--check` fails only when it moves without the
baseline moving with it. **This repository has already solved that shape once
and for the same reason** -- `MISSING_GLYPH_BASELINE` in `generate-doc.sh`,
whose argument is that *a build that is red every day for a known reason is a
build people stop reading*.

**The baseline is a set and not a count**, which is where this departs from
`MISSING_GLYPH_BASELINE` deliberately. A count moves in one direction and can
be satisfied by the wrong token: `aggregate` leaving JetStore on the same day
`transmogrify` joined it nets to zero. A set moves in both directions and the
failure message can name what moved, which is the only thing a reader can act
on.

**What it does not cover is printed on every run**, by `coverage()`, and that is
a deliberate choice rather than a README paragraph. A named mitigation is a
claim about what a test detects, and this one's claim is narrow: it compares
token *names*. `P9-I117` (read 2026-09-20) is the worked example -- both
engines implement `partition_writer` and they wrote different bytes for one
document. Somebody reading a green result here is exactly the person who needs
to know that.
"""

from __future__ import annotations

import json
import re
from dataclasses import dataclass
from pathlib import Path

from . import contract
from .scope import declared_scope

#: The generated Go file that carries JetStore's side. It is the enumeration
#: the Go engine compiles and the one `F24` names, keyed `"GoStruct/token"`.
GO_CONTRACT_RELPATH = Path("jets") / "compute_pipes" / "cpipes_contract_data.go"

#: Where the JetStore tree sits relative to this module: `<root>/tools/
#: cpipes_node/cpipes_node/token_diff.py`. Only a default; `--jetstore`
#: overrides it, and the mutation proof of `AG.3` uses `--go-contract`.
_DEFAULT_JETSTORE_ROOT = Path(__file__).resolve().parents[3]

BASELINE_PATH = Path(__file__).resolve().parent / "token_diff_baseline.json"

# A top-level entry of `CpipesContract`, which gofmt writes at one tab with the
# field map opening on the same line. Field keys sit at two tabs and cannot
# match. Anchored this tightly on purpose: `AA` is adding field keys to a
# neighbouring struct's entry and `AE` may be removing a `CsvSourceSpec`
# discriminator, and neither should be able to move a number here.
_ENTRY = re.compile(r'^\t"([A-Za-z_][A-Za-z0-9_]*)/([^"]+)": \{$', re.MULTILINE)


class TokenDiffError(Exception):
    """The instrument could not measure, which is not the same as a difference."""


@dataclass(frozen=True)
class KindDiff:
    """One kind's three sets, in the order the plan names them."""

    kind: str
    go_only: tuple[str, ...]
    node_only: tuple[str, ...]
    both: tuple[str, ...]

    def as_baseline(self) -> dict[str, list[str]]:
        return {
            "go_only": list(self.go_only),
            "node_only": list(self.node_only),
            "both": list(self.both),
        }


@dataclass(frozen=True)
class Diff:
    """Every kind's three sets, plus where the JetStore side was read."""

    source: Path
    kinds: tuple[KindDiff, ...]
    #: Kinds where the Go file and `cpipes_model` disagree about the universe.
    #: Empty is the expected state and a non-empty one makes `--check` red --
    #: see `_model_disagreements` for why that is a measurement failure rather
    #: than a difference.
    disagreements: tuple[str, ...] = ()

    def by_kind(self, kind: str) -> KindDiff:
        for entry in self.kinds:
            if entry.kind == kind:
                return entry
        raise KeyError(kind)


def go_tokens(path: Path) -> dict[str, tuple[str, ...]]:
    """JetStore's declared tokens by kind, read off the generated Go file.

    Virtual tokens are dropped for the reason `contract.VIRTUAL_TOKEN_PREFIX`
    gives: `~override` is selected by the discriminator being absent and
    `~site` by its value being one no row claims, so neither is a token an
    author writes and neither belongs in a universe the node's scope is
    measured against.

    **An empty parse is refused rather than returned.** A regex that stopped
    matching would report every JetStore token as removed and every node token
    as node-only, which is a red run saying the opposite of what happened. The
    guard is the same argument `scope._load_declarations` makes about a
    registry filled by import.
    """
    try:
        text = path.read_text(encoding="utf-8")
    except OSError as exc:
        raise TokenDiffError(
            f"cannot read JetStore's token enumeration at {path}: {exc}\n"
            "  Pass --jetstore <path to the jetstore checkout>, or --go-contract "
            "<path to cpipes_contract_data.go>."
        ) from exc

    found: dict[str, set[str]] = {kind: set() for kind in _kinds()}
    structs: set[str] = set()
    for struct, token in _ENTRY.findall(text):
        structs.add(struct)
        kind = contract.TOKEN_STRUCTS.get(struct)
        if kind is None or token.startswith(contract.VIRTUAL_TOKEN_PREFIX):
            continue
        found[kind].add(token)

    if not structs:
        raise TokenDiffError(
            f"parsed {path} and found no contract entry at all.\n"
            "  The file's shape changed, or it is not cpipes_contract_data.go.\n"
            '  Expected lines of the form `\\t"GoStruct/token": {`.'
        )
    missing = [kind for kind, tokens in found.items() if not tokens]
    if missing:
        raise TokenDiffError(
            f"parsed {path} and found {len(structs)} struct(s) but no token at "
            f"all for: {', '.join(sorted(missing))}.\n"
            "  The structs that carry them are "
            f"{', '.join(sorted(contract.TOKEN_STRUCTS))}."
        )
    return {kind: tuple(sorted(tokens)) for kind, tokens in found.items()}


def node_tokens() -> dict[str, tuple[str, ...]]:
    """This node's declared tokens by kind.

    `declared_scope()` walks the registry the `Operator` subclasses put
    themselves into, so this is the producer's own declaration and not a second
    list beside it.
    """
    return {kind.value: tokens for kind, tokens in declared_scope().items()}


def _kinds() -> tuple[str, ...]:
    return tuple(sorted(set(contract.TOKEN_STRUCTS.values())))


def _model_disagreements(go: dict[str, tuple[str, ...]]) -> tuple[str, ...]:
    """Kinds where the generated Go file and `cpipes_model` do not agree.

    The Go file is *generated* from the matrix and `cpipes_model.py` is the
    source of truth for the claims, so the two agreeing is an invariant rather
    than a coincidence. When they do not, this instrument's JetStore side is a
    claim about a stale artefact and the three sets mean nothing -- which is a
    failure to *measure*, and is reported as that rather than as drift.
    """
    census = contract.contract_token_census()
    return tuple(
        kind for kind in _kinds() if set(go.get(kind, ())) != set(census.get(kind, ()))
    )


def compute(go_contract: Path | None = None, jetstore: Path | None = None) -> Diff:
    """The three sets, per kind."""
    path = go_contract or (jetstore or _DEFAULT_JETSTORE_ROOT) / GO_CONTRACT_RELPATH
    go = go_tokens(path)
    node = node_tokens()
    kinds = []
    for kind in _kinds():
        g, n = set(go.get(kind, ())), set(node.get(kind, ()))
        kinds.append(
            KindDiff(
                kind=kind,
                go_only=tuple(sorted(g - n)),
                node_only=tuple(sorted(n - g)),
                both=tuple(sorted(g & n)),
            )
        )
    return Diff(source=path, kinds=tuple(kinds), disagreements=_model_disagreements(go))


# --- the baseline -----------------------------------------------------------


def load_baseline(path: Path = BASELINE_PATH) -> dict:
    try:
        return json.loads(path.read_text(encoding="utf-8"))
    except OSError as exc:
        raise TokenDiffError(f"cannot read the baseline at {path}: {exc}") from exc
    except json.JSONDecodeError as exc:
        raise TokenDiffError(f"the baseline at {path} is not JSON: {exc}") from exc


def render_baseline(diff: Diff, measured: str) -> str:
    """The baseline as it is written down, for `--write-baseline`."""
    body = {
        "measured": measured,
        "note": (
            "Declared, not computed. Moving a set here is an act somebody takes "
            "on purpose and explains in the commit message; see R10 and Q-3 in "
            "projects/jetstore_maintenance_01."
        ),
        "kinds": {entry.kind: entry.as_baseline() for entry in diff.kinds},
    }
    return json.dumps(body, indent=2, sort_keys=False) + "\n"


@dataclass(frozen=True)
class Movement:
    """One token that entered or left one of the three sets."""

    kind: str
    set_name: str
    token: str
    direction: str  # "entered" or "left"


def movements(diff: Diff, baseline: dict) -> tuple[Movement, ...]:
    """Every token that moved between the baseline's sets and today's.

    **Both directions.** A baseline check that only notices additions is half a
    check: `aggregate` disappearing from JetStore is as much a change to what
    may be routed where as `transmogrify` appearing, and a count would net the
    two to zero.
    """
    pinned = baseline.get("kinds")
    if not isinstance(pinned, dict):
        raise TokenDiffError(
            "the baseline carries no `kinds` object; it is not a token-diff baseline."
        )
    out: list[Movement] = []
    for kind in sorted(set(pinned) | {entry.kind for entry in diff.kinds}):
        was = pinned.get(kind, {})
        try:
            now = diff.by_kind(kind).as_baseline()
        except KeyError:
            now = {}
        for set_name in ("go_only", "node_only", "both"):
            before = set(was.get(set_name, ()) or ())
            after = set(now.get(set_name, ()) or ())
            for token in sorted(after - before):
                out.append(Movement(kind, set_name, token, "entered"))
            for token in sorted(before - after):
                out.append(Movement(kind, set_name, token, "left"))
    return tuple(out)


# --- rendering --------------------------------------------------------------

_SET_LABEL = {
    "go_only": "JetStore only",
    "node_only": "node only",
    "both": "both",
}

#: What a token in each set means for routing. The failure message is the
#: deliverable of `AG.3` and a count tells a reader nothing they can act on, so
#: every movement is printed with its consequence for `use_python_node_when`.
_CONSEQUENCE = {
    ("go_only", "entered"): (
        "JetStore declares it and this node does not. A step naming it must not "
        "be routed to the Python node: a `use_python_node_when` expression that "
        "evaluates true over such a step aborts the run at startup (X6), naming "
        "the token. Either take it on -- one Operator subclass under "
        "cpipes_node/operators/ -- or keep it out of the Python arm's `when` "
        "expression."
    ),
    ("go_only", "left"): (
        "this node took it on, so a step naming it may now be routed either way."
    ),
    ("go_only", "left", "gone"): (
        "JetStore no longer declares it at all, and this node never did. A "
        "document naming it is refused by both engines; if the corpus authors "
        "it anywhere, that pipeline is broken on either arm."
    ),
    ("node_only", "entered"): (
        "this node declares it and JetStore's contract does not. A document "
        "naming it is refused by JetStore's own builder, so the declaration is "
        "dead: either JetStore dropped the token, or the declaration is a typo."
    ),
    ("node_only", "left"): "it is no longer declared by this node alone.",
    ("both", "entered"): (
        "both engines now declare it, so a step naming it may be routed either "
        "way -- and that is exactly where P9-I117 lives. Two implementations of "
        "one token agreeing is not something this instrument checks."
    ),
    ("both", "left"): (
        "one of the two engines stopped declaring it, so a step naming it can no "
        "longer be routed freely."
    ),
}


def render(diff: Diff) -> str:
    node = node_tokens()
    lines = [
        "JetStore's operator tokens against this node's declared scope",
        "",
        f"  JetStore   {diff.source}",
        "  this node  cpipes_node.scope._REGISTRY, walked by declared_scope()",
        "",
    ]
    for entry in diff.kinds:
        go_total = len(entry.go_only) + len(entry.both)
        lines.append(
            f"{entry.kind}: {len(entry.go_only)} JetStore-only, "
            f"{len(entry.node_only)} node-only, {len(entry.both)} in both "
            f"({go_total} JetStore, {len(node.get(entry.kind, ()))} this node)"
        )
        for set_name in ("go_only", "node_only", "both"):
            lines.append(f"  {_SET_LABEL[set_name]}:")
            lines.extend(_wrap(getattr(entry, set_name)))
        lines.append("")
    if diff.disagreements:
        lines.append(
            "WARNING: the generated Go file and cpipes_model.py disagree about the "
            f"token universe for: {', '.join(diff.disagreements)}."
        )
        lines.append(
            "  The Go file is generated from the matrix and the model is the source "
            "of truth for it, so the sets above are a claim about a stale artefact. "
            "Run `cpipes-contract gofile` before reading them."
        )
        lines.append("")
    lines.append(coverage())
    return "\n".join(lines)


def _wrap(
    tokens: tuple[str, ...], per_line: int = 6, indent: str = "    "
) -> list[str]:
    if not tokens:
        return [indent + "(none)"]
    return [
        indent + "  ".join(tokens[start : start + per_line])
        for start in range(0, len(tokens), per_line)
    ]


#: `AG.2` as a value rather than as a paragraph somewhere else.
COVERAGE = """What this does NOT cover

  * It compares token NAMES. It says nothing about whether two
    implementations of one token agree. Both engines implement
    `partition_writer` and they wrote different bytes for one document
    -- healthcare_corpus P9-I117, read 2026-09-20. Every token under
    `both` above is a name matching a name and nothing more.

  * So P9-I02 -- two engines drift and nothing measures it -- is
    NARROWED by this instrument and not closed. The byte-for-byte
    differential harness is healthcare_corpus's X7, and it is a project
    rather than a check: two engines, one document, a deployed run and a
    diff of what each wrote.

  * The JetStore side is the CONTRACT's universe -- what an author may
    write -- which is one token wider than the engine's dispatch:
    `infer` is rewritten to `ollama` or `vllm` by ResolveInferBackend
    before the graph is built, so no dispatch case exists for it. That
    the dispatch and the contract agree on the rest is JetStore's own
    TestBuiltinOperatorTypesMatchesTheDispatch, not this.

  * It says nothing about FIELDS. A token under `both` may be
    configured with a field one engine reads and the other ignores;
    field-level claims are cpipes-contract's.

  * A site operator is in scope by registration on both sides and
    appears here on neither. A deployment's own operator is invisible
    to this measurement by construction."""


def coverage() -> str:
    """What this instrument does not cover. Printed on every run.

    **`AG.2`, and it is output rather than documentation.** Whoever reads a
    green result is the person who needs to know how narrow the claim is, and
    they are not reading the README at that moment.
    """
    return COVERAGE


def render_check_failure(
    diff: Diff, baseline: dict, moved: tuple[Movement, ...]
) -> str:
    measured = baseline.get("measured", "an undated baseline")
    lines = [
        "The JetStore/node token difference has moved and the baseline has not.",
        "",
        f"  baseline declared {measured}, at {BASELINE_PATH}",
        f"  JetStore read from {diff.source}",
        "",
    ]
    for movement in moved:
        lines.append(
            f"  {movement.kind}: `{movement.token}` {movement.direction} "
            f"`{_SET_LABEL[movement.set_name]}`"
        )
        lines.extend(_paragraph(_consequence(diff, movement), indent="      "))
        lines.append("")
    lines.extend(
        [
            "This is not an assertion about a count. It is a routing warning: the",
            "set of tokens a `use_python_node_when` expression may safely send to",
            "the Python node is not what it was when somebody last looked.",
            "",
            "Two honest repairs, and they are different acts:",
            "  - take the token on, if the Python node should have it; or",
            "  - move the baseline deliberately, with `cpipes-node tokens",
            "    --write-baseline`, and say in the commit message why the gap",
            "    changed. D-206 makes the gap legal; it does not make it invisible.",
        ]
    )
    return "\n".join(lines)


def _consequence(diff: Diff, movement: Movement) -> str:
    """What one movement means for routing.

    **A token leaving `JetStore only` is two different events** and the reader
    has to be told which: this node took it on, or JetStore dropped it. The
    first is progress and the second is a corpus that may name a token neither
    engine has. The sets themselves answer it, so the message does rather than
    listing both and leaving the reader to work it out.
    """
    key: tuple[str, ...] = (movement.set_name, movement.direction)
    if key == ("go_only", "left"):
        try:
            still_known = movement.token in diff.by_kind(movement.kind).both
        except KeyError:  # pragma: no cover - a kind that stopped existing
            still_known = False
        if not still_known:
            key = ("go_only", "left", "gone")
    return _CONSEQUENCE.get(key, "")


def _paragraph(text: str, indent: str, width: int = 78) -> list[str]:
    line, out = indent, []
    for word in text.split():
        if line != indent and len(line) + len(word) + 1 > width:
            out.append(line)
            line = indent + word
        else:
            line = f"{line} {word}" if line != indent else indent + word
    if line != indent:
        out.append(line)
    return out
