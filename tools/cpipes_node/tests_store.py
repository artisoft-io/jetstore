"""The object store seam: one protocol, a directory and a bucket.

The local store is held to S3's *key* rules and not to a filesystem's, because
the whole value of the seam is that a run that works against a directory is
evidence about a run against a bucket. A key that resolves on a filesystem and
not in a bucket — a leading slash, a `..`, a backslash — is refused here rather
than quietly resolved, so the local path cannot be the permissive one.
"""

from __future__ import annotations

from pathlib import Path

import pytest

from cpipes_node.errors import ObjectNotFound, ObjectStoreError
from cpipes_node.store import (
    JETSTORE_BUCKET,
    S3,
    S3_PART_SIZE_BYTES,
    Local,
    ObjectStore,
    is_own_bucket,
    keys_under,
)


def test_a_local_store_round_trips(tmp_path: Path):
    store = Local(tmp_path)
    store.put("a/b/c.csv", b"x,y\n1,2\n")
    assert store.get("a/b/c.csv") == b"x,y\n1,2\n"
    assert (tmp_path / "a" / "b" / "c.csv").is_file()


def test_a_missing_object_is_not_an_empty_one(tmp_path: Path):
    with pytest.raises(ObjectNotFound, match="absent"):
        Local(tmp_path).get("absent/key.csv")


def test_a_listing_is_sorted(tmp_path: Path):
    store = Local(tmp_path)
    for key in ("p/2.csv", "p/10.csv", "p/1.csv"):
        store.put(key, b"")
    assert store.list("p/") == ("p/1.csv", "p/10.csv", "p/2.csv")


def test_a_prefix_is_matched_as_a_string_and_not_as_a_directory(tmp_path: Path):
    # S3 matches a prefix as a string. A store that matched directories would
    # accept `a/b` and miss `a/bc`, and the partitioned run enumerating its
    # inputs would silently see fewer of them.
    store = Local(tmp_path)
    store.put("part/a.csv", b"")
    store.put("partition/b.csv", b"")
    assert store.list("part") == ("part/a.csv", "partition/b.csv")
    assert store.list("part/") == ("part/a.csv",)


@pytest.mark.parametrize("key", ["", "/leading", "has\\backslash", "a/../b", "./a"])
def test_a_key_that_is_not_an_s3_key_is_refused(tmp_path: Path, key):
    with pytest.raises(ObjectStoreError):
        Local(tmp_path).get(key)


def test_download_and_upload_round_trip(tmp_path: Path):
    store = Local(tmp_path / "bucket")
    source = tmp_path / "local.csv"
    source.write_bytes(b"payload")
    store.upload(source, "stage/local.csv")
    target = tmp_path / "back" / "local.csv"
    store.download("stage/local.csv", target)
    assert target.read_bytes() == b"payload"


def test_both_implementations_satisfy_the_one_protocol():
    # Structural rather than nominal, and asserted for the S3 store too: it is
    # exercised by nothing else in this repository, so this is the only thing
    # standing between a renamed method and a deployed failure.
    assert isinstance(Local(Path(".")), ObjectStore)
    assert isinstance(S3(bucket="b"), ObjectStore)


def test_keys_under_several_prefixes_are_deduplicated(tmp_path: Path):
    store = Local(tmp_path)
    store.put("a/1.csv", b"")
    store.put("b/1.csv", b"")
    assert keys_under(store, ["a/", "b/", "a/"]) == ("a/1.csv", "b/1.csv")


# --- the destination bucket (D-242) -----------------------------------------
#
# Go resolves a partition writer's bucket in two steps and this file holds the
# second: `is_own_bucket` is the *sentinel*, checked at the upload in both Go
# write paths, and `for_bucket` is the binding. The first step — which arms of
# the destination switch consult a `bucket` at all — is the operator's and is in
# `tests_transformations.py`.


@pytest.mark.parametrize("spelling", ["", None, "jetstore_bucket"])
def test_both_of_gos_spellings_mean_the_nodes_own_bucket(spelling):
    """`externalBucket == "" || externalBucket == "jetstore_bucket"`.

    Go writes that condition three times — `awsi.go:598`, `s3_device_worker.go:63`
    and `actions_s3_utils.go:349` — and a node honouring one of the two spellings
    would create a bucket literally named `jetstore_bucket` on the other.
    """
    assert is_own_bucket(spelling) is True


def test_a_named_bucket_is_not_the_nodes_own():
    assert is_own_bucket("corpus-out") is False


def test_the_sentinel_is_the_literal_go_checks_against():
    """Against the Go source, not against recollection."""
    from conftest import go_source

    assert f'externalBucket == "{JETSTORE_BUCKET}"' in go_source("jets/awsi/awsi.go")


def test_for_bucket_returns_the_same_store_for_the_nodes_own(tmp_path: Path):
    """The ordinary case allocates nothing, on both implementations."""
    local = Local(tmp_path)
    s3 = S3(bucket="jets-own", region="us-east-1")
    for spelling in ("", None, "jetstore_bucket"):
        assert local.for_bucket(spelling) is local
        assert s3.for_bucket(spelling) is s3


def test_for_bucket_on_s3_binds_the_name_and_keeps_the_rest():
    """The bucket moves and the region and the KMS key do not.

    Asserted as literals: a `for_bucket` that dropped the KMS key would write
    every object of an external destination unencrypted, and nothing downstream
    would say so.
    """
    s3 = S3(bucket="jets-own", region="us-east-2", kms_key_arn="arn:aws:kms:k")
    other = s3.for_bucket("corpus-out")
    assert isinstance(other, S3)
    assert other.bucket == "corpus-out"
    assert other.region == "us-east-2"
    assert other.kms_key_arn == "arn:aws:kms:k"
    assert s3.bucket == "jets-own"


def test_a_local_store_writes_a_named_bucket_into_its_own_directory(tmp_path: Path):
    """The mapped case: another bucket is another directory, at the same key."""
    own, other = tmp_path / "own", tmp_path / "other"
    store = Local(own, buckets={"corpus-out": other})
    store.for_bucket("corpus-out").put("corpus/0000P/part0000-0000001.csv", b"a,b\n")
    assert (other / "corpus/0000P/part0000-0000001.csv").read_bytes() == b"a,b\n"
    # The negative half: nothing landed in the node's own directory.
    assert Local(own).list("") == ()


def test_a_local_store_refuses_an_unmapped_bucket(tmp_path: Path):
    """The dangerous direction made loud.

    A local store that wrote an unmapped bucket under its own root would pass
    every byte comparison while hiding the one thing the deployed run gets
    wrong — the destination.
    """
    store = Local(tmp_path)
    with pytest.raises(ObjectStoreError, match="corpus-out"):
        store.for_bucket("corpus-out")


def test_a_mapped_bucket_carries_the_mapping_forward(tmp_path: Path):
    """So a second hop resolves, rather than the mapping being lost at depth."""
    store = Local(tmp_path / "own", buckets={"b": tmp_path / "b"})
    assert store.for_bucket("b").for_bucket("b") == Local(tmp_path / "b")


# --- put_stream (D-243) ------------------------------------------------------


def test_put_stream_and_put_write_the_same_object(tmp_path: Path):
    """The flag chooses the path and never the bytes."""
    store = Local(tmp_path)
    store.put("buffered.csv", b"a,b\n1,2\n")
    store.put_stream("streamed.csv", lambda sink: sink.write(b"a,b\n1,2\n"))
    assert store.get("streamed.csv") == store.get("buffered.csv")


def test_put_stream_holds_nothing(tmp_path: Path):
    """The property `stream_data_out` buys, and the one bytes cannot show.

    The object is observable on the store **before the writer has finished**,
    which is only true of a sink that is the destination. A `put_stream` that
    buffered and put at the end would pass every byte comparison and fail this.
    """
    store = Local(tmp_path)
    seen: list[bytes] = []

    def write(sink):
        sink.write(b"first\n")
        sink.flush()
        seen.append((tmp_path / "part.csv").read_bytes())
        sink.write(b"second\n")

    store.put_stream("part.csv", write)
    assert seen == [b"first\n"]
    assert store.get("part.csv") == b"first\nsecond\n"


class FakeS3Client:
    """Enough of the boto3 S3 client for a multipart upload, recording calls.

    **The first thing in this repository to exercise `store.S3` at all.** It is
    not AWS and does not claim to be; what it can prove is the part arithmetic,
    the literal bucket and key on every call, and the abort — which is the half
    a deployment cannot cheaply test and the half a rename breaks silently.
    """

    def __init__(self):
        self.calls: list[tuple] = []
        self.parts: list[bytes] = []

    def create_multipart_upload(self, **kw):
        self.calls.append(("create", kw))
        return {"UploadId": "u-1"}

    def upload_part(self, **kw):
        self.calls.append(("part", kw))
        self.parts.append(kw["Body"])
        return {"ETag": f'"etag-{kw["PartNumber"]}"'}

    def complete_multipart_upload(self, **kw):
        self.calls.append(("complete", kw))
        return {}

    def abort_multipart_upload(self, **kw):
        self.calls.append(("abort", kw))
        return {}


def test_s3_put_stream_uploads_parts_and_completes():
    """One part per `S3_PART_SIZE_BYTES`, then the remainder, then complete."""
    client = FakeS3Client()
    store = S3(bucket="corpus-out", _client=client)
    payload = b"x" * (S3_PART_SIZE_BYTES + 7)
    store.put_stream("corpus/0000P/part0000-0000001.csv", lambda s: s.write(payload))

    kinds = [kind for kind, _ in client.calls]
    assert kinds == ["create", "part", "part", "complete"]
    assert b"".join(client.parts) == payload
    assert [len(p) for p in client.parts] == [S3_PART_SIZE_BYTES, 7]
    for _, kw in client.calls:
        assert kw["Bucket"] == "corpus-out"
        assert kw["Key"] == "corpus/0000P/part0000-0000001.csv"
    assert client.calls[-1][1]["MultipartUpload"] == {
        "Parts": [
            {"ETag": '"etag-1"', "PartNumber": 1},
            {"ETag": '"etag-2"', "PartNumber": 2},
        ]
    }


def test_s3_put_stream_sends_one_part_for_a_small_object():
    client = FakeS3Client()
    S3(bucket="b", _client=client).put_stream("k.csv", lambda s: s.write(b"a,b\n"))
    assert [kind for kind, _ in client.calls] == ["create", "part", "complete"]
    assert client.parts == [b"a,b\n"]


def test_s3_put_stream_carries_the_kms_settings_to_the_upload():
    client = FakeS3Client()
    store = S3(bucket="b", kms_key_arn="arn:aws:kms:k", _client=client)
    store.put_stream("k.csv", lambda s: s.write(b"a"))
    _, created = client.calls[0]
    assert created["ServerSideEncryption"] == "aws:kms"
    assert created["SSEKMSKeyId"] == "arn:aws:kms:k"


def test_s3_put_stream_aborts_when_the_encoder_fails():
    """An abandoned multipart upload is billed until it is aborted or expires.

    Asserted rather than assumed because the failure is invisible: the run has
    already gone red for another reason, and the orphaned parts show up on a
    bill months later.
    """
    client = FakeS3Client()
    store = S3(bucket="b", _client=client)

    def boom(_sink):
        raise ValueError("encoder failed")

    with pytest.raises(ValueError, match="encoder failed"):
        store.put_stream("k.csv", boom)
    assert [kind for kind, _ in client.calls] == ["create", "abort"]
    assert client.calls[-1][1]["UploadId"] == "u-1"


def test_the_part_size_is_the_one_go_uploads_with():
    """`transfermanager`'s `PartSizeBytes` at JetStore's own call site."""
    from conftest import go_source

    assert "o.PartSizeBytes = 64 * 1024 * 1024" in go_source("jets/awsi/awsi.go")
    assert S3_PART_SIZE_BYTES == 64 * 1024 * 1024
