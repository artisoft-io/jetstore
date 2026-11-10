---
title: "Workspace assets JetStore installs"
description: "The four asset groups JetStore owns and installs into every workspace at image-build time, and how to change or regenerate them."
kind: overview
sidebar:
  label: Overview
---

`install_workspace_assets` (`jets/workspace_assets/install.go`) installs the files JetStore owns into
every workspace at image-build time, before the workspace is compiled. Each group is a directory of
`jets/workspace_assets/`, installed into the directory of the same name in the workspace, under a
guard that refuses a copy edited in place.

| Group | Page |
|---|---|
| `data_model/` | [The data model JetStore installs](/docs/developer/workspace/assets/data-model/), which also describes the guard every group shares |
| `pipes_config/` | [The pipeline configurations JetStore installs](/docs/developer/workspace/assets/pipeline-configurations/) |
| `user_flows/` | [The user flows JetStore ships](/docs/developer/workspace/assets/user-flows/), and their actions in [Flow actions](/docs/developer/workspace/assets/flow-actions/) |
| `table_configs/` | [The table documents JetStore's flows draw](/docs/developer/workspace/assets/table-configs/) |

To change what a workspace runs, see
[Change or extend an installed asset](/docs/developer/workspace/assets/change-an-installed-asset/);
to rebuild the generated ones, [Regenerate the generated assets](/docs/developer/workspace/assets/regenerate-assets/).
