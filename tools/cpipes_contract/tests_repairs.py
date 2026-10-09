"""Checks on the repair-case library. `python -m pytest tests_repairs.py`.

**The library's own falsifier — `cpipes-contract repairs --check` — runs against
real history and is the stronger check**, because it re-derives every recorded
class from the workspace submodules rather than from a fixture. What it cannot do
is exercise the edges: it only sees the shapes the corpus happens to contain, and
on the day it was written the corpus contained no `mixed` case at all.

So these tests hold the classification rules themselves, and one of them holds a
decision that would otherwise be prose only: `classify` uses strict subset in both
directions, so a repair that both removes and adds a schema error is `mixed` and
not quietly counted as a fix.
"""

import json
from pathlib import Path

from cpipes_contract import repairs as R

# A schema small enough to reason about: `a` is required, and `n` must be a number.
SCHEMA = {
    "type": "object",
    "required": ["a"],
    "properties": {"n": {"type": "number"}},
}


def test_a_repair_the_schema_cannot_see_is_contract_blind():
    before = {"a": 1, "sort": {"expr": "x"}}
    after = {"a": 1, "sort": {"sort_by": ["x"]}}
    assert R.classify(before, after, SCHEMA) == R.CONTRACT_BLIND


def test_a_repair_that_removes_an_error_is_contract_visible():
    assert R.classify({"n": "not a number"}, {"a": 1, "n": 2}, SCHEMA) == R.CONTRACT_VISIBLE


def test_a_fix_that_adds_an_error_is_a_regression():
    assert R.classify({"a": 1}, {"a": 1, "n": "no"}, SCHEMA) == R.REGRESSION


def test_removing_one_error_while_adding_another_is_mixed():
    """**The reason `classify` uses a strict subset rather than a count.**

    A repair that trades one error for another leaves the total unchanged, so a
    length comparison would call this a clean fix. It is not one, and a human
    should look at it.
    """
    before = {"n": "no"}          # missing `a`, and `n` is wrong
    after = {"a": 1, "n": "still no"}  # `a` fixed, `n` still wrong
    # after ⊂ before here, so this pair is visible rather than mixed; build a true
    # mixed pair by moving the defect instead of removing it.
    assert R.classify(before, after, SCHEMA) == R.CONTRACT_VISIBLE
    moved_before = {"a": 1, "n": "no"}
    moved_after = {"n": 2}  # `n` fixed, `a` now missing
    assert R.classify(moved_before, moved_after, SCHEMA) == R.MIXED


def test_error_set_is_keyed_on_where_and_which_rule_not_on_the_message():
    """Two documents failing the same rule at the same place are one error."""
    assert R.error_set({"n": "a"}, SCHEMA) == R.error_set({"n": "b"}, SCHEMA)


def test_diff_sites_reports_a_replaced_object_once():
    """**A site is the defect, not every leaf underneath it.**

    Descending into a replaced object would report a shower of leaf differences
    and make the case unreadable, which defeats the point of storing sites at all.
    """
    sites = list(R.diff_sites({"k": {"a": 1, "b": 2}}, {"k": [1, 2]}))
    assert [s.path for s in sites] == [["k"]]


def test_diff_sites_descends_into_a_matching_shape():
    sites = list(R.diff_sites({"k": {"a": 1, "b": 2}}, {"k": {"a": 1, "b": 3}}))
    assert [s.path for s in sites] == [["k", "b"]]
    assert (sites[0].before, sites[0].after) == (2, 3)


def test_a_list_whose_length_changed_is_one_site():
    sites = list(R.diff_sites({"k": [1, 2]}, {"k": [1, 2, 3]}))
    assert [s.path for s in sites] == [["k"]]


def test_a_record_round_trips():
    case = R.Case(
        name="ws-abc1234", workspace="jets_ws", sha="abc1234", files=["pipes_config/x.pc.json"],
        cls=R.CONTRACT_BLIND, diagnosis="why", sites=[R.Site(["k"], 1, 2)],
    )
    back = R.from_record(json.loads(json.dumps(case.to_record())))
    assert (back.name, back.cls, back.provenance) == (case.name, case.cls, R.PROVENANCE)
    assert [s.path for s in back.sites] == [["k"]]


def test_the_repair_word_test_matches_across_a_hyphen():
    """**The bug this test exists for.** The first version split the subject on
    whitespace, so "error-channel" did not match "error" -- silently dropping the
    sweep commit that supplies most of the contract-visible cases."""
    assert R._looks_like_repair("Remove inert sampling_max_count from error-channel inputs")
    assert R._looks_like_repair("Fixed the lookup table query")
    assert not R._looks_like_repair("Add pipeline coordination for DataProfiling")


def test_every_committed_case_declares_a_known_class_and_provenance():
    """A cheap structural check on the committed library, with no git access."""
    lib = Path(__file__).parent / "repairs" / "library.jsonl"
    cases = R.load(lib)
    assert cases, "the library is empty"
    for c in cases:
        assert c.cls in R.CLASSES, f"{c.name}: unknown class {c.cls}"
        assert c.provenance == R.PROVENANCE, f"{c.name}: wrong provenance"
        assert c.files, f"{c.name}: no files"
        assert c.diagnosis.strip(), f"{c.name}: no diagnosis"
