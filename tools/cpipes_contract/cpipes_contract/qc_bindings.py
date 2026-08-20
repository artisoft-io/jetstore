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
    }
