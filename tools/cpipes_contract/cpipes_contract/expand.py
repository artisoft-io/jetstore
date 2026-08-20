"""Author-time expansion: a template plus bindings becomes an ordinary `.pc.json`.

**`compute_pipes` learns nothing about templates.** Expansion runs when a config is
authored, and its output is a config the runtime cannot distinguish from a hand-written
one. Anything else puts an expander on the hot path of every run and breaks item 2b's
criterion that all 45 live configs validate as they stand. Nothing in this module is
imported by the engine, and the output carries no marker that it was generated.

**The fill seam is a callable, and that is deliberate.** `expand` does not know where a
fragment comes from - the library, a model, or a person - because F.5 proves the
*mechanism* and F.6 proves the *authoring*, and §5.3.9's third qualification is that a
harness which recovers payload from the target proves placement rather than authoring.
Keeping the source behind a callable is what stops F.5's demonstration from being
mistaken for F.6's.

**Two kinds of hole, and the difference is structural.** A hole with a `$body` has its
shape given and only its nested holes filled; a hole without one is filled whole. Both
are validated against `schema_ref` **alone** before being spliced - the Phase 0 sketch's
"validate fragment alone against schema_ref", which is what makes a hole typed rather
than textual.

**`$item` is a variable reference, not a macro.** A `$body` may read a field of the
current iteration item, and that is all: no conditionals, no arithmetic, no expression
of any kind, and no expansion of the result. §5.3.2's warning is about macro languages
acquiring conditionals and arithmetic within about two releases, and the guard against
that is that there is nowhere here to put them. If a template needs a computed value, it
needs a hole.

**Q-14 holds one layer down.** A hole whose type the library cannot illustrate expands
normally; the expander never consults the library and so cannot come to depend on
coverage. Reversing that decision here, where it is hard to see, is the failure this
paragraph exists to prevent.
"""

from __future__ import annotations

import copy
from typing import Callable

from pathlib import Path

from .template import BODY_KEY, HOLE_KEY, Hole, Template, _bundle_members

ITEM_KEY = "$item"

Fill = Callable[[Hole, dict], object]
"""Given a hole and the current context, return the value that fills it."""


class ExpansionError(Exception):
    """Raised when a template cannot be expanded as written."""


def _resolve_items(node: object, ctx: dict) -> object:
    """Substitute `{"$item": "field"}` from the current item. Nothing else."""
    if isinstance(node, dict):
        if ITEM_KEY in node and len(node) == 1:
            field = node[ITEM_KEY]
            item = ctx.get(ITEM_KEY)
            if isinstance(item, dict):
                if field not in item:
                    raise ExpansionError(f"$item asks for {field!r}, which the item does not have")
                return item[field]
            if field in ("", None):
                return item
            raise ExpansionError(f"$item asks for {field!r} but the current item is not an object")
        return {k: _resolve_items(v, ctx) for k, v in node.items()}
    if isinstance(node, list):
        return [_resolve_items(v, ctx) for v in node]
    return node


def expand(
    template: Template,
    context: dict,
    fill: Fill,
    schema: dict | None = None,
) -> tuple[dict, list[str]]:
    """Expand `template` against `context`, filling holes with `fill`.

    `context` binds each `repeat_over` name to a list. Returns the config and the list
    of validation findings - a fragment that does not validate against its own
    `schema_ref` is reported and still spliced, so that an author sees the whole result
    rather than the first failure. A structural problem raises instead, because there is
    no partial answer to give.
    """
    by_name = {h.name: h for h in template.holes}
    findings: list[str] = []
    validators: dict[str, object] = {}

    def validate(hole: Hole, value: object, where: str) -> None:
        if schema is None:
            return
        import jsonschema

        if hole.schema_ref not in validators:
            validators[hole.schema_ref] = jsonschema.Draft202012Validator(
                {
                    "$schema": schema.get("$schema", ""),
                    "$ref": f"#/$defs/{hole.schema_ref}",
                    "$defs": schema["$defs"],
                }
            )
        errors = sorted(
            validators[hole.schema_ref].iter_errors(value),  # type: ignore[attr-defined]
            key=lambda e: len(e.path),
        )
        if errors:
            at = ".".join(str(p) for p in errors[0].path) or "<root>"
            findings.append(
                f"{where}: fragment for hole {hole.name!r} is not a valid "
                f"{hole.schema_ref} at {at}: {errors[0].message}"
            )

    def one(hole: Hole, ctx: dict, marker: dict, where: str) -> object:
        """The value a single occurrence of `hole` produces."""
        if BODY_KEY in marker:
            value = render(copy.deepcopy(marker[BODY_KEY]), ctx, where)
        else:
            value = fill(hole, ctx)
        validate(hole, value, where)
        return value

    def render(node: object, ctx: dict, where: str) -> object:
        if isinstance(node, dict):
            # `$item` is a dict too, so it has to be resolved here rather than left to
            # the scalar branch - a generic dict walk descends past it and emits the
            # marker verbatim, which validates as an object where a string belongs.
            if ITEM_KEY in node and len(node) == 1:
                return _resolve_items(node, ctx)
            if HOLE_KEY in node:
                name = str(node[HOLE_KEY])
                hole = by_name.get(name)
                if hole is None:
                    raise ExpansionError(f"{where}: no declaration for hole {name!r}")
                if hole.repeat_over is None:
                    return one(hole, ctx, node, f"{where}.{name}")
                items = ctx.get(hole.repeat_over)
                if items is None:
                    raise ExpansionError(
                        f"{where}: hole {name!r} repeats over {hole.repeat_over!r}, "
                        f"which the context does not bind"
                    )
                if not isinstance(items, list):
                    raise ExpansionError(
                        f"{where}: {hole.repeat_over!r} must bind a list, got "
                        f"{type(items).__name__}"
                    )
                return [
                    one(hole, {**ctx, ITEM_KEY: item}, node, f"{where}.{name}[{i}]")
                    for i, item in enumerate(items)
                ]
            return {k: render(v, ctx, where) for k, v in node.items()}
        if isinstance(node, list):
            out: list[object] = []
            for item in node:
                rendered = render(item, ctx, where)
                # A repeating hole occupies one list slot and expands into many; splicing
                # rather than nesting is what makes `apply: [{"$hole": ...}]` produce a
                # list of operators instead of a list containing a list.
                if isinstance(item, dict) and HOLE_KEY in item and isinstance(rendered, list):
                    out.extend(rendered)
                else:
                    out.append(rendered)
            return out
        return _resolve_items(node, ctx)

    result = render(copy.deepcopy(template.body), dict(context), template.name)
    if not isinstance(result, dict):
        raise ExpansionError("a template body must expand to an object")
    return result, findings


def from_library(library: list[dict], matrix: Path | None = None) -> Fill:
    """A `Fill` that returns the most-cited library part for a hole's type.

    **This proves placement, not authoring** - §5.3.9's third qualification, and the
    reason criterion 21 does not discharge criterion 22. It exists so F.5 can exercise
    the mechanism end to end without a model, and it is named so that using it in F.6
    would be an obvious error rather than a subtle one.

    A `schema_ref` names a *bundle*, and the library is keyed by `defs_name`, so the
    lookup resolves through bundle membership - the same resolution `template.check`
    does for coverage, and for the same reason: the two artefacts key on different
    things and the bundle layer is what joins them.
    """
    best: dict[str, dict] = {}
    for part in library:
        name = part["defs_name"]
        if name not in best or part["instances"] > best[name]["instances"]:
            best[name] = part
    members = _bundle_members(matrix)

    def pick(schema_ref: str) -> dict | None:
        for candidate in members.get(schema_ref, [schema_ref]):
            if candidate in best:
                return best[candidate]
        return None

    def fill(hole: Hole, ctx: dict) -> object:
        part = pick(hole.schema_ref)
        if part is None:
            raise ExpansionError(
                f"hole {hole.name!r} needs a {hole.schema_ref} and the library has none; "
                f"a library with holes is normal (Q-14), so this filler cannot serve "
                f"every template and a model or an author must"
            )
        return copy.deepcopy(part["value"])

    return fill


def from_target(target: dict, key: str = "write_step_id") -> Fill:
    """A `Fill` that recovers each fragment from an existing config.

    **Criterion 21 permits this and criterion 22 does not**, which is the whole reason
    it is a separate function with a separate name. §5.3.9's third qualification: a
    harness that recovers payload from the target proves *placement*, not authoring. F.5
    is allowed to prove placement; F.6 is the one that must not.

    The current iteration item names which fragment to recover, by `key`. Every operator
    in the target is indexed by that field, so a template derived from a config expands
    back into it - which makes the round trip checkable rather than merely plausible,
    and a round trip that does not reproduce its source is a defect in the mechanism.
    """
    index: dict[object, dict] = {}

    def collect(node: object) -> None:
        if isinstance(node, dict):
            # A hole can sit at any level, so the index is built at every level a
            # fragment can be recovered from: a stage by its step_name, an operator by
            # its output channel's key. Indexing only operators would work for the
            # template that happened to be written first and fail on the next one.
            if key in node:
                index[node[key]] = node
            for name, value in node.items():
                if name == "apply" and isinstance(value, list):
                    for op in value:
                        if isinstance(op, dict):
                            channel = op.get("output_channel")
                            if isinstance(channel, dict) and key in channel:
                                index[channel[key]] = op
                collect(value)
        elif isinstance(node, list):
            for value in node:
                collect(value)

    collect(target)

    def fill(hole: Hole, ctx: dict) -> object:
        item = ctx.get(ITEM_KEY)
        wanted = item.get(key) if isinstance(item, dict) else item
        if wanted not in index:
            raise ExpansionError(
                f"hole {hole.name!r}: the target has no operator whose "
                f"output_channel.{key} is {wanted!r}"
            )
        return copy.deepcopy(index[wanted])

    return fill
