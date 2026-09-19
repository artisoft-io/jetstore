"""A compute pipes node in Python.

The node's entry is three fields — `{id, jp, pe}` — and everything else comes
from `jetsapi.cpipes_execution_status` and from S3. That is the whole seam, and
it is the reason this package can exist beside the Go node rather than in place
of it: what must hold is the contract and the side effects, not a port.

**The package knows nothing about any deployment.** No operator here is named
after a customer, a domain or a corpus; a deployment's own operator arrives
through `site.Registry`, which is this node's `WithOperators`. That is a
constraint worth stating because it is the one this package would lose first:
the first time a corpus-shaped helper is added *here* rather than in the
deployment's own operator, the node stops being a JetStore capability and
becomes one customer's runtime.

**And it is not a second JetStore.** The operator scope is three
transformations of JetStore's nineteen, two pipe kinds of three and two input
channel types of four, declared in `operators/` and enforced at startup by
`config.check_scope`. Every built-in this node is tempted to grow is that risk
arriving. See `scope.py`.

What is built, and what is a seam:

===========================  =============================================
built (P9-T03)               `args`, `settings`, `contract`, `config`,
                             `scope`, `operators`, `site`, `store`, `node`,
                             `main`
built (P9-T04)               `graph`, `runtime`, `expressions`, and `build`
                             on `generator`, `memory` and `fan_out`
`map_record`, `filter`,
the column evaluators        `operators/transformations.py` — P9-T06,
                             reached at `OperatorEnv.column_evaluator`
the partition writer         `operators/transformations.py` — P9-T07
`merge_files`                `operators/pipes.py` — P9-T08
the six side-effect tables   not started — P9-T09
the site operator itself     the deployment's, registered — P9-T05
===========================  =============================================

**A deployment's own operator is written against `runtime`**, which is
`pipesmodel/operator.go`'s Python half: an evaluator is the three of `apply`,
`done` and `finally_`, and a factory is
`(OperatorEnv, OperatorArgs) -> PipeTransformationEvaluator`. Nothing in
`runtime` reaches the channel registry, which is §12.6's line drawn in Python —
an operator is handed the channels its own step declared and can name no other.
"""

from __future__ import annotations

from .args import NodeArgs
from .errors import (
    ConfigInvalid,
    ConfigNotFound,
    NodeError,
    OperatorNotImplemented,
    OperatorOutOfScope,
    ScopeError,
    StartupError,
)
from .runtime import (
    Done,
    InputChannel,
    Lookup,
    NullOperatorEnv,
    OperatorArgs,
    OperatorEnv,
    OutputChannel,
    PipeTransformationEvaluator,
    RowLevelError,
)
from .scope import Operator, ScopeReport, TokenKind, declared_scope
from .site import Registry
from .store import S3, Local, ObjectStore

#: What a deployment imports. The nine names from `runtime` are the operator
#: contract — an author of a site operator needs all of them and nothing else, so
#: they are exported here rather than reached through a submodule path.
__all__ = [
    "S3",
    "ConfigInvalid",
    "ConfigNotFound",
    "Done",
    "InputChannel",
    "Local",
    "Lookup",
    "NodeArgs",
    "NodeError",
    "NullOperatorEnv",
    "ObjectStore",
    "Operator",
    "OperatorArgs",
    "OperatorEnv",
    "OperatorNotImplemented",
    "OperatorOutOfScope",
    "OutputChannel",
    "PipeTransformationEvaluator",
    "Registry",
    "RowLevelError",
    "ScopeError",
    "ScopeReport",
    "StartupError",
    "TokenKind",
    "declared_scope",
]
