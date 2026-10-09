"""The one place this package reaches JetStore's model of the `.pc.json`.

`tools/cpipes_contract/cpipes_model.py` is 1,740 lines of Pydantic v2 covering
the whole document, it is the *source of truth* for the contract claims since
the B.10 flip, and `cpipes-contract reflect --check` is the guard that keeps it
honest against the matrix. **This package does not re-derive any of that.** A
second schema here would be two readers of one rule, which is the failure mode
the whole contract effort exists to remove.

Two things had to be worked around, and both are recorded rather than absorbed.

**`cpipes_model` is not importable.** It is a sibling *file* of the
`cpipes_contract` package, not a module inside it, and the wheel build declares
`packages = ["cpipes_contract"]`, so an installed `cpipes_contract` does not
carry it at all — `cpipes_contract.reflect` itself loads it through
`importlib.util.spec_from_file_location`. So does this module, and
`_load_contract_model` states the search order in its error. **The fix belongs
on the other side of the seam**: moving the file into the package costs one
default path in `cpipes_contract/main.py`, and until that happens a container
image for this node (P9-T14) must ship the contract tree and not only its
wheel.

**The model described the authored document and the node reads the runtime
one; that half is closed upstream as of 2026-09-21.** `pipes_model.go`'s
`ComputePipesConfig` carries fifteen json tags and the contract's
`ComputePipesConfig` carries thirteen: `common_runtime_args` and `pipes_config`
are absent from it, and they are precisely the two a *starter* fills in. Both
starters build the literal that becomes `cpipes_config_json` with
`CommonRuntimeArgs` set and `PipesConfig` set to the step's pipes —
`actions_start_reducing_cp.go` and `actions_start_sharding_cp.go` alike — and
`CoordinateComputePipes` reads both. With `extra="forbid"` on every class the
consequence was flat: **the contract model refused every document a node has
ever been given.** That was invisible to `cpipes-contract check --corpus`,
whose corpus is `pipes_config/**` — authored documents, every one of which
validates.

**The repair is a second projection in the contract rather than a wider first
one**, and `PipesConfig` below is now an alias for it. The two fields stay
`applicable=no` in the matrix and stay out of the emitted schema, because an
authored schema admitting a runtime document is what `negative_suite.json`'s
*root pipes_config (I-14 runtime shape)* case exists to refuse. So the
thirteen-against-fifteen difference was never the defect on its own; the
absence of any class on the runtime side of it was. `tests_config.py` derives
the pair by parsing the Go struct's tags rather than repeating this paragraph.
(`jetstore_maintenance_01` Phase 1 `AD`, closing `healthcare_corpus`'s
`P9-I28`, read 2026-09-20.)

**The union omitted the site operator, and that half is closed upstream as of
2026-09-19.** `TransformationSpecSite` existed in the model, carried the right
fields and was keyed `("TransformationSpec", "~site")` in `_MATRIX_KEYS`, and
the emitted `TransformationSpec` union carried only the nineteen *real* tokens
— so `ComputePipesConfig.model_validate` refused any document containing a
site operator. This module carried a three-class widening for it
(`FanOutWithSite`, `SplitterWithSite`, `ConditionalPipeSpecWithSite`), which
was the "second reader of one rule" the first paragraph refuses, and it is
**deleted** rather than kept: `generate.emit_union_alias` now emits the
complement branch into the alias itself, so every carrier of a transformation
list gets the branch by being generated rather than by being listed here. The
widening's own retirement test — `tests_config
.test_the_contract_model_refuses_a_site_operator` — is what went red and said
which classes to delete, the same shape as `_load_contract_model`'s guard.

**What the upstream repair adds beyond the union is the part to carry
forward.** `TransformationSpecSite.type` now refuses a built-in token at
*validation* rather than only in `json_schema_extra`, so the residual hazard
of the local widening — a malformed built-in failing its own branch and
arriving as an unknown site operator — is unrepresentable: such a document
fails both branches and is reported against its own. `config.parse_config`
carried a post-walk refusal for exactly that hazard and it is deleted with the
widening, because a check that cannot fire reports a clean result over a
subject it can no longer see.
"""

from __future__ import annotations

import importlib.util
import sys
import typing
from pathlib import Path
from types import ModuleType
from typing import Any

from .errors import ContractModelNotFound

# The struct names in `_MATRIX_KEYS` whose tokens this node dispatches on, and
# the kind each one is. Three entries, and they are a *kind assignment* rather
# than a token list: the tokens themselves are read off the model. Adding a
# nineteenth transformation to JetStore moves `contract_tokens` with nothing
# here edited, which is the whole point.
TOKEN_STRUCTS: dict[str, str] = {
    "InputChannelConfig": "input_channel",
    "PipeSpec": "pipe",
    "TransformationSpec": "transformation",
}

# A `~` prefix marks a virtual token in the matrix: `~override` is selected by
# the discriminator being absent and `~site` by its value being one no row
# claims. Neither is a token an author writes, so neither is part of the
# universe the scope gate is measured against.
VIRTUAL_TOKEN_PREFIX = "~"


def _load_contract_model() -> ModuleType:
    """Return the `cpipes_model` module, by import if that ever works and by
    path otherwise.

    The import is tried first and is expected to fail. When it stops failing,
    `tests_config.test_the_contract_model_is_still_not_importable` goes red and
    the fallback below can be deleted — a guard that retires itself, rather
    than a workaround that outlives its cause.
    """
    if "cpipes_model" in sys.modules:
        return sys.modules["cpipes_model"]
    try:  # pragma: no cover - the branch that is expected not to be taken
        import cpipes_model  # type: ignore[import-not-found]

        return cpipes_model
    except ImportError:
        pass

    searched: list[Path] = []
    try:
        import cpipes_contract

        pkg = Path(cpipes_contract.__file__ or "").resolve().parent
        searched.append(pkg.parent / "cpipes_model.py")
    except ImportError:
        pass
    # The worktree layout, for a source checkout that installed nothing.
    searched.append(
        Path(__file__).resolve().parents[2] / "cpipes_contract" / "cpipes_model.py"
    )

    for candidate in searched:
        if candidate.is_file():
            spec = importlib.util.spec_from_file_location("cpipes_model", candidate)
            if spec is None or spec.loader is None:  # pragma: no cover
                continue
            module = importlib.util.module_from_spec(spec)
            # Registered before execution: the module's own `model_rebuild()`
            # pass resolves forward references through `sys.modules`.
            sys.modules["cpipes_model"] = module
            spec.loader.exec_module(module)
            return module

    raise ContractModelNotFound(
        "cannot locate cpipes_model.py, the contract's Pydantic model.\n"
        "  tried: import cpipes_model\n"
        + "".join(f"  tried: {p}\n" for p in searched)
        + "  It is a sibling file of the cpipes_contract package rather than a\n"
        "  module inside it, so an installed wheel does not carry it. Install\n"
        "  cpipes_contract from its source tree, or point PYTHONPATH at\n"
        "  tools/cpipes_contract."
    )


model = _load_contract_model()

ComputePipesConfig = model.ComputePipesConfig
ComputePipesCommonArgs = model.ComputePipesCommonArgs
PipeSpec = model.PipeSpec
TransformationSpec = model.TransformationSpec
TransformationSpecSite = model.TransformationSpecSite
SchemaProviderSpecDefault = model.SchemaProviderSpecDefault


# --- the widenings, both retired --------------------------------------------
#
# Nothing here widens the contract any more. The site widening went the way it
# was designed to go: its guard turned red, named the three classes, and they
# were deleted. **The runtime-fields widening did not, and that is worth the
# paragraph.** Its guard —
# `tests_config.test_the_runtime_only_fields_are_exactly_the_two_widened` —
# compared `PipesConfig.model_fields` against `ComputePipesConfig.model_fields`
# and asserted the difference was the two. That difference is the same whether
# `PipesConfig` is a subclass declared here or the contract's own
# `ComputePipesRuntimeConfig`, so the guard stayed green across the very change
# it was written to detect: **it pinned the difference and the thing that moved
# was the ownership.** Retired guards are written against a *cause*, and this
# one was written against a *symptom* that the repair preserves on purpose.
#
# So it is renamed and given the assertion it was missing — that the runtime
# model is the contract's and this package declares no subclass of it — rather
# than deleted, because a widening retired on an untested premise is a widening
# that comes back.

#: The document a node is actually handed. An **alias**, deliberately: the name
#: is what every call site in this package spells, so the move out of here cost
#: no call site and the next reader finds the shape under the name they already
#: know. The current step's pipes arrive under `pipes_config` and not under
#: `conditional_pipes_config`, which is why `when` on a *step* is the starter's
#: to evaluate and `when` on an *operator* is the node's (P9-T04).
PipesConfig = model.ComputePipesRuntimeConfig


# Empty, and kept rather than deleted: it is the inventory a future widening
# would join, and an empty tuple states *none* where a deleted name states
# nothing. **Nothing reads it today** — the comment it carried claimed a test
# did, and no test ever has, which is recorded rather than quietly repaired
# because a comment asserting a check that does not exist is exactly what this
# package spends its own tests refusing.
WIDENED_CLASSES: tuple[type, ...] = ()


def builtin_transformation_members() -> tuple[type, ...]:
    """The tagged members of the transformation union — the built-ins.

    The union is `Annotated[Union[Annotated[Union[<19 tagged>], ...],
    TransformationSpecSite], ...]` since the complement branch joined it, so
    reaching the built-ins is two unwrappings rather than one. It is here
    rather than in the one test that wants it because this module is the one
    place that reaches the model's shape, and a second unwrap written in a test
    is a second reader of that shape — which is what went red when the alias
    gained its branch.
    """
    outer = typing.get_args(typing.get_args(TransformationSpec)[0])
    tagged = [m for m in outer if m is not TransformationSpecSite]
    if len(tagged) != 1:  # pragma: no cover - the alias stopped being widened
        return tuple(outer)
    return tuple(typing.get_args(typing.get_args(tagged[0])[0]))


def contract_tokens(kind: str) -> tuple[str, ...]:
    """The tokens JetStore's contract knows for one kind, sorted.

    Read off `_MATRIX_KEYS`, which is the model's own class → (struct, token)
    index. This is the universe the declared scope is a subset of, and the set
    `tests_scope.py` asserts every non-declared member of aborts.
    """
    keys: dict[str, tuple[str, str]] = model._MATRIX_KEYS
    return tuple(
        sorted(
            token
            for _cls, (struct, token) in keys.items()
            if TOKEN_STRUCTS.get(struct) == kind
            and not token.startswith(VIRTUAL_TOKEN_PREFIX)
        )
    )


def contract_token_census() -> dict[str, tuple[str, ...]]:
    """Every kind's tokens, for the one place a count is asserted."""
    return {kind: contract_tokens(kind) for kind in sorted(set(TOKEN_STRUCTS.values()))}


def column_types() -> tuple[str, ...]:
    """Every `TransformationColumnSpec` type the contract declares, sorted.

    Not a `TokenKind`: a column type is not something the node *dispatches* on
    the way it dispatches an operator, so it has no place in `TOKEN_STRUCTS`.
    It is read off the same index for the same reason — `columns.py` implements
    a subset and refuses the rest, and the universe it is a subset **of** has to
    be the contract's own or the two lists can drift with nothing going red
    (P3-I20).
    """
    keys: dict[str, tuple[str, str]] = model._MATRIX_KEYS
    return tuple(
        sorted(
            token
            for _cls, (struct, token) in keys.items()
            if struct == "TransformationColumnSpec"
            and not token.startswith(VIRTUAL_TOKEN_PREFIX)
        )
    )


def spec_kind(obj: Any) -> str | None:
    """The kind of a validated model instance, or None if it names no token.

    Derived from `_MATRIX_KEYS` by class name, so the walk in `config.py` has
    no list of field paths to go stale: it walks *objects* and asks each one
    what it is. The lookup still climbs the MRO: this package subclasses nothing
    now, but `ComputePipesRuntimeConfig` is itself a subclass of a class the
    matrix keys, so the climb is what makes a runtime document answer at all.
    """
    keys: dict[str, tuple[str, str]] = model._MATRIX_KEYS
    for cls in type(obj).__mro__:
        entry = keys.get(cls.__name__)
        if entry is not None:
            return TOKEN_STRUCTS.get(entry[0])
    return None
