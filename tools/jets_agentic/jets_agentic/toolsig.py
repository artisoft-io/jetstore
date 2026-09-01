"""The MCP tool-signature emitter (A3.2) — the tool catalogue, typed, classed.

**Six tools as of T.1/T.2 (2026-08-31), and the count is deliberately not in this
line any more.** It said "three tools" through Phase 1's fourth and would have
said "four" through Phase 3's sixth; a docstring that carries an inventory is a
docstring that lags the list twenty lines below it, which `TOOLS` already is.

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
`$ref`s the item-2b schema the same way. No read-only tool
takes an entity-typed parameter — the first entity-typed parameters arrive
with the Phase-2 write tools — so `entity_ref` is exercised by the emitter's
own self-check rather than by a shipped signature. External `$ref`s are the
*typed* truth; the stdio adapter relaxes them to a self-contained schema at
the wire, because an MCP client cannot resolve a cross-file reference.

**A path argument is not the local-disk shape `Workspace`'s comment refuses**
(T.2, I-94's remedy). That rule is about the *workspace root* — the stdio
adapter resolves it, so the registry interface never learns where a checkout
sits on this machine. A **workspace-relative** path names a file *inside* a
workspace the handle already resolved, and carries nothing about the host. The
distinction is what lets `compile_rule_file` and `validate_cpipes_config` take
a path instead of a payload, which is the whole of I-94's remedy: I-68 measured
the model reproducing a rule file correctly 9 times in 18, worse with a
verbatim-copy instruction, and a model that never holds the text cannot mangle
it.

**Neither tool is path-only, and the residue is stated rather than discovered.**
A rule or a config the model composed and has not saved has no path, so the
content argument stays. `oneOf` on the two `required` sets is what says exactly
one of them.
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
        "name": "list_workspace_files",
        "description": (
            "List the live workspace's authored source files - rule files, "
            "pipeline configurations, lookup tables, process configurations "
            "and documentation - as paths relative to the workspace root. "
            "Optionally filtered by a glob, where * matches within one path "
            "segment and ** matches across segments "
            "(e.g. 'pipes_config/*.pc.json', '**/*.jr'). Compiled outputs "
            "and version-control metadata are not listed. Read-only."
        ),
        "reversibility": "na",
        "min_tier": "T0",
        "input_schema": {
            "type": "object",
            "properties": {
                "pattern": {
                    "type": "string",
                    "description": (
                        "Glob over workspace-relative paths. * matches "
                        "within a path segment, ** across segments. "
                        "Defaults to every listable file."
                    ),
                },
                "max_results": {
                    "type": "integer",
                    "minimum": 1,
                    "description": (
                        "Stop after this many matches, so a large workspace "
                        "cannot fill the context. The reply says whether it "
                        "was truncated."
                    ),
                },
            },
            "additionalProperties": False,
        },
    },
    {
        "name": "read_workspace_file",
        "description": (
            "Read one of the live workspace's authored source files by its "
            "workspace-relative path - the path list_workspace_files "
            "returns. The file is read, never written. Read-only."
        ),
        "reversibility": "na",
        "min_tier": "T0",
        "input_schema": {
            "type": "object",
            "properties": {
                "path": {
                    "type": "string",
                    "description": (
                        "Workspace-relative path, exactly as "
                        "list_workspace_files reports it "
                        "(e.g. 'jet_rules/main.jr'). Not an absolute path, "
                        "and it may not leave the workspace."
                    ),
                },
                "max_bytes": {
                    "type": "integer",
                    "minimum": 1,
                    "description": (
                        "Stop after this many bytes. The reply says whether "
                        "the content was truncated."
                    ),
                },
            },
            "required": ["path"],
            "additionalProperties": False,
        },
    },
    {
        "name": "validate_cpipes_config",
        "description": (
            "Validate a compute-pipes configuration with the real "
            "startup-time validator (CpipesStartup.ValidatePipeSpecConfig), "
            "step by step, and report every diagnostic. Validates a copy; "
            "the submitted config is never executed or stored. Give it "
            "EITHER 'config_path' - the workspace-relative path of an "
            "existing .pc.json, which is preferred whenever the "
            "configuration is already in the workspace, because the tool "
            "then reads the file itself and nothing has to be copied - OR "
            "'config', the configuration object itself, for one that has "
            "not been saved. Read-only."
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
                        ".pc.json file. Use this only for a configuration "
                        "that is not in the workspace; prefer config_path."
                    ),
                },
                "config_path": {
                    "type": "string",
                    "description": (
                        "Workspace-relative path of a .pc.json to validate "
                        "(e.g. 'pipes_config/loader.pc.json'). The tool "
                        "reads the file, so the configuration is never "
                        "carried through this call."
                    ),
                },
            },
            "oneOf": [{"required": ["config"]}, {"required": ["config_path"]}],
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
            "meaning. Read-only. Give it EITHER 'rule_path' - the "
            "workspace-relative path of an existing .jr, which is preferred "
            "whenever the rule is already in the workspace, because the "
            "tool then reads the file itself and the source is never "
            "carried through this call - OR 'rule_text', the source itself, "
            "for a rule that has not been saved."
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
                        "in a .jr file, import statements included. Use "
                        "this only for a rule that is not in the "
                        "workspace; prefer rule_path."
                    ),
                },
                "rule_path": {
                    "type": "string",
                    "description": (
                        "Workspace-relative path of a .jr file to compile "
                        "(e.g. 'jet_rules/main.jr'). It is compiled at that "
                        "path, so its imports resolve exactly as they do in "
                        "the workspace."
                    ),
                },
                "file_name": {
                    "type": "string",
                    "description": (
                        "Name to compile the text under, so diagnostics "
                        "about it are attributed to a recognisable file. "
                        "Must be a bare file name ending in .jr, no path "
                        "separators. Defaults to a generated name. Applies "
                        "to rule_text only; rule_path is compiled under its "
                        "own name."
                    ),
                },
            },
            "oneOf": [{"required": ["rule_text"]}, {"required": ["rule_path"]}],
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
