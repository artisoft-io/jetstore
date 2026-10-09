"""`R10`: the token diff, and the baseline that makes it able to go red.

**The mutation tests are the point of this file.** A baseline check that has
never been seen to fail is a claim about what it would detect, and this
repository's own rule is that such a claim is checkable by breaking the thing
on purpose. So the Go side is copied to a fixture, a token is added, a token is
removed, a token is taken on by the node, and each is asserted to name what
moved -- rather than asserting that the check passes today, which it would do
just as happily with the comparison deleted.

**Three of these tests exist because two other tracks are editing the file this
one parses.** `AA` is adding field keys to `ConditionalPipeSpec`'s contract
entry and `AE` may be removing the `CsvSourceSpec/csv_file` discriminator, and
neither is a change to the operator token universe. If either could move a
number here, that is a defect in the instrument rather than in their work, so
it is asserted rather than assumed.
"""

from __future__ import annotations

import json
import re
from pathlib import Path

import pytest

from cpipes_node import contract, main, token_diff
from cpipes_node.scope import Operator, TokenKind
from cpipes_node.token_diff import TokenDiffError

GO_CONTRACT = Path(__file__).resolve().parents[2] / token_diff.GO_CONTRACT_RELPATH
BASELINE = token_diff.BASELINE_PATH


# --- the measurement --------------------------------------------------------


def test_the_go_side_is_where_the_instrument_looks_for_it():
    assert GO_CONTRACT.is_file(), (
        f"{GO_CONTRACT} is missing; the default --jetstore derivation in "
        "token_diff.py assumes tools/cpipes_node lives two levels under the "
        "JetStore root."
    )


def test_the_difference_is_where_the_baseline_says_it_is():
    """The green run, and the only assertion here that is about today.

    It is a *set* comparison rather than a count for the reason the module
    docstring gives: `aggregate` leaving JetStore on the day `transmogrify`
    joined it nets to zero.
    """
    diff = token_diff.compute()
    moved = token_diff.movements(diff, token_diff.load_baseline())
    assert moved == (), [
        f"{m.kind}: {m.token} {m.direction} {m.set_name}" for m in moved
    ]


def test_the_transformation_baseline_is_the_one_the_plan_declares():
    """16 / 0 / 3, re-derived rather than copied.

    `F37` measured this on 2026-09-22 against a tree that has since moved, so
    this asserts the plan's declared number against the checkout rather than
    restating it. The other two kinds are asserted by the baseline file; this
    one is spelled out because it is the number three documents carry.
    """
    entry = token_diff.compute().by_kind("transformation")
    assert (len(entry.go_only), len(entry.node_only), len(entry.both)) == (16, 0, 3)
    assert entry.both == ("filter", "map_record", "partition_writer")


def test_the_generated_go_file_agrees_with_the_model_it_comes_from():
    """The invariant that makes the Go side worth reading at all.

    `cpipes_model.py` is the source of truth for the contract and the Go file
    is generated from the matrix, so a disagreement means this instrument's
    JetStore side is a claim about a stale artefact. It is a *measurement*
    failure rather than drift, and `--check` reports it as one.
    """
    assert token_diff.compute().disagreements == ()


def test_the_virtual_tokens_are_not_part_of_the_universe():
    """`~override` and `~site` are discriminator outcomes, not authored tokens.

    Reading them as tokens would put two permanent JetStore-only entries in the
    diff that no node could ever take on.
    """
    tokens = token_diff.go_tokens(GO_CONTRACT)
    assert not [t for kind in tokens for t in tokens[kind] if t.startswith("~")]
    assert "~site" in GO_CONTRACT.read_text(), "the fixture stopped having one"


def test_every_run_states_what_it_does_not_cover(capsys):
    """`AG.2`, asserted as output rather than as documentation.

    The clause names `P9-I117` and `P9-I02` because those are the two claims a
    reader would otherwise take this instrument to have settled.
    """
    assert main.main(["tokens"]) == main.EXIT_OK
    printed = capsys.readouterr().out
    assert "What this does NOT cover" in printed
    assert "P9-I117" in printed and "P9-I02" in printed
    assert "NARROWED by this instrument and not closed" in printed
    # And on the checking path too, which is the one somebody runs in CI and
    # the one whose green result is most likely to be read as an all-clear.
    assert main.main(["tokens", "--check"]) == main.EXIT_OK
    assert "What this does NOT cover" in capsys.readouterr().out


# --- the mutations ----------------------------------------------------------


@pytest.fixture
def go_fixture(tmp_path) -> Path:
    """A copy of the real Go enumeration, for a test to break on purpose."""
    target = tmp_path / "cpipes_contract_data.go"
    target.write_text(GO_CONTRACT.read_text(), encoding="utf-8")
    return target


def _add_token(path: Path, token: str) -> None:
    anchor = '\t"TransformationSpec/vllm": {'
    text = path.read_text()
    assert anchor in text
    entry = f'\t"TransformationSpec/{token}": {{\n\t\t"comment":        {{}},\n\t}},\n'
    path.write_text(text.replace(anchor, entry + anchor, 1))


def _remove_token(path: Path, token: str) -> None:
    text = path.read_text()
    match = re.search(
        rf'^\t"TransformationSpec/{re.escape(token)}": \{{.*?^\t\}},\n',
        text,
        re.MULTILINE | re.DOTALL,
    )
    assert match, token
    path.write_text(text[: match.start()] + text[match.end() :])


@pytest.fixture
def agreeing_model(monkeypatch):
    """Let a test move the Go side and the model together.

    A mutation of the Go file alone is *also* a real failure -- somebody
    hand-edited a generated file -- and `--check` reports it as a failure to
    measure. But the event this instrument exists for is a token landing in
    JetStore properly, through `cpipes_model.py` and the regeneration, and that
    event has the two agreeing. So the census follows the fixture here, and the
    Go-only mutation gets its own test below.
    """

    def follow(path: Path):
        census = dict(contract.contract_token_census())
        census.update(token_diff.go_tokens(path))
        monkeypatch.setattr(contract, "contract_token_census", lambda: census)

    return follow


def test_a_twentieth_jetstore_token_goes_red_naming_it(
    go_fixture, agreeing_model, capsys
):
    _add_token(go_fixture, "transmogrify")
    agreeing_model(go_fixture)

    code = main.main(["tokens", "--check", "--go-contract", str(go_fixture)])

    assert code == main.EXIT_BASELINE_MOVED
    err = capsys.readouterr().err
    assert "transmogrify" in err
    assert "entered `JetStore only`" in err
    # The deliverable is the message, and a count is not actionable: it has to
    # read as a routing warning.
    assert "use_python_node_when" in " ".join(err.split())
    assert "routing warning" in " ".join(err.split())
    assert "aborts the run at startup" in " ".join(err.split())


def test_a_jetstore_token_disappearing_goes_red_too(go_fixture, agreeing_model, capsys):
    """The half a check that only notices additions does not have.

    A count would net an addition and a removal to zero; the set does not.
    """
    _remove_token(go_fixture, "sort")
    agreeing_model(go_fixture)

    code = main.main(["tokens", "--check", "--go-contract", str(go_fixture)])

    assert code == main.EXIT_BASELINE_MOVED
    err = capsys.readouterr().err
    assert "`sort` left `JetStore only`" in err
    # And it says which of the two events it was: JetStore dropped it, rather
    # than this node having taken it on.
    assert "JetStore no longer declares it at all" in " ".join(err.split())


def test_an_addition_and_a_removal_do_not_cancel(go_fixture, agreeing_model, capsys):
    _add_token(go_fixture, "transmogrify")
    _remove_token(go_fixture, "sort")
    agreeing_model(go_fixture)

    assert (
        main.main(["tokens", "--check", "--go-contract", str(go_fixture)])
        == main.EXIT_BASELINE_MOVED
    )
    err = capsys.readouterr().err
    assert "transmogrify" in err and "sort" in err


def test_a_token_leaving_both_is_named(go_fixture, agreeing_model, capsys):
    """JetStore drops a token this node implements.

    `filter` is in `both`, so removing it from the Go side moves it into `node
    only` -- a declaration that is now dead, which is a different repair from
    either of the others.
    """
    _remove_token(go_fixture, "filter")
    agreeing_model(go_fixture)

    assert (
        main.main(["tokens", "--check", "--go-contract", str(go_fixture)])
        == main.EXIT_BASELINE_MOVED
    )
    err = capsys.readouterr().err
    assert "`filter` entered `node only`" in err
    assert "`filter` left `both`" in err
    # Normalised, because the consequence paragraph is wrapped for a terminal
    # and a fragment of it straddles a line break.
    assert "so the declaration is dead" in " ".join(err.split())


def test_the_node_taking_a_token_on_goes_red_as_well(capsys):
    """The movement this project would actually like to see, and it is still red.

    A baseline is a pin on the *difference*, not a floor under it. Narrowing the
    gap is as much a change to what may be routed where as widening it, and the
    repair -- move the baseline and say why -- is the same act.
    """

    class Aggregate(Operator):
        kind = TokenKind.TRANSFORMATION
        token = "aggregate"

        @classmethod
        def build(cls, env, spec):  # pragma: no cover - never called here
            return None

    from cpipes_node import scope

    try:
        code = main.main(["tokens", "--check"])
    finally:
        del scope._REGISTRY[(TokenKind.TRANSFORMATION, "aggregate")]

    assert code == main.EXIT_BASELINE_MOVED
    err = capsys.readouterr().err
    assert "`aggregate` left `JetStore only`" in err
    assert "`aggregate` entered `both`" in err
    # And the `both` message is where P9-I117 has to be said again: two
    # implementations of one token agreeing is not what this checks.
    assert "P9-I117" in err


def test_an_unchanged_tree_against_an_untouched_baseline_stays_green(go_fixture):
    """The other direction of the mutation proof.

    A check that goes red on a mutation and also goes red on nothing is not a
    check. The fixture is a byte-for-byte copy at a different path, so this also
    shows the result does not depend on where the file was read from.
    """
    assert main.main(["tokens", "--check", "--go-contract", str(go_fixture)]) == (
        main.EXIT_OK
    )


def test_a_go_only_mutation_is_reported_as_a_stale_artefact(go_fixture, capsys):
    """A hand-edited generated file is a third outcome, not drift.

    Exit 3 rather than 4, because the Go file disagreeing with the model it is
    generated from means *which way the difference moved* is not established.
    The token is still named, because it is the lead a reader follows.
    """
    _add_token(go_fixture, "transmogrify")

    code = main.main(["tokens", "--check", "--go-contract", str(go_fixture)])

    assert code == main.EXIT_REFUSED
    err = capsys.readouterr().err
    assert "transmogrify" in err
    assert "disagrees with cpipes_model.py" in err


# --- the instrument's own failure modes -------------------------------------


def test_a_parse_that_matches_nothing_is_refused_rather_than_reported(tmp_path):
    """The failure that would otherwise read as total drift.

    A regex that stopped matching returns empty sets, which look exactly like
    JetStore having dropped every token it has. It is refused instead, and the
    refusal is exit 3 rather than the 4 a moved baseline gets.
    """
    empty = tmp_path / "cpipes_contract_data.go"
    empty.write_text("package compute_pipes\n")
    with pytest.raises(TokenDiffError, match="no contract entry at all"):
        token_diff.compute(go_contract=empty)
    assert main.main(["tokens", "--check", "--go-contract", str(empty)]) == (
        main.EXIT_REFUSED
    )


def test_a_parse_that_loses_one_kind_is_refused(go_fixture):
    """Half a parse is worse than none: it reports one kind as wholly removed."""
    text = go_fixture.read_text()
    go_fixture.write_text(
        re.sub(r'^\t"PipeSpec/', '\t"PipeSpecX/', text, flags=re.MULTILINE)
    )
    with pytest.raises(TokenDiffError, match="no token at all for: pipe"):
        token_diff.compute(go_contract=go_fixture)


def test_a_missing_file_names_the_flag_that_fixes_it(tmp_path):
    with pytest.raises(TokenDiffError, match="--go-contract"):
        token_diff.compute(go_contract=tmp_path / "nowhere.go")


def test_a_baseline_that_is_not_one_is_refused(tmp_path):
    bad = tmp_path / "baseline.json"
    bad.write_text(json.dumps({"measured": "2026-09-21"}))
    with pytest.raises(TokenDiffError, match="not a token-diff baseline"):
        token_diff.movements(token_diff.compute(), token_diff.load_baseline(bad))


# --- insensitivity to the two tracks editing the same file ------------------


def test_a_field_added_to_another_structs_entry_moves_nothing(go_fixture):
    """Track `AA` adds `use_python_node` and `use_python_node_when`.

    Two map entries inside `"ConditionalPipeSpec/*"`, which is a *field* list
    and not a token. If this moved a number, the instrument would be reporting
    drift every time anybody added a field to any struct.
    """
    before = token_diff.go_tokens(go_fixture)
    text = go_fixture.read_text()
    anchor = '\t"ConditionalPipeSpec/*": {\n'
    assert anchor in text
    go_fixture.write_text(
        text.replace(
            anchor,
            anchor + '\t\t"use_python_node":      {},\n'
            '\t\t"use_python_node_when": {},\n',
            1,
        )
    )
    assert token_diff.go_tokens(go_fixture) == before


def test_removing_the_csv_file_discriminator_moves_nothing(go_fixture):
    """Track `AE` may remove `CsvSourceSpec/csv_file`.

    It is a discriminator on a source spec rather than an operator token, so it
    is outside `contract.TOKEN_STRUCTS` and cannot reach the three sets. Pinned
    rather than argued, because "it is a different struct" is exactly the kind
    of claim that is true until somebody widens the regex.
    """
    text = go_fixture.read_text()
    assert '\t"CsvSourceSpec/csv_file": {' in text
    before = token_diff.go_tokens(go_fixture)
    match = re.search(
        r'^\t"CsvSourceSpec/csv_file": \{.*?^\t\},\n', text, re.MULTILINE | re.DOTALL
    )
    assert match
    go_fixture.write_text(text[: match.start()] + text[match.end() :])
    assert token_diff.go_tokens(go_fixture) == before


def test_gofmt_realignment_of_a_field_block_moves_nothing(go_fixture):
    """The entries are matched at one tab; fields sit at two and are realigned.

    gofmt re-pads a field block whenever its widest key changes, which `AA`'s
    two additions will do. The anchor is what makes that a non-event.
    """
    before = token_diff.go_tokens(go_fixture)
    text = go_fixture.read_text()
    # Collapse every field line's alignment padding to a single space.
    squashed = re.sub(r'^(\t\t"[^"]+":) +', r"\1 ", text, flags=re.MULTILINE)
    assert squashed != text
    go_fixture.write_text(squashed)
    assert token_diff.go_tokens(go_fixture) == before


# --- the baseline file itself -----------------------------------------------


def test_the_baseline_carries_the_date_it_was_measured():
    """A number measured from a tree this file does not pin is dated or it rots."""
    baseline = token_diff.load_baseline()
    assert re.fullmatch(r"\d{4}-\d{2}-\d{2}", baseline["measured"])


def test_writing_the_baseline_round_trips(tmp_path, go_fixture):
    written = tmp_path / "baseline.json"
    assert (
        main.main(
            [
                "tokens",
                "--go-contract",
                str(go_fixture),
                "--baseline",
                str(written),
                "--write-baseline",
                "2026-09-21",
            ]
        )
        == main.EXIT_OK
    )
    assert json.loads(written.read_text()) == json.loads(BASELINE.read_text())
