"""The declared operator scope, and the refusal that makes it mean something.

**This is X6.** A `.pc.json` naming an operator this node does not declare
aborts at startup naming the token, and never silently skips it. It is the Go
engine's own rule about an unknown `TransformationSpec` type and this project's
rule about a missing reference table, and the argument is the same in all
three: a lookup that misses and a value that was never there look identical to
everything downstream.

**The declaration is the producer's own.** An in-scope token is exactly one
`Operator` subclass under `cpipes_node/operators/`, and `declared_scope()`
walks the registry those subclasses put themselves into. There is no tuple of
strings anywhere for an edit to widen, because a list of what to check is the
wrong shape for the question — omission and completion produce the same output
(healthcare_corpus P3-I20). Adding a token means adding a class; forgetting to
declare a class means the token is out of scope and the node aborts on it,
which is the loud direction.

**Whether it is implemented is derived too.** A subclass that defines `build`
is implemented and one that does not is declared and owed; `owed_by` names the
task that owes it. Nothing carries a hand-set `implemented` flag, so the two
cannot disagree.

**Three refusals, and they are three because three different people repair
them.** A token no kind of scope knows is the author's; a token JetStore has
and this node has not taken on is a scope decision (P9-I04, D-206); a declared
token with no `build` is a task in this package. `ScopeReport` keeps them
apart and `coordinate` raises them in that order.

**The site half is a registry rather than a token.** A deployment's own
operator has no token this package could name — naming one would make this
package know about that deployment — so a transformation token that is not a
declared built-in is in scope exactly when a site factory is registered under
it, which is where the Go builder's `default:` branch sends it. That is
`site.py`.
"""

from __future__ import annotations

import importlib
from dataclasses import dataclass, field
from enum import StrEnum
from typing import Any, ClassVar

from .errors import OperatorNotImplemented, OperatorOutOfScope


class TokenKind(StrEnum):
    """The three places a `.pc.json` names something this node dispatches on.

    They are three kinds and not one namespace because the contract keys them
    on three different structs, and `merge_files` is the reason it matters: it
    is a *pipe* type, not a transformation, so a single flat scope would put it
    beside `map_record` and make a document naming `{"type": "merge_files"}` in
    an `apply` list look acceptable.
    """

    INPUT_CHANNEL = "input_channel"
    PIPE = "pipe"
    TRANSFORMATION = "transformation"


class Operator:
    """One declared token. Subclassing is the declaration.

    A subclass sets `kind` and `token`; `__init_subclass__` registers it. A
    subclass that defines `build` is implemented, and one that does not is
    declared and owed by the task named in `owed_by`.
    """

    kind: ClassVar[TokenKind]
    token: ClassVar[str]
    #: The task that owes the implementation. Read only for an unimplemented
    #: declaration, and required there: "not implemented" without a name is a
    #: dead end for whoever meets it.
    owed_by: ClassVar[str] = ""
    #: One line saying what the operator does, for `cpipes-node scope`.
    summary: ClassVar[str] = ""

    def __init_subclass__(cls, **kwargs: Any) -> None:
        super().__init_subclass__(**kwargs)
        if "token" not in cls.__dict__:
            # An intermediate base that names no token declares nothing.
            return
        register(cls)

    @classmethod
    def implemented(cls) -> bool:
        """True when this class defines `build` itself.

        Derived rather than declared, so a stub cannot claim to be finished and
        a finished operator cannot be left marked as a stub.
        """
        return "build" in cls.__dict__

    @classmethod
    def build(cls, env: Any, spec: Any) -> Any:
        """Build the runtime object for one authored step.

        The base raises, which is what an unimplemented declaration is. The
        signature is deliberately loose while the channel graph (P9-T04) has
        not fixed what an `env` is; narrowing it is that task's, and this
        docstring is the seam.
        """
        raise OperatorNotImplemented(
            f"operator '{cls.token}' ({cls.kind}) is declared and not implemented; "
            f"owed by {cls.owed_by or 'nobody — which is itself the defect'}"
        )


_REGISTRY: dict[tuple[TokenKind, str], type[Operator]] = {}


def register(cls: type[Operator]) -> None:
    """Put one declaration in the registry, refusing a second claim on a token.

    A duplicate is refused rather than won by the later import, because two
    classes claiming one token is a merge artefact and the quiet resolution —
    last import wins — depends on import order, which nothing here controls.
    """
    if "kind" not in cls.__dict__ and not hasattr(cls, "kind"):
        raise TypeError(f"{cls.__name__} declares a token and no kind")
    key = (TokenKind(cls.kind), cls.token)
    existing = _REGISTRY.get(key)
    if existing is not None and existing is not cls:
        raise TypeError(
            f"two operators declare {key[0]} token '{key[1]}': "
            f"{existing.__module__}.{existing.__qualname__} and "
            f"{cls.__module__}.{cls.__qualname__}"
        )
    if not cls.implemented() and not cls.owed_by:
        raise TypeError(
            f"{cls.__module__}.{cls.__qualname__} declares token '{cls.token}' "
            "with no build and no owed_by; an unimplemented declaration must "
            "name the task that owes it"
        )
    _REGISTRY[key] = cls


_loaded = False


def _load_declarations() -> None:
    """Import the declaring modules once.

    A registry populated by import is a registry that is empty when nobody
    imported, and an empty scope makes the gate pass over everything — the
    shape of failure this repository has recorded thirty-seven times. So the
    import is here, in the accessor, rather than left to a caller to remember,
    and `tests_scope.py` asserts the scope is non-empty after a bare
    `import cpipes_node.scope`.
    """
    global _loaded
    if _loaded:
        return
    _loaded = True
    importlib.import_module("cpipes_node.operators")


def declared_scope() -> dict[TokenKind, tuple[str, ...]]:
    """Every declared token, by kind, sorted.

    Sorted because this is printed, compared and asserted against, and a set's
    iteration order is not a fact about the node.
    """
    _load_declarations()
    out: dict[TokenKind, tuple[str, ...]] = {}
    for kind in TokenKind:
        out[kind] = tuple(sorted(t for (k, t) in _REGISTRY if k == kind))
    return out


def declaration(kind: TokenKind, token: str) -> type[Operator] | None:
    _load_declarations()
    return _REGISTRY.get((TokenKind(kind), token))


def declarations() -> tuple[type[Operator], ...]:
    """Every declaration, in a deterministic order."""
    _load_declarations()
    return tuple(_REGISTRY[k] for k in sorted(_REGISTRY, key=lambda k: (k[0], k[1])))


class FindingKind(StrEnum):
    """Which of the two X6 refusals a finding is.

    A field rather than a phrase in `reason`, because the caller has to branch
    on it and a caller that branches by reading a message is a caller that
    breaks when the message is improved.
    """

    OUT_OF_SCOPE = "out_of_scope"
    UNIMPLEMENTED = "unimplemented"


@dataclass(frozen=True)
class Finding:
    """One token the gate refused, and where in the document it sat."""

    category: FindingKind
    kind: TokenKind
    token: str
    where: str
    reason: str

    def __str__(self) -> str:
        return f"{self.where}: {self.kind} '{self.token}': {self.reason}"


@dataclass
class ScopeReport:
    """What the gate found, in document order.

    Two lists rather than one with a severity field, for the reason `errors.py`
    gives: the distinction is who repairs it, and a caller that has to read a
    string to find that out will not.
    """

    out_of_scope: list[Finding] = field(default_factory=list)
    unimplemented: list[Finding] = field(default_factory=list)
    #: (kind, token, where) for every token the document named and the gate
    #: accepted. Kept so a check can assert it examined something — a gate that
    #: walked nothing reports no findings and looks identical to a clean pass.
    accepted: list[tuple[TokenKind, str, str]] = field(default_factory=list)

    @property
    def clean(self) -> bool:
        return not self.out_of_scope and not self.unimplemented

    def raise_if_out_of_scope(self) -> None:
        if not self.out_of_scope:
            return
        raise OperatorOutOfScope(
            "this node's operator scope does not cover "
            f"{len(self.out_of_scope)} token(s) the pipeline names:\n"
            + "".join(f"  {f}\n" for f in self.out_of_scope)
            + render_scope()
        )

    def raise_if_unimplemented(self) -> None:
        if not self.unimplemented:
            return
        raise OperatorNotImplemented(
            f"{len(self.unimplemented)} token(s) the pipeline names are declared "
            "and not yet built:\n" + "".join(f"  {f}\n" for f in self.unimplemented)
        )


def render_scope() -> str:
    """The scope, as the abort message prints it.

    The message names the scope searched and not only the token, because
    "unknown operator: cgt_scrub" leaves the author unable to tell a typo from
    a built-in this node has not taken on, and those have different repairs.
    """
    lines = ["  the scope searched:\n"]
    for kind, tokens in declared_scope().items():
        for token in tokens:
            cls = declaration(kind, token)
            assert cls is not None
            mark = "" if cls.implemented() else f"  (declared, owed by {cls.owed_by})"
            lines.append(f"    {kind}: {token}{mark}\n")
    lines.append(
        "    transformation: any token a deployment registered as a site "
        "operator (see cpipes_node.site)\n"
    )
    return "".join(lines)


def classify(
    kind: TokenKind,
    token: str,
    where: str,
    site_tokens: frozenset[str] = frozenset(),
    contract_tokens: tuple[str, ...] = (),
) -> Finding | None:
    """Judge one token. None when it is in scope and built.

    `contract_tokens` is what JetStore's own model knows for this kind, and it
    is here only to make the message better: a token JetStore has and this node
    has not taken on is a *scope* question (P9-I04) and a token nobody has is a
    typo, and telling the author which one they have is most of the value of
    the abort.
    """
    cls = declaration(kind, token)
    if cls is not None:
        if cls.implemented():
            return None
        return Finding(
            FindingKind.UNIMPLEMENTED,
            kind,
            token,
            where,
            f"declared and not implemented; owed by {cls.owed_by}",
        )
    if kind is TokenKind.TRANSFORMATION and token in site_tokens:
        # A registered site operator is in scope by registration, and whether
        # it works is its own author's business — this node knows nothing about
        # its shape, which is the whole of what `site_config` means.
        return None
    if token in contract_tokens:
        reason = (
            "a JetStore built-in this node does not implement "
            "(whether the subset is permanent is D-206 / P9-I04)"
        )
    elif kind is TokenKind.TRANSFORMATION:
        reason = "neither a declared built-in nor a registered site operator"
    else:
        reason = "not a token this node declares"
    return Finding(FindingKind.OUT_OF_SCOPE, kind, token, where, reason)
