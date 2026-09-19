"""The site-operator registry: this node's `WithOperators`.

`jets/compute_pipes/site_operators.go` is the contract being mirrored, and the
three rules that matter are mirrored exactly, because a Python node that
resolved a token differently from the Go node would make the two engines
disagree about what a `.pc.json` *means* rather than about how fast it runs —
which is the one class of divergence the conformance instrument (P9-T22) cannot
absorb.

1. **A built-in wins.** In Go the eighteen cases are tried first and the
   registry is consulted only in the `default:` branch; here `classify` asks
   the declaration registry before the site registry. A site cannot change what
   an existing `.pc.json` means.
2. **A collision is registered and logged, not refused.** Go keeps the entry
   and warns, on the ground that losing silently is worse than losing loudly.
   The same here, and for the same reason — refusing would make a deployment's
   `main` fail to start over an operator it will simply never reach.
3. **An empty name or a null factory is dropped with a warning**, because
   neither can ever be called.

**What is deliberately *not* mirrored is where the refusal happens.** Go cannot
check a site token at startup at all: the registry is an argument to
`CoordinateComputePipes`, which runs in `cp_node`, while the starters that see
the document run elsewhere — `validateSiteOperatorSpec` says so in terms and
cites I-779. In this node the registry and the document are in one process at
startup, so X6's abort is available and is taken. The consequence for P9-T22 is
that a document naming an unregistered operator fails in *both* engines and at
*different moments*; a conformance instrument comparing output bytes sees both
fail, and one comparing the moment of failure would report a divergence that is
an improvement.
"""

from __future__ import annotations

import logging
from collections.abc import Callable
from typing import Any, Protocol

log = logging.getLogger(__name__)

#: A factory is handed the operator environment and the authored spec and
#: returns the runtime object. Both types are the channel graph's to fix
#: (P9-T04), which is why this is not narrowed yet.
SiteOperatorFactory = Callable[[Any, Any], Any]


class SiteOperatorRegistry(Protocol):
    def tokens(self) -> frozenset[str]: ...
    def factory(self, token: str) -> SiteOperatorFactory | None: ...


class Registry:
    """A deployment's operators, by token.

    Instance rather than module state, so a test — and a deployment embedding
    two nodes in one process — can hold two, and so that nothing registered by
    an import can leak into a run that did not ask for it.
    """

    def __init__(self) -> None:
        self._operators: dict[str, SiteOperatorFactory] = {}

    def with_operators(self, operators: dict[str, SiteOperatorFactory]) -> Registry:
        """Register this deployment's own operators. Returns self, to chain.

        Merges, later calls winning on a repeated name — `WithOperators`'s
        documented behaviour.
        """
        from .scope import TokenKind, declared_scope

        reserved = set(declared_scope()[TokenKind.TRANSFORMATION])
        for name, factory in operators.items():
            if not name:
                log.warning(
                    "an operator registered under an empty name can never be "
                    "reached; ignored"
                )
                continue
            if factory is None:
                log.warning("operator %r registered with a nil factory; ignored", name)
                continue
            if name in reserved:
                log.warning(
                    "%r is a built-in this node declares; the built-in wins and "
                    "this factory will never be called",
                    name,
                )
            self._operators[name] = factory
        return self

    def tokens(self) -> frozenset[str]:
        return frozenset(self._operators)

    def factory(self, token: str) -> SiteOperatorFactory | None:
        return self._operators.get(token)

    def __len__(self) -> int:
        return len(self._operators)


EMPTY = Registry()
"""The registry a node runs with when the deployment registered nothing.

A separate name rather than `None` at every call site, so the "no site
operators" case is a value rather than a branch."""
