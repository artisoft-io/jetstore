"""The Python counterpart of the Go mains' `utils.UseJetStoreLogger()`.

`jets/utils/use_zap_logger.go` makes every Go main log one JSON object per record,
through zap, so that a value carrying CR/LF cannot forge a second log record
(CWE-117, log injection; `logger_replacement.md` at the repository root). The
Python cp_node Lambda logged through the runtime's plain-text handler, where a
pipeline-config string, an S3 key or an event field containing a newline is
written as is.

`setup()` gives the Python side the same property and the same shape: one line per
record, `{"level": ..., "ts": ..., "msg": ...}`, the field names zap's production
encoder writes, plus `stacktrace` when the record carries an exception. The JSON
encoder escapes CR, LF and every other control character inside `msg`, which is
the whole of the defence, as it is in Go.

Call it once, at an entry point, before anything logs. As in Go, a dev-mode
entry point keeps the plain logging.
"""

from __future__ import annotations

import json
import logging
import sys
import time

__all__ = ["JsonFormatter", "setup"]


class JsonFormatter(logging.Formatter):
    """One JSON object per record, in zap's production field names."""

    def format(self, record: logging.LogRecord) -> str:
        entry: dict[str, str] = {
            "level": record.levelname.lower(),
            "ts": self.formatTime(record),
            "msg": record.getMessage(),
        }
        if record.exc_info:
            entry["stacktrace"] = self.formatException(record.exc_info)
        elif record.stack_info:
            entry["stacktrace"] = self.formatStack(record.stack_info)
        return json.dumps(entry, ensure_ascii=False)

    def formatTime(self, record: logging.LogRecord, datefmt: str | None = None) -> str:  # noqa: N802
        """ISO 8601 in UTC with milliseconds, as zap's `ISO8601TimeEncoder`."""
        secs = time.strftime("%Y-%m-%dT%H:%M:%S", time.gmtime(record.created))
        return f"{secs}.{int(record.msecs):03d}Z"


def setup(level: str | None = None, dev_mode: bool = False, logger: logging.Logger | None = None) -> None:
    """Make the root logger write JSON, the way `UseJetStoreLogger` does in Go.

    The entry point passes `level` (`LOG_LEVEL`, default INFO) and `dev_mode`
    (`JETSTORE_DEV_MODE` set) from its environment: this package keeps its
    environment reads to the modules the Lambda's variables are derived from
    (`tests_deployment.py`).

    The formatter is put on the logger's existing handlers rather than on a new
    one: the Lambda runtime installs its own handler on the root logger, and
    adding a second would write every record twice. With no handler (a local
    run), one stdout handler is added. `logger` is the root logger unless given,
    which is for tests.
    """
    target = logger if logger is not None else logging.getLogger()
    target.setLevel((level or "INFO").upper())
    if dev_mode:
        target.info("JETSTORE_DEV_MODE is set: using plain logging")
        return
    if not target.handlers:
        target.addHandler(logging.StreamHandler(sys.stdout))
    formatter = JsonFormatter()
    for handler in target.handlers:
        handler.setFormatter(formatter)
    if logger is None:
        logging.captureWarnings(True)
