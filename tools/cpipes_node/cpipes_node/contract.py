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

**The union omits the site operator.** `TransformationSpecSite` exists in the
model, carries the right fields and is keyed `("TransformationSpec", "~site")`
in `_MATRIX_KEYS` — and the emitted `TransformationSpec` union contains only
the nineteen *real* tokens, because `generate.py` builds the union from the
real rows and emits virtual tokens as free-standing classes. The consequence is
sharp and it is not this package's alone: **`ComputePipesConfig.model_validate`
refuses any document containing a site operator**, which is every document a
deployment with its own operator will ever author, this phase's included. The
widening below composes the contract's *own two classes* into the union the
contract does not emit; it invents no field and no shape. `tests_config.py`
derives the set of classes that must be widened from the model by reflection,
so a pipe kind that gains an `apply` list is not silently missed.
"""

from __future__ import annotations

import importlib.util
import sys
from pathlib import Path
from types import ModuleType
from typing import Annotated, Any

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
ConditionalPipeSpec = model.ConditionalPipeSpec
PipeSpecFanOut = model.PipeSpecFanOut
PipeSpecMergeFiles = model.PipeSpecMergeFiles
PipeSpecSplitter = model.PipeSpecSplitter
TransformationSpec = model.TransformationSpec
TransformationSpecSite = model.TransformationSpecSite
SchemaProviderSpecDefault = model.SchemaProviderSpecDefault


# --- the widening -----------------------------------------------------------
#
# `union_mode="left_to_right"` and not a discriminated union, because the two
# branches cannot share a discriminator: the left one is keyed on nineteen
# literals and the right one's key is `str`. Left-to-right is what makes a
# built-in token reach its own branch — `TransformationSpecSite.type` is a bare
# `str` and would otherwise swallow every token including the built-ins.
#
# The residual hazard is worth stating, because it is the price of the shim: a
# *malformed* built-in fails the left branch and can then satisfy the right
# one, arriving as a site operator under a built-in's name. `config.py` refuses
# that on arrival and re-raises the left branch's error, so the author is told
# their `map_record` is malformed rather than that their site operator is
# unknown.
#
# **Every `valid-type` ignore below is the first finding's cost, not a second
# one.** `cpipes_model` is loaded through `importlib` because it is not
# importable, so a static checker sees `TransformationSpec` as a module
# attribute rather than as a type. Pydantic resolves them at
# `model_rebuild()`, and `tests_config.py` exercises every one of these classes
# against the real documents — so the types are checked, by running rather than
# by reading. Moving `cpipes_model.py` into its package removes all four.
TransformationSpecOrSite = Annotated[
    TransformationSpec | TransformationSpecSite,  # type: ignore[valid-type]
    Field(union_mode="left_to_right"),
]


class FanOutWithSite(PipeSpecFanOut):  # type: ignore[misc, valid-type]
    apply: list[TransformationSpecOrSite] | None = Field(default=None)


class SplitterWithSite(PipeSpecSplitter):  # type: ignore[misc, valid-type]
    apply: list[TransformationSpecOrSite] | None = Field(default=None)


PipeSpecWithSite = Annotated[
    FanOutWithSite | PipeSpecMergeFiles | SplitterWithSite,  # type: ignore[valid-type]
    Field(discriminator="type"),
]


class ConditionalPipeSpecWithSite(ConditionalPipeSpec):  # type: ignore[misc, valid-type]
    pipes_config: list[PipeSpecWithSite] = Field()


class PipesConfig(ComputePipesConfig):  # type: ignore[misc, valid-type]
    """The document a node is actually handed.

    Three widenings, each composed from the contract's own classes: the site
    operator the transformation union omits, and the two fields a starter fills
    in that the authored-document model does not carry. Nothing else differs,
    and `tests_config.py` asserts that by validating all of JetStore's own
    `.pc.json` corpus through this class and through the contract's, requiring
    both to accept every one of them.
    """

    #: Filled by the starters; read by `CoordinateComputePipes` for the mode,
    #: the session, the step id and the file key.
    common_runtime_args: ComputePipesCommonArgs | None = Field(default=None)  # type: ignore[valid-type]
    #: The current step's pipes. The reducing starter writes this and not
    #: `conditional_pipes_config`, so a node's document has the steps already
    #: chosen — which is why `when` on a *step* is the starter's to evaluate
    #: and `when` on an *operator* is the node's (P9-T04).
    pipes_config: list[PipeSpecWithSite] | None = Field(default=None)
    conditional_pipes_config: list[ConditionalPipeSpecWithSite] | None = Field(
        default=None
    )
    reducing_pipes_config: list[list[PipeSpecWithSite]] | None = Field(default=None)


for _cls in (
    FanOutWithSite,
    SplitterWithSite,
    ConditionalPipeSpecWithSite,
    PipesConfig,
):
    _cls.model_rebuild(_types_namespace={**vars(model), **globals()})


# The classes above are the widened ones; the test that the set is complete
# reads this tuple and the model, never a sentence in a docstring.
WIDENED_CLASSES: tuple[type, ...] = (
    FanOutWithSite,
    SplitterWithSite,
    ConditionalPipeSpecWithSite,
    PipesConfig,
)


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


def spec_kind(obj: Any) -> str | None:
    """The kind of a validated model instance, or None if it names no token.

    Derived from `_MATRIX_KEYS` by class name, so the walk in `config.py` has
    no list of field paths to go stale: it walks *objects* and asks each one
    what it is. `FanOutWithSite` and `SplitterWithSite` answer through their
    bases, which is why the lookup climbs the MRO.
    """
    keys: dict[str, tuple[str, str]] = model._MATRIX_KEYS
    for cls in type(obj).__mro__:
        entry = keys.get(cls.__name__)
        if entry is not None:
            return TOKEN_STRUCTS.get(entry[0])
    return None
