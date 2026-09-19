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

**The model describes the authored document and the node reads the runtime
one.** `pipes_model.go`'s `ComputePipesConfig` carries fifteen json tags and
the generated `ComputePipesConfig` carries thirteen: `common_runtime_args` and
`pipes_config` are missing, and they are precisely the two a *starter* fills
in. `actions_start_reducing_cp.go` builds the literal that becomes
`cpipes_config_json` with `CommonRuntimeArgs` set and `PipesConfig` set to the
step's pipes, and `CoordinateComputePipes` reads both. With `extra="forbid"`
on every class, the consequence is flat: **the contract model refuses every
document a node has ever been given.** That is invisible to
`cpipes-contract check --corpus`, whose corpus is `pipes_config/**` — authored
documents, all 48 of which validate. The widening below adds the two fields
using `ComputePipesCommonArgs`, which the model already defines and nothing
references, and the pipe union. `tests_config.py` derives the pair by parsing
the Go struct's tags rather than repeating this paragraph.

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

from pydantic import Field

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


# --- the widening -----------------------------------------------------------
#
# One widening remains and it is the runtime-fields one. The `valid-type`
# ignores are the first finding's cost, not a second one: `cpipes_model` is
# loaded through `importlib` because it is not importable, so a static checker
# sees `PipeSpec` as a module attribute rather than as a type. Pydantic
# resolves them at `model_rebuild()`, and `tests_config.py` exercises this
# class against the real documents — so the types are checked, by running
# rather than by reading. Moving `cpipes_model.py` into its package removes
# them.
class PipesConfig(ComputePipesConfig):  # type: ignore[misc, valid-type]
    """The document a node is actually handed.

    One widening, composed from the contract's own classes: the two fields a
    starter fills in that the authored-document model does not carry. Nothing
    else differs, and `tests_config.py` asserts that by validating all of
    JetStore's own `.pc.json` corpus through this class and through the
    contract's, requiring both to accept every one of them.
    """

    #: Filled by the starters; read by `CoordinateComputePipes` for the mode,
    #: the session, the step id and the file key.
    common_runtime_args: ComputePipesCommonArgs | None = Field(default=None)  # type: ignore[valid-type]
    #: The current step's pipes. The reducing starter writes this and not
    #: `conditional_pipes_config`, so a node's document has the steps already
    #: chosen — which is why `when` on a *step* is the starter's to evaluate
    #: and `when` on an *operator* is the node's (P9-T04).
    pipes_config: list[PipeSpec] | None = Field(default=None)  # type: ignore[valid-type]


PipesConfig.model_rebuild(_types_namespace={**vars(model), **globals()})


# The class above is the widened one; the test that the set is complete reads
# this tuple and the model, never a sentence in a docstring.
WIDENED_CLASSES: tuple[type, ...] = (PipesConfig,)


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
    what it is. The lookup climbs the MRO because this package's own subclasses
    of contract classes — `PipesConfig` today, three more until the site
    widening retired — answer through their bases.
    """
    keys: dict[str, tuple[str, str]] = model._MATRIX_KEYS
    for cls in type(obj).__mro__:
        entry = keys.get(cls.__name__)
        if entry is not None:
            return TOKEN_STRUCTS.get(entry[0])
    return None
