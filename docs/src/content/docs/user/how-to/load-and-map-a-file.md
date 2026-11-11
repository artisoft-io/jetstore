---
title: Load and map a file
description: Check that a new file arrived and registered, load it into its staging table, check its columns against its source mapping, and run its mapping pipeline.
kind: how-to
sidebar:
  order: 1
---

**Before you start**: the file's source configuration exists for its client, organisation and object
type (the *Source Configuration* flow), and your account has `run_pipelines`. The example is `CI`'s
eligibility file from `D1` for January 2026.

1. **Put the file in the bucket** at the path its source configuration expects, for example
   `jetstore/input/client=CI/org=D1/object_type=hc:Eligibility/year=2026/month=1/day=5/<file name>`.
   [Files and the input registry](/docs/user/concepts/files-and-the-input-registry/) explains the path.

2. **Check it registered.** On **Home**, open the **Data Registry** tab. The file has a row with
   source type `file`, its staging table and a session id. It appears within seconds of landing; if
   it does not, see the Installation Guide's [troubleshooting ingest](/docs/install/how-to/troubleshoot-ingest/).

   ![The Data Registry tab with the January CI eligibility file, source type file and session 1767603600101, ticked.](../../../../assets/screenshots/user/how-to/load-and-map-a-file/file-registered.png)

3. **Choose its source.** Open the brand menu, **JetStore Workspace**, choose **Source Mapping**, and
   tick the source, here `CI_D1_hc:Eligibility`.

   ![Source Mapping's first step: CI's source configurations, with CI_D1_hc:Eligibility ticked and the Load Data and Loader Status buttons.](../../../../assets/screenshots/user/how-to/load-and-map-a-file/select-source.png)

4. **Load it.** Press **Load Data**, tick the file in the [Load Files](/docs/user/screens/flows/load-files/)
   list, and press **Load Files & Done**. The loader copies the file's rows into the staging table, in
   the background.

   ![The Load Files flow with the January eligibility file ticked, above the Load Files & Done button.](../../../../assets/screenshots/user/how-to/load-and-map-a-file/load-files.png)

5. **Check the load.** Open **Source Mapping** again and press **Loader Status**. The file's row says
   `completed`, with the records loaded and the bad records. A bad record is a row the loader could not
   read; the error message column says why.

   ![File Loader Status: the three CI files of 5 January 2026, each completed with no bad records; the eligibility file loaded 22 records.](../../../../assets/screenshots/user/how-to/load-and-map-a-file/file-loader-status.png)

   To see the rows, tick the file and press **View Loaded Data**: see
   [Look at loaded data](/docs/user/how-to/look-at-loaded-data/).

6. **Check the mapping.** Open **Source Mapping**, tick the source and press **Next**. Each row maps a
   data model property to one of the file's columns, with an optional cleansing function. Every
   source input column should be one the loaded file has; **Configure Mapping** changes the mapping,
   which needs `client_config`. See [Source Mapping](/docs/user/screens/flows/source-mapping/).

   ![Source Mapping's File Mapping step for CI_D1_hc:Eligibility: each target data property, such as hc:Member_ID, with its source input column, first ten of 27 rows.](../../../../assets/screenshots/user/how-to/load-and-map-a-file/file-mapping.png)

7. **Run the mapping pipeline**, here `Map_Eligibility`, with the file as its main input: see
   [Run a pipeline](/docs/user/how-to/run-a-pipeline/).

**How to tell it worked**: when the run completes, the **Data Registry** tab has a new
`domain_table` row for the mapped table, `hc:Eligibility`, with the run's session id and the file's
key.

![The Data Registry tab with the hc:Eligibility domain table, session 1767604500201, ticked: the table Map_Eligibility wrote from the January file.](../../../../assets/screenshots/user/how-to/load-and-map-a-file/mapped-table-registered.png)

Loading (steps 4 and 5) is for looking at a file; the mapping pipeline reads the file from the bucket
whether it was loaded or not.
