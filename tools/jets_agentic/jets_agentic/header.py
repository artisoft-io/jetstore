"""The ownership header carried by every installed workspace asset (A21.4).

§3.9.4 makes JetStore-owned files reserved inside a client workspace and
overwritten at build time, which only works if a knowledge engineer opening one
can tell. The header says so in the file itself, and it carries the machine
token `jetstore-owned-asset` that the install guard
(`jets/workspace_assets/install.go`) looks for: a file at a reserved path that
lacks the token was never installed by JetStore, which is the `usi_ws`
`jets_model.jr` case exactly, and is diagnosed differently from a JetStore
asset somebody edited in place.

One wording, two carriers — `#` comments for `.jr`, a `jetstore-owned-asset`
object for JSON — because JSON has no comments and the token has to survive in
both. The hand-authored assets (`jets_model.jr`) carry the same block written
out literally; there is nothing generating them to stamp it.
"""

from __future__ import annotations

import textwrap

TOKEN = "jetstore-owned-asset"

# The sentence the guard's error message quotes back. Kept short enough to sit
# in a comment block and in a JSON string without reflowing.
NOTE = (
    "Installed into this workspace at image build time and overwritten on every "
    "build; a local edit is lost, and the build fails rather than clobbering it "
    "silently. Extend the model from your own file that imports this one."
)


def comment_block(filename: str, source: str, width: int = 87) -> list[str]:
    """The `.jr` form: the token, what generated it, and the extension rule."""
    rule = "# " + "=" * width
    lines = [rule, f"# {TOKEN}: data_model/{filename} — DO NOT EDIT."]
    lines += ["# " + ln for ln in textwrap.wrap(f"Source: {source}", width)]
    lines += ["# " + ln for ln in textwrap.wrap(NOTE, width)]
    lines += [f'#   import "data_model/{filename}"', rule]
    return lines


def json_object(filename: str, source: str) -> dict:
    """The JSON form, emitted under the `jetstore-owned-asset` key — which is
    where the token lives — as the first key, so it is the first thing read."""
    return {
        "name": f"data_model/{filename}",
        "owner": "jetstore",
        "do_not_edit": True,
        "source": source,
        "note": NOTE,
    }


def json_comment(filename: str, source: str) -> str:
    """The JSON *Schema* form. A schema document gets the token in `$comment`
    rather than in a key of its own: `$comment` is a standard annotation
    keyword, and an unknown top-level keyword is one more thing for a decoder's
    schema loader to have an opinion about."""
    return f"{TOKEN}: data_model/{filename} — DO NOT EDIT. Source: {source} {NOTE}"
