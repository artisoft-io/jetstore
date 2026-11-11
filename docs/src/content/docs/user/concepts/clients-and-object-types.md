---
title: Clients, organisations and object types
description: Who the data belongs to, who sends it, and what kind of data it is, and why JetStore keeps the three apart.
kind: explanation
sidebar:
  order: 2
---

Every file JetStore receives answers three questions: **whose data is it**, **who sent it**, and
**what kind of data is it**. JetStore names the answers the *client*, the *organisation* and the
*object type*, and almost everything else is keyed by them.

## Clients

A **client** is the party the data belongs to, and the party the results are for. It has a short
code and a description: in `jets_ws`, the client `CI` is *Clinical Intel*.

Every piece of a client's configuration carries its code, and so does every file, run and session.
That is what lets one installation hold several clients without their data or their settings
meeting. It is also what the **client filter** in the top bar uses: choose a client there and every
screen's tables show that client's rows only.

## Organisations, or vendors

An **organisation** is whoever sends files on the client's behalf: a claims processor, a pharmacy
benefit manager, an eligibility system. The brand menu calls it a *vendor*, its table a *client
organization*, and the database and file names an *org*; they are the same thing. `CI` has one, `D1`, described as the *base vendor for clinical intel files*.

Organisations exist because two vendors never send the same kind of data in the same layout. A
client that receives medical claims from two vendors needs two descriptions of how to read them, and
the organisation is what tells them apart. An organisation belongs to one client.

Clients and their organisations are added in the *Clients & Vendors* flow.

## Object types

An **object type** is the kind of data a file holds, named the way the workspace's data model names
it. `jets_ws` defines, among others:

| Object type | Data model class | Meaning |
|---|---|---|
| `hc:Eligibility` | `hc:Eligibility` | who is covered, by which plan, from when to when |
| `hc:MedicalClaim` | `hc:MedicalClaim` | a medical claim line: diagnoses, procedures, providers, dates |
| `hc:PharmacyClaim` | `hc:PharmacyClaim` | a pharmacy claim: the drug dispensed, its quantity and days of supply |

The `hc:` in front is a namespace from the data model: the classes and properties of the healthcare
model all start with it, those of the Clinical Intel model with `cintel:`, and JetStore's own with
`jets:`.

**An object type is the bridge between a file and the rules.** Each one names the class of the data
model that a row of the file becomes once it is mapped. The rules are written against that class,
never against a file's columns, so they do not change when a new vendor's file arrives.

## Why the three are kept apart, and who sets each

| | Client | Organisation | Object type |
|---|---|---|---|
| Answers | whose data | who sent it | what kind of data |
| Example | `CI`, *Clinical Intel* | `D1` | `hc:Eligibility` |
| Defined in | the database | the database | the workspace |
| Changed through | the *Clients & Vendors* flow | the *Clients & Vendors* flow | the workspace's `process_config/` files |
| Needs | `client_config` | `client_config` | `workspace_ide` |

Clients and organisations are data: adding one is a business event, and it takes no change to the
rules. **Object types belong to the workspace**, because each needs a class in the data model and
rules that read it. A Knowledge Engineer defines them in the workspace's initialisation script,
`process_config/base__workspace_init_db.sql` in `jets_ws`, and they reach the database when that
script is loaded.

## How they come together

The three meet in a **source configuration**: one for each combination of client, organisation and
object type that sends files. `CI` has one for each of `hc:Eligibility`, `hc:MedicalClaim` and
`hc:PharmacyClaim` from `D1`. They also meet in the name of every file, which says which client,
vendor and object type it belongs to. The next two pages take these in turn:
[Source configurations and domain keys](/docs/user/concepts/source-configurations/), and
[Files and the input registry](/docs/user/concepts/files-and-the-input-registry/).
