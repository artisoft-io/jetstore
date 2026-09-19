"""The image's default Lambda handler for the Python `cp_node`.

**This file is the image's and not the package's, and that is why it lives in a
build context rather than in `tools/cpipes_node/cpipes_node/`.**
`awslambda.Node`'s docstring says in terms that *a deployment composes this
rather than importing a ready-made handler*, because a deployment's own
operators reach the node the way Go's do — as an argument — and a handler that
read a registry out of module state would make the composition invisible. The
container image **is** the deployment artefact, so the composition belongs to
the image.

A site shipping its own operators derives from this image, adds a module of this
shape with its own `Registry`, and points the function's `Cmd` at it; nothing
here needs a build argument for that, because `DockerImageCode_FromEcr` sets
`Cmd` at synth (`build_cpipes_lambdas.go`), so the handler a deployment runs is
visible in the synthesised template rather than baked into a layer.

## The database connection

`cpipes_node` imports no database driver — `ExecutionStatusConfigSource` takes
anything with a DB-API `cursor()` and `Node.connect` is where a deployment
supplies one. This module supplies it, and **mirrors the Go path rather than
inventing a DSN**:

    cdk/jetstore_one/lambdas/dbc/db_connection.go  ->  awsi.GetDsnFromSecret
    jets/awsi/awsi.go                                 GetDsnFromJson

which reads the secret named by `JETS_DSN_SECRET`, parses it as JSON and builds
`postgresql://<username>:<escaped password>@<host>:<port>/postgres` with
`pool_max_conns` set to the pool size. `USING_SSH_TUNNEL`, when present in the
environment, replaces the host with `localhost` — the same rule on the same
variable as the Go path, which is what lets a local run against the ssh tunnel
reach the database the deployed node reaches.

**The escaping is `quote_plus` because the Go original is `url.QueryEscape`, and
that is a mirror of something this repository should probably change.**
`QueryEscape` encodes a space as `+`, which means a space only inside a query
string; a password lands in the URI's *userinfo*, where libpq reads `+`
literally. So a password containing a space produces a DSN that fails to
authenticate — in Go today, and here, identically. Mirroring is the deliberate
choice: a Python node that connected where the Go node could not would be a
silent divergence between two engines, which is the one thing P9-I02 exists to
worry about. The defect is JetStore's and is reported rather than repaired
(P9-I52).

**Two things the Go path has that this does not, both deliberate.** It does not
poll `jetsapi.secret_rotation` to reopen the pool after a rotation: that is a
cold-start-lifetime concern the Go main solves because its pool outlives many
invocations, and the cost of not solving it here is a failed invocation and a
retry rather than a wrong answer. And it opens the connection **lazily, at the
first invocation**, not at import — a cold start that succeeded and an
invocation that cannot reach its database are different things to see in a log,
which is `awslambda.Node`'s own argument.

`psycopg` and `boto3` are imported inside the function for the same reason:
importing a driver at module scope would make `import handler` require one, and
the one place this module is imported with no database in front of it is the
image's own build-time check.
"""

from __future__ import annotations

import json
import logging
import os
from typing import Any
from urllib.parse import quote_plus

from cpipes_node.awslambda import Node
from cpipes_node.settings import Settings

log = logging.getLogger(__name__)
logging.getLogger().setLevel(os.environ.get("LOG_LEVEL", "INFO"))


def dsn_from_secret_json(secret_json: str, pool_size: int) -> str:
    """`awsi.GetDsnFromJson`, in Python.

    A free function taking the secret's *text* so that it is exercisable without
    Secrets Manager: the only thing the AWS call contributes is that string. The
    image's build runs it against a fixed input and asserts the result, which is
    the only test this file can have — nothing under `dockerfiles/` is collected
    by a test runner, and the image is the artefact that ships it.
    """
    m = json.loads(secret_json)
    host = m["host"]
    if "USING_SSH_TUNNEL" in os.environ:
        host = "localhost"
        log.info("LOCAL TESTING using ssh tunnel (expecting ssh tunnel open)")
    return (
        f"postgresql://{m['username']}:{quote_plus(str(m['password']))}"
        f"@{host}:{int(m['port'])}/postgres?pool_max_conns={pool_size}"
    )


def connect(settings: Settings) -> Any:
    """Open one connection, from the secret the environment names."""
    import boto3  # noqa: PLC0415 - see the module docstring
    import psycopg  # noqa: PLC0415

    client = boto3.client("secretsmanager", region_name=settings.region)
    secret = client.get_secret_value(SecretId=settings.dsn_secret)["SecretString"]
    return psycopg.connect(dsn_from_secret_json(secret, settings.db_pool_size))


#: One per cold start, exactly as the Go main's package-level `dbConnection` is.
#: `Settings.from_env` runs at this line, so a function missing one of the three
#: required variables fails at *import* with all of them named — the Go main's
#: panic before `lambda.Start`, in the only place a Python handler has for it.
node = Node(connect=connect)


def lambda_handler(event: dict[str, Any], context: Any = None) -> Any:
    """`func handler(ctx, arg ComputePipesNodeArgs) error`, in Python."""
    return node.handler(event, context)
