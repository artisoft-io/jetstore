---
title: Look at loaded data
description: Browse the rows of a file loaded into its staging table, or of a table a pipeline wrote, from Home's Data Registry tab.
kind: how-to
sidebar:
  order: 2
---

**Before you start**: your account has `jetstore_read`. A file's rows are in the database only once it
has been loaded (see [Load and map a file](/docs/user/how-to/load-and-map-a-file/)); a pipeline's
table, once its run has completed.

1. On **Home**, open the **Data Registry** tab.
2. Tick the row you want. A `file` row is a file and its staging table, such as
   `CI_D1_hc:PharmacyClaim`; a `domain_table` row is a table a pipeline wrote, such as
   `hc:Eligibility`. Each row is one session of that table.

   ![The Data Registry tab with the hc:Eligibility domain table, session 1767604500201, ticked.](../../../../assets/screenshots/user/how-to/look-at-loaded-data/data-registry.png)

3. Press **View Loaded Data**. The table opens on that session's rows only, ten to a page; the
   columns sort when you press their names.

   A domain table carries the data model's properties, with the member as its domain key:

   ![The table hc:Eligibility for one session: the data model's properties, with each member's id as hc:Eligibility:domain_key, first ten of 22 rows.](../../../../assets/screenshots/user/how-to/look-at-loaded-data/domain-table.png)

   A staging table carries the vendor's own columns, as they arrived:

   ![The staging table CI_D1_hc:PharmacyClaim: the vendor's columns, such as Claim_ID and Claim_Type, first ten of 257 lines of the file.](../../../../assets/screenshots/user/how-to/look-at-loaded-data/staging-table.png)

**Other ways to the same rows**: the *File Loader Status* screen's **View Loaded Data** opens a loaded
file's staging table, and the [Query Tool](/docs/user/how-to/query-the-data/) can select any of them
by `session_id`. The [table view](/docs/user/screens/table-view/) page describes the screen.
