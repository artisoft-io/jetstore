"""The object store, behind one seam, with a local implementation and an S3 one.

**The local store is not a test double.** X2, X3, X5 and X7 each need an
executable oracle, this repository has no docker-compose, and a node that can
only be run inside AWS is a node whose byte-for-byte comparisons are run by
hand. So the local path is the same code with a different store, and `Local` is
the implementation every check takes.

What is withheld from the protocol is as deliberate as what is on it. There is
no `open()` returning a file handle and no `presign()`: the first would let an
operator hold a stream across the node's lifetime, which is the shape streaming
would need and which is deferred by instruction (charter §1.3); the second is
meaningless for a local directory and having it would make the local path the
degenerate one rather than the ordinary one.

Keys are S3 keys — forward-slashed, no leading slash, no `..` — in both
implementations, so a key that works against a directory works against a
bucket. `Local` refuses anything else rather than resolving it, because a key
that escapes the root resolves perfectly well on a filesystem and not at all in
a bucket, and a run that works locally and fails deployed is the failure this
seam exists to prevent.
"""

from __future__ import annotations

from collections.abc import Iterable
from dataclasses import dataclass
from pathlib import Path, PurePosixPath
from typing import Protocol, runtime_checkable

from .errors import ObjectNotFound, ObjectStoreError


def validate_key(key: str) -> str:
    """Refuse anything that is not an S3 object key. Returns the key."""
    if not key:
        raise ObjectStoreError("empty object key")
    if key.startswith("/"):
        raise ObjectStoreError(f"object key must not start with '/': {key!r}")
    if "\\" in key:
        raise ObjectStoreError(f"object key must not contain a backslash: {key!r}")
    parts = PurePosixPath(key).parts
    if any(p in ("..", ".") for p in parts):
        raise ObjectStoreError(f"object key must not contain '.' or '..': {key!r}")
    return key


@runtime_checkable
class ObjectStore(Protocol):
    """What the node needs of S3, and nothing else."""

    def get(self, key: str) -> bytes: ...

    def put(self, key: str, data: bytes) -> None: ...

    def list(self, prefix: str) -> tuple[str, ...]:
        """Keys under `prefix`, sorted.

        Sorted rather than in the store's own order: a partitioned run's inputs
        are enumerated through this, and a corpus that is a function of a
        listing order is a corpus that is a function of the store.
        """
        ...

    def download(self, key: str, path: Path) -> None: ...

    def upload(self, path: Path, key: str) -> None: ...


@dataclass(frozen=True)
class Local:
    """A directory standing in for a bucket."""

    root: Path

    def _path(self, key: str) -> Path:
        return self.root / validate_key(key)

    def get(self, key: str) -> bytes:
        path = self._path(key)
        try:
            return path.read_bytes()
        except FileNotFoundError as exc:
            raise ObjectNotFound(f"no object at {key!r} under {self.root}") from exc

    def put(self, key: str, data: bytes) -> None:
        path = self._path(key)
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)

    def list(self, prefix: str) -> tuple[str, ...]:
        # A prefix is not a directory: S3 matches it as a string, and a store
        # that matched directories would accept `a/b` and miss `a/bc`.
        if prefix:
            validate_key(prefix.rstrip("/") or prefix)
        found: list[str] = []
        for path in self.root.rglob("*"):
            if not path.is_file():
                continue
            key = path.relative_to(self.root).as_posix()
            if key.startswith(prefix):
                found.append(key)
        return tuple(sorted(found))

    def download(self, key: str, path: Path) -> None:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(self.get(key))

    def upload(self, path: Path, key: str) -> None:
        self.put(key, path.read_bytes())


@dataclass
class S3:
    """A bucket, through boto3.

    boto3 is imported here rather than at module scope so that importing this
    package — which every test does — does not require the AWS SDK. **Nothing
    in this class is exercised by any check in this repository**, because there
    is no AWS to exercise it against; that is stated rather than papered over,
    and what would settle it is the first deployment (P9-T15) or a moto
    fixture, which is a dependency nobody has asked for yet.
    """

    bucket: str
    region: str | None = None
    kms_key_arn: str | None = None
    _client: object | None = None

    def client(self) -> object:
        if self._client is None:
            try:
                import boto3  # type: ignore[import-not-found]
            except ImportError as exc:  # pragma: no cover - depends on the install
                raise ObjectStoreError(
                    "the S3 object store needs boto3; install cpipes_node[s3], "
                    "or run against a local store with --store"
                ) from exc
            self._client = boto3.client("s3", region_name=self.region)
        return self._client

    def _extra(self) -> dict[str, str]:
        if not self.kms_key_arn:
            return {}
        return {"ServerSideEncryption": "aws:kms", "SSEKMSKeyId": self.kms_key_arn}

    def get(self, key: str) -> bytes:
        import botocore.exceptions  # type: ignore[import-not-found]

        try:
            resp = self.client().get_object(Bucket=self.bucket, Key=validate_key(key))  # type: ignore[attr-defined]
        except botocore.exceptions.ClientError as exc:
            if exc.response.get("Error", {}).get("Code") in ("NoSuchKey", "404"):
                raise ObjectNotFound(f"no object at {key!r} in {self.bucket}") from exc
            raise ObjectStoreError(str(exc)) from exc
        return resp["Body"].read()

    def put(self, key: str, data: bytes) -> None:
        self.client().put_object(  # type: ignore[attr-defined]
            Bucket=self.bucket, Key=validate_key(key), Body=data, **self._extra()
        )

    def list(self, prefix: str) -> tuple[str, ...]:
        paginator = self.client().get_paginator("list_objects_v2")  # type: ignore[attr-defined]
        keys: list[str] = []
        for page in paginator.paginate(Bucket=self.bucket, Prefix=prefix):
            keys.extend(item["Key"] for item in page.get("Contents", ()))
        return tuple(sorted(keys))

    def download(self, key: str, path: Path) -> None:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(self.get(key))

    def upload(self, path: Path, key: str) -> None:
        self.put(key, path.read_bytes())


def keys_under(store: ObjectStore, prefixes: Iterable[str]) -> tuple[str, ...]:
    """Every key under any of `prefixes`, sorted and de-duplicated."""
    found: set[str] = set()
    for prefix in prefixes:
        found.update(store.list(prefix))
    return tuple(sorted(found))
