"""`jetstore_logging.setup()`: one JSON line per record, and no forged record.

The Python counterpart of `TestUseJetStoreLoggerNeutralizesLogForging`
(`jets/utils/use_zap_logger_test.go`). Each test sets up its own
standalone logger (`logging.Logger`, outside the manager's registry), not the
root one: pytest attaches its capture handlers to the loggers it knows of, and
reformatting those would leak into other tests' `caplog`.
"""

import io
import json
import logging

import pytest

from cpipes_node import jetstore_logging


@pytest.fixture
def logger_stream(request):
    """A fresh logger with one handler writing to a buffer."""
    logger = logging.Logger(f"jetstore_logging_test.{request.node.name}")
    logger.propagate = False
    stream = io.StringIO()
    logger.handlers = [logging.StreamHandler(stream)]
    yield logger, stream
    logger.handlers = []


def test_a_forged_record_stays_inside_one_entry(logger_stream):
    logger, stream = logger_stream
    jetstore_logging.setup(logger=logger)
    forged = 'alice\r\n{"level":"info","ts":"2026-01-01T00:00:00.000Z","msg":"admin logged in"}'
    logger.info("user %s", forged)
    lines = stream.getvalue().splitlines()
    assert len(lines) == 1
    entry = json.loads(lines[0])
    assert entry["level"] == "info"
    assert entry["msg"] == f"user {forged}"
    assert entry["ts"].endswith("Z")


def test_an_exception_is_a_stacktrace_field_on_the_same_line(logger_stream):
    logger, stream = logger_stream
    jetstore_logging.setup(logger=logger)
    try:
        raise ValueError("bad\nvalue")
    except ValueError:
        logger.exception("failed")
    lines = stream.getvalue().splitlines()
    assert len(lines) == 1
    entry = json.loads(lines[0])
    assert entry["level"] == "error"
    assert "ValueError: bad\nvalue" in entry["stacktrace"]


def test_the_existing_handler_is_reused_not_doubled(logger_stream):
    logger, stream = logger_stream
    jetstore_logging.setup(logger=logger)
    jetstore_logging.setup(logger=logger)
    logger.warning("once")
    assert len(logger.handlers) == 1
    assert len(stream.getvalue().splitlines()) == 1


def test_a_logger_with_no_handler_gets_one():
    logger = logging.Logger("jetstore_logging_test.bare")
    logger.propagate = False
    logger.handlers = []
    jetstore_logging.setup(logger=logger)
    assert len(logger.handlers) == 1
    assert isinstance(logger.handlers[0].formatter, jetstore_logging.JsonFormatter)
    logger.handlers = []


def test_dev_mode_keeps_plain_logging(logger_stream):
    logger, stream = logger_stream
    jetstore_logging.setup(dev_mode=True, logger=logger)
    logger.info("plain")
    assert stream.getvalue().splitlines()[-1] == "plain"


def test_the_level_is_applied(logger_stream):
    logger, stream = logger_stream
    jetstore_logging.setup(level="warning", logger=logger)
    logger.info("dropped")
    assert stream.getvalue() == ""
