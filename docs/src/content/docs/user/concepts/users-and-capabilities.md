---
title: Users, roles and capabilities
description: How a JetStore account is created and given its rights, the roles JetStore ships, and what each capability opens in the screens.
kind: explanation
sidebar:
  order: 8
---

What you can see and do in JetStore depends on your account's **capabilities**. You do not hold
capabilities directly: an account holds **roles**, and each role is a set of capabilities.

## Accounts

**An account is created by its owner**, with *Register* on the sign-in page: a name, an email and a
password. A new account is **inactive**, and cannot sign in until someone with the `user_admin`
capability activates it and gives it roles, on the *User Administration* screen. That screen can also
deactivate an account, change its roles, or delete it.

Each installation also has its **administrator account**, named when the installation is deployed.
It holds every capability, and it cannot be deactivated or deleted from *User Administration*, so the
installation always keeps one account that can manage users.

## Roles

A role is only a name for a set of capabilities. JetStore checks capabilities, never roles, so what a
role allows is entirely the list below. JetStore ships these roles:

| Role | For | Capabilities |
|---|---|---|
| `ops_user` | someone who loads files and runs pipelines | `jetstore_read`, `run_pipelines`, `user_profile` |
| `client_advocate` | someone who also maintains clients' configuration | those of `ops_user`, and `client_config` |
| `knowledge_engineer` | the Knowledge Engineer, who also works on the workspace | those of `client_advocate`, and `workspace_ide`, `infer_server_admin`, `agent_supervision`, `agent_phi_access` and `runbook_execute` |
| `designated_admin` | a Knowledge Engineer who may also delete sessions and manage users | those of `knowledge_engineer`, and `delete_session` and `user_admin` |

There is no hierarchy between roles: `designated_admin` lists every capability of
`knowledge_engineer` rather than inheriting them. A fifth role, `system_role`, belongs to the account
JetStore uses to start pipelines automatically, and is not for people.

An account can hold several roles, and has every capability any of them grants.

## What each capability opens

| Capability | Allows | In the screens |
|---|---|---|
| `jetstore_read` | reading JetStore's data | every table on every screen: Home's tabs, the registries, a table's rows, a run's details and errors. Without it, the server refuses the reads behind them |
| `run_pipelines` | loading files and running pipelines | *Start Pipeline*, *Put Schema Event* and *Check Pending* on Home's *Pipelines Status* tab; the *Start Pipeline* and *Load Files* flows; *Drop Staging Table* |
| `client_config` | changing a client's configuration | saving in the *Clients & Vendors*, *Source Configuration*, *Source Mapping* and *Pipeline Configuration* flows, and on the *Rules Configuration* screen |
| `user_profile` | changing your own git profile | the *Git Profile* screen |
| `workspace_ide` | working on the workspace, and querying the database freely | the *Workspaces* and *Workspace IDE* screens: editing, compiling, committing, pulling, loading and exporting client configuration, rules unit tests; and the *Query Tool* |
| `infer_server_admin` | starting and stopping the infer server, and managing its models | the *Infer Server Admin* screen. Starting the server starts paid GPU capacity |
| `agent_supervision` | supervising JetStore's AI agents | the *Proposals*, *Incidents* and *Detection* screens: reviewing, approving or rejecting the changes an agent proposes, and reading what it did |
| `agent_phi_access` | seeing the fields of an agent's records marked as protected health information | on the agent supervision screens, those fields are shown; without it they are withheld, and the screen says so |
| `runbook_execute` | starting and controlling the active workspace's runbooks | the *Runbooks* screen. Each step of a runbook also needs its own capability: a query needs `jetstore_read`, a pipeline start `run_pipelines` |
| `delete_session` | deleting sessions completely | *Delete* on Home's *Session Registry* tab |
| `user_admin` | managing accounts | the *User Administration* screen |

## How the screens show it

**A screen or button you lack the capability for is shown disabled**, not hidden: the *Query Tool*
entry in the top bar, say, or *Delete* on the *Session Registry* tab. You can see that it exists and
ask for it.

**The disabled button is a courtesy; the server is the guard.** Every action is checked again by the
server when it arrives, whatever the screen showed, so a capability cannot be gained by reaching a
screen some other way.

## Why capabilities rather than roles

Capabilities are cut along responsibilities rather than along screens. Editing a rule file (`workspace_ide`) and approving an AI agent's change (`agent_supervision`) are
different decisions, so they are different capabilities, even though a Knowledge Engineer usually
holds both. Seeing protected health information inside an agent's record (`agent_phi_access`) is
separate again, because it is client data inside a governance record. Deleting a session
(`delete_session`), which cannot be undone, is given to one role only.

Because the screens and the server check only capabilities, a role is just a bundle of them: what an
account may do is always read from the capabilities its roles grant.

## See also

- [Sign in and register](/docs/user/screens/sign-in/)
- [User Administration](/docs/user/screens/user-administration/)
- [Git Profile](/docs/user/screens/git-profile/)
- [Manage users](/docs/user/how-to/manage-users/)
