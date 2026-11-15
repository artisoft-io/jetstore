---
title: "Regenerate the generated assets"
description: "Regenerate the projected user flows and the agentic data model after changing what they are generated from."
kind: how-to
sidebar:
  order: 7
---

Some of the assets JetStore installs are emitted by a generator and committed. Regenerate them when
what they are generated from changes, and commit the diff with whatever caused it.

## The projected user flows

```sh
cd tools/cpipes_contract
python -m cpipes_contract templates --project ../../jets/workspace_assets/user_flows
```

A change to `project.py` or to a template that does not move these files is a change with no
observable effect — which is why they are committed rather than generated at build time, and it is
the same argument the fixtures beside the templates are committed under.

## The table documents and the ported flows are not generated

The `.tc.json` documents of `table_configs/` and the `.uf.json` of the ported flows were emitted from
the Flutter corpus until 2026-11-14. The corpus cannot be regenerated, so nothing regenerates them
now: edit the committed document. `jetsclient_ide/src/datatable/table.test.ts` and
`jetsclient_ide/src/userflow/schema.test.ts` still compare each one nobody has edited with its
translation from the corpus, so list a document you edit in the test as well (`HAND_MAINTAINED_KEYS`
for a table, `HAND_AUTHORED` for a flow, whose entry in the corpus is deleted); the test's message
says so.

`npm run emit-schema` in `jetsclient_ide` writes the JSON Schemas of the UserFlow documents, and
nothing else.

## The agentic data model

The `jets_agentic.*` files of `data_model/` are emitted by `jets-agentic generate` from
`tools/jets_agentic/jets_agentic/model.py`; edit the Python source and regenerate, and
`jets-agentic generate --check` verifies the committed outputs match their source.
