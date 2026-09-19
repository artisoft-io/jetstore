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
from cpipes_node.store import S3, Local, ObjectStore, keys_under


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
