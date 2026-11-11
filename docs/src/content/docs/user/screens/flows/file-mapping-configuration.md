---
title: File Mapping Configuration
description: The mapping worksheet of a source, one row per property of its object type with the column, cleansing function, argument, default and error message that fill it, its checks and its three buttons.
kind: reference
---

*File Mapping Configuration* is the **mapping worksheet** of one source: for each property of the
source's object type, the column of the file that fills it and how to clean the value on the way.
What a mapping is, and the functions it can use, is in
[How the columns are mapped](/docs/user/concepts/source-configurations/#how-the-columns-are-mapped).
The worksheet has one step.

**Capability:** `client_config`, to save; `jetstore_read` to open it.

**Opened from** the *Source Mapping* flow (*JetStore Workspace* menu, *Source Mapping*): select a
data source, press *Next*, then **Configure Mapping** above its mapping. It returns to *Source
Mapping* when it ends. Its key is `mapFileUF`, and its address names the staging table and the object
type it maps.

![The File Mapping Worksheet of CI_D1_hc:Eligibility: a row for each property of hc:Eligibility, hc:Effective_Date, hc:Group_Number, hc:Medicare_Indicator and the member's address, city and date of birth, each with its input column, cleansing function, argument, default value and error message; the required properties are marked with an asterisk.](../../../../../assets/screenshots/user/screens/flows/file-mapping-configuration/worksheet.png)

## The rows

One row for each property of the object type's class, in alphabetical order, headed by the
property's name. **A name ending in `*` is required**: every record must have a value for it.

| Field | What it does |
|---|---|
| **Input Column** | The column of the file that fills the property. It suggests the columns of the source's staging table as you type, the columns the property's name matches first. |
| **Cleansing Function** | A function applied to the value, from JetStore's list: `trim`, `scrub_characters`, `validate_date`… Leave it on *Select cleansing function* for none. |
| **Function Argument** | The function's argument, such as the characters `scrub_characters` removes. Required by some functions, and must be empty when no function is chosen. |
| **Default Value** | The value to use when the column is empty or the function returns nothing. |
| **Error Message** | The error to record when the column is empty or the function returns nothing and there is no default. The record is then reported on [Process Errors](/docs/user/screens/process-errors/). |

**The suggestions come from the staging table**, which exists once a file of the source has been
loaded with the [Load Files](/docs/user/screens/flows/load-files/) flow. Before then, *Input Column*
has nothing to suggest, and any name you type is refused.

The worksheet opens with the mapping already saved. A property with no saved mapping starts with the
column its data model names as its default, if any, or else a column with the property's own name,
when the file has one.

## The checks

Each row is checked when the worksheet opens and again as you edit it, and the error shows under the
field:

- *Input Column* must be a column of the staging table;
- a required property needs an *Input Column*, or else a *Default Value* or an *Error Message*;
- *Function Argument* is required by some functions and refused without a function;
- a row cannot have both a *Default Value* and an *Error Message*.

## Buttons

At the bottom of the worksheet:

| Button | What it does | Capability |
|---|---|---|
| **Save** | Checks every row, then replaces the source's whole mapping with the worksheet and returns to *Source Mapping*. Disabled while a row fails its check. | `client_config` |
| **Save as Draft** | Saves the worksheet as it is, without the checks, to finish later. Enabled only while a row fails its check. | `client_config` |
| **Cancel** | Leaves without saving. | |

The mapping applies to the next run of the source's mapping pipeline, such as `Map_Eligibility`; it
does not change rows already mapped.

## See also

- [Load and map a file](/docs/user/how-to/load-and-map-a-file/)
