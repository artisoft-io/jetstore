"""The corpus: which `.pc.json` count, and how the matrix is measured against them.

**Only `workspaces/*/pipes_config/**` is the corpus.** The `.pc.json` under
`workspaces/*/data/` are notes and reference material for developers — JetStore never
loads them — and they preserve config shapes that have since been retired. Counting them
does not merely add noise, it manufactures contradictions: every apparent disagreement
between the validator and the corpus in the B.1 seed came from that directory.

The rule that follows is the one the plan already implies but never states: **the
validator wins.** It runs on every execution, so a shape it rejects cannot be in service.
A config that contradicts it is not evidence against it; it is evidence of its own age.
Corpus evidence is therefore strong for presence and weak for absence, and worth nothing
at all against the validator.

Counting is driven by the matrix rather than by a hand-written traversal: a field row
with a `ref_struct` says where a child of that type is found, and the child's own
discriminator says which row it is. So the matrix measures itself, and the reach of the
walk is exactly the coverage of the matrix — `unreachable` in the report below is the
worklist for B.2-B.6, not a bug.
"""

from __future__ import annotations

import json
from collections import Counter, defaultdict
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Iterator

from .matrix_schema import (
    ANY_TOKEN,
    NONE,
    VIRTUAL_PREFIX,
    Container,
    Matrix,
    TypeRow,
    variant_matches,
)

# `workspaces/<ws>/pipes_config/...`, at any depth below it - usi_ws and jets_ws keep a
# `test/` subdirectory, and those configs do run.
LIVE_PARENT = "pipes_config"

# Within the live corpus, a `test/` directory holds configs that exercise a feature
# rather than run a client's data. They are real configs and they must validate, so they
# stay in the corpus - but a field attested only there is weaker evidence than one a
# production pipeline depends on, and the review is ordered by evidence strength (R-1).
TEST_PARENT = "test"


def is_production(path: Path) -> bool:
    return TEST_PARENT not in path.parts

TypeKey = tuple[str, str]

# The document root. Hard-coded because `ComputePipesConfig` and `ConditionalPipeSpec`
# have no matrix rows yet; delete this and start the walk from ("ComputePipesConfig", "*")
# once B.2 adds them.
_ROOTS: list[tuple[str, TypeKey]] = [
    ("pipes_config", ("PipeSpec", "")),
    ("reducing_pipes_config", ("PipeSpec", "")),
    ("conditional_pipes_config", ("PipeSpec", "")),
]


def config_files(root: Path) -> tuple[list[Path], list[Path]]:
    """Split the workspaces' `.pc.json` into (live, retired)."""
    live: list[Path] = []
    retired: list[Path] = []
    for path in sorted((root / "workspaces").rglob("*.pc.json")):
        (live if LIVE_PARENT in path.parts else retired).append(path)
    return live, retired


def _root_pipes(doc: Any) -> Iterator[tuple[str, Any]]:
    """Yield every `PipeSpec` of a document, with its dot path."""
    if not isinstance(doc, dict):
        return
    for pipe_index, pipe in enumerate(doc.get("pipes_config") or []):
        yield f"pipes_config.{pipe_index}", pipe
    for group_index, group in enumerate(doc.get("reducing_pipes_config") or []):
        for pipe_index, pipe in enumerate(group or []):
            yield f"reducing_pipes_config.{group_index}.{pipe_index}", pipe
    for step_index, step in enumerate(doc.get("conditional_pipes_config") or []):
        if not isinstance(step, dict):
            continue
        for pipe_index, pipe in enumerate(step.get("pipes_config") or []):
            yield f"conditional_pipes_config.{step_index}.pipes_config.{pipe_index}", pipe


@dataclass
class Measurement:
    """What the corpus says, measured rather than remembered."""

    instances: Counter[TypeKey] = field(default_factory=Counter)
    field_counts: dict[TypeKey, Counter[str]] = field(
        default_factory=lambda: defaultdict(Counter)
    )
    prod_instances: Counter[TypeKey] = field(default_factory=Counter)
    prod_field_counts: dict[TypeKey, Counter[str]] = field(
        default_factory=lambda: defaultdict(Counter)
    )
    exemplars: dict[TypeKey, tuple[str, str]] = field(default_factory=dict)
    # Set when the exemplar comes from a production config; a test one is only kept
    # until a production occurrence turns up.
    exemplar_is_prod: dict[TypeKey, bool] = field(default_factory=dict)
    # Keys seen on a typed node that no field row accounts for: candidates for a missing
    # row, or - once the matrix is complete - dead keys, which is the B.14 audit.
    unknown_keys: dict[TypeKey, Counter[str]] = field(
        default_factory=lambda: defaultdict(Counter)
    )
    # (ref_struct, discriminator value) pairs the matrix cannot resolve: the worklist.
    unreachable: Counter[tuple[str, str]] = field(default_factory=Counter)
    files: int = 0


class _Walker:
    def __init__(self, matrix: Matrix) -> None:
        self.by_key: dict[TypeKey, TypeRow] = {
            (t.go_struct, t.type_token): t for t in matrix.types
        }
        self.by_struct: dict[str, list[TypeRow]] = defaultdict(list)
        for t in matrix.types:
            self.by_struct[t.go_struct].append(t)
        self.fields: dict[TypeKey, list] = defaultdict(list)
        for f in matrix.fields_:
            self.fields[(f.go_struct, f.type_token)].append(f)
        self.out = Measurement()

    def discriminator_default(self, ref_struct: str, discriminator: str) -> str | None:
        """The default of a struct's discriminator, as recorded on its own field row.

        The rows of one struct must agree on it, since it decides which row an untagged
        node belongs to; disagreement is reported rather than resolved.
        """
        seen = {
            row.default
            for t in self.by_struct.get(ref_struct, [])
            for row in self.fields[(t.go_struct, t.type_token)]
            if row.json_key == discriminator and row.default != NONE
        }
        if len(seen) > 1:
            self.out.unreachable[(ref_struct, f"!ambiguous default {sorted(seen)}")] += 1
            return None
        return seen.pop() if seen else None

    def resolve(self, ref_struct: str, node: Any) -> TypeKey | None:
        """Which row is this node? The struct plus, when it discriminates, its token.

        Virtual tokens are tested first, in their row order, before the discriminator
        value is consulted - the same dispatch order as the code they describe
        (eval_expression.go:208 tests `arg`, then `lhs`, then `type`), which is why
        the file order of a struct's virtual rows is significant.
        """
        rows = self.by_struct.get(ref_struct)
        if not rows:
            self.out.unreachable[(ref_struct, "?")] += 1
            return None
        if isinstance(node, dict):
            for t in rows:
                if t.type_token.startswith(VIRTUAL_PREFIX) and variant_matches(
                    t.variant_when, node
                ):
                    return (ref_struct, t.type_token)
        value_rows = [
            t for t in rows if not t.type_token.startswith(VIRTUAL_PREFIX)
        ]
        if not value_rows:
            self.out.unreachable[(ref_struct, "!no value token matched")] += 1
            return None
        if len(value_rows) == 1 and value_rows[0].type_token == ANY_TOKEN:
            return (ref_struct, ANY_TOKEN)
        discriminator = value_rows[0].discriminator
        token = node.get(discriminator) if isinstance(node, dict) else None
        if token is None:
            # A discriminator with a default is usually absent from the config: not one of
            # the 90 live splitter_config nodes writes `type`, and all of them mean
            # `standard`. Reading the token off the wire alone would leave them untyped, so
            # fall back to the default the matrix already records on the field row.
            token = self.discriminator_default(ref_struct, discriminator)
        key = (ref_struct, str(token))
        if key not in self.by_key:
            self.out.unreachable[(ref_struct, str(token))] += 1
            return None
        return key

    def visit(self, key: TypeKey, node: Any, rel: str, path: str, prod: bool) -> None:
        if not isinstance(node, dict):
            return
        self.out.instances[key] += 1
        if prod:
            self.out.prod_instances[key] += 1
        # Prefer a production exemplar: the fragment library is a catalogue of parts to
        # imitate, and a test config is a weaker thing to hold up as the example.
        if key not in self.out.exemplars or (prod and not self.out.exemplar_is_prod[key]):
            # An empty path means the node *is* the document, which is the root type's
            # case. It is written as `-`, since a cell is never left empty.
            self.out.exemplars[key] = (rel, path or NONE)
            self.out.exemplar_is_prod[key] = prod

        accounted: set[str] = set()
        for row in self.fields[key]:
            accounted.add(row.json_key)
            if row.json_key not in node:
                continue
            self.out.field_counts[key][row.json_key] += 1
            if prod:
                self.out.prod_field_counts[key][row.json_key] += 1
            if row.ref_struct == NONE:
                continue
            value = node[row.json_key]
            for child, child_path in self._children(row.container, value, row.json_key):
                child_key = self.resolve(row.ref_struct, child)
                if child_key is not None:
                    # `path` is empty at the document root, where a dotted join would
                    # produce a leading dot that resolves against nothing.
                    below = f"{path}.{child_path}" if path else child_path
                    self.visit(child_key, child, rel, below, prod)
        for seen in node:
            if seen not in accounted:
                self.out.unknown_keys[key][seen] += 1

    @staticmethod
    def _children(
        container: Container, value: Any, key: str
    ) -> Iterator[tuple[Any, str]]:
        if container is Container.OBJECT:
            yield value, key
        elif container is Container.ARRAY:
            for i, item in enumerate(value or []):
                yield item, f"{key}.{i}"
        elif container is Container.ARRAY2:
            for i, group in enumerate(value or []):
                for j, item in enumerate(group or []):
                    yield item, f"{key}.{i}.{j}"
        elif container is Container.MAP:
            for name, item in (value or {}).items():
                yield item, f"{key}.{name}"


def measure(matrix: Matrix, root: Path) -> Measurement:
    """Walk the live corpus, counting what the matrix can reach."""
    walker = _Walker(matrix)
    live, _ = config_files(root)
    walker.out.files = len(live)
    for path in live:
        rel = path.relative_to(root).as_posix()
        prod = is_production(path)
        doc = json.loads(path.read_text(encoding="utf-8"))
        for pipe_path, pipe in _root_pipes(doc):
            key = walker.resolve("PipeSpec", pipe)
            if key is not None:
                walker.visit(key, pipe, rel, pipe_path, prod)
    return walker.out


def drift(matrix: Matrix, measurement: Measurement) -> list[str]:
    """Where the recorded numbers disagree with the measured ones."""
    problems: list[str] = []
    for t in matrix.types:
        key = (t.go_struct, t.type_token)
        measured_prod = measurement.prod_instances.get(key, 0)
        if t.corpus_prod_instances != measured_prod:
            problems.append(
                f"types {t.go_struct}/{t.type_token}: corpus_prod_instances "
                f"{t.corpus_prod_instances}, measured {measured_prod}"
            )
        measured = measurement.instances.get(key, 0)
        if t.corpus_instances != measured:
            problems.append(
                f"types {t.go_struct}/{t.type_token}: corpus_instances "
                f"{t.corpus_instances}, measured {measured}"
            )
        exemplar = measurement.exemplars.get(key)
        if exemplar is not None and LIVE_PARENT not in Path(t.exemplar_file).parts:
            problems.append(
                f"types {t.go_struct}/{t.type_token}: exemplar is not a live config: "
                f"{t.exemplar_file}"
            )
    for f in matrix.fields_:
        key = (f.go_struct, f.type_token)
        if key not in measurement.instances:
            continue  # the walk never reached this type; nothing measured to compare
        measured_prod = measurement.prod_field_counts[key].get(f.json_key, 0)
        if f.corpus_prod_count != measured_prod:
            problems.append(
                f"fields {f.go_struct}/{f.type_token}.{f.json_key}: corpus_prod_count "
                f"{f.corpus_prod_count}, measured {measured_prod}"
            )
        measured = measurement.field_counts[key].get(f.json_key, 0)
        if f.corpus_count != measured:
            problems.append(
                f"fields {f.go_struct}/{f.type_token}.{f.json_key}: corpus_count "
                f"{f.corpus_count}, measured {measured}"
            )
    return problems


def apply(matrix: Matrix, measurement: Measurement) -> None:
    """Write the measured counts and a live exemplar back onto the rows.

    Counting by hand is how the seed acquired both its bad exemplars and its wrong
    totals. The numbers are measured or they are not written.
    """
    for t in matrix.types:
        key = (t.go_struct, t.type_token)
        t.corpus_instances = measurement.instances.get(key, 0)
        t.corpus_prod_instances = measurement.prod_instances.get(key, 0)
        exemplar = measurement.exemplars.get(key)
        if exemplar is not None:
            t.exemplar_file, t.exemplar_path = exemplar
        # When the walk found none, a hand-proposed exemplar is left standing rather
        # than erased: while extraction is partial the walk cannot reach every type,
        # and the proposal is still verified by `check --corpus`. Once the type
        # becomes reachable the measured exemplar replaces it.
    for f in matrix.fields_:
        key = (f.go_struct, f.type_token)
        if key in measurement.instances:
            f.corpus_count = measurement.field_counts[key].get(f.json_key, 0)
            f.corpus_prod_count = measurement.prod_field_counts[key].get(f.json_key, 0)
