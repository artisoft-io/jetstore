---
title: User Guide
description: Who the User Guide is for, the four kinds of page it holds, and a path through it.
kind: overview
sidebar:
  label: Overview
  order: 0
---

The User Guide is for the people who configure and run JetStore day to day. It has two readers in
mind:

- **A client's analyst**, who checks that the client's files arrived and were read correctly, starts
  pipelines, reads their results and their errors, and queries the data.
- **An ArtiSoft Knowledge Engineer**, who does all of that and also builds the workspace: the data
  model, the rules, the pipeline definitions, the reports and the user flows. The Knowledge Engineer
  also sets up each client's configuration: its vendors, its files and how their columns map.

Both are technical. The guide assumes you are at ease with tables, file formats and a little SQL. It
does not assume you can read Go or build JetStore: that is the
[Developer's Guide](/docs/developer/). How JetStore is deployed into an AWS account is the
[Installation Guide](/docs/install/).

## Four kinds of page

Each page is one kind, and says so. A page that mixes them is hard to use: the reader who wants a
list of fields should not have to wade through the reasoning, and the reader who wants the reasoning
should not have to find it between steps.

| Kind | Answers | Where |
|---|---|---|
| **Concepts** | *why is it like this?* What each idea in JetStore is, and why it is that way | [Concepts](/docs/user/concepts/) |
| **Tutorials** | *teach me*: a whole task, start to finish, to learn by doing | Tutorials |
| **How-to guides** | *help me do this*: one task, as numbered steps | How-to guides |
| **Screens** | *tell me exactly*: each screen and each user flow, its controls and columns, and the capability it needs | Screens |

The *Help* button in JetStore's top bar opens the page for the screen you are on.

## A path through the guide

1. **Read [JetStore in one page](/docs/user/concepts/jetstore/)**, then the other concepts in the
   order the [Concepts](/docs/user/concepts/) page lists them. Each builds on the one before.
2. **Do the first tutorial**, which takes a new file all the way to a pipeline's output. If you will
   write rules, do the second as well: a first rule change, compiled and tested.
3. **From then on, use the how-to guides and the screen pages as you need them.** Press *Help* on a
   screen to land on its page.

What you can do on a screen depends on your account's roles. [Users, roles and
capabilities](/docs/user/concepts/users-and-capabilities/) explains which capability opens what.

## The example used throughout

Every example in this guide uses the `jets_ws` workspace and its client **CI** (*Clinical Intel*): the
vendor `D1`, the source configurations `hc:Eligibility`, `hc:MedicalClaim` and `hc:PharmacyClaim`,
the mapping pipelines `Map_Eligibility`, `Map_MedicalClaim` and `Map_PharmacyClaim`, and the
`Patient_Profile_Template` pipeline that builds a profile of each patient from them. The data in the
screenshots is a small invented set of patients, so every screen shows the same case.
