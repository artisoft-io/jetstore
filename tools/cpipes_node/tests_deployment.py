"""The deployed function's environment against what this package reads.

**The CDK map for the Python node Lambda was derived from this package once,
on 2026-09-18, and then drifted.** At that date `settings.py` was the only
module reading the environment and the map carried its five variables and
`LOG_LEVEL`. `merge.py` later began reading the four S3 area prefixes, the map
did not follow, and the first deployed run built every stage key from an empty
prefix - `/process_name=...` - which the object store refused (2026-09-27).

So the relation is asserted rather than remembered, in both directions and from
the package's own constants: every variable a module reads is in the map, and
the map carries nothing the package does not read except the handler's
`LOG_LEVEL`. A module that starts reading the environment somewhere else fails
the third test, which is what keeps the first two honest.
"""

from __future__ import annotations

import re
from pathlib import Path

from cpipes_node import merge, settings

HERE = Path(__file__).parent
PACKAGE = HERE / "cpipes_node"
#: `tools/cpipes_node` -> `tools` -> the JetStore tree.
CDK = HERE.parents[1] / "cdk" / "jetstore_one" / "stack" / "build_cpipes_lambdas.go"

#: The handler's, not the package's (dockerfiles/cpipes_node_lambda/handler.py).
HANDLER_ONLY = {"LOG_LEVEL"}

READ_BY_PACKAGE = set(settings.REQUIRED) | set(settings.OPTIONAL) | {
    merge.STAGE_PREFIX_ENV,
    merge.OUTPUT_PREFIX_ENV,
    merge.INPUT_PREFIX_ENV,
    merge.SCHEMA_EVENTS_PREFIX_ENV,
}


def python_lambda_environment() -> set[str]:
    """The keys of the `Environment` map in the CpipesPythonNodeLambda block."""
    src = CDK.read_text()
    start = src.index('NewDockerImageFunction(stack, jsii.String("CpipesPythonNodeLambda")')
    env = src.index("Environment: &map[string]*string{", start)
    body = src[env : src.index("\n\t\t\t},", env)]
    return set(re.findall(r'^\s*"([A-Za-z0-9_]+)":', body, re.MULTILINE))


def test_the_function_carries_every_variable_the_package_reads():
    missing = READ_BY_PACKAGE - python_lambda_environment()
    assert missing == set(), f"read by cpipes_node, absent from the Lambda: {sorted(missing)}"


def test_the_function_carries_nothing_the_package_does_not_read():
    extra = python_lambda_environment() - READ_BY_PACKAGE - HANDLER_ONLY
    assert extra == set(), f"on the Lambda, read by nothing here: {sorted(extra)}"


def test_only_the_two_modules_the_constants_come_from_read_the_environment():
    readers = sorted(
        p.relative_to(PACKAGE).as_posix()
        for p in PACKAGE.rglob("*.py")
        if re.search(r"os\.environ|os\.getenv", p.read_text())
    )
    assert readers == ["merge.py", "settings.py"], (
        "a module reads the environment that this file does not derive the "
        f"Lambda's map from: {readers}"
    )
