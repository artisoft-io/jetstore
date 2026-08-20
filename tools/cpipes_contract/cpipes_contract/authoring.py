"""Criterion 22: fragments authored from an English description, not recovered.

**Why this is a separate module from `expand`.** §5.3.9's third qualification is that a
harness recovering payload from the target proves *placement*, not authoring, and §6.4
says criterion 21 does not discharge criterion 22. F.4 put the fill behind a callable and
F.5 supplied `from_target`; this supplies the third filler, the one that asks a model to
write a fragment it has never seen, constrained by the hole's own schema. Nothing here
reads the library or the target.

**The bundle layer is what makes this possible at all.** A hole's `schema_ref` names a
bundle, and `subschema` emits a self-contained document rooted at it. Addressed at
`TransformationSpec` the schema would be 41,040 tokens against a 32,768-token context and
would not fit; addressed at a bundle the widest is 15,513. I-29 recorded that as a
blocker and the bundle layer resolved it - this module is the first thing that could not
have been written before.

**Reported per hole, never in aggregate.** One bad hole in a 453-column config would
otherwise report as total failure, and decision 13's rule against an aggregate figure
applies here for the same reason it applies to compile-pass: a total over a skewed
population measures the population.
"""

from __future__ import annotations

import json
import urllib.error
import urllib.request
from dataclasses import dataclass, field
from pathlib import Path

from .expand import Fill
from .template import Hole, _bundle_members


def _bundle_tokens(matrix: Path) -> dict[str, list[str]]:
    """Bundle -> the discriminator *values* it admits, which is what a model must emit."""
    import csv

    out: dict[str, list[str]] = {}
    with open(matrix / "bundle_members.csv", newline="") as fh:
        for row in csv.DictReader(fh):
            out.setdefault(row["bundle"], []).append(row["type_token"])
    return out

DEFAULT_HOST = "http://localhost:11434"


def subschema(schema: dict, name: str) -> dict:
    """A self-contained document rooted at one `$defs` entry.

    The Python twin of `jets/agentic/prompt.Subschema`, and it exists for the same
    reason: Ollama's `format` takes a schema document, not a reference into one, so
    "constrain to #/$defs/X" has to become a document whose root is X.
    """
    defs = schema["$defs"]
    if name not in defs:
        raise KeyError(f"no $defs entry named {name!r}")
    keep = {name}
    frontier = [name]
    while frontier:
        current = frontier.pop()

        def walk(node: object) -> None:
            if isinstance(node, dict):
                for key, value in node.items():
                    if key == "$ref" and isinstance(value, str) and value.startswith("#/$defs/"):
                        target = value.split("/")[-1]
                        if target not in keep:
                            keep.add(target)
                            frontier.append(target)
                    else:
                        walk(value)
            elif isinstance(node, list):
                for value in node:
                    walk(value)

        walk(defs[current])
    return {
        "$schema": schema.get("$schema", "https://json-schema.org/draft/2020-12/schema"),
        "$ref": f"#/$defs/{name}",
        "$defs": {k: defs[k] for k in sorted(keep)},
    }


@dataclass
class Attempt:
    """One hole, one authoring attempt."""

    hole: str
    schema_ref: str
    item: object = None
    ok: bool = False
    error: str = ""
    prompt_tokens: int = 0
    eval_tokens: int = 0


@dataclass
class Report:
    """Per-hole outcomes. **There is deliberately no aggregate.**"""

    model: str
    attempts: list[Attempt] = field(default_factory=list)

    def per_hole(self) -> dict[str, tuple[int, int]]:
        """hole -> (passed, attempted)."""
        out: dict[str, list[int]] = {}
        for a in self.attempts:
            row = out.setdefault(a.hole, [0, 0])
            row[1] += 1
            if a.ok:
                row[0] += 1
        return {k: (v[0], v[1]) for k, v in out.items()}

    def render(self) -> str:
        lines = [f"model: {self.model}", "", f"{'hole':24s}{'schema_ref':26s}{'pass':>6s}{'of':>4s}  commonest failure"]
        for hole, (ok, n) in sorted(self.per_hole().items()):
            ref = next(a.schema_ref for a in self.attempts if a.hole == hole)
            errs = [a.error for a in self.attempts if a.hole == hole and a.error]
            top = max(set(errs), key=errs.count)[:60] if errs else ""
            lines.append(f"{hole:24s}{ref:26s}{ok:>6d}{n:>4d}  {top}")
        lines.append("")
        lines.append(
            "No aggregate figure is published. One bad hole in a 453-column config "
            "would report as total failure, and a rate over holes of different "
            "difficulty measures the mix rather than the model (decision 13)."
        )
        return "\n".join(lines)


_VALIDATORS: dict[int, object] = {}


def _validator(document: dict):
    """One validator per schema document, keyed by identity - they are cached upstream."""
    import jsonschema

    key = id(document)
    if key not in _VALIDATORS:
        _VALIDATORS[key] = jsonschema.Draft202012Validator(document)
    return _VALIDATORS[key]


def _post(host: str, path: str, payload: dict, timeout: int) -> dict:
    request = urllib.request.Request(
        f"{host}{path}",
        data=json.dumps(payload).encode(),
        headers={"Content-Type": "application/json"},
    )
    with urllib.request.urlopen(request, timeout=timeout) as response:
        return json.loads(response.read())


def reachable(host: str = DEFAULT_HOST, timeout: int = 5) -> tuple[bool, str]:
    """Is the infer server answering? Returns (ok, detail)."""
    try:
        with urllib.request.urlopen(f"{host}/api/tags", timeout=timeout) as response:
            body = json.loads(response.read())
        names = [m["name"] for m in body.get("models", [])]
        return True, ", ".join(names) or "no models loaded"
    except urllib.error.HTTPError as err:
        return False, f"HTTP {err.code} - the tunnel is up and the service is not"
    except Exception as err:  # noqa: BLE001 - any failure here means "not ready"
        return False, str(err)


def from_model(
    schema: dict,
    matrix: Path,
    model: str,
    host: str = DEFAULT_HOST,
    timeout: int = 180,
    report: Report | None = None,
    context_tokens: int = 32768,
    reserve_tokens: int = 8192,
) -> Fill:
    """A `Fill` that asks the infer server to author each fragment.

    The hole's `prompt` is the English description criterion 22 is about; the hole's
    `schema_ref` becomes the `format`, so the model is constrained by the contract rather
    than asked to remember it. **Neither the library nor the target is consulted.**
    """
    members = _bundle_members(matrix)
    tokens = _bundle_tokens(matrix)
    cache: dict[str, dict] = {}
    budget = context_tokens - reserve_tokens

    def fill(hole: Hole, ctx: dict) -> object:
        if hole.schema_ref not in cache:
            cache[hole.schema_ref] = subschema(schema, hole.schema_ref)
        fmt = cache[hole.schema_ref]
        size = len(json.dumps(fmt, separators=(",", ":"))) // 4
        if size > budget:
            # Refused here rather than sent and truncated. An over-budget schema gets
            # cut by the server, which changes what constrains generation without
            # saying so - the failure I-29 called out, and the reason `Fits` exists.
            attempt = Attempt(hole=hole.name, schema_ref=hole.schema_ref, item=ctx.get("$item"))
            attempt.error = (
                f"schema for {hole.schema_ref} is ~{size} tokens against a {budget} "
                f"budget; refused rather than truncated"
            )
            if report is not None:
                report.attempts.append(attempt)
            return {"$unfilled": hole.name}
        item = ctx.get("$item")
        declarations = as_typescript(fmt, hole.schema_ref)
        instruction = (
            f"{hole.prompt}\n\n"
            f"These are the types you must produce, as TypeScript declarations:\n\n"
            f"```typescript\n{declarations}\n```\n\n"
            f"Produce exactly one JSON value for this hole. It must satisfy the schema "
            f"you have been given, which is the JetStore cpipes contract for "
            f"{hole.schema_ref}."
            # **Tokens, not class names.** `_bundle_members` returns `defs_name`s
            # because that is what the library is keyed by; naming them here told the
            # model that `TransformationColumnSpecValue` was a legal `type`, and it
            # dutifully wrote exactly that. The discriminator value is the token.
            + (
                f"\n\nThe `type` field must be one of: {', '.join(tokens[hole.schema_ref])}."
                if hole.schema_ref in tokens
                else ""
            )
            + (f"\n\nThis instance is for: {json.dumps(item)}" if item is not None else "")
        )
        attempt = Attempt(hole=hole.name, schema_ref=hole.schema_ref, item=item)
        try:
            body = _post(
                host,
                "/api/generate",
                {
                    "model": model,
                    "prompt": instruction,
                    "format": fmt,
                    "stream": False,
                    "options": {"temperature": 0},
                },
                timeout,
            )
            attempt.prompt_tokens = body.get("prompt_eval_count", 0)
            attempt.eval_tokens = body.get("eval_count", 0)
            value = json.loads(body["response"])
            # **`ok` means the fragment validates, not that JSON came back.** The first
            # version set it on a successful parse, and reported 9/9 and 1/1 while every
            # fragment was invalid - a criterion-22 pass rate of 100% where the truth
            # was 0%. Ollama's `format` did not enforce this schema, so a parseable
            # response is no evidence at all and the check has to be ours.
            errors = sorted(
                _validator(fmt).iter_errors(value), key=lambda e: len(e.path)
            )
            if errors:
                at = ".".join(str(p) for p in errors[0].path) or "<root>"
                attempt.error = f"{at}: {errors[0].message[:160]}"
            else:
                attempt.ok = True
            if report is not None:
                report.attempts.append(attempt)
            return value
        except Exception as err:  # noqa: BLE001
            attempt.error = str(err)[:200]
            if report is not None:
                report.attempts.append(attempt)
            # A hole the model could not fill is a *result*, not a crash: the run must
            # continue so the other holes are still measured. Criterion 22 is a per-hole
            # pass rate, and a rate needs the failures counted rather than raised.
            return {"$unfilled": hole.name}

    return fill


# ---------------------------------------------------------------------------
# The schema, in the prompt as well as in `format`
# ---------------------------------------------------------------------------


def as_typescript(document: dict, root: str) -> str:
    """Render a subschema as TypeScript type declarations.

    **`format` alone is not sufficient**, measured here and independently by the
    `tools/sample_projects/tasks_go` sample: with the type declaration removed from that
    sample's prompt the model stops returning a valid task list, and `format` carrying
    the same schema does not save it. F.6's first reading of 0 of 9 was a version that
    sent the schema only as `format`.

    TypeScript rather than JSON Schema because it is what the sample uses and because it
    is far denser - a discriminated union is one line per variant instead of a `oneOf`
    with a mapping - which matters when the budget is what I-29 was about.
    """
    defs = document.get("$defs", {})
    lines: list[str] = []

    def ts(node: dict, depth: int = 0) -> str:
        if "$ref" in node:
            return node["$ref"].split("/")[-1]
        if "oneOf" in node or "anyOf" in node:
            branches = node.get("oneOf") or node.get("anyOf")
            parts = [ts(b, depth) for b in branches if b.get("type") != "null"]
            nullable = any(b.get("type") == "null" for b in branches)
            out = " | ".join(dict.fromkeys(parts)) or "unknown"
            return f"{out} | null" if nullable else out
        if "enum" in node:
            return " | ".join(json.dumps(v) for v in node["enum"])
        if "const" in node:
            return json.dumps(node["const"])
        kind = node.get("type")
        if kind == "array":
            return f"{ts(node.get('items', {}), depth)}[]"
        if kind == "object":
            props = node.get("properties")
            if not props:
                return "Record<string, unknown>"
            required = set(node.get("required", []))
            inner = "; ".join(
                f"{k}{'' if k in required else '?'}: {ts(v, depth + 1)}" for k, v in props.items()
            )
            return "{ " + inner + " }"
        return {"string": "string", "integer": "number", "number": "number", "boolean": "boolean"}.get(
            kind, "unknown"
        )

    for name in sorted(defs):
        node = defs[name]
        body = ts(node)
        doc = (node.get("description") or "").strip().split("\n")[0]
        if doc:
            lines.append(f"// {doc[:110]}")
        lines.append(f"type {name} = {body};")
    lines.append("")
    lines.append(f"// Produce one value of type {root}.")
    return "\n".join(lines)
