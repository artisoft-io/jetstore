"""`generate` reproduces the committed model. `python -m pytest tests_generate.py`.

The model is edited by hand and `reflect` carries the edit into the matrix; these
tests hold the other direction, that the matrix still emits the model. Until
2026-11-27 nothing did, and by then a regeneration would have reordered a dozen
classes, rewritten three descriptions and deleted three blocks of code.
"""

import os
import subprocess
import sys
import tempfile
from pathlib import Path

from cpipes_contract.generate import RegionError, hand_regions, render, splice, Emitter
from cpipes_contract.matrix_schema import Matrix

HERE = Path(__file__).parent
MODEL = HERE / "cpipes_model.py"
MATRIX = Matrix.load(HERE / "matrix")


def test_the_model_is_what_the_matrix_generates():
    """The check `generate --check` makes. A failure names the first differing line;
    `cpipes-contract generate --check` prints the diff."""
    have = MODEL.read_text().splitlines()
    want = render(MATRIX, MODEL).splitlines()
    for n, (h, w) in enumerate(zip(have, want), 1):
        assert h == w, (
            f"cpipes_model.py:{n} is not what the matrix generates:\n  committed: {h}\n"
            f"  generated: {w}\nreflect the edit into the matrix (`reflect`), or mark "
            "code the matrix cannot carry as a hand-written region"
        )
    assert len(have) == len(want), "cpipes_model.py and the generated model differ in length"


def test_generate_is_the_same_under_any_hash_seed():
    """The emitter sorts everything it writes, so set and dict order cannot reach the
    output. Asserted rather than assumed, since a drifted model was the first
    suspicion."""
    outputs = []
    with tempfile.TemporaryDirectory() as tmp:
        for seed in ("0", "12345"):
            out = Path(tmp) / f"model_{seed}.py"
            subprocess.run(
                [
                    sys.executable, "-c",
                    "import sys; from cpipes_contract.main import main; sys.exit(main(sys.argv[1:]))",
                    "generate", "--out", str(out),
                ],
                cwd=HERE, check=True, capture_output=True,
                env={**os.environ, "PYTHONHASHSEED": seed},
            )
            outputs.append(out.read_bytes())
    assert outputs[0] == outputs[1]


def test_every_region_of_the_model_is_carried():
    regions = hand_regions(MODEL.read_text())
    assert [anchor for anchor, _ in regions] == [
        "before TableSpec", "field TableSpec.columns", "before Element", "end",
    ]
    generated = render(MATRIX, MODEL)
    for _, lines in regions:
        assert "\n".join(lines) in generated


def test_a_region_goes_above_its_anchor_with_the_anchor_spacing():
    emitted = Emitter(MATRIX).emit()
    region = ["# >>> hand-written: before VllmSpec", "X = 1", "# <<< hand-written"]
    lines = splice(emitted, [("before VllmSpec", region)]).split("\n")
    at = lines.index("class VllmSpec(_Base):")
    assert lines[at - 3 : at] == [*region[-1:], "", ""]
    assert lines[at - 5 : at - 2] == region


def test_a_region_whose_anchor_is_not_emitted_is_refused():
    emitted = Emitter(MATRIX).emit()
    for anchor in ("before NoSuchClass", "field TableSpec.no_such_field", "after TableSpec"):
        try:
            splice(emitted, [(anchor, ["# >>> hand-written: " + anchor, "# <<< hand-written"])])
        except RegionError as err:
            assert anchor in str(err)
        else:
            raise AssertionError(f"spliced a region anchored `{anchor}`")


def test_an_unclosed_region_is_refused():
    try:
        hand_regions("x = 1\n# >>> hand-written: end\ny = 2\n")
    except RegionError as err:
        assert "never closes" in str(err)
    else:
        raise AssertionError("read an unclosed region")
