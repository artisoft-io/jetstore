---
title: "Regenerate the generated assets"
description: "Regenerate the projected user flows and the emitted table documents after changing what they are generated from."
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

## The table documents, and the emitted flow documents

The `.tc.json` documents of `table_configs/`, and the emitted `.uf.json` of the ported flows, are
emitted from the Flutter corpus, not hand-written:

```sh
cd jetsclient_ide
UPDATE_SCHEMA=1 npm run emit-schema
```

which writes each document to whichever directory it is committed in.

## The agentic data model

The `jets_agentic.*` files of `data_model/` are emitted by `jets-agentic generate` from
`tools/jets_agentic/jets_agentic/model.py`; edit the Python source and regenerate, and
`jets-agentic generate --check` verifies the committed outputs match their source.
