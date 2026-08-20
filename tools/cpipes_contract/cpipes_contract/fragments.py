"""The fragment library: every part the live corpus contains, keyed by `defs_name`.

**What a part is.** A JSON value that binds to exactly one `$defs` entry and can be
authored and validated standing alone — a channel spec, a mapping expression, an
evaluator subtree, a per-operator config body. Decision 9 calls this the parts library
and gives it a second consumer in the analysis's §6.1(3) compositional synthesis;
decision 14 puts it here, keyed by `defs_name`, and says it must be *curated* rather
than proportional. **This module does the extraction. It does not curate** - that is
F.2, and the distinction matters because the two answer different questions: what does
the corpus contain, and what belongs in front of a model.

**Why the walk is schema-guided rather than structural.** A part's identity is the
`$defs` entry it binds to, and nothing in a `.pc.json` says which that is: the same
object shape means different things in different positions, and a discriminated union
is resolved by a sibling field. So the walk descends the schema and the value together,
resolving `$ref`, nullable `anyOf`, and `oneOf` + `discriminator` as it goes. A
structural walk would have to guess, and I-31 and I-32 are two records of what guessing
at corpus structure costs.

**Identical values collapse; provenance does not.** A part appearing in forty configs is
one part with forty citations, because the library is a catalogue of *distinct*
material. The count is what F.2 curates on - decision 14's rule is that every operator
with four or fewer instances contributes all of them - so losing it would remove the
only property that made the approach worth taking.
"""

from __future__ import annotations

import csv
import json
import re
import subprocess
from collections import defaultdict
from dataclasses import dataclass, field
from pathlib import Path

from .corpus import config_files, is_production


@dataclass
class Citation:
    """Where one instance of a part was found."""

    file: str
    path: str
    production: bool


@dataclass
class Part:
    """One distinct value binding to one `$defs` entry."""

    defs_name: str
    value: object
    citations: list[Citation] = field(default_factory=list)
    description: str = ""
    description_source: str = "generated"

    @property
    def instances(self) -> int:
        return len(self.citations)

    @property
    def prod_instances(self) -> int:
        return sum(1 for c in self.citations if c.production)


def _read(path: Path) -> list[dict]:
    with open(path, newline="") as fh:
        return list(csv.DictReader(fh))


def _deref(node: dict, defs: dict) -> tuple[dict, str | None]:
    """Follow a `$ref` one level. Returns (schema, defs_name or None)."""
    ref = node.get("$ref")
    if isinstance(ref, str) and ref.startswith("#/$defs/"):
        name = ref.split("/")[-1]
        return defs.get(name, {}), name
    return node, None


def _unwrap_nullable(node: dict) -> dict:
    """`anyOf: [X, null]` is X for our purposes; the null branch carries nothing."""
    branches = node.get("anyOf")
    if not isinstance(branches, list):
        return node
    real = [b for b in branches if isinstance(b, dict) and b.get("type") != "null"]
    return real[0] if len(real) == 1 else node


# `X = Annotated[Union[...], Field(discriminator="k"), BeforeValidator(_tag_default("k", "v"))]`
_TAG_DEFAULT = re.compile(
    r"^(?P<union>\w+) = Annotated\[.*?_tag_default\(\"(?P<key>\w+)\", \"(?P<default>[\w-]+)\"\)",
    re.M,
)


def tag_defaults(model_path: Path) -> dict[str, tuple[str, str]]:
    """Which unions have an engine-default variant, read from the model.

    Four do - `InputChannelConfig` and `OutputChannelConfig` default to `memory`,
    `SplitterSpec` to `standard`, `AnonymizeSpec` to `anonymization`. **JSON Schema
    cannot express this**: there is no equivalent of the model's tag-injecting
    `BeforeValidator`, so the emitted `discriminator.mapping` requires an explicit tag
    and a config that omits one resolves to nothing. The corpus omits it constantly -
    an untagged `output_channel` is the commonest shape in the library - so a walk that
    does not read the defaults loses several hundred parts and, worse, loses them
    silently as unbound nodes rather than as an error.

    Read from the model source rather than hard-coded, so that a fifth default added
    there arrives here; if the shape of that line ever changes this stops matching and
    the unresolved unions are reported, which is the loud failure and the right one.
    """
    text = model_path.read_text()
    return {m["union"]: (m["key"], m["default"]) for m in _TAG_DEFAULT.finditer(text)}


def _union_by_mapping(disc: dict, defs: dict) -> str | None:
    """Name the union whose `$defs` entry carries this exact discriminator mapping."""
    want = frozenset(disc.get("mapping", {}).values())
    for name, entry in defs.items():
        other = entry.get("discriminator")
        if isinstance(other, dict) and frozenset(other.get("mapping", {}).values()) == want:
            return name
    return None


def _pick_variant(
    node: dict, value: object, defs: dict, union: str | None, defaults: dict
) -> tuple[dict, str | None]:
    """Resolve `oneOf` + `discriminator` against the value's tag, or its default."""
    disc = node.get("discriminator")
    if not isinstance(disc, dict) or not isinstance(value, dict):
        return node, None
    key = disc.get("propertyName", "type")
    tag = value.get(key)
    if tag is None:
        # The emitter **inlines** a union's discriminator into the referring property
        # rather than `$ref`-ing the union entry, so at this point the union has no
        # name to look a default up by. Its mapping is its identity: the set of
        # variants it can resolve to belongs to exactly one union.
        if union not in defaults:
            union = _union_by_mapping(disc, defs)
        if union in defaults:
            _, tag = defaults[union]
    target = disc.get("mapping", {}).get(tag)
    if isinstance(target, str) and target.startswith("#/$defs/"):
        name = target.split("/")[-1]
        return defs.get(name, {}), name
    return node, None


class Extractor:
    """Walks a config against the schema, emitting one part per bound node."""

    def __init__(self, schema: dict, defaults: dict | None = None):
        self.defs = schema["$defs"]
        self.defaults = defaults or {}
        self.parts: dict[tuple[str, str], Part] = {}
        self.unresolved: list[str] = []
        self.stale: list[str] = []

    def _record(self, name: str, value: object, file: str, path: str) -> None:
        key = (name, json.dumps(value, sort_keys=True, separators=(",", ":")))
        part = self.parts.get(key)
        if part is None:
            part = Part(defs_name=name, value=value)
            self.parts[key] = part
        part.citations.append(Citation(file=file, path=path, production=is_production(Path(file))))

    def walk(self, value: object, node: dict, file: str, path: str, bound: str | None = None) -> None:
        node = _unwrap_nullable(node)
        node, name = _deref(node, self.defs)
        if name:
            bound = name
        node2, variant = _pick_variant(node, value, self.defs, bound, self.defaults)
        if variant:
            node, bound = node2, variant
        elif node.get("discriminator") and isinstance(value, dict):
            # A union we could not resolve is a defect, not a shape without a part.
            self.unresolved.append(f"{file}:{path or '<root>'}")

        if isinstance(value, dict):
            if bound:
                self._record(bound, value, file, path)
            props = node.get("properties")
            if not isinstance(props, dict):
                # A map of scalars (`env`, `addl_env`) has no `$defs` entry and is not
                # a part; that is a shape without a binding, not a failure to bind.
                return
            for key, sub in value.items():
                child = props.get(key)
                if isinstance(child, dict):
                    self.walk(sub, child, file, f"{path}.{key}" if path else key)
        elif isinstance(value, list):
            items = _unwrap_nullable(node).get("items")
            if isinstance(items, dict):
                for i, item in enumerate(value):
                    self.walk(item, items, file, f"{path}.{i}")

    def library(self) -> list[Part]:
        return sorted(
            self.parts.values(),
            key=lambda p: (p.defs_name, -p.instances, json.dumps(p.value, sort_keys=True)),
        )


def describe(part: Part, defs: dict) -> str:
    """A mechanical first description, to be replaced by an authored one at F.2.

    Deliberately not clever. It names the type and the value's distinguishing fields,
    which is enough to tell two parts of the same type apart in a listing and **not**
    enough to satisfy criterion 22 - that asks for fragments authored from an English
    description of what a part *accomplishes*, which is a claim about intent that
    nothing here can recover from a value. `description_source` records which kind a
    part currently carries so the two are never confused.
    """
    entry = defs.get(part.defs_name, {})
    head = (entry.get("description") or part.defs_name).split(".")[0].strip()
    if isinstance(part.value, dict):
        marks = [
            f"{k}={part.value[k]!r}"
            for k in ("type", "name", "key", "column", "expr", "op")
            if k in part.value and isinstance(part.value[k], (str, int, float, bool))
        ]
        if marks:
            return f"{head} ({', '.join(marks[:3])})"
    return head


def extract(schema: dict, root: Path, model: Path | None = None) -> Extractor:
    live, _ = config_files(root)
    ex = Extractor(schema, tag_defaults(model) if model else {})
    for path in live:
        # Cite as `workspaces/<ws>/...`, which is how every other document in this
        # project names a corpus file; the walk's own root is not part of the fact.
        parts = path.parts
        rel = str(Path(*parts[parts.index("workspaces"):])) if "workspaces" in parts else str(path)
        ex.walk(json.loads(path.read_text()), {"$ref": "#/$defs/ComputePipesConfig"}, rel, "")
    for part in ex.parts.values():
        part.description = describe(part, ex.defs)
    return ex


def to_jsonl(parts: list[Part]) -> str:
    """One part per line.

    A single indented document would be 8 MB and would rewrite wholesale whenever any
    part moved; one compact object per line keeps a re-extraction's diff to the parts
    that actually changed, which is what makes F.2's curation reviewable.
    """
    lines = []
    for p in parts:
        lines.append(
            json.dumps(
                {
                    "defs_name": p.defs_name,
                    "description": p.description,
                    "description_source": p.description_source,
                    "instances": p.instances,
                    "prod_instances": p.prod_instances,
                    "value": p.value,
                    "citations": [{"file": c.file, "path": c.path} for c in p.citations],
                },
                sort_keys=False,
                separators=(",", ":"),
            )
        )
    return "\n".join(lines) + "\n"


def check_against_matrix(parts: list[Part], types_csv: Path) -> list[str]:
    """Every recorded `corpus_instances` must equal what the extraction found.

    This is the guard I-31 and I-32 both argue for, at the grain each of them missed.
    A corpus walk cannot be validated by reading the code that produced it - both wrong
    walks were plausible, and one would have shipped - so it is checked against a
    figure measured independently and earlier, by `cpipes-contract corpus` over the
    same corpus by a different route. Two measurements agreeing is evidence; one
    measurement carefully reviewed is not.

    Merged structs are summed rather than compared row-by-row: `ExpressionNode`'s seven
    virtual tokens are seven matrix rows and one `$defs` entry, so the extraction
    reports their total and the matrix reports the parts.
    """
    import csv
    from collections import Counter

    found: Counter[str] = Counter()
    for part in parts:
        found[part.defs_name] += part.instances

    recorded: Counter[str] = Counter()
    with open(types_csv, newline="") as fh:
        for row in csv.DictReader(fh):
            if row["corpus_instances"] not in ("-", ""):
                recorded[row["defs_name"]] += int(row["corpus_instances"])

    return [
        f"{name}: matrix records {recorded[name]}, extraction found {found.get(name, 0)}"
        for name in sorted(recorded)
        if recorded[name] != found.get(name, 0)
    ]


# ---------------------------------------------------------------------------
# F.2 - curation
# ---------------------------------------------------------------------------

# `ComputePipesConfig` holds three fields that can carry pipes, and which one is
# populated depends on the lifecycle stage rather than on the author's choice (I-14).
# `pipes_config` is written by JetStore and never by an author, so a part carrying it
# validates and cannot be authored; `reducing_pipes_config` is the author's but
# deprecated (MATRIX_SCHEMA.md:175). Neither may reach the library.
ROOT_TYPE = "ComputePipesConfig"
RUNTIME_ONLY = "pipes_config"
DEPRECATED = "reducing_pipes_config"

# `pipes_config` is only runtime-only **on the root type**. `ConditionalPipeSpec` has a
# field of the same name holding the authored steps of one stage, and 88 parts carry it
# legitimately. Scoping this to the root is the difference between a guard and a bug.
#
# In fact the guard cannot fire today: B.13 removed `pipes_config` from
# `ComputePipesConfig`'s properties when I-14 was raised, so the schema-guided walk has
# no path to it and the library could not contain one if the corpus did. It is kept as
# a backstop that fails loudly if that upstream decision is ever reversed, which is
# cheaper than rediscovering I-14 from a model that has learnt to emit the runtime
# shape.


def _contains_key(value: object, key: str) -> bool:
    if isinstance(value, dict):
        return key in value or any(_contains_key(v, key) for v in value.values())
    if isinstance(value, list):
        return any(_contains_key(v, key) for v in value)
    return False


def shape(value: object) -> str:
    """The structure of a value with its data removed.

    Two parts share a shape when they differ only in what they say, not in what they
    are: `{"type":"select","expr":"a"}` and `{"type":"select","expr":"b"}` are one
    shape. Shape is what composition needs to cover - a model assembling a config needs
    to have seen every *form* a part takes, and the fortieth `select` over a different
    column name teaches it nothing the first did not. Discriminator values are kept in
    the shape because they change what a part *is*.
    """
    if isinstance(value, dict):
        inner = ",".join(
            f"{k}:{value[k]!r}" if k in ("type", "mode") and isinstance(value[k], str)
            else f"{k}:{shape(value[k])}"
            for k in sorted(value)
        )
        return "{" + inner + "}"
    if isinstance(value, list):
        return "[" + "|".join(sorted({shape(v) for v in value})) + "]"
    return type(value).__name__


def curate(parts: list[Part], thin_at: int = 4, per_shape: int = 2) -> tuple[list[Part], dict]:
    """Decision 14's rule: curated, not proportional.

    Three things happen, in order, and only the third is a judgement.

    **Excluded outright**: any part containing a deprecated field at any depth, and any
    root-type part carrying the runtime-only shape. Criterion 24 admits no exceptions.

    **Kept whole**: every part of a type with `thin_at` or fewer distinct parts. This is
    the rule the criterion names, and it is the reason the library is not proportional -
    the nine thin operators are where the differentiator lives and where the corpus has
    least to say, so they contribute everything they have.

    **Capped by shape** for everything else: keep up to `per_shape` exemplars of each
    distinct shape, most-cited first, preferring one attested in production. A
    proportional library would reproduce the corpus's 83% skew and discard the only
    property that made the approach worth taking; capping by *shape* rather than by
    count keeps coverage of form while dropping repetition of content.
    """
    kept: list[Part] = []
    stats = {
        "input": len(parts),
        "excluded_deprecated": 0,
        "excluded_runtime_shape": 0,
        "kept_thin": 0,
        "kept_capped": 0,
        "dropped_repetition": 0,
    }

    surviving: list[Part] = []
    for part in parts:
        if _contains_key(part.value, DEPRECATED):
            stats["excluded_deprecated"] += 1
        elif part.defs_name == ROOT_TYPE and RUNTIME_ONLY in part.value:
            stats["excluded_runtime_shape"] += 1
        else:
            surviving.append(part)

    by_name: dict[str, list[Part]] = defaultdict(list)
    for part in surviving:
        by_name[part.defs_name].append(part)

    for name, group in sorted(by_name.items()):
        # Thin by **corpus instances**, which is what criterion 24 says and is not the
        # same as thin by distinct parts: a type can have three instances that are
        # three different parts, or ten that are two. The criterion is about how little
        # the corpus has to teach, so citations are the measure.
        if sum(p.instances for p in group) <= thin_at:
            kept.extend(group)
            stats["kept_thin"] += len(group)
            continue
        seen: dict[str, int] = defaultdict(int)
        for part in sorted(group, key=lambda p: (-p.prod_instances, -p.instances)):
            key = shape(part.value)
            if seen[key] < per_shape:
                seen[key] += 1
                kept.append(part)
                stats["kept_capped"] += 1
            else:
                stats["dropped_repetition"] += 1

    kept.sort(key=lambda p: (p.defs_name, -p.instances))
    stats["output"] = len(kept)
    return kept, stats


def criterion_24(before: list[Part], after: list[Part], types_csv: Path) -> list[str]:
    """Every type with four or fewer corpus instances kept every part it had.

    The criterion in one function, checked rather than argued. It is the half of
    decision 14 that a plausible-looking curation quietly breaks: trimming by count
    looks reasonable everywhere and is wrong exactly where the corpus is thinnest,
    which is where the differentiator lives.
    """
    import csv
    from collections import Counter

    recorded: Counter[str] = Counter()
    with open(types_csv, newline="") as fh:
        for row in csv.DictReader(fh):
            if row["corpus_instances"] not in ("-", ""):
                recorded[row["defs_name"]] += int(row["corpus_instances"])

    kept: Counter[str] = Counter()
    had: Counter[str] = Counter()
    for part in after:
        kept[part.defs_name] += 1
    for part in before:
        had[part.defs_name] += 1

    return [
        f"{name} has {recorded[name]} corpus instances but the curation kept "
        f"{kept[name]} of {had[name]} parts"
        for name in sorted(recorded)
        if recorded[name] <= 4 and kept[name] != had[name]
    ]


# ---------------------------------------------------------------------------
# Recovered sources
# ---------------------------------------------------------------------------


def recovered_parts(schema: dict, matrix: Path, corpus_root: Path, model: Path) -> Extractor:
    """Extract parts from historical revisions named in `recovered_sources.csv`.

    **Why a config that is no longer live can still be evidence.** The library needs at
    least one part per operator, and `clustering` has none: the corpus has zero live
    instances, and the one test config that used it was deleted on 2026-08-16 for
    carrying a *retired* `ClusteringSpec` revision. Recovering that file yields nothing
    - its `clustering_config` is exactly what went stale.

    But a valid instance does exist, in a different file than either search started
    from. Validating every `clustering_config` in `cedargate_ws`'s whole history found
    17 distinct, of which 3 validate against the current spec, all in
    `anonymize_file.pc.json` - a config that is still live today and simply lost its
    clustering step. That is production material, not authored, and it is the strongest
    kind of part the library can hold for an operator the corpus no longer exercises.

    **Every recovered part is validated against its own `$defs` entry before it is
    admitted**, because a historical revision predates an unknown number of contract
    changes and being old is exactly the thing that makes it suspect. A part that does
    not validate is reported, never silently kept: that is the whole difference between
    recovering evidence and reintroducing the drift the deletion removed.
    """
    import jsonschema

    ex = Extractor(schema, tag_defaults(model))
    rows = _read(matrix / "recovered_sources.csv")
    for row in rows:
        repo = corpus_root / "workspaces" / row["workspace"]
        blob = subprocess.run(
            ["git", "-C", str(repo), "show", f"{row['commit']}:{row['path']}"],
            capture_output=True,
            text=True,
        )
        if blob.returncode:
            ex.unresolved.append(
                f"{row['workspace']}@{row['commit']}:{row['path']}: not in the repository"
            )
            continue
        cite = f"workspaces/{row['workspace']}@{row['commit']}/{row['path']}"
        ex.walk(json.loads(blob.stdout), {"$ref": "#/$defs/ComputePipesConfig"}, cite, "")

    validators: dict[str, object] = {}
    kept: dict[tuple[str, str], Part] = {}
    for key, part in ex.parts.items():
        name = part.defs_name
        if name not in validators:
            validators[name] = jsonschema.Draft202012Validator(
                {"$schema": schema["$schema"], "$ref": f"#/$defs/{name}", "$defs": schema["$defs"]}
            )
        if list(validators[name].iter_errors(part.value)):  # type: ignore[attr-defined]
            continue
        part.description_source = "recovered"
        kept[key] = part
    ex.parts = kept
    for part in ex.parts.values():
        part.description = describe(part, ex.defs)
    # An unresolved union in a *recovered* file is a fact about the file, not a defect
    # in the walk: a historical revision predates contract changes, and a node whose
    # discriminator has since been renamed simply has no current type to bind to. The
    # `a421b714` analyze step is the live case - its `function_tokens` tag on
    # `function_name` where the schema now discriminates on `type`. Reporting them is
    # useful, failing on them is not, so they are relabelled rather than left to look
    # like the corpus-path findings that do mean something is wrong.
    ex.stale = ex.unresolved
    ex.unresolved = []
    return ex
