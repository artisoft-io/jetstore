"""F.7, criterion 23: bindings for the `qc_report` template, read off a live config.

**This recovers payload from the target, and that is what criterion 23 permits.**
§5.3.9's third qualification draws the line: a harness that reads the target proves
*placement* - that a fixed skeleton can put fragments in the right places and produce a
byte-identical config - and says nothing about whether anyone could author those
fragments. Criterion 22 is the one that must not read the target, and F.6 answered it
with `from_model`. Keeping this in its own module, named for what it does, is the same
guard `from_target` gets for the same reason.

**Positional, with no per-file special-casing.** Every rule here is stated over the
shape all eight `qc_*` configs share; nothing keys off a file name. That is what makes
running it over the whole family a test of the template rather than a demonstration.
"""

from __future__ import annotations

from itertools import takewhile


class QcShapeError(ValueError):
    """The config does not have the shape `derive` reads, said by name."""


def status_column(columns: list[dict]) -> dict:
    """The report pipe's `case` column that writes `status`, found by what it writes.

    **Not `columns[9]`, which is what this read until PL-036.** The index was right
    for all eight `qc_*` configs and agreed with them by a count nothing states: a
    config with one more or one fewer column before it would have read a different
    column, and the failure would have been a `KeyError` on `case_expr` or, worse, a
    `case` column that is not the status one. A column is the status column when it
    is of type `case` and one of its legs' `then`, or its `else_expr`, assigns
    `status`. Exactly one must; none or several is a `QcShapeError` naming the count.
    """
    def writes_status(column: dict) -> bool:
        if column.get("type") != "case":
            return False
        assigned = [*(column.get("else_expr") or [])]
        for leg in column.get("case_expr") or []:
            assigned.extend(leg.get("then") or [])
        return any(a.get("name") == "status" for a in assigned)

    found = [c for c in columns if writes_status(c)]
    if len(found) != 1:
        raise QcShapeError(
            f"expected one `case` column writing `status` in the report pipe, "
            f"found {len(found)}"
        )
    return found[0]


def derive(config: dict) -> dict:
    """The bindings that expand `qc_report` back into `config`."""
    rpc = config["reducing_pipes_config"]
    stage0, stage1 = rpc[0], rpc[1]

    # The last three pipes of stage 1 are the aggregate, the partition hash and the
    # writer. Anything before them is the optional remap pipe - present in five of the
    # eight - which the template carries as a hole repeating over a list of 0 or 1.
    optional, aggregate = stage1[:-3], stage1[-3]
    agg_columns = aggregate["apply"][0]["columns"]
    metrics = agg_columns[3:]          # past dw_rawfilename, layout_name, n

    def family(kind: str) -> list[dict]:
        return [c for c in metrics if c.get("type") == kind]

    emitters = rpc[2][2]["apply"]

    # The status column's `case_expr` opens with zero or more test-harness filters,
    # each forcing `status` to 'pass' for a family of metrics the deployment did not
    # calculate, and ends with the ratio-threshold legs the template carries literally.
    # Three of the eight configs carry four filters and the other five carry none, which
    # is the same 0-or-n positional shape as the optional remap pipe above.
    #
    # **The leading run is taken by shape rather than by slicing the literal tail off.**
    # `case_expr[:-4]` gives the same answer today and couples this file to a count of
    # legs that lives in the template - two files agreeing by a constant neither
    # mentions. Testing the leg instead makes them agree by construction, and it fails
    # in the right direction: a filter written *after* the ratio legs is not recovered,
    # and must not be, because `caseExprEvaluator.Update` returns on the first match
    # (`Update`, `jets/compute_pipes/column_evaluators_case_expr.go:24`; the first-match
    # test is at `:36` and its `return nil` at `:43`).
    #
    # **The pair is recovered as a pair, and that is not a stylistic choice.** Deriving
    # the substring from the environment variable's name works for `calculateHedis` ->
    # 'Hedis' and breaks on the HCC pair: `${calculateCMSHCC}` and `${calculateHHSHCC}`
    # both test `contains 'HCC'`, because the two calculations write into one family of
    # field ids. So `env_key` and `substring` travel together, read off the leg.
    def is_status_filter(leg: dict) -> bool:
        when = leg.get("when") or {}
        lhs, rhs = when.get("lhs") or {}, when.get("rhs") or {}
        return (
            when.get("op") == "and"
            and lhs.get("op") == "=="
            and rhs.get("op") == "contains"
            and (rhs.get("lhs") or {}).get("expr") == "field_id"
        )

    status_case = status_column(rpc[3][1]["apply"][0]["columns"])["case_expr"]
    status_filters = [
        {"env_key": leg["when"]["lhs"]["lhs"]["expr"],
         "substring": leg["when"]["rhs"]["rhs"]["expr"]}
        for leg in takewhile(is_status_filter, status_case)
    ]

    return {
        # **The template model has no globals**, so the values that vary per config but
        # not per item ride on the top-level `$item`. See I-43.
        "$item": {
            "data_channel": config["channels"][0]["name"],
            "data_columns": config["channels"][0]["columns"],
            "metrics_channel": config["channels"][1]["name"],
            "metric_columns": config["channels"][1]["columns"],
            "report": config["context"][1]["expr"],
            "file_type": config["context"][2]["expr"],
            "cluster_config": config["cluster_config"],
            "input_channel": stage0[0]["input_channel"],
            "partition_key": stage0[0]["apply"][0]["columns"][2]["hash_expr"],
            "data_writer": stage0[0]["apply"][0]["output_channel"]["name"],
            "data_out": stage0[1]["apply"][0]["output_channel"]["name"],
            # Not a renamed channel but a re-shaped one: deleting the optional pipe
            # rewires its consumer to a *stage* read, which gains `type` and
            # `read_step_id`. §5.3.9 recorded this as a derivation rule of the Phase 0
            # harness; here it is a binding, because the expander has no rules.
            "stage1_agg_input": aggregate["input_channel"],
            "metrics_mapped": aggregate["apply"][0]["output_channel"]["name"],
            "metrics_writer": stage1[-2]["apply"][0]["output_channel"]["name"],
            "metrics_out": stage1[-1]["apply"][0]["output_channel"]["name"],
            "metrics_aggregated": rpc[2][1]["apply"][0]["output_channel"]["name"],
        },
        "input_columns": [{"spec": c} for c in stage0[0]["apply"][0]["columns"][3:]],
        "stage1_map_pipes": [{"pipe": p} for p in optional],
        # The four metric families, which the corpus writes grouped and in this order.
        "distinct_metrics": [{"name": c["name"], "expr": c["expr"]} for c in family("distinct_count")],
        "sum_metrics": [{"name": c["name"], "expr": c["expr"]} for c in family("sum")],
        "metrics": [{"name": c["name"], "where": c["where"]} for c in family("count")],
        "map_reduce_metrics": [{"spec": c} for c in family("map_reduce")],
        # **Emitters are their own list, not a second pass over the metrics.** §5.3.9
        # found the metric marker at two levels and treated it as one list read twice;
        # in three of the eight the two levels differ in length, because one
        # `map_reduce` column contributes several report rows.
        "emitters": [
            {"field": e["columns"][2]["expr"], "field_id": e["columns"][3]["expr"],
             "numerator": e["columns"][4], "denominator": e["columns"][5]}
            for e in emitters
        ],
        "all_metrics": [{"name": c["name"]} for c in rpc[2][0]["apply"][0]["columns"][1:]],
        # Empty for the five reports that carry no filter, which the expander splices
        # away. It has to be **bound to the empty list rather than omitted**: an unbound
        # `repeat_over` raises `ExpansionError` (`expand`, `expand.py:138`).
        "status_filters": status_filters,
    }
