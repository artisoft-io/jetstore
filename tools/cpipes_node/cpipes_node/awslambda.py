"""The lambda entry, in the shape `cp_node`'s `main.go` has.

The Go main does four things before `lambda.Start`: read and floor
`CPIPES_DB_POOL_SIZE`, refuse a missing `JETS_DSN_SECRET`, `JETS_REGION` or
`JETS_BUCKET`, open the database pool once at cold start, and register the
jetrules proxy. The handler then does one: hand the event to
`CoordinateComputePipes`.

The same division here, with one difference that is the whole point of the
module. **A deployment composes this rather than importing a ready-made
handler**, because its own operators reach the node the way Go's do — as an
argument — and a handler that read a registry out of module state would make
the composition invisible:

    # the deployment's own lambda module
    from cpipes_node import Registry
    from cpipes_node.awslambda import Node

    node = Node(site_operators=Registry().with_operators({"my_op": my_factory}))
    handler = node.handler

**The pool is not opened here and that is deliberate.** The Go main opens one
at cold start through `dbc.NewDbConnection`, which reads the secret, and this
package imports no database driver at all — `ExecutionStatusConfigSource` takes
a connection. `Node.connect` is where a deployment supplies one, and P9-T09
owns what the node then writes through it. Until that lands, a `Node` without a
`connect` refuses at invocation rather than at import, which is the right
moment: a cold start that succeeded and an invocation that cannot read its
configuration are different things to see in a log.
"""

from __future__ import annotations

from collections.abc import Callable
from dataclasses import dataclass, field
from typing import Any

from .args import NodeArgs
from .config import ExecutionStatusConfigSource
from .errors import StartupError
from .node import coordinate
from .settings import Settings
from .site import EMPTY, SiteOperatorRegistry
from .store import S3


@dataclass
class Node:
    """One cold start's worth of state, and the handler over it."""

    settings: Settings = field(default_factory=Settings.from_env)
    site_operators: SiteOperatorRegistry = EMPTY
    #: Returns a DB-API connection. Called once, lazily, on first invocation.
    connect: Callable[[Settings], Any] | None = None
    _connection: Any = None

    def connection(self) -> Any:
        if self._connection is None:
            if self.connect is None:
                raise StartupError(
                    "no database connection: a deployment supplies one with "
                    "Node(connect=...), which is where the Go main's "
                    "dbc.NewDbConnection(JETS_DSN_SECRET) goes. This package "
                    "imports no driver."
                )
            self._connection = self.connect(self.settings)
        return self._connection

    def handler(self, event: dict[str, Any], context: Any = None) -> Any:
        """`func handler(ctx, arg ComputePipesNodeArgs) error`, in Python.

        `NodeArgs` forbids an unknown field, so an event shaped for a future
        Go entry is refused here rather than half-understood.
        """
        return coordinate(
            NodeArgs(**event),
            ExecutionStatusConfigSource(self.connection()),
            store=S3(
                bucket=self.settings.bucket,
                region=self.settings.region,
                kms_key_arn=self.settings.kms_key_arn or None,
            ),
            settings=self.settings,
            site_operators=self.site_operators,
        )
