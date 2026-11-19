"""Aliases between the names this package records and the corpus as it is on disk.

**The data here names the corpus's workspaces by neutral names** - `org1_ws`, `org2_ws`
and so on - and so does every class prefix, environment variable or file name in a
fragment that would otherwise say whose deployment it came from. The corpus itself is
read from a checkout of the deployments' own repositories, whose directories and
contents carry their real names. This module is the one place the two meet.

**The mapping is not in this repository, and that is the point of it.** It is supplied
from outside, by `CPIPES_CONTRACT_ALIASES` naming a JSON file whose object maps each
real token to the neutral one that replaces it:

    {"acme_ws": "org1_ws", "acme": "org1", "ACME": "ORG1"}

Two directions use it:

- **anonymise** (real to neutral) is applied to everything read out of the corpus that
  is recorded or compared: fragment values and their citations, repair cases, and the
  corpus configurations the template tests reproduce.
- **resolve** (neutral to real) is applied to a recorded path before it is opened, so
  `workspaces/org1_ws/pipes_config/x.pc.json` opens the directory the file names.
  Where two real tokens share a neutral one, the first in the file wins.

A token matches only as a whole word: not preceded or followed by a letter or a digit,
so `_`, `:`, `-`, `.` and `/` separate it from its neighbours. Longer tokens are
tried first. A compound such as a class name that runs a token into other letters is
listed as a token of its own.

**Without the file, both directions are the identity**, and a recorded neutral path
does not resolve. Every consumer treats that as it treats a workspace that is not
checked out: a citation is set aside, a test that needs the corpus skips.
"""

from __future__ import annotations

import json
import os
import re
from functools import lru_cache
from pathlib import Path
from typing import Any

ENV = "CPIPES_CONTRACT_ALIASES"


class Aliases:
    def __init__(self, real_to_neutral: dict[str, str] | None = None):
        self.forward: dict[str, str] = dict(real_to_neutral or {})
        self.backward: dict[str, str] = {}
        for real, neutral in self.forward.items():
            self.backward.setdefault(neutral, real)
        self._fwd = _pattern(self.forward)
        self._bwd = _pattern(self.backward)

    def __bool__(self) -> bool:
        return bool(self.forward)

    def anonymise(self, text: str) -> str:
        """Replace every real token in `text` by its neutral one."""
        if self._fwd is None:
            return text
        return self._fwd.sub(lambda m: self.forward[m.group(0)], text)

    def resolve(self, text: str) -> str:
        """Replace every neutral token in `text` by the real one: for a path to open."""
        if self._bwd is None:
            return text
        return self._bwd.sub(lambda m: self.backward[m.group(0)], text)

    def anonymise_json(self, value: Any) -> Any:
        """`anonymise` applied to every string in a parsed JSON value, keys included."""
        if not self:
            return value
        if isinstance(value, str):
            return self.anonymise(value)
        if isinstance(value, list):
            return [self.anonymise_json(v) for v in value]
        if isinstance(value, dict):
            return {self.anonymise(k): self.anonymise_json(v) for k, v in value.items()}
        return value

    def path(self, root: Path, recorded: str | Path) -> Path:
        """The file a recorded path names, under `root`."""
        return root / self.resolve(str(recorded))


def _pattern(tokens: dict[str, str]) -> re.Pattern[str] | None:
    if not tokens:
        return None
    alternation = "|".join(re.escape(t) for t in sorted(tokens, key=len, reverse=True))
    return re.compile(rf"(?<![A-Za-z0-9])(?:{alternation})(?![A-Za-z0-9])")


def from_file(path: Path) -> Aliases:
    data = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(data, dict) or not all(
        isinstance(k, str) and isinstance(v, str) and k and v for k, v in data.items()
    ):
        raise ValueError(f"{path}: an alias file is one JSON object of non-empty strings")
    return Aliases(data)


@lru_cache(maxsize=None)
def _cached(path: str) -> Aliases:
    return from_file(Path(path))


def load() -> Aliases:
    """The aliases `CPIPES_CONTRACT_ALIASES` names, or none when it is unset."""
    path = os.environ.get(ENV, "")
    return _cached(path) if path else Aliases()
