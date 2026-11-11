---
title: Source configurations and domain keys
description: What a source configuration tells JetStore about a client's file, how its columns are mapped onto the data model, and what a domain key is for.
kind: explanation
sidebar:
  order: 3
---

A **source configuration** is JetStore's description of one stream of files: those of one object
type, from one organisation, for one client. There is exactly one for each such combination. `CI`
has three that the examples use, all from `D1`: `hc:Eligibility`, `hc:MedicalClaim` and
`hc:PharmacyClaim`.

It answers the questions JetStore cannot answer from the file itself: how to read it, where to load
it, which column identifies whom, and how its columns become the data model's properties.

## How to read the file

- **The format.** A CSV file with a header row is the common case, and the one all three of `CI`'s
  sources use. JetStore also reads headerless CSV, XLSX with or without a header row, fixed-width
  files and Parquet. A headerless or fixed-width file needs its columns described, in the source
  configuration or by a schema provider.
- **The compression**, if any.
- **One file or many parts.** A source can send each delivery as a single file, or as a folder of
  part files. A folder is complete when its sentinel arrives: an empty file that says the parts are
  all there. JetStore then treats the folder as one delivery.
- **The staging table.** Each source has a table of its own, named for the client, the organisation
  and the object type: `CI_D1_hc:Eligibility`. Loading a file copies its rows there unchanged, so you
  can look at what arrived before anything is mapped.

These are set in the *Source Configuration* flow, which needs the `client_config` capability.

## How the columns are mapped

A vendor's columns are not the data model's properties. The **source mapping** says, for each
property of the object type's class, which column fills it, and how to clean the value on the way:

| Property | Column | Function | Argument |
|---|---|---|---|
| `hc:Member_ID` | `Member_ID` | | |
| `hc:Member_DOB` | `Member_DOB` | | |
| `hc:Seconday_Diagnosis_Code1` | `Diagnosis_Code1` | `scrub_characters` | `.` |
| `hc:NDC` | `NDC` | `scrub_characters` | `-` |

These rows are from `CI`'s mappings. Most columns map straight across. The diagnosis codes have their
dots removed and the drug codes their dashes, because the vendor writes `E11.9` where the data model
and its lookups expect `E119`. A mapping can also give a default value, and an error message to
record when a value is missing.

The functions are a fixed list that JetStore provides, among them `trim`, `to_upper`,
`scrub_characters`, `find_and_replace`, `apply_regex`, `parse_amount`, `validate_date`, `to_zip5` and
`ndc10_to_11`. A mapping picks from the list rather than carrying code of its own, so a new source
needs no programming, only choices.

A mapping is applied by a mapping pipeline: `Map_Eligibility` reads an eligibility file, applies
`CI_D1_hc:Eligibility`'s mapping to each row, and writes the result to the `hc:Eligibility` table. The
mapping is edited in the *Source Mapping* flow.

## Domain keys

A **domain key** says which column, or which columns, identify the entity a row is about. In `CI`'s
three source configurations it is the same:

```json
{"hc:Eligibility": "Member_ID", "jets:hashing_override": "none"}
```

That reads: *for the `hc:Eligibility` key, use the `Member_ID` column, and do not hash it.* A member's
eligibility row, medical claims and pharmacy claims all carry the same key value, because all three
sources name the same key.

**That is what lets JetStore keep one patient's records together.** When a pipeline brings the three
sources together, it groups their rows by the key's value, and each group, one member's eligibility
and all of that member's claims, goes to the rule engine as one unit. The same value decides which
worker processes the group when a run is spread over several, so a member's records are never split.
Without a shared key, the claims of a member could not be put beside the member's eligibility at all.

A few more things a domain key can do:

- **A composite key.** A list of columns, such as `["Member_ID", "Plan_Name"]`, when no single column
  identifies the entity.
- **Several keys.** A source can name a key for more than one object type, when its rows belong to
  more than one kind of entity. The file is then registered once for each key.
- **Hashing.** An installation can be set to replace a key's value with a hash of it (MD5 or SHA-1,
  written as a UUID). `jets:hashing_override` overrides the installation's choice for one source:
  `none` keeps the raw value, which is what `jets_ws` does.

The workspace also declares the domain key of each class a pipeline writes to a table, in
`process_config/base__workspace_init_db.sql`. In `jets_ws` the three mapped classes, `hc:Eligibility`,
`hc:MedicalClaim` and `hc:PharmacyClaim`, are all keyed by `hc:Member_ID`, which is how the pipelines
that read the mapped tables bring a member's rows together again.

## Why a source configuration is data

A source configuration and its mapping are rows in the database, not files in the workspace. A new
vendor sending a file the data model already knows, eligibility for example, is then a new source
configuration and a new mapping, made in the screens, with no change to the rules and no new release
of the workspace. Only a new *kind* of data, a new object type, needs the workspace to change.

Every column of the tables behind these pages, `source_config` and `process_mapping`, is listed in the
[system tables](/docs/developer/reference/generated/system-tables/) reference.
