"""F.2b: what the harvested test fixtures are actually worth to the library.

The Go side (`tools/cpipes_contract/fixtures`) reads the fixtures out of
`jets/compute_pipes/*_test.go` and resolves the test scope around them. This side asks
the question that decides whether source 2 pays: **do they validate as contract
fragments, and do they fill a gap the corpus leaves?**

**They largely do not, and the reason is worth more than the fragments.** A§6.1 source 2
calls the fixtures "author-written, correct by construction". They are correct *for their
test*, which is a different claim. A test builds the minimum its code path needs, so a
fixture is routinely partial; a negative test builds something deliberately invalid; and
a round-trip test uses a sentinel where a domain value belongs - `device_writer_type:
"S3"` appears in `actions_start_common_test.go` purely to be read back, and no such
writer exists. As few-shot material a sentinel is worse than nothing, because it teaches
a value the contract does not have.

So this module reports rather than merges. Nothing here writes into the library.
"""

from __future__ import annotations

import csv
import json
from pathlib import Path


def load(path: Path) -> list[dict]:
    """The harvested fixtures, as the Go side wrote them."""
    return [json.loads(line) for line in path.read_text().splitlines() if line.strip()]


def validate(fixtures: list[dict], schema: dict) -> tuple[list[dict], list[dict]]:
    """Split the fixtures into those that are contract fragments and those that are not.

    A fixture is judged against its own type alone, exactly as a library part is: the
    hole it would fill names that type, so that is the only question worth asking.
    """
    import jsonschema

    validators: dict[str, object] = {}
    good: list[dict] = []
    bad: list[dict] = []
    for fixture in fixtures:
        name = fixture["defs_name"]
        if name not in schema["$defs"]:
            bad.append({**fixture, "why": "not a $defs entry"})
            continue
        if name not in validators:
            validators[name] = jsonschema.Draft202012Validator(
                {
                    "$schema": schema.get("$schema", ""),
                    "$ref": f"#/$defs/{name}",
                    "$defs": schema["$defs"],
                }
            )
        errors = sorted(
            validators[name].iter_errors(fixture["value"]),  # type: ignore[attr-defined]
            key=lambda e: len(e.path),
        )
        if errors:
            at = ".".join(str(p) for p in errors[0].path) or "<root>"
            bad.append({**fixture, "why": f"{at}: {errors[0].message[:120]}"})
        else:
            good.append(fixture)
    return good, bad


def coverage(good: list[dict], library: list[dict], matrix: Path) -> dict:
    """Which contract types the fixtures would add, and which they merely duplicate.

    **The gap is the whole point of source 2.** A§6.1 argues the fixtures are worth
    harvesting because they concentrate where the corpus is thin, so a count of fixtures
    is the wrong measure and a count of *types the corpus cannot illustrate* is the right
    one.
    """
    with open(matrix / "types.csv", newline="") as fh:
        declared = {row["defs_name"] for row in csv.DictReader(fh) if row.get("defs_name")}
    have = {part["defs_name"] for part in library}
    empty = declared - have
    from_fixtures = {f["defs_name"] for f in good}
    return {
        "declared": len(declared),
        "covered_by_corpus": len(have & declared),
        "empty": sorted(empty),
        "filled_by_fixtures": sorted(empty & from_fixtures),
        "duplicated_by_fixtures": sorted(from_fixtures & have),
    }


def render(good: list[dict], bad: list[dict], cov: dict) -> str:
    """The report. Denominators visible, per decision 13."""
    from collections import Counter

    lines = [
        f"{len(good) + len(bad)} fixture(s) harvested: "
        f"**{len(good)} validate**, {len(bad)} do not",
        "",
        f"{'type':28} {'valid':>6} {'invalid':>8}  commonest reason",
    ]
    ok, no = Counter(f["defs_name"] for f in good), Counter(f["defs_name"] for f in bad)
    why: dict[str, Counter] = {}
    for f in bad:
        why.setdefault(f["defs_name"], Counter())[f["why"]] += 1
    for name in sorted(set(ok) | set(no), key=lambda n: -(ok[n] + no[n])):
        reason = why[name].most_common(1)[0][0][:60] if name in why else ""
        lines.append(f"{name:28} {ok[name]:6} {no[name]:8}  {reason}")
    lines += [
        "",
        f"contract types: {cov['declared']} declared, {cov['covered_by_corpus']} covered by "
        f"the corpus, {len(cov['empty'])} empty",
        f"  fixtures fill  : {', '.join(cov['filled_by_fixtures']) or 'none'}",
        f"  fixtures repeat: {len(cov['duplicated_by_fixtures'])} type(s) the corpus already covers",
        "",
        "**Not merged.** A fixture is correct for its test, which is not the same as valid",
        "under the contract - see this module's docstring, and I-45.",
    ]
    return "\n".join(lines)
