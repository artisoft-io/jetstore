"""The object store, behind one seam, with a local implementation and an S3 one.

**The local store is not a test double.** X2, X3, X5 and X7 each need an
executable oracle, this repository has no docker-compose, and a node that can
only be run inside AWS is a node whose byte-for-byte comparisons are run by
hand. So the local path is the same code with a different store, and `Local` is
the implementation every check takes.

What is withheld from the protocol is as deliberate as what is on it. There is
no `open()` returning a file handle and no `presign()`: the first would let an
operator hold a stream across the node's lifetime, and the second is meaningless
for a local directory — having it would make the local path the degenerate one
rather than the ordinary one.

`put_stream` is the one thing the first paragraph used to rule out, and D-243 is
why it is here: `stream_data_out` is a flag a document sets and the Go engine
honours (`s3_device_writter.go:40`), and a flag this node accepted and ignored is
this repository's standing class. It is deliberately **push**-shaped — the caller
is handed a sink and writes into it — rather than a reader the store pulls from,
because a pull seam needs a thread to bridge an encoder that writes, and the
node's whole execution model is the one that has no concurrency to reason about.

Keys are S3 keys — forward-slashed, no leading slash, no `..` — in both
implementations, so a key that works against a directory works against a
bucket. `Local` refuses anything else rather than resolving it, because a key
that escapes the root resolves perfectly well on a filesystem and not at all in
a bucket, and a run that works locally and fails deployed is the failure this
seam exists to prevent.

# A bucket that is not the node's own (D-242)

`for_bucket` is the seam an authored `output_channel.bucket` reaches. **A store
is bound to one bucket**, as Go's `S3DeviceWriter` is bound to one
`externalBucket`, so the resolution happens once where the destination is
computed and the writer holds a store that already points at the right place.

The sentinel is Go's and it is spelled twice there — once in the builder's
`spec.OutputChannel.Bucket != "jetstore_bucket"` guard and once at the upload in
`awsi.UploadToS3FromReader` / `s3_device_worker.go:63` — so an empty value and
the literal `jetstore_bucket` both mean *the node's own bucket*. Both spellings
are honoured here, in `is_own_bucket`, because a document may carry either and a
node that honoured one of them would write to a bucket named `jetstore_bucket`.
"""

from __future__ import annotations

import io
from collections.abc import Callable, Iterable, Mapping
from dataclasses import dataclass, field
from pathlib import Path, PurePosixPath
from typing import IO, Any, Protocol, runtime_checkable

from .errors import ObjectNotFound, ObjectStoreError

#: Go's literal for "the JetStore bucket", checked at two sites on both write
#: paths (`pipe_transformation_partition_writer.go:486`, `awsi.go:598`,
#: `s3_device_worker.go:63`). A document may spell the node's own bucket as the
#: empty value or as this, and the two mean the same thing.
JETSTORE_BUCKET = "jetstore_bucket"


def is_own_bucket(bucket: str | None) -> bool:
    """True when `bucket` names the node's own bucket rather than another.

    Both of Go's spellings, in one place, so that a caller cannot honour one of
    them: an empty value and `jetstore_bucket` are the same destination.
    """
    return not bucket or bucket == JETSTORE_BUCKET


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

    def put_stream(self, key: str, write: Callable[[IO[bytes]], None]) -> None:
        """Put an object whose bytes are produced into a sink as they are made.

        The caller is handed a writable binary sink and returns when it has
        written the whole object; the store decides whether that sink is a file,
        a buffer or an S3 multipart upload. **No implementation may hold the
        whole object**, which is the property `stream_data_out` exists to buy
        and the one an assertion about bytes cannot see.
        """
        ...

    def for_bucket(self, bucket: str | None) -> ObjectStore:
        """This store, or one bound to `bucket` (D-242).

        `is_own_bucket(bucket)` returns `self`, so the ordinary case allocates
        nothing and the two spellings of "the node's own" are one branch.
        """
        ...


@dataclass(frozen=True)
class Local:
    """A directory standing in for a bucket.

    `buckets` maps an external bucket name to the directory standing in for
    *it*, and an unmapped name is **refused rather than written next door**. A
    local run that named another account's bucket and quietly wrote into the
    node's own root would be the deployed defect reproduced in the one place it
    could be caught, so the local store makes the same distinction S3 does.
    """

    root: Path
    #: `compare=False` keeps the frozen dataclass hashable with a mapping field.
    buckets: Mapping[str, Path] = field(default_factory=dict, compare=False)

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

    def put_stream(self, key: str, write: Callable[[IO[bytes]], None]) -> None:
        """Open the file and hand it over. Nothing is buffered."""
        path = self._path(key)
        path.parent.mkdir(parents=True, exist_ok=True)
        with path.open("wb") as sink:
            write(sink)

    def for_bucket(self, bucket: str | None) -> ObjectStore:
        if is_own_bucket(bucket):
            return self
        assert bucket is not None
        root = self.buckets.get(bucket)
        if root is None:
            named = ", ".join(sorted(self.buckets)) or "none"
            raise ObjectStoreError(
                f"the document writes to bucket {bucket!r} and this local store "
                f"stands in for {named}. A local store is one directory; another "
                "bucket is another directory, and writing it under this one "
                "would make a local run pass over the destination a deployed run "
                "would get wrong. Map it with "
                f"Local(root, buckets={{{bucket!r}: Path(...)}})."
            )
        return Local(root=root, buckets=self.buckets)


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

    def put_stream(self, key: str, write: Callable[[IO[bytes]], None]) -> None:
        """A multipart upload driven by the encoder, holding one part at a time.

        **Multipart rather than `upload_fileobj`**, and the reason is the shape
        of the two APIs rather than a preference: `upload_fileobj` *pulls* from a
        reader, and an encoder *pushes* into a sink, so bridging the two needs a
        thread and a pipe — which is exactly what Go does (`io.Pipe` plus a
        goroutine, `s3_device_writter.go:44`) and exactly what this node's
        execution model is built to avoid. A multipart upload takes bytes as they
        arrive, so the push seam needs no concurrency at all.

        What this costs against Go, stated rather than discovered: Go's
        `transfermanager` uploads its parts with `Concurrency = 10`, and these go
        one at a time. That is throughput and never bytes — the object is
        identical — and it is the same trade the whole node makes.
        """
        key = validate_key(key)
        client = self.client()
        started = client.create_multipart_upload(  # type: ignore[attr-defined]
            Bucket=self.bucket, Key=key, **self._extra()
        )
        upload_id = started["UploadId"]
        sink = _MultipartSink(client, self.bucket, key, upload_id)
        try:
            write(sink)
            parts = sink.finish()
        except BaseException:
            client.abort_multipart_upload(  # type: ignore[attr-defined]
                Bucket=self.bucket, Key=key, UploadId=upload_id
            )
            raise
        client.complete_multipart_upload(  # type: ignore[attr-defined]
            Bucket=self.bucket,
            Key=key,
            UploadId=upload_id,
            MultipartUpload={"Parts": parts},
        )

    def for_bucket(self, bucket: str | None) -> ObjectStore:
        if is_own_bucket(bucket):
            return self
        assert bucket is not None
        # The client is per-service and not per-bucket, so it is shared rather
        # than rebuilt: a second client would be a second credential chain
        # resolution for no difference in what is addressed.
        return S3(
            bucket=bucket,
            region=self.region,
            kms_key_arn=self.kms_key_arn,
            _client=self._client,
        )


#: `transfermanager`'s `PartSizeBytes` at both JetStore call sites
#: (`awsi.go:602` and `:619`), so a part this node uploads is the size a part the
#: Go engine uploads is. It bounds what `put_stream` holds.
S3_PART_SIZE_BYTES = 64 * 1024 * 1024


class _MultipartSink(io.RawIOBase):
    """The writable half of an S3 multipart upload.

    A `RawIOBase` because pyarrow wraps whatever it is handed in its own
    `PythonFile` and reads `closed`, `flush` and `tell` off it; inheriting them
    is cheaper and less wrong than writing four one-line methods.
    """

    def __init__(self, client: Any, bucket: str, key: str, upload_id: str) -> None:
        super().__init__()
        self._client = client
        self._bucket = bucket
        self._key = key
        self._upload_id = upload_id
        self._buffer = bytearray()
        self._parts: list[dict[str, Any]] = []
        self._offset = 0

    def writable(self) -> bool:
        return True

    def write(self, data: Any) -> int:  # type: ignore[override]
        chunk = bytes(data)
        self._buffer.extend(chunk)
        self._offset += len(chunk)
        while len(self._buffer) >= S3_PART_SIZE_BYTES:
            self._upload(bytes(self._buffer[:S3_PART_SIZE_BYTES]))
            del self._buffer[:S3_PART_SIZE_BYTES]
        return len(chunk)

    def tell(self) -> int:
        return self._offset

    def _upload(self, body: bytes) -> None:
        number = len(self._parts) + 1
        result = self._client.upload_part(
            Bucket=self._bucket,
            Key=self._key,
            UploadId=self._upload_id,
            PartNumber=number,
            Body=body,
        )
        self._parts.append({"ETag": result["ETag"], "PartNumber": number})

    def finish(self) -> list[dict[str, Any]]:
        """Upload what is left and return the part list.

        The remainder goes up **unconditionally**, including when it is empty and
        no part has gone before it: a multipart upload with no parts cannot be
        completed, and a partition with no rows never reaches here anyway
        (`PartitionWriterPipe._flush` returns first).
        """
        if self._buffer or not self._parts:
            self._upload(bytes(self._buffer))
            self._buffer.clear()
        return list(self._parts)


def keys_under(store: ObjectStore, prefixes: Iterable[str]) -> tuple[str, ...]:
    """Every key under any of `prefixes`, sorted and de-duplicated."""
    found: set[str] = set()
    for prefix in prefixes:
        found.update(store.list(prefix))
    return tuple(sorted(found))
