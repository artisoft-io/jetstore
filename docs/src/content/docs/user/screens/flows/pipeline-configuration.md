---
title: Pipeline Configuration
description: The Pipeline Configuration flow, which adds, edits and deletes a client's pipeline configurations, their data sources, their automation and their rule configuration.
kind: reference
---

A **pipeline configuration** says, for one client and one process, which data sources a run reads,
how often it runs, whether it starts on its own when its files arrive, and the rule configuration it
adds. The *Pipeline Configuration* flow lists them and walks through adding or editing one, page by
page, to a summary that saves it. How a pipeline configuration and a run relate is explained in
[Pipelines and sessions](/docs/user/concepts/pipelines-and-sessions/).

**Reach it** from the *JetStore Workspace* menu at the left of the top bar, *Pipeline Configuration*.
Reading needs `jetstore_read`; saving and deleting, a data source included, need `client_config`.

## The list

![The pipeline configurations, among them CI's Map_Eligibility, Map_MedicalClaim, Map_PharmacyClaim and Patient_Profile_Template, with the + Add and Delete buttons, and Close and Edit below.](../../../../../assets/screenshots/user/screens/flows/pipeline-configuration/select-pipeline-config.png)

| Column | What it shows |
|---|---|
| **Client** | The client, or `Any` for JetStore's own. |
| **Process** | The process the pipeline runs. |
| **Domain Key** | The domain key its rule sessions are grouped by. |
| **Main Source Type** | The type of its main data source. |
| **Automated** | `1` if it starts on its own when its files arrive. |
| **Description** | Free text. |
| **Loaded At** | When it was last saved. |

| Button | What it does |
|---|---|
| **+ Add** | Starts a new pipeline configuration, on the add page. |
| **Delete** | Deletes the selected pipeline configuration, after a confirmation. |
| **Close** | Leaves the flow. |
| **Edit** | Edits the selected pipeline configuration, from the main data source. |

Every page after this one has **Previous**, **Cancel**, which abandons the add or the edit and returns
to this list, and **Next**.

## Adding: client and process

![The add page, with the Client and Process pickers and the Description.](../../../../../assets/screenshots/user/screens/flows/pipeline-configuration/add-pipeline-config.png)

**Client** and **Process** are required; **Description** is optional.

## The main data source

![The main data source page of Patient_Profile_Template: the client's data sources, the hc:Eligibility domain table ticked, and the Add/Update Data Source Configuration button.](../../../../../assets/screenshots/user/screens/flows/pipeline-configuration/main-process-input.png)

Tick the **main data source**: the input a run is started on, and whose records each rule session is
built around. The table lists the client's data sources:

| Column | What it shows |
|---|---|
| **Client**, **Organization** | Whose data it is. |
| **Domain Key** | The key its records are grouped by. |
| **Domain Class** | The class its records belong to. |
| **Source Type** | `file` for a staging table loaded from files, `domain_table` for a table a pipeline wrote. |
| **Table Name** | The table it is read from. |
| **Lookback Periods** | How many earlier periods are read with the current one. |
| **User**, **Loaded At** | Who saved it, and when. |

**Add/Update Data Source Configuration** opens the *New Process Input* dialog, which adds a data
source, or updates the selected one: choose a table from those registered for the client, and give
its **Lookback Periods**, the number of periods to include in the rule session. **Save** saves it.

## Merged data sources and historical data

![The merged data sources of Patient_Profile_Template: hc:MedicalClaim and hc:PharmacyClaim with six lookback periods each, and the Add Data Source to Merge button.](../../../../../assets/screenshots/user/screens/flows/pipeline-configuration/merged-inputs.png)

Two pages of the same shape:

- **Merged Data Sources**: other sources whose records join the main source's in each rule session,
  matched on the domain key.
- **Data sources for historical data**: sources whose records are read as history.

On each, **Add Data Source to Merge** (or **Add Data Source for Historical Data**) opens a page that
lists the client's data sources; tick one and press **Next** to add it. **Show/Hide Select Row**
shows the tick boxes, and **Remove** takes the selected source out of the set.

![The historical data page of Patient_Profile_Template, with no source, and the Add Data Source for Historical Data button.](../../../../../assets/screenshots/user/screens/flows/pipeline-configuration/injected-inputs.png)

## Automation and rule configuration

![The automation page: Execution Frequency, Automation Mode, and the Rule Configuration box.](../../../../../assets/screenshots/user/screens/flows/pipeline-configuration/automation.png)

| Field | Rule |
|---|---|
| **Execution Frequency** | Required: *Monthly*, *Weekly* or *Daily*, the period a run's source period covers. |
| **Automation Mode** | Required: *Automated* starts a run when the pipeline's files have arrived; *Manual* leaves it to a person. |
| **Rule Configuration (csv or json)** | Required: the objects added to every rule session of this pipeline, as on the [Rules Configuration](/docs/user/screens/rules-configuration/) screen. `[]` when there are none. |

## The summary

![The summary of Patient_Profile_Template: client CI, monthly and automated, its description, its three data sources and its empty rule configuration, with Previous, Cancel and Save.](../../../../../assets/screenshots/user/screens/flows/pipeline-configuration/summary.png)

The summary shows the **Client**, the **Process Name**, the frequency and the mode, the
**Description**, which can still be changed, the **Data Sources** (the main, merged and historical
ones), and the rule configuration. **Save** saves the pipeline configuration and returns to the list.
