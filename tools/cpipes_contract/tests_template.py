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
