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
from cpipes_contract.matrix_schema import FieldRow, Matrix, TypeRow
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


def _git(root: Path, *args: str) -> str:
    return subprocess.run(
        ["git", "-C", str(root), "-c", "user.name=t", "-c", "user.email=t@t", *args],
        check=True,
        capture_output=True,
        text=True,
    ).stdout


REAL = Matrix.load(HERE / "matrix")


def _type_row(go_struct: str, doc_ref: str) -> TypeRow:
    return REAL.types[0].model_copy(update={"go_struct": go_struct, "doc_ref": doc_ref})


def _field_row(go_struct: str, field_name: str, json_key: str, ref: str) -> FieldRow:
    return REAL.fields_[0].model_copy(
        update={
            "go_struct": go_struct,
            "type_token": "*",
            "field_name": field_name,
            "json_key": json_key,
            "declared_in": go_struct,
            "evidence_ref": ref,
        }
    )


def _commit(repo: Path, message: str, model: str | None = None, matrix: Matrix | None = None) -> None:
    """Commit a version of model.go and of the matrix, as one commit."""
    if model is not None:
        (repo / "model.go").write_text(model)
    if matrix is not None:
        matrix.save(repo / "matrix")
    _git(repo, "add", "-A")
    _git(repo, "commit", "-q", "-m", message)


@pytest.fixture
def repo(tmp_path: Path) -> Path:
    _git(tmp_path, "init", "-q")
    return tmp_path


SPEC = "package m\n\ntype Spec struct {\n\tName string `json:\"name\"`\n}\n"


def test_relocate_carries_a_line_that_moved(repo: Path):
    matrix = Matrix(types=[_type_row("Spec", "model.go:3")], fields_=[], constraints=[])
    _commit(repo, "one", SPEC, matrix)
    _commit(
        repo, "two",
        'package m\n\nimport "fmt"\n\nvar _ = fmt.Sprint\n\n'
        "type Spec struct {\n\tName string `json:\"name\"`\n}\n",
    )
    assert check_citations(matrix, repo) != []

    moves = relocate(matrix, repo, repo / "matrix")
    assert [(m.old, m.new) for m in moves] == [("model.go:3", "model.go:7")]
    assert apply(moves) == 1
    assert check_citations(matrix, repo) == []


def test_relocate_does_not_guess_when_the_line_itself_changed(repo: Path):
    matrix = Matrix(types=[_type_row("Spec", "model.go:3")], fields_=[], constraints=[])
    _commit(repo, "one", SPEC, matrix)
    _commit(
        repo, "two",
        "package m\n\ntype Renamed struct {\n\tName string `json:\"name\"`\n}\n"
        "\n// Spec is mentioned here.\n",
    )

    moves = relocate(matrix, repo, repo / "matrix")
    assert len(moves) == 1 and moves[0].new is None
    assert apply(moves) == 0
    assert matrix.types[0].doc_ref == "model.go:3"


# Two structs with a field of the same name, which `--code`'s substring rule
# cannot tell apart. The citation is ShufflingSpec's Comment, at line 9.
TWO_STRUCTS = (
    "package m\n\n"
    "// filler 1\n// filler 2\n// filler 3\n// filler 4\n// filler 5\n"
    "type ShufflingSpec struct {\n"
    "\tComment string `json:\"comment,omitempty\"`\n"
    "\tSize    int    `json:\"size\"`\n"
    "}\n\n"
    "type DistinctSpec struct {\n"
    "\tComment    string   `json:\"comment,omitempty\"`\n"
    "\tDistinctOn []string `json:\"distinct_on\"`\n"
    "}\n"
)
# The fillers removed: DistinctSpec's Comment is now at line 9.
FILLERS_REMOVED = TWO_STRUCTS.replace(
    "// filler 1\n// filler 2\n// filler 3\n// filler 4\n// filler 5\n", ""
)
# Two lines added at the top: line 9 names nothing, ShufflingSpec's Comment is at
# 6 and DistinctSpec's at 11.
IMPORT_ADDED = FILLERS_REMOVED.replace("package m\n\n", 'package m\n\nimport "fmt"\n\n', 1)


def _comment_matrix(ref: str) -> Matrix:
    return Matrix(
        types=[], fields_=[_field_row("ShufflingSpec", "Comment", "comment", ref)], constraints=[]
    )


def test_relocate_starts_from_the_commit_that_wrote_the_citation(repo: Path):
    """The same-name field: the newest revision in which line 9 names `comment` is
    the one where it is DistinctSpec's, and anchoring there carried the citation
    onto DistinctSpec's Comment at 11, which `--code` accepts. The commit that
    wrote the citation says line 9 was ShufflingSpec's, which is at 6 now."""
    matrix = _comment_matrix("model.go:9")
    _commit(repo, "the citation is written", TWO_STRUCTS, matrix)
    _commit(repo, "lines removed above", FILLERS_REMOVED)
    _commit(repo, "lines added above", IMPORT_ADDED)
    assert check_citations(matrix, repo) != []

    moves = relocate(matrix, repo, repo / "matrix")
    assert [(m.old, m.new) for m in moves] == [("model.go:9", "model.go:6")]


def test_relocate_all_finds_a_citation_on_a_namesake(repo: Path):
    """A citation `--code` accepts can still be on the wrong line: once the fillers
    go, line 9 is DistinctSpec's Comment. `all_citations` follows every citation
    from the commit that wrote it, and says where it belongs."""
    matrix = _comment_matrix("model.go:9")
    _commit(repo, "the citation is written", TWO_STRUCTS, matrix)
    _commit(repo, "lines removed above", FILLERS_REMOVED)
    assert check_citations(matrix, repo) == []

    assert relocate(matrix, repo, repo / "matrix") == []
    moves = relocate(matrix, repo, repo / "matrix", all_citations=True)
    assert [(m.old, m.new, m.rejected) for m in moves] == [("model.go:9", "model.go:4", False)]


def test_relocate_recovers_a_citation_carried_onto_a_namesake(repo: Path):
    """A later commit carried the citation onto DistinctSpec's Comment. At that
    commit its line declares a field of DistinctSpec, not of ShufflingSpec, so it
    was wrong when written, and the citation it replaced is followed instead."""
    _commit(repo, "the citation is written", TWO_STRUCTS, _comment_matrix("model.go:9"))
    matrix = _comment_matrix("model.go:11")
    _commit(repo, "a bad carry", IMPORT_ADDED, matrix)
    assert check_citations(matrix, repo) == []

    [move] = relocate(matrix, repo, repo / "matrix", all_citations=True)
    assert (move.old, move.new) == ("model.go:11", "model.go:6")
    assert "model.go:9" in move.reason


def test_relocate_follows_a_changed_line_within_its_struct(repo: Path):
    """Where the cited line itself changed, the field is looked for in the struct
    that held it, and nowhere else."""
    matrix = _comment_matrix("model.go:9")
    _commit(repo, "the citation is written", TWO_STRUCTS, matrix)
    _commit(
        repo, "the field is retyped and lines are added above",
        IMPORT_ADDED.replace(
            "\tComment string `json:\"comment,omitempty\"`\n\tSize",
            "\tComment any `json:\"comment,omitempty\"` // retyped\n\tSize",
        ),
    )
    moves = relocate(matrix, repo, repo / "matrix")
    assert [(m.old, m.new) for m in moves] == [("model.go:9", "model.go:6")]


def test_relocate_leaves_an_uncommitted_citation_for_a_reader(repo: Path):
    _commit(repo, "the citation is written", TWO_STRUCTS, _comment_matrix("model.go:9"))
    matrix = _comment_matrix("model.go:2")
    matrix.save(repo / "matrix")
    [move] = relocate(matrix, repo, repo / "matrix")
    assert move.new is None and "working tree" in move.reason


def test_every_citation_is_where_its_history_says():
    """`relocate --all` over the real matrix: no citation `--code` accepts sits on
    a line other than the one its history leads to."""
    shallow = subprocess.run(
        ["git", "-C", str(REPO), "rev-parse", "--is-shallow-repository"],
        capture_output=True, text=True,
    )
    if shallow.returncode != 0 or shallow.stdout.strip() != "false":
        pytest.skip("needs the repository's full history")
    moves = relocate(Matrix.load(HERE / "matrix"), REPO, HERE / "matrix", all_citations=True)
    misplaced = [f"{m.where}: {m.old} -> {m.new}" for m in moves if m.new is not None]
    assert misplaced == [], (
        f"{len(misplaced)} citation(s) on a line they were not written for; "
        "`cpipes-contract relocate --code ../.. --all --apply` carries them:\n"
        + "\n".join(misplaced[:20])
    )
