"""Checks on the template model. `python -m pytest tests_template.py` or run directly.

The negative cases matter more than the positive one: every finding `check` claims to
make is a rule that will be leaned on by F.4's expander, and a rule nothing tests is a
comment.
"""

import copy
import json
from pathlib import Path

from cpipes_contract.template import Template, check

HERE = Path(__file__).parent
SCHEMA = json.loads((HERE / "cpipes_schema.json").read_text())
BASE = json.loads((HERE / "templates" / "qc_metrics.template.json").read_text())


def findings(mutate=None):
    doc = copy.deepcopy(BASE)
    if mutate:
        mutate(doc)
    found, _ = check(Template.model_validate(doc), SCHEMA, None, HERE / "matrix")
    return found


def test_the_authored_template_is_clean():
    assert findings() == []


def test_schema_ref_must_be_addressable():
    def m(d):
        d["holes"][0]["schema_ref"] = "NotAType"

    assert any("is not a $defs entry" in f for f in findings(m))


def test_hole_names_are_unique():
    def m(d):
        d["holes"].append(dict(d["holes"][0]))

    assert any("declared more than once" in f for f in findings(m))


def test_every_marker_has_a_declaration():
    def m(d):
        d["holes"] = [h for h in d["holes"] if h["name"] != "partition_output"]

    assert any("undeclared hole" in f for f in findings(m))


def test_every_declaration_is_placed():
    def m(d):
        d["holes"].append({"name": "orphan", "schema_ref": "ColumnMapping", "prompt": "x"})

    assert any("never placed" in f for f in findings(m))


def test_a_repeating_hole_must_sit_in_a_list():
    def m(d):
        d["holes"].append(
            {"name": "bad", "schema_ref": "ColumnMapping", "repeat_over": "x", "prompt": "y"}
        )
        d["body"]["conditional_pipes_config"][0]["pipes_config"][0]["input_channel"] = {
            "$hole": "bad"
        }

    assert any("not a list position" in f for f in findings(m))


def test_nested_holes_are_found_through_a_body():
    """The §5.3.9 case: the inner hole is inside the outer hole's `$body`."""
    tpl = Template.model_validate(copy.deepcopy(BASE))
    paths = dict(tpl.markers())
    assert "$body" in paths["metric_columns"], paths
    outer = next(h for h in tpl.holes if h.name == "metric_operators")
    inner = next(h for h in tpl.holes if h.name == "metric_columns")
    # both levels declare their own repeat, which is the whole point
    assert outer.repeat_over == "metrics"
    assert inner.repeat_over == "input_columns"
    assert paths["metric_columns"].startswith(paths["metric_operators"])




# --- F.4: expansion -------------------------------------------------------------

from cpipes_contract.expand import ExpansionError, expand, from_library  # noqa: E402

LIBRARY = [
    json.loads(line)
    for line in (HERE / "fragments" / "library.curated.jsonl").read_text().splitlines()
    if line.strip()
]
CTX = {
    "metrics": [
        {"channel": "qc_a.mapped", "channel_spec": "qc_metrics"},
        {"channel": "qc_b.mapped", "channel_spec": "qc_metrics"},
    ],
    "input_columns": ["member_id", "dob"],
}


def expanded():
    tpl = Template.model_validate(copy.deepcopy(BASE))
    return expand(tpl, CTX, from_library(LIBRARY, HERE / "matrix"), SCHEMA)


def test_expansion_produces_a_valid_config():
    import jsonschema

    cfg, findings = expanded()
    assert findings == []
    v = jsonschema.Draft202012Validator(
        {"$schema": SCHEMA["$schema"], "$ref": "#/$defs/ComputePipesConfig", "$defs": SCHEMA["$defs"]}
    )
    assert list(v.iter_errors(cfg)) == []


def test_the_output_carries_no_marker_of_being_generated():
    """`compute_pipes` must not be able to tell. No $hole, $body or $item survives."""
    cfg, _ = expanded()
    blob = json.dumps(cfg)
    assert "$hole" not in blob and "$body" not in blob and "$item" not in blob


def test_both_levels_repeat_independently():
    """The §5.3.9 case: outer over metrics, inner over columns, neither inferred."""
    cfg, _ = expanded()
    ops = cfg["conditional_pipes_config"][0]["pipes_config"][0]["apply"]
    assert len(ops) == len(CTX["metrics"])
    for op in ops:
        assert len(op["columns"]) == len(CTX["input_columns"])


def test_a_repeating_hole_splices_rather_than_nests():
    cfg, _ = expanded()
    ops = cfg["conditional_pipes_config"][0]["pipes_config"][0]["apply"]
    assert all(isinstance(o, dict) for o in ops), "expanded into a list of lists"


def test_item_reads_the_current_iteration():
    cfg, _ = expanded()
    ops = cfg["conditional_pipes_config"][0]["pipes_config"][0]["apply"]
    assert [o["output_channel"]["name"] for o in ops] == [m["channel"] for m in CTX["metrics"]]


def test_an_unbound_repeat_over_raises():
    tpl = Template.model_validate(copy.deepcopy(BASE))
    try:
        expand(tpl, {"metrics": CTX["metrics"]}, from_library(LIBRARY, HERE / "matrix"), SCHEMA)
    except ExpansionError as e:
        assert "input_columns" in str(e)
    else:
        raise AssertionError("expected ExpansionError for the unbound collection")


def test_a_bad_fragment_is_reported_and_still_spliced():
    """An author sees the whole result, not the first failure."""
    tpl = Template.model_validate(copy.deepcopy(BASE))
    cfg, findings = expand(tpl, CTX, lambda hole, ctx: {"nonsense": True}, SCHEMA)
    assert findings, "a bad fragment must be reported"
    assert cfg["conditional_pipes_config"], "and the config must still be produced"




# --- F.5: criterion 21 ----------------------------------------------------------

from cpipes_contract.expand import from_target  # noqa: E402

TARGET_PATH = (
    HERE / ".." / ".." / ".." / "workspaces" / "walrus_ws" / "pipes_config" / "map_claim.pc.json"
)


def _round_trip():
    """Expand the map_claim template against the config it was derived from."""
    tpl = Template.model_validate(
        json.loads((HERE / "templates" / "map_claim_load_stages.template.json").read_text())
    )
    ctx = json.loads((HERE / "templates" / "map_claim_load_stages.bindings.json").read_text())
    target = json.loads(TARGET_PATH.read_text())
    cfg, findings = expand(tpl, ctx, from_target(target, key="step_name"), SCHEMA)
    return tpl, target, cfg, findings


def test_criterion_21_the_template_is_clean_and_expands():
    tpl, _, _, findings = _round_trip()
    assert check(tpl, SCHEMA, None, HERE / "matrix")[0] == []
    assert findings == []


def test_criterion_21_the_expansion_validates_under_the_2b_schema():
    import jsonschema

    _, _, cfg, _ = _round_trip()
    v = jsonschema.Draft202012Validator(
        {"$schema": SCHEMA["$schema"], "$ref": "#/$defs/ComputePipesConfig", "$defs": SCHEMA["$defs"]}
    )
    assert list(v.iter_errors(cfg)) == []


def test_criterion_21_the_expansion_regenerates_its_target_exactly():
    """The round trip is what makes the criterion checkable rather than plausible.

    A template derived from a live config, expanded with payload recovered from that
    same config, must reproduce it byte for byte. Anything less and the mechanism is
    losing or inventing something, and no amount of validating would show which.

    `ValidatePipeSpecConfig` is the criterion's other half and is not run from here -
    it needs the Go harness. Both the expansion and the target return
    `{"ok": true, "steps": 8}`, checked 2026-08-20:

        go run ./tools/cpipes_contract/harness < cases.json
    """
    _, target, cfg, _ = _round_trip()
    assert json.dumps(cfg, sort_keys=True) == json.dumps(target, sort_keys=True)


if __name__ == "__main__":
    import sys

    fails = 0
    for name, fn in sorted(globals().items()):
        if name.startswith("test_") and callable(fn):
            try:
                fn()
                print(f"  PASS {name}")
            except AssertionError as e:
                fails += 1
                print(f"  FAIL {name}: {e}")
    print(f"\n{fails} failure(s)")
    sys.exit(1 if fails else 0)


# ---------------------------------------------------------------------------
# Q-15: the prompt is the constraint, not the schema
# ---------------------------------------------------------------------------


def test_the_fit_check_measures_the_prompt_not_the_schema():
    """`ConditionalPipeSpec` is the case that settled Q-15.

    Its JSON Schema is ~49,008 tokens and its prompt ~11,800, against a 24,576 budget.
    The old check sized the schema and refused the hole as unauthorable; the schema
    never occupied the context, because Ollama compiles `format` into a sampling
    grammar. Measured on `granite4.1:3b`, `format` costs 2-3 prompt tokens whether its
    schema is 5,252 or 41,029 - so the number the check used was not a cost at all.
    """
    from cpipes_contract.authoring import estimate_tokens, instruction_for, subschema
    from cpipes_contract.template import Hole

    hole = Hole(
        name="stage",
        schema_ref="ConditionalPipeSpec",
        prompt="A pipeline stage.",
    )
    schema_size = estimate_tokens(
        json.dumps(subschema(SCHEMA, hole.schema_ref), separators=(",", ":"))
    )
    prompt_size = estimate_tokens(instruction_for(hole, SCHEMA, HERE / "matrix"))
    budget = 32768 - 8192

    assert schema_size > budget, "the type this test is about must be over by the old measure"
    assert prompt_size < budget, "and under by the new one, or the test proves nothing"
    assert prompt_size * 3 < schema_size, "the two must differ by more than rounding"


def test_the_examples_are_counted_because_they_are_sent():
    """The other half of the correction: few-shot examples occupy context too.

    For `AnalyzePipe` they are ~8,733 tokens against ~2,987 of TypeScript - nearly three
    times the declarations, and the old check counted neither. A fit check that omits
    the largest term is not conservative, it is wrong in the flattering direction.
    """
    from cpipes_contract.authoring import estimate_tokens, instruction_for
    from cpipes_contract.template import Hole

    library = [
        json.loads(line)
        for line in (HERE / "fragments" / "library.curated.jsonl").read_text().splitlines()
        if line.strip()
    ]
    hole = Hole(name="op", schema_ref="AnalyzePipe", prompt="Analyse the input columns.")
    bare = estimate_tokens(instruction_for(hole, SCHEMA, HERE / "matrix", None))
    shown = estimate_tokens(instruction_for(hole, SCHEMA, HERE / "matrix", library))
    assert shown > bare * 2, "the examples must dominate, or this type is the wrong witness"


def test_every_bundle_is_authorable():
    """The bundle layer's promise, restated against what actually occupies the window.

    F.8 sized the bundles by JSON Schema and reported the worst at 15,513 of 24,576.
    Measured as prompts with their examples, the worst is `AnalyzePipe` at ~11,818 - so
    the promise holds under the corrected measure as well, which was not guaranteed:
    the examples could have eaten the headroom the TypeScript freed.
    """
    import csv

    from cpipes_contract.authoring import estimate_tokens, instruction_for
    from cpipes_contract.template import Hole

    library = [
        json.loads(line)
        for line in (HERE / "fragments" / "library.curated.jsonl").read_text().splitlines()
        if line.strip()
    ]
    budget = 32768 - 8192
    with open(HERE / "matrix" / "bundles.csv", newline="") as fh:
        bundles = [row["bundle"] for row in csv.DictReader(fh) if row["bundle"]]
    over = []
    for name in bundles:
        hole = Hole(name="h", schema_ref=name, prompt="Author one fragment.")
        size = estimate_tokens(instruction_for(hole, SCHEMA, HERE / "matrix", library))
        if size > budget:
            over.append((name, size))
    assert over == [], f"over budget: {over}"


# --- F.7: criterion 23 ----------------------------------------------------------

from cpipes_contract.qc_bindings import derive  # noqa: E402

QC = HERE / ".." / ".." / ".." / "workspaces" / "cedargate_ws" / "pipes_config"
QC_TEMPLATE = Template.model_validate(
    json.loads((HERE / "templates" / "qc_report.template.json").read_text())
)
QC_EXACT = ("qc_hra", "qc_laboratory", "qc_disability", "qc_participation", "qc_eligibility")
QC_NOT_EXACT = ("qc_biometric", "qc_pharmacyclaim", "qc_medicalclaim")


def _qc(name):
    target = json.loads((QC / f"{name}.pc.json").read_text())
    cfg, bad = expand(QC_TEMPLATE, derive(target), lambda h, c: {"$UNFILLED": h.name}, SCHEMA)
    return target, cfg, bad


def test_the_qc_template_is_clean():
    findings, _ = check(QC_TEMPLATE, SCHEMA, None, HERE / "matrix")
    assert findings == []


def test_criterion_23_two_of_the_five_non_exact_reproduce_exactly():
    """§5.3.9 left five `qc_*` configs short of byte-for-byte. Two of them now are.

    Criterion 23 asks for one. `qc_participation` (14 diffs in Phase 0) and
    `qc_eligibility` (54) both reproduce exactly from `qc_TEMPLATE` plus the hole set,
    and the three Phase 0 already had stay exact - which is the check that matters, since
    a hole added for one config can easily break another.

    **This proves placement, not authoring** - §5.3.9's third qualification, unchanged.
    `derive` reads the target.
    """
    for name in QC_EXACT:
        target, cfg, bad = _qc(name)
        assert bad == [], f"{name}: {bad}"
        assert json.dumps(cfg, sort_keys=True) == json.dumps(target, sort_keys=True), name


def test_no_fragment_comes_from_a_model_or_the_library():
    """Every hole is filled by its own `$body`; the `fill` seam is never reached.

    That is stronger than F.5's round trip, which recovered whole operators with
    `from_target`. Here the skeleton supplies the shape and the bindings supply only
    values, so a `fill` callable that returns nonsense changes nothing.
    """
    calls = []
    target = json.loads((QC / "qc_participation.pc.json").read_text())
    cfg, _ = expand(
        QC_TEMPLATE, derive(target),
        lambda h, c: (calls.append(h.name), {"$UNFILLED": h.name})[1], SCHEMA,
    )
    assert calls == []
    assert json.dumps(cfg, sort_keys=True) == json.dumps(target, sort_keys=True)


def test_the_three_that_do_not_reproduce_are_recorded_as_bounded():
    """The honest half of criterion 23: what is left, and that it is not the mechanism.

    Nine, ten and thirty-three leaves respectively, in four categories that are all
    per-config payload the template does not yet name - `format`/`compression` on an
    output channel, a metric's `rdf_type`, a `count` over a column rather than `*`, and
    a whole `output_tables` section. None of them is a varying skeleton, and none needs
    anything the hole model cannot express. The test pins the *size* of what is left so
    that it cannot quietly grow.
    """
    def leaves(o, p=""):
        if isinstance(o, dict):
            for k, v in o.items():
                yield from leaves(v, f"{p}.{k}")
        elif isinstance(o, list):
            for i, v in enumerate(o):
                yield from leaves(v, f"{p}[{i}]")
        else:
            yield p, o

    budget = {"qc_biometric": 9, "qc_pharmacyclaim": 33, "qc_medicalclaim": 10}
    for name in QC_NOT_EXACT:
        target, cfg, bad = _qc(name)
        assert bad == [], f"{name}: {bad}"
        a, b = dict(leaves(cfg)), dict(leaves(target))
        n = len(set(a) ^ set(b)) + sum(1 for k in set(a) & set(b) if a[k] != b[k])
        assert n <= budget[name], f"{name}: {n} differing leaves, was {budget[name]}"
