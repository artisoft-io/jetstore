"""The node's refusals.

Every one of them aborts. There is no tolerant mode and no flag that turns one
into a warning, for the reason the Go node states about a missing reference
table and this package states about an operator token: **a node that silently
skips a step produces output that looks complete and is not**, and the consumer
discovers it by getting a corpus rather than by getting an error.

The classes are distinguished by *who repairs them*, which is why
`OperatorOutOfScope` and `OperatorNotImplemented` are two types rather than one
with a field. The first sends the reader to the authored `.pc.json` or to a
deployment's operator registration; the second sends them to a task in this
package that has not landed. A caller that cannot tell them apart would report
an authoring mistake as a missing feature.
"""

from __future__ import annotations


class NodeError(Exception):
    """Base of every refusal this package makes."""


class StartupError(NodeError):
    """The node refused before doing any work.

    Everything raised out of `coordinate` before the graph is handed a row is
    one of these, so a caller can tell "this run never started" from "this run
    started and failed", which is the distinction the execution-status row
    turns on.
    """


class ContractModelNotFound(StartupError):
    """`cpipes_model.py` could not be located. See `contract.py`."""


class ConfigNotFound(StartupError):
    """No `.pc.json` for this pipeline execution key."""


class ConfigInvalid(StartupError):
    """The `.pc.json` does not satisfy the contract model."""


class ScopeError(StartupError):
    """Base of the two X6 refusals, so a caller may catch the pair."""


class OperatorOutOfScope(ScopeError):
    """The document names a token this node does not declare.

    Names the token and the scope searched, because a message that says only
    "unknown operator" leaves the reader unable to tell a typo from a built-in
    this node has not taken on.
    """


class OperatorNotImplemented(ScopeError):
    """The document names a token this node declares and has not built yet.

    Raised at startup rather than at the step, deliberately: a run that writes
    four of twelve tables and then discovers the fifth operator is a stub has
    already written four tables.
    """


class ObjectStoreError(NodeError):
    """The object store refused or could not answer."""


class ObjectNotFound(ObjectStoreError):
    """No object at that key."""
