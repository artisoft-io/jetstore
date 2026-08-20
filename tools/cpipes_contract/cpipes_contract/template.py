"""The template model: a config with typed holes, declared rather than inferred.

**The one architectural fork.** Phase 0's §5.3.2 argued against a placeholder expander
over JSON text, from precedent — macro languages over configuration acquire conditionals
and arithmetic within about two releases. §5.3.9 then produced a measurement: a
text-marker expander **cannot determine which level of a nested structure repeats**,
because the metric marker occurs at both the `apply` level and the `columns` level
within it and nothing in the text distinguishes them. **A typed hole declares its repeat
level; a string cannot carry that declaration.** If an implementation ends up inferring
the level, it has reintroduced the failure the experiment demonstrated - which is why
`repeat_over` is a field here and not a convention.

**Holes are declared beside the body, not embedded in it.** The plan's sketch shows a
`Hole(...)` sitting where a node would be, and that is the right mental model; the file
format separates them so that the body stays readable as the `.pc.json` it will become,
and so that a hole's authored metadata - its prompt above all - is not buried inside a
structure a reader is scanning for shape. The body carries `{"$hole": "<name>"}` where a
node is missing, and every marker must have a declaration and every declaration a marker.

**A `schema_ref` naming a type the library has no part for is legitimate**, settled
2026-08-20 (Q-14). Twenty of the contract's types have no exemplar after extraction,
curation and recovery, and a new operator will always arrive before its corpus does. So
coverage is *reported* rather than enforced: `check` says which holes have nothing to
show a model, and an author decides whether that matters for this template. A `Hole`
that assumed an exemplar existed would be expensive to loosen; this is cheap to tighten.

Expansion is author-time and the output is an ordinary `.pc.json` - `compute_pipes`
learns nothing about templates. That is F.4's work; this module is the model and its
checks.
"""

from __future__ import annotations

import json
from pathlib import Path

from pydantic import BaseModel, ConfigDict, Field

HOLE_KEY = "$hole"
BODY_KEY = "$body"


class Hole(BaseModel):
    """One node of a config, left to be filled."""

    model_config = ConfigDict(extra="forbid")

    name: str = Field(description="Unique within the template; the body refers to it by this name.")
    schema_ref: str = Field(
        description="The `defs_name` of the contract type that fills this hole. A fragment is "
        "validated against this entry alone, which is what makes a hole typed rather than textual."
    )
    repeat_over: str | None = Field(
        default=None,
        description="Names the collection this hole iterates, or null when it fills exactly once. "
        "**Declared, never inferred** - the metric marker of §5.3.9 occurs at two nesting levels "
        "and no rule over the body can say which one repeats.",
    )
    prompt: str = Field(
        description="What this hole is for, in English, addressed to the knowledge engineer or the "
        "model that fills it."
    )


class Template(BaseModel):
    """A `ComputePipesConfig` with selected nodes replaced by typed holes."""

    model_config = ConfigDict(extra="forbid")

    name: str
    description: str = Field(description="What configs this template produces, and when to reach for it.")
    holes: list[Hole]
    body: dict = Field(description="A ComputePipesConfig shape carrying `{\"$hole\": \"<name>\"}` markers.")

    def markers(self) -> list[tuple[str, str]]:
        """Every `$hole` marker in the body, as (name, path).

        A marker may carry a `$body` giving the shape each repetition takes, with its
        own markers inside. **That nesting is the whole point of the model**: §5.3.9's
        failure was an expander that could not tell the `apply` level from the `columns`
        level within it, and a flat list of holes reproduces exactly that ambiguity - the
        inner hole would have nowhere to sit and would have to be inferred back into
        position. A marker's `$body` says where it sits, and its own `repeat_over` says
        what it repeats over, so both levels are declared rather than derived.
        """
        found: list[tuple[str, str]] = []

        def walk(node: object, path: str) -> None:
            if isinstance(node, dict):
                if HOLE_KEY in node:
                    found.append((str(node[HOLE_KEY]), path))
                    if isinstance(node.get(BODY_KEY), (dict, list)):
                        walk(node[BODY_KEY], f"{path}.{BODY_KEY}")
                    return
                for key, value in node.items():
                    walk(value, f"{path}.{key}" if path else key)
            elif isinstance(node, list):
                for i, item in enumerate(node):
                    walk(item, f"{path}.{i}")

        walk(self.body, "")
        return found


def load(path: Path) -> Template:
    return Template.model_validate_json(path.read_text())


def _bundle_members(matrix: Path | None) -> dict[str, list[str]]:
    """Map a bundle name to the `defs_name`s it admits.

    A hole's `schema_ref` names a **bundle** rather than a raw type - that is what the
    bundle layer is for, and a hole bound to `TransformationSpec` would name a schema
    that does not fit the context at all. But the library is keyed by `defs_name`, so
    coverage has to be resolved through the membership before it means anything.
    """
    if matrix is None:
        return {}
    import csv

    kind: dict[str, str] = {}
    with open(matrix / "bundles.csv", newline="") as fh:
        for row in csv.DictReader(fh):
            kind[row["bundle"]] = row["applies_to"]
    out: dict[str, list[str]] = {}
    with open(matrix / "bundle_members.csv", newline="") as fh:
        for row in csv.DictReader(fh):
            bundle, token = row["bundle"], row["type_token"]
            base = kind.get(bundle, "")
            if not base:
                continue
            camel = "".join(p[:1].upper() + p[1:] for p in token.split("_"))
            out.setdefault(bundle, []).append(f"{base}{camel}")
    return out


DEFAULT_CONTEXT_TOKENS = 32768
DEFAULT_RESERVE_TOKENS = 8192


def check(
    template: Template,
    schema: dict,
    library: list[dict] | None = None,
    matrix: Path | None = None,
) -> tuple[list[str], list[str]]:
    """Return (findings, notes).

    Findings are defects: a marker with no declaration, a declaration with no marker, a
    duplicate name, a `schema_ref` that is not an addressable type. Notes are the
    coverage report Q-14 asked for rather than an error - a hole whose type the library
    cannot illustrate is authorable, and the author is told so.
    """
    findings: list[str] = []
    notes: list[str] = []
    defs = schema["$defs"]

    seen: set[str] = set()
    for hole in template.holes:
        if hole.name in seen:
            findings.append(f"hole {hole.name!r} is declared more than once")
        seen.add(hole.name)
        if hole.schema_ref not in defs:
            findings.append(
                f"hole {hole.name!r}: schema_ref {hole.schema_ref!r} is not a $defs entry"
            )

    marked = template.markers()
    used = {name for name, _ in marked}
    for name, path in marked:
        if name not in seen:
            findings.append(f"body has a marker for undeclared hole {name!r} at {path or '<root>'}")
    for hole in template.holes:
        if hole.name not in used:
            findings.append(f"hole {hole.name!r} is declared but never placed in the body")

    # A repeating hole must sit inside a list: that is where its expansion goes.
    positions = dict((name, path) for name, path in marked)
    for hole in template.holes:
        if hole.repeat_over is None:
            continue
        path = positions.get(hole.name)
        if path is not None and not path.rsplit(".", 1)[-1].isdigit():
            findings.append(
                f"hole {hole.name!r} repeats over {hole.repeat_over!r} but sits at {path}, "
                f"which is not a list position; a repeating hole expands into a list"
            )

    # A `schema_ref` too large for the infer server's context can still be *recovered*
    # into - no model is involved - so this is a note rather than a finding, and the
    # refusal belongs where a model is actually asked. That is the same split
    # `jets/agentic/prompt` makes: the schema is emitted regardless and Task validation
    # is what refuses. `ConditionalPipeSpec` is the live case at 49,008 tokens, which is
    # I-29 arriving at an entry point the bundle layer does not cover.
    budget = DEFAULT_CONTEXT_TOKENS - DEFAULT_RESERVE_TOKENS
    for hole in template.holes:
        if hole.schema_ref not in defs:
            continue
        from .authoring import subschema

        tokens = len(json.dumps(subschema(schema, hole.schema_ref), separators=(",", ":"))) // 4
        if tokens > budget:
            notes.append(
                f"hole {hole.name!r} fills {hole.schema_ref}, whose schema is ~{tokens} tokens "
                f"against a {budget} budget - it can be recovered into but **not authored** "
                f"(criterion 22 is unreachable for this hole; name a bundle instead)"
            )

    if library is not None:
        have: dict[str, int] = {}
        for part in library:
            have[part["defs_name"]] = have.get(part["defs_name"], 0) + 1
        members = _bundle_members(matrix)
        for hole in template.holes:
            targets = members.get(hole.schema_ref, [hole.schema_ref])
            covered = {t: have.get(t, 0) for t in targets}
            total = sum(covered.values())
            empty = sorted(t for t, n in covered.items() if n == 0)
            if total == 0:
                notes.append(
                    f"hole {hole.name!r} fills {hole.schema_ref}, which the library has no part for "
                    f"- authorable, but nothing can be shown as an example (Q-14)"
                )
            elif empty:
                notes.append(
                    f"hole {hole.name!r} fills {hole.schema_ref}: {total} part(s) across "
                    f"{len(targets) - len(empty)} of {len(targets)} member types; no example for "
                    f"{', '.join(empty)}"
                )
    return findings, notes
