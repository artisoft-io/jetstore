"""X6: the declared scope, and that a token outside it aborts by name.

**The universe is derived, not listed.** Every test here that needs "the
tokens JetStore has" reads them off `cpipes_model._MATRIX_KEYS` through
`contract.contract_tokens`. A hand-kept list of the nineteen would make
`test_every_contract_token_outside_the_scope_is_refused` pass over a twentieth
the day JetStore added one — which is the shape of failure this repository has
recorded thirty-seven times, and the one the phase's highest risk (P9-R01) runs
straight into.

**One assertion here is deliberately a literal**, and it is
`test_the_declared_scope_is_the_subset_the_charter_names`. The scope is a
*decision* rather than a derivation; pinning it is what makes widening it an
act somebody has to take on purpose. It carries the charter's own six and the
two the charter's pipeline cannot exist without, and it names P9-I26.
"""

from __future__ import annotations

import logging

import pytest

from cpipes_node import contract, scope, site
from cpipes_node.errors import OperatorNotImplemented, OperatorOutOfScope
from cpipes_node.scope import FindingKind, Operator, TokenKind

CHARTER_SCOPE = {
    TokenKind.INPUT_CHANNEL: ("generator", "memory"),
    TokenKind.PIPE: ("fan_out", "merge_files"),
    TokenKind.TRANSFORMATION: ("filter", "map_record", "partition_writer"),
}


def test_the_registry_is_populated_by_reading_it():
    # A registry filled by import is empty when nobody imported, and an empty
    # scope makes the gate pass over everything while reporting a clean run.
    # `declared_scope` imports the declarations itself; this asserts that it
    # does, from a module that imported no operator.
    declared = scope.declared_scope()
    assert sum(len(tokens) for tokens in declared.values()) == 7


def test_the_declared_scope_is_the_subset_the_charter_names():
    assert scope.declared_scope() == CHARTER_SCOPE


def test_every_declared_token_is_one_the_contract_knows():
    # The other direction of the gate: a scope entry naming a token JetStore
    # does not have would be dead forever and nothing else would notice.
    for kind, tokens in scope.declared_scope().items():
        known = contract.contract_tokens(kind.value)
        assert set(tokens) <= set(known), f"{kind}: {set(tokens) - set(known)}"


def test_every_contract_token_outside_the_scope_is_refused():
    declared = scope.declared_scope()
    refused = 0
    for kind in TokenKind:
        for token in contract.contract_tokens(kind.value):
            if token in declared[kind]:
                continue
            finding = scope.classify(
                kind, token, "$", contract_tokens=contract.contract_tokens(kind.value)
            )
            assert finding is not None, f"{kind} '{token}' is neither declared nor out"
            assert finding.category is FindingKind.OUT_OF_SCOPE
            assert token in str(finding)
            refused += 1
    # 26 addressable tokens in the contract, 7 declared. A count rather than a
    # bare loop, because a loop over an empty set is the failure this file is
    # about.
    assert refused == 19
    assert sum(len(t) for t in contract.contract_token_census().values()) == 26


def test_a_token_nobody_has_is_refused_with_a_different_reason():
    # The twenty-ninth operator, in the plainest form: a token no engine knows.
    # Its repair is the author's, where a built-in's is a scope decision, so
    # the two must not read the same.
    typo = scope.classify(
        TokenKind.TRANSFORMATION,
        "map_recrod",
        "$.apply[0]",
        contract_tokens=contract.contract_tokens("transformation"),
    )
    builtin = scope.classify(
        TokenKind.TRANSFORMATION,
        "jetrules",
        "$.apply[0]",
        contract_tokens=contract.contract_tokens("transformation"),
    )
    assert typo is not None and builtin is not None
    assert typo.category is builtin.category is FindingKind.OUT_OF_SCOPE
    assert typo.reason != builtin.reason
    assert "site operator" in typo.reason
    assert "P9-I04" in builtin.reason


def test_a_twenty_ninth_operator_that_did_not_join_the_scope_fails_loudly():
    # JetStore adds a built-in; this node does not. The scope does not grow by
    # the contract growing, and the abort names the token.
    report = scope.ScopeReport()
    report.out_of_scope.append(
        scope.Finding(
            FindingKind.OUT_OF_SCOPE,
            TokenKind.TRANSFORMATION,
            "transmogrify",
            "$.pipes_config[0].apply[0]",
            "neither a declared built-in nor a registered site operator",
        )
    )
    with pytest.raises(OperatorOutOfScope) as exc:
        report.raise_if_out_of_scope()
    assert "transmogrify" in str(exc.value)
    assert "the scope searched" in str(exc.value)


def test_a_declared_token_with_no_build_is_a_different_refusal():
    finding = scope.classify(TokenKind.TRANSFORMATION, "map_record", "$")
    assert finding is not None
    assert finding.category is FindingKind.UNIMPLEMENTED
    assert "P9-T06" in finding.reason
    report = scope.ScopeReport(unimplemented=[finding])
    with pytest.raises(OperatorNotImplemented):
        report.raise_if_unimplemented()
    # And the two refusals are catchable apart, which is what "distinguishable"
    # has to mean for a caller that is not reading the message.
    assert not issubclass(OperatorNotImplemented, OperatorOutOfScope)
    assert not issubclass(OperatorOutOfScope, OperatorNotImplemented)


def test_an_implemented_declaration_passes(monkeypatch):
    class Implemented(Operator):
        kind = TokenKind.TRANSFORMATION
        token = "sort"

        @classmethod
        def build(cls, env, spec):
            return "built"

    try:
        assert Implemented.implemented() is True
        assert scope.classify(TokenKind.TRANSFORMATION, "sort", "$") is None
        assert Implemented.build(None, None) == "built"
    finally:
        del scope._REGISTRY[(TokenKind.TRANSFORMATION, "sort")]


def test_implementedness_is_derived_and_not_declared():
    """Nothing carries a flag, so the two cannot disagree.

    The first half is the derivation and holds for every declaration. The second
    was `assert not cls.implemented()` with the note *update this file when the
    first one lands*, and P9-T04 is when: `generator`, `memory` and `fan_out` have
    a `build` and the other five do not. **The census is asserted rather than the
    absence**, so a token gaining or losing an implementation is a failure here
    and not a silence — and every unimplemented one still has to name its owner,
    which is the half that was always about the declaration rather than about
    progress.
    """
    implemented: list[str] = []
    for cls in scope.declarations():
        assert cls.implemented() == ("build" in cls.__dict__)
        if cls.implemented():
            implemented.append(cls.token)
        else:
            assert cls.owed_by, f"{cls.__name__} declares a stub owing nobody"
    # P9-T04's three — the two input-channel types and the one pipe kind the
    # graph runs — **and `merge_files`, which P9-T08 added**. The three
    # transformations are P9-T06's and P9-T07's.
    #
    # This line read `["fan_out", "generator", "memory"]` until P9-T08 landed,
    # and it is **inverted rather than loosened**: the census is the assertion,
    # so the day a token gains an implementation this test goes red and somebody
    # states the new census. Relaxing it to a subset check would have gone green
    # over P9-T08 and over every task after it.
    assert sorted(implemented) == ["fan_out", "generator", "memory", "merge_files"]


def test_the_base_build_refuses_and_names_the_task():
    from cpipes_node.operators.transformations import PartitionWriter

    with pytest.raises(OperatorNotImplemented) as exc:
        PartitionWriter.build(None, None)
    assert "partition_writer" in str(exc.value)
    assert "P9-T07" in str(exc.value)


def test_two_classes_cannot_claim_one_token():
    with pytest.raises(TypeError, match="two operators declare"):

        class Duplicate(Operator):
            kind = TokenKind.TRANSFORMATION
            token = "map_record"

            @classmethod
            def build(cls, env, spec):  # pragma: no cover - never reached
                return None


def test_a_stub_must_name_the_task_that_owes_it():
    with pytest.raises(TypeError, match="no build and no owed_by"):

        class Orphan(Operator):
            kind = TokenKind.TRANSFORMATION
            token = "distinct"


def test_an_intermediate_base_declares_nothing():
    before = scope.declared_scope()

    class Intermediate(Operator):
        kind = TokenKind.PIPE

    assert scope.declared_scope() == before


def test_the_abort_message_names_the_whole_scope():
    rendered = scope.render_scope()
    for kind, tokens in scope.declared_scope().items():
        for token in tokens:
            assert f"{kind}: {token}" in rendered
    assert "site operator" in rendered


def test_a_registered_site_operator_is_in_scope():
    registry = site.Registry().with_operators({"healthcare_corpus": lambda e, s: None})
    assert registry.tokens() == frozenset({"healthcare_corpus"})
    assert (
        scope.classify(
            TokenKind.TRANSFORMATION,
            "healthcare_corpus",
            "$",
            site_tokens=registry.tokens(),
        )
        is None
    )
    # ...and only for a transformation. A site registration cannot make a pipe
    # kind or a channel type appear.
    assert (
        scope.classify(
            TokenKind.PIPE, "healthcare_corpus", "$", site_tokens=registry.tokens()
        )
        is not None
    )


def test_a_site_operator_cannot_shadow_a_declared_built_in(caplog):
    # `WithOperators` keeps the entry and warns rather than refusing, because a
    # deployment's main failing to start over an operator it will never reach
    # is worse than the operator never being reached. Mirrored.
    with caplog.at_level(logging.WARNING):
        registry = site.Registry().with_operators({"map_record": lambda e, s: None})
    assert "map_record" in registry.tokens()
    assert any("built-in" in r.getMessage() for r in caplog.records)
    # The built-in still wins: `classify` asks the declaration registry first.
    finding = scope.classify(
        TokenKind.TRANSFORMATION, "map_record", "$", site_tokens=registry.tokens()
    )
    assert finding is not None and finding.category is FindingKind.UNIMPLEMENTED


def test_an_unreachable_registration_is_dropped(caplog):
    with caplog.at_level(logging.WARNING):
        registry = site.Registry().with_operators(
            {"": lambda e, s: None, "null": None, "ok": lambda e, s: None}
        )
    assert registry.tokens() == frozenset({"ok"})


def test_the_scope_is_ordered_and_not_a_set():
    # Printed, compared and asserted against, so its order is a fact about the
    # node rather than about a hash seed.
    for tokens in scope.declared_scope().values():
        assert list(tokens) == sorted(tokens)
    assert [c.token for c in scope.declarations()] == [
        t for kind in TokenKind for t in scope.declared_scope()[kind]
    ]
