"""T.3: the three candidate-selection methods the generate-and-compile run compares.

Criterion 38 asks for compile-pass measured for **lexical against semantic** candidate
selection, per operator with denominators. I-53 measured the same two on a *proxy* —
key-path Jaccard against the part that was actually written — and found the largest gap
either way at 0.034, with four of eleven evaluable types at zero headroom against a
perfect ranker. I-96 took that as grounds to decline retrieval and close item 13, and
recorded the reversal condition this module exists to test: **if semantic beats lexical
on compile-pass, the decision was taken on the wrong metric.**

**What is swapped is the ordering *within* a member type, and nothing else.** `examples_for`
picks one part per member type before a second of any, because three `select` examples
teach less than one each of select, value and eval. That property is orthogonal to
ranking, and I-96 says so in terms — *"the swap is within type, so diversity survives;
I-53 does not state that constraint and it is the one to keep"*. So the three methods here
are three `rank` functions handed to one unchanged picker, rather than three pickers. A
comparison that also changed the diversity rule would not be measuring the ranking.

**The query is the hole's prompt and its `$item`, which is not what I-53 queried with.**
I-53 queried a part's `name` — an identifier — against embedded JSON, and I-54 recorded
that as a fair test of the only semantic retrieval the corpus then supported rather than
of semantic retrieval. Here the query is the English the authoring harness is actually
given, so prose is on one side for the first time. That is a *different* experiment from
I-53's and its result does not correct I-53's; what it does is answer the question I-96
deferred, on the metric I-96 named.

**The semantic arm needs an embedding model and refuses rather than degrading.** If no
model on the inference server will embed, `Embedder.vectors` raises with the server's own
message: an arm that silently fell back to lexical would report the two methods tied and
that reading would be indistinguishable from a real tie.
"""

from __future__ import annotations

import json
import math
import re
from dataclasses import dataclass, field
from typing import Callable, Iterable

CITED = "cited"
LEXICAL = "lexical"
SEMANTIC = "semantic"
METHODS = (CITED, LEXICAL, SEMANTIC)

_WORD = re.compile(r"[a-z0-9]+")


def tokenise(text: str) -> set[str]:
    """Lowercased alphanumeric runs, with `snake_case` and `camelCase` split.

    A cpipes fragment's content is identifiers — `input_row`, `channelSpecName`,
    `jets_partition` — so a tokeniser that keeps them whole matches almost nothing
    against an English prompt, and the lexical arm would be measured at zero for a
    reason that is about the tokeniser rather than about lexical retrieval.
    """
    spaced = re.sub(r"(?<=[a-z0-9])(?=[A-Z])", " ", text)
    return set(_WORD.findall(spaced.lower()))


def document_for(part: dict) -> str:
    """What a candidate is matched *as*.

    The part's own name and its value, not its `description`: I-54 established that all
    3,909 library parts carry the matrix's **type** description, so within a type the
    description is identical text and carries no discriminating signal at all. Matching
    on it would rank every candidate equally and call that a method.
    """
    return f"{part.get('name', '')} {json.dumps(part.get('value', {}))}"


def jaccard(a: set[str], b: set[str]) -> float:
    if not a or not b:
        return 0.0
    return len(a & b) / len(a | b)


# ---------------------------------------------------------------------------
# Embeddings
# ---------------------------------------------------------------------------


@dataclass
class Embedder:
    """`/api/embed` with a cache, for the semantic arm.

    Not implemented over the `embed` operator L.4 shipped, deliberately: that operator
    runs inside a cpipes pipeline over a corpus in a database, and this needs a vector
    for one query string at authoring time. They share the `inferBackend` seam and
    nothing else.
    """

    model: str
    host: str
    timeout: int = 180
    _cache: dict[str, list[float]] = field(default_factory=dict)

    def vectors(self, texts: list[str]) -> list[list[float]]:
        import urllib.error
        import urllib.request

        missing = [t for t in dict.fromkeys(texts) if t not in self._cache]
        for i in range(0, len(missing), 64):
            batch = missing[i : i + 64]
            payload = json.dumps({"model": self.model, "input": batch}).encode()
            req = urllib.request.Request(
                f"{self.host}/api/embed",
                data=payload,
                headers={"Content-Type": "application/json"},
            )
            try:
                with urllib.request.urlopen(req, timeout=self.timeout) as resp:
                    body = json.loads(resp.read())
            except urllib.error.HTTPError as err:
                detail = err.read().decode(errors="replace")[:400]
                raise RuntimeError(
                    f"the semantic arm needs an embedding model and {self.model!r} "
                    f"would not embed: {detail}"
                ) from err
            if "embeddings" not in body:
                raise RuntimeError(
                    f"the semantic arm needs an embedding model and {self.model!r} "
                    f"would not embed: {body.get('error', body)}"
                )
            for text, vec in zip(batch, body["embeddings"]):
                self._cache[text] = vec
        return [self._cache[t] for t in texts]


def cosine(a: list[float], b: list[float]) -> float:
    num = sum(x * y for x, y in zip(a, b))
    na = math.sqrt(sum(x * x for x in a))
    nb = math.sqrt(sum(y * y for y in b))
    if na == 0 or nb == 0:
        return 0.0
    return num / (na * nb)


# ---------------------------------------------------------------------------
# The three rankers
# ---------------------------------------------------------------------------

Ranker = Callable[[list[dict]], list[dict]]


def ranker_for(
    method: str, query: str, embedder: Embedder | None = None
) -> Ranker:
    """A function ordering one member type's candidates best-first.

    `cited` is the shipped ordering — most production instances, then most instances —
    and is the baseline both other arms are measured against rather than a third
    contender. It is what `examples_for` has always done.
    """
    if method == CITED:
        return lambda parts: sorted(
            parts, key=lambda p: (-p.get("prod_instances", 0), -p["instances"])
        )
    if method == LEXICAL:
        q = tokenise(query)
        return lambda parts: sorted(
            parts,
            key=lambda p: (
                -jaccard(q, tokenise(document_for(p))),
                -p.get("prod_instances", 0),
                -p["instances"],
            ),
        )
    if method == SEMANTIC:
        if embedder is None:
            raise ValueError("the semantic arm needs an embedder")

        def rank(parts: list[dict]) -> list[dict]:
            docs = [document_for(p) for p in parts]
            vecs = embedder.vectors([query] + docs)
            qv, dvs = vecs[0], vecs[1:]
            scored = list(zip(parts, (cosine(qv, d) for d in dvs)))
            return [
                p
                for p, _ in sorted(
                    scored,
                    key=lambda pair: (
                        -pair[1],
                        -pair[0].get("prod_instances", 0),
                        -pair[0]["instances"],
                    ),
                )
            ]

        return rank
    raise ValueError(f"{method!r} is not one of {METHODS}")


def query_for(prompt: str, item: object) -> str:
    """The text a hole is ranked against: its English prompt, plus the binding this
    repetition is for. A repeating hole asks for a different fragment each time round,
    and a query that ignored `$item` would rank once and reuse it — which is not a
    retrieval method, it is a cache."""
    if item is None:
        return prompt
    return f"{prompt}\n{json.dumps(item)}"


def pick(
    candidates_by_type: dict[str, list[dict]], rank: Ranker, limit: int
) -> list[dict]:
    """One part per member type, best-first within a type, before any second of a type.

    Lifted out of `examples_for` unchanged so the three methods share it exactly. The
    outer ordering — which member type gets its first pick first — stays keyed on
    instance count rather than on the ranker's score, because it is the diversity rule
    rather than part of the ranking under test.
    """
    best = {name: rank(parts) for name, parts in candidates_by_type.items()}
    picked: list[dict] = []
    round_ = 0
    while len(picked) < limit and any(len(v) > round_ for v in best.values()):
        for name in sorted(best, key=lambda n: -best[n][0]["instances"]):
            if len(picked) >= limit:
                break
            if len(best[name]) > round_:
                picked.append(best[name][round_])
        round_ += 1
    return picked


def candidates_by_type(
    schema_ref: str, library: Iterable[dict], members: dict[str, list[str]]
) -> dict[str, list[dict]]:
    wanted = members.get(schema_ref, [schema_ref])
    out: dict[str, list[dict]] = {}
    for part in library:
        if part["defs_name"] in wanted:
            out.setdefault(part["defs_name"], []).append(part)
    return out
