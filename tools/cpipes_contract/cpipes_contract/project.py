"""The projection: a template and its bindings become a UserFlow document. Task M.4.

**Author-time, like `expand`, and for the same reason.** The output is an ordinary
`.uf.json` / `.form.json` pair that `jetsclient_ide` interprets without knowing a
template exists. Nothing here is imported by the interpreter, and nothing the
interpreter reads carries a `schema_ref` — the 2026-08-20 gate settled that the two
projects share the *interpreter*, not the schema, and putting a template's type
vocabulary into a UserFlow document would reverse that on the first commit.

## What the projection walks, and why it is two surfaces

**Holes and scalar bindings both** (M.1). Every one of the contract's 200 `$defs`
entries is an object type, so a hole cannot produce a bare string (I-43) and everything
a config needs that *is* a bare string lives in the bindings. For two of the three
shipped templates the bindings are *entirely* that residue: a wizard walking only holes
could fill each `ConditionalPipeSpec` of `map_claim_load_stages` and could not choose
which three entities to load.

## One state per *fill*, not per hole

A hole carrying a `$body` is a loop rather than a blank (I-76), and a hole nested inside
a repeating hole fires the product of the two counts — `qc_metrics.metric_columns` fires
nine times from two lists of three. **So this module does not walk the template.** It
drives `expand` with a recording `Fill` and projects what the expander actually asks
for. That is deliberate: §5.3.9's failure was an expander that could not tell one
nesting level from another, and a second traversal written here could drift from the
first without either being obviously wrong. There is one traversal, and the projection
is a consumer of it.

## How a composite property is rendered (M.2)

Three renderings, chosen by two structural tests on the property rather than by taste —
its arity, and whether its target closes a cycle in the contract:

* **A list of typed objects is not a field at all.** It is where a nested hole goes; the
  template declares it and the count lives in the bindings. A list is a count decision,
  and `repeat_over` already says that.
* **A single typed object is a nested state** — this projection recursing on itself. The
  target of such a property is exactly what a `schema_ref` names, so the state it
  produces is the state a hole of that type would have produced.
* **A property that closes a cycle gets no state**, and says so with a `label`. Walking
  from the seven hole types reaches 110 types and finds 20 back edges over 16 property
  slots; a generator emitting one state per level cannot unroll a cycle. The refusal is
  on the *edge* rather than the type, so `InputChannelConfigStage` loses
  `merge_channels` and keeps its other eleven properties.

**The `text` field under the `json` rule is not used for a composite.** `validateForm`
applies that rule by calling `JSON.parse` and nothing else, so a typed object arriving
through it is checked only when the finished document is — the untyped fill §5.3.2
refused, with a border. The one place a `json` rule is emitted is a list of *scalars*,
where what the rule cannot check is arity and element type rather than a whole contract
type, and where there is no variant to choose and no structure to nest.

## What the author is choosing, and what they are not

**The bindings size the flow.** A projected flow edits the values of a fixed shape; it
cannot add a metric, because the number of states is decided when the document is
generated and UserFlow has no way to create one at run time. Changing a count means
editing the bindings and re-projecting. That is a real boundary and it is stated rather
than discovered at M.5.
"""

from __future__ import annotations

import json
import re
from pathlib import Path

from .expand import expand
from .template import Hole, Template

SCHEMA_VERSION = 1

#: The action the end state dispatches to assemble the collected form state into a
#: `.pc.json`. It is a *name*: the interpreter resolves it against a registry compiled
#: into the IDE bundle, so a flow naming it does not load until that registration
#: exists. Emitted anyway — a document that describes only the part already built is
#: the silent kind of incompleteness, and the four validation layers this artefact is
#: checked against do not read the registry either.
APPLY_ACTION = "cpipesTemplateApply"

_IDENTIFIER = re.compile(r"^[A-Za-z0-9_.-]+$")


class ProjectionError(Exception):
    """Raised when a template cannot be projected as written."""


def _identifier(*parts: object) -> str:
    """A dotted key. `Identifier` admits `.`, so a whole path is expressible as one key.

    That is what makes the nested state cheap: form state is flat — `FormStateValue` is
    `string | string[]` and cannot hold an object — so a nested value is carried as
    leaves whose keys spell the path back. **Nothing on the interpreter side had to
    change for this**; the regex already allowed it.

    Characters the regex does not admit are folded to `_` rather than rejected, because
    a binding may legitimately be named `$item` and a key is an internal handle, not the
    path itself. The path stays readable in the field's label.
    """
    key = ".".join(str(p) for p in parts if p != "")
    key = re.sub(r"[^A-Za-z0-9_.-]", "_", key)
    if not _IDENTIFIER.match(key):
        raise ProjectionError(f"{key!r} cannot be made into a bare identifier")
    return key


def _label(name: str) -> str:
    """`output_channel` becomes `Output channel`. Nothing cleverer."""
    text = name.replace("_", " ").strip()
    return text[:1].upper() + text[1:] if text else name


def _unwrap(sch: dict) -> dict:
    """Strip a nullable `anyOf`/`oneOf` wrapper, which is how pydantic emits optional."""
    for key in ("anyOf", "oneOf"):
        if key in sch and "discriminator" not in sch:
            members = [m for m in sch[key] if m.get("type") != "null"]
            if len(members) == 1:
                return _unwrap(members[0])
    return sch


def _refname(sch: dict) -> str | None:
    ref = sch.get("$ref")
    return ref.split("/")[-1] if ref else None


class Contract:
    """The contract, with the few queries this module asks of it."""

    def __init__(self, schema: dict) -> None:
        self.defs: dict = schema["$defs"]

    def is_union(self, name: str) -> bool:
        return name in self.defs and "oneOf" in self.defs[name]

    def variants(self, name: str) -> list[str]:
        if not self.is_union(name):
            return [name]
        return [_refname(m) or "" for m in self.defs[name]["oneOf"]]

    def items_for(self, name: str) -> list[dict]:
        """A union's variant choices, read from its `discriminator.mapping`.

        **The mapping's key is exactly the `type` literal a config needs**, which is why
        `ui_refresh` handed this half back rather than taking it: I.2a owes a field kind
        that *accepts* a list, not the thing that computes one. All three multi-variant
        hole types carry an explicit discriminator.
        """
        disc = self.defs[name].get("discriminator")
        if not disc:
            raise ProjectionError(f"{name} is a union with no discriminator to build items from")
        return [
            {"value": literal, "label": _label(literal)}
            for literal in sorted(disc["mapping"])
        ]

    def discriminator_property(self, name: str) -> str:
        return self.defs[name]["discriminator"]["propertyName"]

    def properties(self, name: str) -> dict:
        return self.defs.get(name, {}).get("properties", {})

    def required(self, name: str) -> set[str]:
        return set(self.defs.get(name, {}).get("required", []))

    def classify(self, psch: dict) -> tuple[str, list[str]]:
        """`(kind, targets)`; kind is one of const, enum, scalar, scalar_list, object,
        object_list, or map."""
        sch = _unwrap(psch)
        depth = 0
        while sch.get("type") == "array":
            sch = _unwrap(sch.get("items", {}))
            depth += 1
        targets: list[str] = []
        ref = _refname(sch)
        if ref:
            targets = [ref]
        elif "oneOf" in sch or "anyOf" in sch:
            for member in sch.get("oneOf") or sch.get("anyOf"):
                member = _unwrap(member)
                name = _refname(member)
                if name:
                    targets.append(name)
        if targets:
            return ("object_list" if depth else "object"), targets
        if sch.get("type") == "object":
            return "map", []
        if "const" in sch:
            return "const", []
        if "enum" in sch:
            return ("enum_list" if depth else "enum"), []
        return ("scalar_list" if depth else "scalar"), []

    def back_edges(self, roots: list[str]) -> set[tuple[str, str]]:
        """`(owning type, property)` pairs whose target is already on the path.

        Computed rather than listed, so a contract change moves the refusals with it.
        """
        found: set[tuple[str, str]] = set()

        def walk(name: str, stack: frozenset[str]) -> None:
            for variant in self.variants(name):
                if variant not in self.defs:
                    continue
                for prop, psch in self.properties(variant).items():
                    _, targets = self.classify(psch)
                    for target in targets:
                        if target not in self.defs:
                            continue
                        concrete = set(self.variants(target))
                        if target in stack or concrete & stack:
                            found.add((variant, prop))
                        else:
                            walk(target, stack | concrete | {target, variant})

        for root in roots:
            walk(root, frozenset(self.variants(root)) | {root})
        return found


class _Fills:
    """Records what the expander asks a filler for, in the order it asks.

    A `Fill` sees the hole and the context and not the path, so the identity of one
    occurrence is its ordinal among that hole's calls. That is stable because
    `expand`'s traversal is deterministic, and it is what a state key needs — an
    `Identifier` admits `.` but not `[`, so occurrence three of `metric_columns` is
    `metric_columns.3` rather than `metric_columns[3]`.
    """

    def __init__(self) -> None:
        self.calls: list[tuple[Hole, int]] = []
        self._seen: dict[str, int] = {}

    def __call__(self, hole: Hole, _context: dict) -> object:
        index = self._seen.get(hole.name, 0)
        self._seen[hole.name] = index + 1
        self.calls.append((hole, index))
        # A placeholder, not a fragment. Validation is off for this run: the point is
        # the call sequence, and an empty object keeps `expand`'s list splicing honest.
        return {}


def _is_record(path: str, in_list: bool) -> bool:
    """Whether a dict at `path` is a *container of bindings* rather than a binding.

    The distinction is I-78's and it is the one the classifier turns on: **an
    object-valued binding could have been a hole; a scalar-valued one could not.** So a
    dict is descended into when it is the root, an element of a list, or the singleton
    `$item` record — those are rows of bindings. A dict found as the *value* of a
    property inside such a row is a contract-typed object arriving as untyped context,
    and this projection reports it rather than rendering it.
    """
    return path == "" or in_list or path.endswith("$item")


def _walk_bindings(context: dict) -> tuple[list[tuple[str, object]], list[str]]:
    """`(scalar leaves, object-valued bindings)`, both as dotted paths.

    Cross-checked against M.1's census, which counted these by hand from the same three
    files: 1/3/21 scalars and 0/0/8 objects for `map_claim_load_stages`, `qc_metrics`
    and `qc_report`. A traversal that disagrees with a count derived another way is
    wrong until shown otherwise — the lesson of M.2's first corpus walk, which handled
    one level of list nesting and reported zero `where` clauses where `grep -c` finds
    136.
    """
    scalars: list[tuple[str, object]] = []
    objects: list[str] = []

    def walk(node: object, path: str, in_list: bool) -> None:
        if isinstance(node, dict):
            if not _is_record(path, in_list):
                objects.append(path)
                return
            for key, value in node.items():
                walk(value, f"{path}.{key}" if path else key, False)
        elif isinstance(node, list):
            if all(not isinstance(v, (dict, list)) for v in node):
                scalars.append((path, node))
                return
            for i, item in enumerate(node):
                walk(item, f"{path}.{i}", True)
        else:
            scalars.append((path, node))

    walk(context, "", False)
    return scalars, objects


def _note_field(text: str) -> dict:
    """A `label`. **A refusal is visible rather than silent**, which is the whole of why
    this exists: a control that does nothing and says nothing is the failure I-77
    measured on the Flutter side, and a `label` costs one of the four field kinds that
    were already there."""
    return {"field": "label", "text": text}


class _Block:
    """A run of states with one entry and possibly several exits.

    **The exits are why this is a type rather than a list.** A variant chooser branches
    into one state per variant, and every branch has to continue to whatever follows the
    *whole* choice — not to the next variant's form. Chaining states in walk order gets
    that wrong in a way nothing catches: the document validates, every state is
    reachable, and an author who picks `select` walks into `value`'s form next.
    """

    def __init__(self, entry: str, exits: list[str]) -> None:
        self.entry = entry
        self.exits = exits


class _Projector:
    """Builds the two documents. One instance per projection."""

    def __init__(self, contract: Contract, hole_types: list[str]) -> None:
        self.c = contract
        self.back = contract.back_edges(hole_types)
        self.states: dict[str, dict] = {}
        self.forms: dict[str, dict] = {}
        self.notes: list[str] = []

    # -- one state --------------------------------------------------------------

    def add(self, key: str, description: str, rows: list[list[dict]]) -> str:
        if key in self.states:
            raise ProjectionError(f"two states would take the key {key!r}")
        self.states[key] = {"description": description, "formConfig": key}
        self.forms[key] = {
            "rows": rows,
            "actions": [
                {"action": "ufPrevious", "label": "Back", "style": "secondary"},
                {"action": "ufNext", "label": "Next", "style": "primary",
                 "enableOnlyWhenFormValid": True},
                {"action": "ufCancel", "label": "Cancel", "style": "secondary"},
            ],
        }
        return key

    def link(self, frm: str, to: str) -> None:
        self.states[frm]["defaultNextState"] = to

    def chain(self, blocks: list[_Block]) -> _Block:
        """Link a sequence of blocks and return the run as one block."""
        blocks = [b for b in blocks if b is not None]
        if not blocks:
            raise ProjectionError("an empty chain has no entry")
        for first, second in zip(blocks, blocks[1:]):
            for exit_key in first.exits:
                self.link(exit_key, second.entry)
        return _Block(blocks[0].entry, blocks[-1].exits)

    # -- fields -----------------------------------------------------------------

    def fields_for(self, type_name: str, prefix: str) -> tuple[list[list[dict]], list[tuple[str, dict, str]]]:
        """One concrete type's form rows, plus the nested states it owes.

        A nested entry is `(key prefix, property schema, label)`; the property schema is
        carried rather than a type name because an inline `oneOf` of refs has no `$defs`
        entry of its own and its discriminator sits on the property.

        **Required properties first, then optional, separated by a `label`.** Every
        declarable property gets a field: `InputChannelConfigStage` declares 43 and the
        corpus sets a median of 5 (I-80), so this is a long form — and the alternative,
        offering only what four workspaces happen to use, makes a legal config
        unreachable in the IDE while remaining legal in the file. **A wizard that cannot
        express a legal config is a worse failure than a wizard that is long**, and the
        ordering costs nothing.
        """
        required = self.c.required(type_name)
        req: list[list[dict]] = []
        opt: list[list[dict]] = []
        nested: list[tuple[str, dict, str]] = []

        for prop, psch in self.c.properties(type_name).items():
            kind, targets = self.c.classify(psch)
            key = _identifier(prefix, prop)
            rows = req if prop in required else opt

            if kind == "const":
                # The discriminator, already chosen by the dropdown that reached this
                # state. Asking twice is how the two come to disagree.
                continue
            if (type_name, prop) in self.back:
                rows.append([_note_field(
                    f"{_label(prop)} — not editable here. This property can nest inside itself, "
                    f"so it keeps whatever the template gives it.")])
                self.notes.append(f"refused, back edge: {type_name}.{prop}")
                continue
            if kind == "map":
                rows.append([_note_field(
                    f"{_label(prop)} — not editable here. Its keys are chosen by the author, and "
                    f"neither a field nor a state can name a key that does not exist yet.")])
                self.notes.append(f"refused, authored keys: {type_name}.{prop}")
                continue
            if kind == "object_list":
                self.notes.append(
                    f"no field: {type_name}.{prop} is a list of typed objects — where a nested "
                    f"hole goes, not a blank this flow fills")
                continue
            if kind == "object":
                nested.append((key, psch, _label(prop)))
                rows.append([_note_field(f"{_label(prop)} — configured in the following step.")])
                continue

            rows.append([self.value_field(prop, psch, kind, key, prop in required)])

        out: list[list[dict]] = list(req)
        if opt:
            if out:
                out.append([{"field": "spacer"}])
            out.append([{"field": "label", "text": "Optional"}])
            out.extend(opt)
        return (out or [[_note_field("Nothing to configure at this step.")]]), nested

    def value_field(self, prop: str, psch: dict, kind: str, key: str, is_required: bool) -> dict:
        label = _label(prop)
        if kind in ("enum", "enum_list"):
            literals = _unwrap(psch).get("enum", [])
            field: dict = {
                "field": "dropdown",
                "key": key,
                "label": label,
                "items": [{"value": "", "label": f"Select {label.lower()}…"}]
                + [{"value": str(v), "label": _label(str(v))} for v in literals],
            }
        else:
            field = {"field": "text", "key": key, "label": label}
            hint = _unwrap(psch).get("description") or psch.get("description")
            if hint:
                field["hint"] = hint[:200]
            if kind == "scalar_list":
                # **The one place a `json` rule is emitted, and it is not the escape M.2
                # rejected.** For a typed object that rule checks syntax and never the
                # contract; for a list of scalars what it cannot check is arity and
                # element type — there is no variant to choose and nothing to nest.
                field["maxLines"] = 3
                field["hint"] = 'A JSON list, e.g. ["a", "b"]'
                field["rules"] = [{"rule": "json", "message": f"{label} must be a JSON list"}]
        if is_required:
            field.setdefault("rules", []).append(
                {"rule": "required", "message": f"{label} is required"})
        return field

    # -- a typed value, which may be a union ------------------------------------

    def project_value(self, key: str, targets: list[str], disc: dict | None, title: str) -> _Block:
        """States for one typed value: a variant choice where there is one, then fields.

        **A mandatory choice is a valueless prompt item *and* a `required` rule**, and
        neither half alone does it — `validateForm` treats `""` as empty, so the prompt
        item is what makes "chose nothing" invalid. `defaultItemPos` and `isReadOnly`
        are deliberately not set: a variant choice has no sensible default, and
        pre-selecting one would silently bias what gets authored.
        """
        concrete: list[str] = []
        for target in targets:
            concrete.extend(self.c.variants(target))
        concrete = [c for c in concrete if c in self.c.defs]
        if not concrete:
            raise ProjectionError(f"{key}: nothing concrete to project")

        if len(concrete) == 1:
            rows, nested = self.fields_for(concrete[0], key)
            state = self.add(key, f"{title} ({concrete[0]})", rows)
            return self.chain([_Block(state, [state])] + self.nested_blocks(nested))

        if disc is None:
            raise ProjectionError(f"{key}: a union with no discriminator to choose from")
        mapping = {literal: ref.split("/")[-1] for literal, ref in disc["mapping"].items()}
        choice_key = _identifier(key, disc["propertyName"])
        chooser = self.add(
            key,
            f"{title}: choose which kind",
            [[{
                "field": "dropdown",
                "key": choice_key,
                "label": title,
                "items": [{"value": "", "label": f"Select {title.lower()}…"}]
                + [{"value": literal, "label": _label(literal)} for literal in sorted(mapping)],
                "rules": [{"rule": "required", "message": f"Choose {title.lower()}"}],
            }]],
        )
        choices: list[dict] = []
        exits: list[str] = []
        for literal in sorted(mapping):
            variant = mapping[literal]
            if variant not in self.c.defs:
                continue
            branch_key = _identifier(key, literal)
            rows, nested = self.fields_for(variant, branch_key)
            branch = self.add(branch_key, f"{title} — {_label(literal)}", rows)
            run = self.chain([_Block(branch, [branch])] + self.nested_blocks(nested))
            choices.append({"when": {"op": "equals", "key": choice_key, "value": literal},
                            "nextState": run.entry})
            exits.extend(run.exits)
        if not choices:
            raise ProjectionError(f"{key}: a union whose variants are all unknown")
        self.states[chooser]["choices"] = choices
        return _Block(chooser, exits)

    def nested_blocks(self, nested: list[tuple[str, dict, str]]) -> list[_Block]:
        blocks: list[_Block] = []
        for key, psch, label in nested:
            sch = _unwrap(psch)
            _, targets = self.c.classify(psch)
            disc = sch.get("discriminator")
            if disc is None and len(targets) == 1 and self.c.is_union(targets[0]):
                disc = self.c.defs[targets[0]].get("discriminator")
            blocks.append(self.project_value(key, targets, disc, label))
        return blocks


class Projection:
    """The two documents, and what the projection would not do."""

    def __init__(self, key: str, flow: dict, forms: dict, notes: list[str]) -> None:
        self.key = key
        self.flow = flow
        self.forms = forms
        self.notes = notes

    @property
    def state_count(self) -> int:
        return len(self.flow["states"])

    def longest_walk(self) -> int:
        """States an author actually visits on the longest single path.

        **The number that means "step-at-a-time", as against the document's size.** A
        variant chooser enumerates alternatives, so a flow's state count grows with what
        the author *could* choose while the walk grows with what they *must*.
        """
        states = self.flow["states"]
        memo: dict[str, int] = {}

        def depth(key: str, seen: frozenset[str]) -> int:
            if key in seen:
                return 0
            if key in memo:
                return memo[key]
            state = states[key]
            nexts = [c["nextState"] for c in state.get("choices", [])]
            if state.get("defaultNextState"):
                nexts.append(state["defaultNextState"])
            value = 1 + max((depth(n, seen | {key}) for n in nexts), default=0)
            memo[key] = value
            return value

        return depth(self.flow["startAtKey"], frozenset())

    def unreachable(self) -> list[str]:
        """States no transition reaches. A projected flow should have none.

        `validateFlow` reports one as a warning by default and as an error under
        `JETS_USERFLOW_STRICT_REACHABILITY`, so a flow that ships one is refused by a
        strict deployment and merely noisy elsewhere. Checked here so that the generator
        is what fails, rather than the deployment.
        """
        states = self.flow["states"]
        reached = {self.flow["startAtKey"]}
        frontier = [self.flow["startAtKey"]]
        while frontier:
            state = states[frontier.pop()]
            nexts = [c["nextState"] for c in state.get("choices", [])]
            if state.get("defaultNextState"):
                nexts.append(state["defaultNextState"])
            for key in nexts:
                if key not in reached:
                    reached.add(key)
                    frontier.append(key)
        return sorted(set(states) - reached)

    def write(self, directory: Path) -> list[Path]:
        directory.mkdir(parents=True, exist_ok=True)
        flow_path = directory / f"{self.key}.uf.json"
        form_path = directory / f"{self.key}.form.json"
        flow_path.write_text(json.dumps(self.flow, indent=2) + "\n")
        form_path.write_text(
            json.dumps({"schemaVersion": SCHEMA_VERSION, "forms": self.forms}, indent=2) + "\n")
        return [flow_path, form_path]


def project(template: Template, context: dict, schema: dict) -> Projection:
    """`template` + `bindings` → a `.uf.json` / `.form.json` pair.

    The bindings are an input rather than an output: they supply every repeat count
    (`expand.py:138` reads it from the context and never from the filler), so the shape
    of the flow is decided here and the author edits values within it.
    """
    contract = Contract(schema)
    fills = _Fills()
    expand(template, context, fills, None)

    projector = _Projector(contract, sorted({h.schema_ref for h in template.holes}))
    blocks: list[_Block] = []

    # 1. The scalar bindings, first — they are where the repeat counts came from, so an
    #    author meets the entities before the steps that configure them (M.1).
    scalars, objects = _walk_bindings(context)
    if scalars:
        rows = [[projector.value_field(
            path.split(".")[-1],
            {"type": "array"} if isinstance(value, list) else {"type": "string"},
            "scalar_list" if isinstance(value, list) else "scalar",
            _identifier("bindings", path),
            True,
        )] for path, value in scalars]
        for (path, _), row in zip(scalars, rows):
            row[0]["label"] = path
        blocks.append(_Block(
            projector.add("bindings", "The values this template is parameterised by", rows),
            ["bindings"]))
    for path in objects:
        projector.notes.append(
            f"no field: binding {path} is an object — a hole that was never declared (I-78), "
            f"not a case to accommodate")

    # 2. One state per fill, in the order the expander asks for them.
    for hole, index in fills.calls:
        key = _identifier(hole.name, index) if hole.repeat_over else _identifier(hole.name)
        title = f"{_label(hole.name)} {index + 1}" if hole.repeat_over else _label(hole.name)
        blocks.append(projector.project_value(
            key, [hole.schema_ref],
            contract.defs.get(hole.schema_ref, {}).get("discriminator"), title))

    if not blocks:
        raise ProjectionError(f"{template.name}: nothing to configure")

    run = projector.chain(blocks)
    for key in run.exits:
        state = projector.states[key]
        state.pop("defaultNextState", None)
        state["isEnd"] = True
        state["stateAction"] = APPLY_ACTION

    flow = {"schemaVersion": SCHEMA_VERSION, "startAtKey": run.entry, "states": projector.states}
    result = Projection(template.name, flow, projector.forms, projector.notes)
    stranded = result.unreachable()
    if stranded:
        raise ProjectionError(f"{template.name}: {len(stranded)} unreachable states, "
                              f"first {stranded[0]!r}")
    return result
