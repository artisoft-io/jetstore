"""Checks on the projection. `python -m pytest tests_project.py`.

**The schemas these tests validate against are the ones Go embeds**, not the ones the
browser compiles: `jets/userflow/schema/*.json` is what `santhosh-tekuri/jsonschema`
reads on the save path, and `schema_copy_test.go` on that side keeps the copy honest
against the TypeScript. So a pair that passes here passes the enforcement point, and the
browser's Zod parse is presentation — which is the division the IDE side stated when
it offered to run all four layers.

**Two tests exist to hold decisions that would otherwise be prose only.** A decision
rejected the `text`-field-carrying-JSON rendering for a typed object; `test_no_json_escape_for_a
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


def test_the_triple_validates_against_the_schemas_go_embeds():
  for name in TEMPLATES:
      """**Three documents, not two** — the loader reads all three and two were emitted once.
      The action document was added to this check in the same change that
      started emitting it, because a document nothing validates is how the pair came to
      look complete."""
      p = projection(name)
      for document, filename in (
          (p.flow, "userflow.schema.json"),
          ({"schemaVersion": 1, "forms": p.forms}, "form.schema.json"),
          (p.actions, "action.schema.json"),
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
      """The rejection, enforced rather than described.

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

      Silence is the failure mode: a wizard that simply omits a property tells the
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
    """1/3/~~21~~ ~~22~~ 24 scalar sites and 0/0/8 object sites, counted by hand.

    **A traversal is worth what a second count says it is.** The first corpus walk
    handled one level of list nesting, silently skipped `reducing_pipes_config`, and
    reported zero `where` clauses where `grep -c` finds 136 — a plausible small number,
    which is worse than an obviously odd one.

    **`qc_report` moved from 21 to 22 on 2026-09-17**, when the status filters became a
    repeating hole. `status_filters` is `[]` in these
    bindings — `qc_participation` carries no filter — and `_walk_bindings` counts a list
    whose members are all scalars as one scalar site, which an empty list vacuously is.
    So the three sites that were already empty lists here (`stage1_map_pipes`,
    `sum_metrics`, `map_reduce_metrics`) are its precedent rather than a discrepancy, and
    the count of a *binding site* is what this asserts, not a count of anything authored.
    **And to 24 on 2026-11-19**, when the two notification context keys became
    parameters of the template rather than literals in its body.
    `sizing` is what keeps a repeat binding out of the offered fields, which the sibling
    test below holds.
    """
    import re

    def sites(paths):
        return {re.sub(r"\.\d+(?=\.|$)", "[]", p) for p in paths}

    expected = {
        "map_claim_load_stages": (1, 0),
        "qc_metrics": (3, 0),
        "qc_report": (24, 8),
    }
    for name, (n_scalar, n_object) in expected.items():
        context = json.loads((HERE / "templates" / f"{name}.bindings.json").read_text())
        scalars, objects = _walk_bindings(context)
        assert len(sites(p for p, _ in scalars)) == n_scalar, name
        assert len(sites(objects)) == n_object, name


def test_an_end_state_offers_completed_and_no_other_state_does():
  for name in TEMPLATES:
      """`isEnd` is in the flow document and the button set is in the form document.

      Neither schema can see the other, so this rule is not expressible in either — the
      third instance of a cross-document rule, and the reason it is a test rather than a comment.
      `step` runs the state action on `ufNext` too, so the failure was not a config that
      did not save: it was a config that saved and then said *"No next step from …"*.

      **This biconditional is about what *this generator* emits, and it is stricter than
      the corpus rule. Do not lift it out as one.** The rule the shipped flows actually
      keep is that an end state's form offers no *advancing* button; two of the eleven
      ported flows finished through a custom action and declared no `uf*` action at all
      (`fmMappingFormUF`, `rfkSubmitSchemaEvent`). A validator asserting `ufCompleted`
      would reject both — exactly that was drafted once and the corpus refused it.
      (One remains: `rfkSubmitSchemaEvent` left with `registerFileKeyUF` on 2026-10-01,
      and `fmMappingFormUF` still ends on `mapperOk`. Checked 2026-10-27.)
      This projection has one way to end, so the tighter assertion is the right one
      *here* and the wrong one anywhere else.
      """
      p = projection(name)
      for key, state in p.flow["states"].items():
          buttons = {a["action"] for a in p.forms[key]["actions"]}
          ends = state.get("isEnd", False)
          assert ("ufCompleted" in buttons) is ends, f"{name}: {key}"
          assert ("ufNext" in buttons) is not ends, f"{name}: {key}"


def test_the_skeleton_is_the_expansion_with_markers_where_values_go():
  for name in TEMPLATES:
      """The apply substitutes rather than expands, so the skeleton must *be* the config.

      Checked by expanding a second time with the real bindings and an inert filler, and
      comparing everything that is not a marker. A skeleton that had drifted from the
      expansion would produce a config no author asked for, and nothing downstream of
      the wizard would know.
      """
      from cpipes_contract.expand import expand
      from cpipes_contract.project import FILL_MARKER, BINDING_MARKER

      tpl = tmod.load(HERE / "templates" / f"{name}.template.json")
      context = json.loads((HERE / "templates" / f"{name}.bindings.json").read_text())
      plain, _ = expand(tpl, context, lambda hole, ctx: {"__fill__": hole.name}, None)
      skeleton = projection(name).plan["skeleton"]

      def agree(a, b, path=""):
          if isinstance(b, dict) and len(b) == 1 and FILL_MARKER in b:
              assert isinstance(a, dict) and "__fill__" in a, f"{name} at {path}: not a fill"
              return
          if isinstance(b, str) and b.startswith(BINDING_MARKER):
              return
          assert type(a) is type(b), f"{name} at {path}: {type(a)} vs {type(b)}"
          if isinstance(b, dict):
              assert set(a) == set(b), f"{name} at {path}: {set(a) ^ set(b)}"
              for k in b:
                  agree(a[k], b[k], f"{path}.{k}")
          elif isinstance(b, list):
              assert len(a) == len(b), f"{name} at {path}: {len(a)} vs {len(b)}"
              for i, (x, y) in enumerate(zip(a, b)):
                  agree(x, y, f"{path}[{i}]")
          else:
              assert a == b, f"{name} at {path}: {a!r} vs {b!r}"

      agree(plain, skeleton)


def test_every_marker_names_a_key_some_form_collects():
  for name in TEMPLATES:
      """A marker with no field behind it is a value the wizard can never supply.

      A binding marker names a form key directly; a fill marker names a state, and what
      it collects is that state's own keys or those of the states below it. Both are
      checked against the forms rather than against the projector's own bookkeeping,
      which would agree with itself.
      """
      from cpipes_contract.project import FILL_MARKER, BINDING_MARKER

      p = projection(name)
      keys = {
          field["key"]
          for form in p.forms.values()
          for row in form["rows"]
          for field in row
          if "key" in field
      }

      def walk(node):
          if isinstance(node, dict):
              if len(node) == 1 and FILL_MARKER in node:
                  at = node[FILL_MARKER]
                  assert any(k == at or k.startswith(f"{at}.") for k in keys), f"{name}: {at}"
                  return
              for value in node.values():
                  walk(value)
          elif isinstance(node, list):
              for value in node:
                  walk(value)
          elif isinstance(node, str) and node.startswith(BINDING_MARKER):
              key = node[len(BINDING_MARKER):]
              assert key in keys, f"{name}: {key} has no field"

      walk(p.plan["skeleton"])


def test_a_binding_that_sizes_the_flow_is_not_offered_as_a_field():
    """The fixed plan size, made visible where it binds rather than stated in a document.

    `qc_metrics.input_columns` is three strings and nine states; an editable field there
    would take an edit and drop it, because a flow has no way to make a state. It is a
    label, and the label says how many.
    """
    p = projection("qc_metrics")
    fields = {
        field["key"]
        for form in p.forms.values()
        for row in form["rows"]
        for field in row
        if "key" in field
    }
    assert "bindings.input_columns" not in fields
    labels = [
        field["text"]
        for row in p.forms["bindings"]["rows"]
        for field in row
        if field["field"] == "label"
    ]
    assert any("Input columns" in t and "3 entries" in t for t in labels), labels


def test_the_demonstrated_config_validates_against_the_contract():
    """The demonstration's output, checked at the layer no UserFlow schema can reach.

    `projections/qc_metrics.demonstrated.pc.json` is what the wizard produced when
    `jetsclient_ide/src/cpipes/templateApply.test.ts` walked the projection with the
    shipped engine and interpreter. **Every validation layer validates a *flow* document;
    this validates the *config* those documents produce**, which is the only place a
    whole class of defect is visible — and the class is not hypothetical. The
    demonstration's first run emitted a `partition_writer` operator with no `type`,
    because a `const` property is not a field and nothing was carrying it. Four layers
    passed that pair, and this check is a `required` violation away from it.
    """
    config = json.loads((HERE / "projections" / "qc_metrics.demonstrated.pc.json").read_text())
    validator = jsonschema.Draft202012Validator(
        {
            "$schema": SCHEMA["$schema"],
            "$ref": "#/$defs/ComputePipesConfig",
            "$defs": SCHEMA["$defs"],
        }
    )
    errors = list(validator.iter_errors(config))
    assert not errors, jsonschema.exceptions.best_match(errors).message


def test_every_const_property_is_carried_rather_than_dropped():
  for name in TEMPLATES:
      """A `const` is not a field, and it is not nothing either.

      `fields_for` skips one on the grounds that a variant chooser already supplied it —
      which holds for a union and not for a single concrete type, where there is no
      chooser. Checked from the schema rather than from the projector's own bookkeeping,
      so that the plan agreeing with itself is not what passes.
      """
      p = projection(name)
      contract_defs = SCHEMA["$defs"]
      for prefix, fixed in p.plan["constants"].items():
          assert fixed, f"{name}: {prefix} records an empty constant set"
          for prop, value in fixed.items():
              matches = [
                  d for d in contract_defs.values()
                  if isinstance(d, dict) and d.get("properties", {}).get(prop, {}).get("const") == value
              ]
              assert matches, f"{name}: {prefix}.{prop}={value!r} is no type's const"


def test_the_walk_is_shorter_than_the_document():
    """The number that means *step-at-a-time* is the walk, not the state count.

    `qc_metrics` projects to 119 states and is walked in 34, because a chooser
    enumerates what the author *could* pick while the walk counts what they *must*.
    """
    p = projection("qc_metrics")
    assert p.state_count > 100
    assert p.longest_walk() < p.state_count / 3


def test_a_parameter_is_offered_with_its_default_label_and_help():
    """`qc_report`'s two notification context keys: a default to see and change, and why.

    The keys are what a deployment's notification template reads, so the right value
    depends on the deployment rather than on the pipeline. The flow offers the neutral
    default from the bindings, under the parameter's label, with its help in the row
    below; the skeleton puts the collected value where the config's context key goes.
    """
    p = projection("qc_report")
    rows = p.forms["bindings"]["rows"]
    want = {
        "bindings._item.file_type_key": "$NOTIFICATION_FILE_TYPE",
        "bindings._item.notification_type_key": "$NOTIFICATION_TYPE",
    }
    for key, default in want.items():
        at = next(i for i, row in enumerate(rows) if row[0].get("key") == key)
        field = rows[at][0]
        assert field["defaultValue"] == default
        assert field["label"].startswith("Notification key for the")
        assert any(r["rule"] == "required" for r in field["rules"])
        help_row = rows[at + 1][0]
        assert help_row["field"] == "label" and "{{NAME}}" in help_row["text"]

    from cpipes_contract.project import BINDING_MARKER

    context = p.plan["skeleton"]["context"]
    assert [c["key"] for c in context[2:4]] == [BINDING_MARKER + k for k in want]
    # Every other binding field stays as it was: no default, labelled by its path.
    others = [row[0] for row in rows if row[0].get("field") == "text" and row[0]["key"] not in want]
    assert others and all("defaultValue" not in f for f in others)


def test_a_parameter_with_no_value_in_the_bindings_is_refused():
    """The default comes from the bindings, so a parameter they do not bind is a defect."""
    from cpipes_contract.project import ProjectionError

    tpl = tmod.load(HERE / "templates" / "qc_report.template.json")
    context = json.loads((HERE / "templates" / "qc_report.bindings.json").read_text())
    del context["$item"]["notification_type_key"]
    try:
        project(tpl, context, SCHEMA)
    except ProjectionError as err:
        assert "notification_type_key" in str(err), err
    else:
        raise AssertionError("projected a parameter the bindings do not bind")


def test_the_installed_qc_documents_are_the_projection():
    """What `install_workspace_assets` ships for the two `qc_*` templates is what they project to.

    Nothing else asserts it, and a hand edit to an installed document is overwritten by
    the next regeneration without anyone seeing it go. `map_claim_load_stages` is left
    out: its installed set lags its template, which is a separate change.
    """
    import tempfile

    installed = HERE.parent.parent / "jets" / "workspace_assets" / "user_flows"
    for name in ("qc_metrics", "qc_report"):
        with tempfile.TemporaryDirectory() as out:
            for path in projection(name).write(Path(out)):
                assert path.read_bytes() == (installed / path.name).read_bytes(), path.name


if __name__ == "__main__":
    # **Runs without pytest, unlike its sibling.** No pytest is installed against this
    # package on any machine here, so `tests_template.py` cannot currently be run at all
    # — recorded as debt rather than fixed here, since installing a test runner is
    # not this change to make.
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
