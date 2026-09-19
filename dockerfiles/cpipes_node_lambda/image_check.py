"""The checks `Dockerfile.cpipes_python_lambda` runs at build time.

**Why the image's build is where these live.** Nothing under `dockerfiles/` is
collected by a test runner, and the two things worth asserting here are both
properties of the *image* rather than of the package: where the contract's
Pydantic model ended up on this image's `sys.path`, and that the handler this
image ships composes. Running them in a `RUN` layer means a broken image cannot
be pushed, which is the same division `Dockerfile.cpipes_native_lambda` makes
when it runs `ldd libjets.so` in its own build.

It is run with the three variables of `cpipes_node.settings.REQUIRED` set to
values that are obviously not a deployment's: importing `handler` composes a
`Node`, and a `Node` reads its settings from the environment. Nothing here opens
a connection — `Node.connect` is called lazily at the first invocation.

Exits non-zero with the failing assertion, which is what stops the build.
"""

from __future__ import annotations

import os
import pathlib
import sys

# 1. The contract model, and *where* it was found.
#
# `cpipes_model.py` is a sibling file of the `cpipes_contract` package rather
# than a module in it, and that package's wheel declares
# `packages = ["cpipes_contract"]` — so an installed wheel does not carry the
# model at all (P9-I27). The Dockerfile copies it next to the installed package,
# which is the second path `cpipes_node.contract._load_contract_model` searches
# and is also, because site-packages is on `sys.path`, the first: a plain
# `import cpipes_model` succeeds on this image. Both are asserted, because
# "a model was found" and "the model the node loads is the one this image put
# there" are different claims and only the second rules out a stale copy.
import cpipes_model  # noqa: E402

import cpipes_contract  # noqa: E402
import cpipes_node.contract as node_contract  # noqa: E402
from cpipes_node.scope import declared_scope  # noqa: E402

site = pathlib.Path(cpipes_contract.__file__).resolve().parent.parent
expected_model = site / "cpipes_model.py"

assert pathlib.Path(cpipes_model.__file__).resolve() == expected_model, (
    f"import cpipes_model resolved to {cpipes_model.__file__}, want {expected_model}"
)
assert pathlib.Path(node_contract.model.__file__).resolve() == expected_model, (
    f"the node loaded its contract model from {node_contract.model.__file__}, "
    f"want {expected_model}"
)

# 2. The handler composes, and carries a connection factory.
#
# A `Node` with no `connect` refuses at invocation rather than at import, so an
# image whose handler forgot one would pass every structural check here and fail
# on its first real event. The assertion is the cheap guard against that.
sys.path.insert(0, os.environ["LAMBDA_TASK_ROOT"])
import handler  # noqa: E402

assert handler.node.connect is not None, "handler.node carries no connect"
assert callable(handler.lambda_handler), "handler.lambda_handler is not callable"

# 3. The DSN mirror, against a fixed input.
#
# `handler.dsn_from_secret_json` is `awsi.GetDsnFromJson` rewritten, and this is
# the only place it is exercised. The password is chosen to pin the escaping:
# a space, a slash and a colon each escape differently, and the space is the one
# that distinguishes `url.QueryEscape` (`+`, which is what Go writes and what
# this mirrors) from `quote(safe="")` (`%20`).
SECRET = '{"username": "jets", "password": "p a/s:s", "host": "db.example", "port": 5432}'
WANT = "postgresql://jets:p+a%2Fs%3As@db.example:5432/postgres?pool_max_conns=7"
got = handler.dsn_from_secret_json(SECRET, 7)
assert got == WANT, f"dsn_from_secret_json gave\n  {got}\nwant\n  {WANT}"

os.environ["USING_SSH_TUNNEL"] = "1"
tunnelled = handler.dsn_from_secret_json(SECRET, 7)
del os.environ["USING_SSH_TUNNEL"]
assert "@localhost:5432/" in tunnelled, (
    f"USING_SSH_TUNNEL did not redirect the host: {tunnelled}"
)

# 4. What the image ships, printed into the build log.
#
# The scope census is the one line that says this image's node is the one the
# package declares; a count rather than a list, because the list is
# `cpipes-node scope`'s job and this is a build log.
print("contract model:", cpipes_model.__file__)
print("declared scope:", {kind: len(v) for kind, v in declared_scope().items()})
print("image checks passed")
