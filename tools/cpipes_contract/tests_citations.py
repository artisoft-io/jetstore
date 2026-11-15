"""The matrix's citations, checked by the suite rather than only by `check --code`.

`check --code` is a command somebody has to remember to run, and it was not run: a
change that adds a few lines above a cited declaration moves every citation below it,
and the matrix had collected hundreds of such misses before anybody looked. This
test runs the same rule over every citation into this repository, so the suite fails
at the change that moves a cited line, while the cause is still in the diff.
`cpipes-contract relocate --code ../.. --apply` is the repair.

Citations into `workspaces/` are out of scope here: they resolve only in a checkout
holding the workspaces beside JetStore, which `check --code` covers when it is run
from one.
"""

from __future__ import annotations

import subprocess
from pathlib import Path

import pytest

from cpipes_contract.check import check_citations
from cpipes_contract.matrix_schema import Matrix
from cpipes_contract.relocate import apply, relocate

HERE = Path(__file__).parent
REPO = HERE.parent.parent


def test_every_citation_into_jetstore_names_what_it_cites():
    matrix = Matrix.load(HERE / "matrix")
    unchecked: list[str] = []
    # A workspaces root that holds no workspace: every `workspaces/` citation is
    # set aside rather than reported, whatever this checkout has beside it.
    problems = check_citations(matrix, REPO, HERE / "matrix", unchecked)
    assert problems == [], (
        f"{len(problems)} citation(s) no longer name what they cite; "
        "`cpipes-contract relocate --code ../.. --apply` carries moved lines:\n"
        + "\n".join(problems[:20])
    )
    # The set-aside citations are the generator evidence, and only that.
    assert all(u.split(": ", 1)[1].startswith("workspaces/") for u in unchecked)


# --- relocate -------------------------------------------------------------------


def _git(root: Path, *args: str) -> None:
    subprocess.run(
        ["git", "-C", str(root), "-c", "user.name=t", "-c", "user.email=t@t", *args],
        check=True,
        capture_output=True,
    )


@pytest.fixture
def repo(tmp_path: Path) -> Path:
    _git(tmp_path, "init", "-q")
    (tmp_path / "model.go").write_text(
        "package m\n\ntype Spec struct {\n\tName string `json:\"name\"`\n}\n"
    )
    _git(tmp_path, "add", "model.go")
    _git(tmp_path, "commit", "-q", "-m", "one")
    return tmp_path


def _one_type_matrix(doc_ref: str) -> Matrix:
    real = Matrix.load(HERE / "matrix")
    row = real.types[0].model_copy(update={"go_struct": "Spec", "doc_ref": doc_ref})
    return Matrix(types=[row], fields_=[], constraints=[])


def test_relocate_carries_a_line_that_moved(repo: Path):
    (repo / "model.go").write_text(
        'package m\n\nimport "fmt"\n\nvar _ = fmt.Sprint\n\n'
        "type Spec struct {\n\tName string `json:\"name\"`\n}\n"
    )
    _git(repo, "commit", "-q", "-am", "two")
    matrix = _one_type_matrix("model.go:3")
    assert check_citations(matrix, repo) != []

    moves = relocate(matrix, repo)
    assert [(m.old, m.new) for m in moves] == [("model.go:3", "model.go:7")]
    assert apply(moves) == 1
    assert check_citations(matrix, repo) == []


def test_relocate_does_not_guess_when_the_line_itself_changed(repo: Path):
    (repo / "model.go").write_text(
        "package m\n\ntype Renamed struct {\n\tName string `json:\"name\"`\n}\n"
        "\n// Spec is mentioned here.\n"
    )
    _git(repo, "commit", "-q", "-am", "two")
    matrix = _one_type_matrix("model.go:3")

    moves = relocate(matrix, repo)
    assert len(moves) == 1 and moves[0].new is None
    assert apply(moves) == 0
    assert matrix.types[0].doc_ref == "model.go:3"
