"""The five environment variables the Go `cp_node` main reads, and the refusal.

`cdk/jetstore_one/lambdas/compute_pipes/cp_node/main.go` names them in its
header comment and checks three of them before `lambda.Start`:

    JETS_BUCKET           required
    JETS_DSN_SECRET       required
    JETS_REGION           required
    CPIPES_DB_POOL_SIZE   optional, floored at 3
    JETS_S3_KMS_KEY_ARN   optional

The refusal is mirrored, including its shape: the Go main collects *every*
missing variable and panics once with all of them, rather than failing on the
first. That is worth copying — a deployment missing three variables that is
told about one is a deployment redeployed three times.

The pool-size floor is mirrored too, warning and clamping rather than refusing,
which is what the Go main does. It is carried here although this node opens no
pool of its own: the value reaches `P9-T09`'s side effects, and a setting read
in two places with two floors is two things to keep right.
"""

from __future__ import annotations

import logging
import os
from collections.abc import Mapping
from dataclasses import dataclass

from .errors import StartupError

log = logging.getLogger(__name__)

MINIMUM_DB_POOL_SIZE = 3

#: Required, in the order the Go main reports them.
REQUIRED = ("JETS_DSN_SECRET", "JETS_REGION", "JETS_BUCKET")
OPTIONAL = ("CPIPES_DB_POOL_SIZE", "JETS_S3_KMS_KEY_ARN")


@dataclass(frozen=True)
class Settings:
    bucket: str
    region: str
    dsn_secret: str
    db_pool_size: int = MINIMUM_DB_POOL_SIZE
    kms_key_arn: str = ""

    @classmethod
    def from_env(cls, env: Mapping[str, str] | None = None) -> Settings:
        source = os.environ if env is None else env
        missing = [name for name in REQUIRED if not source.get(name)]
        if missing:
            raise StartupError(
                "invalid argument(s): "
                + "; ".join(f"{name} must be provided" for name in missing)
            )
        pool = MINIMUM_DB_POOL_SIZE
        raw = source.get("CPIPES_DB_POOL_SIZE", "")
        if raw:
            try:
                pool = int(raw)
            except ValueError:
                # The Go main ignores an unparseable value and keeps the
                # default. Mirrored, and logged, because silently taking 3 for
                # "eight" is the kind of thing nobody looks for.
                log.warning(
                    "CPIPES_DB_POOL_SIZE=%r is not an integer; using %d",
                    raw,
                    MINIMUM_DB_POOL_SIZE,
                )
                pool = MINIMUM_DB_POOL_SIZE
        if pool < MINIMUM_DB_POOL_SIZE:
            log.warning(
                "DB pool size must be at least %d, using env CPIPES_DB_POOL_SIZE, "
                "setting to %d",
                MINIMUM_DB_POOL_SIZE,
                MINIMUM_DB_POOL_SIZE,
            )
            pool = MINIMUM_DB_POOL_SIZE
        return cls(
            bucket=source["JETS_BUCKET"],
            region=source["JETS_REGION"],
            dsn_secret=source["JETS_DSN_SECRET"],
            db_pool_size=pool,
            kms_key_arn=source.get("JETS_S3_KMS_KEY_ARN", ""),
        )
