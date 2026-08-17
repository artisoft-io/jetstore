# JetStore-owned workspace assets — data model

This directory is the single home for data-model assets that JetStore owns and
installs into client workspaces at image-build time (`Dockerfile.compile_ws`
copies them into `$WORKSPACES_REPO/$WORKSPACE/data_model/` before
`compile_workspace` runs). Files here are **installed, not authored in place**
in a workspace — a locally modified copy of a JetStore-owned file fails the
install rather than being overwritten.

## The two-authoring-surface rule

There are two ways a data model gets written, and the split is deliberate, not
drift:

- **The agentic model (`jets_agentic.jr`, `jets_agentic.meta.json`) is
  schema-first in Python.** Its source of truth is
  `tools/jets_agentic/jets_agentic/model.py`; the files here are emitted by
  `jets-agentic generate` and carry a GENERATED header. Edit the Python source
  and regenerate — never edit the emitted files. `jets-agentic generate
  --check` verifies the committed outputs match their source, and
  `jets-agentic compile` compiles the emitted `.jr` in a throwaway workspace
  and asserts the class/vocabulary/round-trip properties.

- **Existing data models and all rules stay in `.jr`.** Client workspaces
  author their own models and rules in the JetRules DSL exactly as before;
  nothing migrates. Client extensions to JetStore-owned models live in the
  client's own `.jr` files importing the installed ones, never inside the
  installed files themselves.

Why two surfaces: the agentic entities need property metadata the `.jr`
grammar does not carry (descriptions, defaults, requiredness,
`data_classification`) and downstream emitters (JSON Schema for constrained
decoding, glossary, tool signatures) that must stay consistent with one
source. Rule-visible metadata is emitted as `triple(...)` facts; emitter-only
metadata goes to the sidecar (`jets_agentic.meta.json`). Whether existing
models ever follow the schema-first route is a later decision that the
Phase-0 experience should inform, not pre-empt.

## Namespace

Every class and property the agentic model emits carries the reserved
`jetsa:` prefix. Workspace property names are one flat workspace-wide
namespace (the compiler enforces global uniqueness of property names), so
client models must not use the `jetsa:` prefix. Before the first install into
a workspace, `jets-agentic precheck <workspace.db>` verifies no emitted name
collides with the target's `domain_classes`, `data_properties` or
`object_properties`, and fails naming the collision.
