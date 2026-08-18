"""The MCP tool-signature emitter (A3.2) — three tools, typed, classed.

Emits `jets/agentic/tools/jets_agentic_tools.json`, which the Go tool
registry embeds: the signatures live here, in the one Python source, and Go
binds behaviour to the names — a registry test fails if the two sets drift.

Every tool carries its **reversibility class** (proposal §6.3: `na` for
reads, `reversible`, `irreversible` — the last is definitionally absent from
any catalogue) and its **minimum autonomy tier**, validated against
`AutonomyTier`, from day one: retrofitting either across a Phase-2 catalogue
is worse than carrying them with three tools.

Parameter typing: a parameter that is a domain entity `$ref`s into the
item-2a schema via `entity_ref()`; `validate_cpipes_config`'s payload
`$ref`s the item-2b schema the same way. None of the three read-only tools
takes an entity-typed parameter — the first entity-typed parameters arrive
with the Phase-2 write tools — so `entity_ref` is exercised by the emitter's
own self-check rather than by a shipped signature. External `$ref`s are the
*typed* truth; the stdio adapter relaxes them to a self-contained schema at
the wire, because an MCP client cannot resolve a cross-file reference.
"""

from __future__ import annotations

import json

from . import model as M

# Reversibility classes, proposal §6.3. Irreversible tools are excluded from
# any catalogue by definition; the value exists so the exclusion is visible.
REVERSIBILITY = ("na", "reversible", "irreversible")

DOMAIN_SCHEMA = "jets/workspace_assets/data_model/jets_agentic.schema.json"
CPIPES_SCHEMA = "tools/cpipes_contract/cpipes_schema.json"


def entity_ref(entity_name: str) -> dict:
    """A parameter typed against a domain entity — §5.2's 'parameters typed
    against domain entities', concretely. Raises on an unknown entity so a
    signature cannot point at nothing."""
    if entity_name not in {e.__name__ for e in M.ENTITIES}:
        raise KeyError(f"no entity named {entity_name} to $ref")
    return {"$ref": f"{DOMAIN_SCHEMA}#/$defs/{entity_name}"}


TOOLS: list[dict] = [
    {
        "name": "list_domain_classes",
        "description": (
            "List every domain class of the live workspace - the "
            "workspace-wide view from build/classes.json: name, base "
            "classes, whether it persists as a table, and where it was "
            "declared. Read-only."
        ),
        "reversibility": "na",
        "min_tier": "T0",
        "input_schema": {
            "type": "object",
            "properties": {},
            "additionalProperties": False,
        },
    },
    {
        "name": "describe_domain_class",
        "description": (
            "Describe one domain class of the live workspace: its "
            "properties and types from the compiled model, its "
            "documentation from the jets_agentic sidecar (schema-first "
            "classes) or the workspace triples (jr-authored classes), with "
            "any controlled vocabulary inlined in full. A class declared "
            "with no authored metadata says so rather than erroring. "
            "Read-only."
        ),
        "reversibility": "na",
        "min_tier": "T0",
        "input_schema": {
            "type": "object",
            "properties": {
                "name": {
                    "type": "string",
                    "description": (
                        "The class name as declared, prefix included "
                        "(e.g. jetsa:Incident, hc:Claim)."
                    ),
                }
            },
            "required": ["name"],
            "additionalProperties": False,
        },
    },
    {
        "name": "validate_cpipes_config",
        "description": (
            "Validate a compute-pipes configuration with the real "
            "startup-time validator (CpipesStartup.ValidatePipeSpecConfig), "
            "step by step, and report every diagnostic. Validates a copy; "
            "the submitted config is never executed or stored. Read-only."
        ),
        "reversibility": "na",
        "min_tier": "T0",
        "input_schema": {
            "type": "object",
            "properties": {
                "config": {
                    "$ref": f"{CPIPES_SCHEMA}#",
                    "description": (
                        "The cpipes config object, as it would appear in a "
                        ".pc.json file."
                    ),
                }
            },
            "required": ["config"],
            "additionalProperties": False,
        },
    },
    {
        "name": "compile_rule_file",
        "description": (
            "Compile JetRule source with the real compiler (compilerv2) "
            "against a throwaway copy of the live workspace, and report "
            "every diagnostic with the file and line it belongs to. The "
            "submitted text is compiled, never saved: the live workspace, "
            "its workspace.db and its build outputs are untouched. Imports "
            "resolve against the workspace's own rule files, because a rule "
            "file is inlined with its data model before parsing and cannot "
            "be compiled without the vocabulary that gives its terms "
            "meaning. Read-only."
        ),
        "reversibility": "na",
        "min_tier": "T0",
        "input_schema": {
            "type": "object",
            "properties": {
                "rule_text": {
                    "type": "string",
                    "description": (
                        "The JetRule source to compile, as it would appear "
                        "in a .jr file, import statements included."
                    ),
                },
                "file_name": {
                    "type": "string",
                    "description": (
                        "Name to compile the text under, so diagnostics "
                        "about it are attributed to a recognisable file. "
                        "Must be a bare file name ending in .jr, no path "
                        "separators. Defaults to a generated name."
                    ),
                },
            },
            "required": ["rule_text"],
            "additionalProperties": False,
        },
    },
]


def emit() -> str:
    for tool in TOOLS:
        assert tool["reversibility"] in REVERSIBILITY, tool["name"]
        assert tool["min_tier"] in {m.value for m in M.AutonomyTier}, tool["name"]
    # entity_ref's self-check: the mechanism must hold even while no shipped
    # signature uses it (the Phase-2 write tools are its first consumers).
    assert entity_ref("Incident")["$ref"].endswith("#/$defs/Incident")
    return json.dumps(
        {
            "catalog": "jets_agentic_tools",
            "version": M.MODEL_VERSION,
            "generated_by": "jets-agentic generate; source tools/jets_agentic/jets_agentic/toolsig.py - do not edit",
            "schemas": {
                "domain_model": DOMAIN_SCHEMA,
                "cpipes_config": CPIPES_SCHEMA,
            },
            "tools": TOOLS,
        },
        indent=2,
        sort_keys=False,
    ) + "\n"
