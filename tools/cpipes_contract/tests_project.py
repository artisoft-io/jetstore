"""Checks on the projection. `python -m pytest tests_project.py`.

**The schemas these tests validate against are the ones Go embeds**, not the ones the
browser compiles: `jets/userflow/schema/*.json` is what `santhosh-tekuri/jsonschema`
reads on the save path, and `schema_copy_test.go` on that side keeps the copy honest
against the TypeScript. So a pair that passes here passes the enforcement point, and the
browser's Zod parse is presentation — which is the division `ui_refresh` stated when
they offered to run all four layers.

**Two tests exist to hold decisions that would otherwise be prose only.** M.2 rejected
the `text`-field-carrying-JSON rendering for a typed object; `test_no_json_escape_for_a
_typed_object` is what stops it coming back. And the block linking is what a variant
chooser needs and a linear chain gets wrong invisibly — the document still validates and
every state is still reachable — so `test_variant_branches_rejoin` checks the graph
rather than the file.
"""

import json
from pathlib import Path

import jsonschema

from cpipes_contract import template as tmod
from cpipes_contract.project import _walk_bindings, project

HERE = Path(__file__).parent
SCHEMA = json.loads((HERE / "cpipes_schema.json").read_text())
GO_SCHEMA = HERE.parent.parent / "jets" / "userflow" / "schema"
TEMPLATES = ["map_claim_load_stages", "qc_metrics", "qc_report"]


def projection(name):
    tpl = tmod.load(HERE / "templates" / f"{name}.template.json")
    context = json.loads((HERE / "templates" / f"{name}.bindings.json").read_text())
    return project(tpl, context, SCHEMA)


def test_pair_validates_against_the_schemas_go_embeds():
  for name in TEMPLATES:
      p = projection(name)
      for document, filename in (
          (p.flow, "userflow.schema.json"),
          ({"schemaVersion": 1, "forms": p.forms}, "form.schema.json"),
      ):
          schema = json.loads((GO_SCHEMA / filename).read_text())
          errors = sorted(
              jsonschema.Draft202012Validator(schema).iter_errors(document),
              key=lambda e: len(e.path),
          )
          assert not errors, f"{name}/{filename}: {errors[0].message} at " + ".".join(
              str(x) for x in errors[0].path
          )


def test_no_unreachable_states():
  for name in TEMPLATES:
      """A strict deployment refuses one, and nothing ships the policy to the browser —
      so the author would see a warning and *then* have the save refused. The generator
      is the right place for this to fail."""
      assert projection(name).unreachable() == []


def test_every_state_names_a_form_this_document_carries():
  for name in TEMPLATES:
      p = projection(name)
      missing = [s["formConfig"] for s in p.flow["states"].values() if s["formConfig"] not in p.forms]
      assert missing == []


def test_variant_branches_rejoin():
  for name in TEMPLATES:
      """Every branch of a chooser must continue to the same place.

      Chaining states in walk order gets this wrong silently: the document validates, no
      state is unreachable, and an author who picks `select` walks into `value`'s form
      next. The graph is the only thing that shows it.
      """
      states = projection(name).flow["states"]

      def successor(key, seen=frozenset()):
          """Where this branch ends up once its own nested states are walked."""
          while True:
              if key in seen:
                  return key
              state = states[key]
              nxt = state.get("defaultNextState")
              if nxt is None:
                  return None if state.get("isEnd") else key
              seen = seen | {key}
              key = nxt

      for key, state in states.items():
          if "choices" not in state:
              continue
          ends = {successor(choice["nextState"]) for choice in state["choices"]}
          assert len(ends) == 1, f"{name}: branches of {key} end at {ends}"


def test_no_json_escape_for_a_typed_object():
  for name in TEMPLATES:
      """M.2's rejection, enforced rather than described.

      `validateForm` applies the `json` rule with `JSON.parse` and nothing else, so a
      typed object arriving through a text field is checked only when the finished
      document is. The rule is emitted for a list of *scalars* and nowhere else; a field
      carrying one is single-line-free and says so in its hint.
      """
      p = projection(name)
      for form in p.forms.values():
          for row in form["rows"]:
              for field in row:
                  rules = field.get("rules", [])
                  if any(r["rule"] == "json" for r in rules):
                      assert field["field"] == "text"
                      assert field.get("hint", "").startswith("A JSON list"), field


def test_a_refusal_is_visible():
  for name in TEMPLATES:
      """A back edge produces a `label` rather than nothing.

      Silence is the failure mode (I-77): a wizard that simply omits a property tells the
      author nothing about why the config it produces is not the config they meant.
      """
      p = projection(name)
      refused = [n for n in p.notes if n.startswith("refused")]
      if not refused:
          continue
      labels = [
          field["text"]
          for form in p.forms.values()
          for row in form["rows"]
          for field in row
          if field["field"] == "label"
      ]
      assert any("not editable here" in text for text in labels)


def test_the_bindings_walk_agrees_with_the_M1_census():
    """1/3/21 scalar sites and 0/0/8 object sites, counted by hand at M.1.

    **A traversal is worth what a second count says it is.** M.2's first corpus walk
    handled one level of list nesting, silently skipped `reducing_pipes_config`, and
    reported zero `where` clauses where `grep -c` finds 136 — a plausible small number,
    which is worse than an obviously odd one.
    """
    import re

    def sites(paths):
        return {re.sub(r"\.\d+(?=\.|$)", "[]", p) for p in paths}

    expected = {
        "map_claim_load_stages": (1, 0),
        "qc_metrics": (3, 0),
        "qc_report": (21, 8),
    }
    for name, (n_scalar, n_object) in expected.items():
        context = json.loads((HERE / "templates" / f"{name}.bindings.json").read_text())
        scalars, objects = _walk_bindings(context)
        assert len(sites(p for p, _ in scalars)) == n_scalar, name
        assert len(sites(objects)) == n_object, name


def test_the_walk_is_shorter_than_the_document():
    """The number that means *step-at-a-time* is the walk, not the state count.

    `qc_metrics` projects to 119 states and is walked in 34, because a chooser
    enumerates what the author *could* pick while the walk counts what they *must*.
    """
    p = projection("qc_metrics")
    assert p.state_count > 100
    assert p.longest_walk() < p.state_count / 3


if __name__ == "__main__":
    # **Runs without pytest, unlike its sibling.** No pytest is installed against this
    # package on any machine here, so `tests_template.py` cannot currently be run at all
    # — recorded as debt rather than fixed under M.4, since installing a test runner is
    # not this task's change to make.
    import traceback

    failed = 0
    for _name, _fn in sorted(globals().items()):
        if not _name.startswith("test_") or not callable(_fn):
            continue
        try:
            _fn()
            print(f"ok   {_name}")
        except Exception:
            failed += 1
            print(f"FAIL {_name}")
            traceback.print_exc()
    raise SystemExit(1 if failed else 0)
