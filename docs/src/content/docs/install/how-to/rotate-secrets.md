---
title: Rotate secrets
description: The secrets a JetStore stack holds, how their rotation is scheduled, how to rotate one now, and how a rotation reaches the running apiserver and Lambdas.
kind: how-to
sidebar:
  order: 6
---

This guide is written from the code it describes, not from a run in an account: the secrets and their
schedules in `cdk/jetstore_one` (`BuildSecrets` and `AddSecretRotationSchedules` in
`stack/build_secrets.go`, the database secret in `NewJetstoreOneStack`, `jetstore_one.go`), the
rotation Lambda (`cdk/jetstore_one/lambdas/rotate_secret/main.go`), and what reads a rotation:
`SecretsRotated` (`jets/apiserver/secrets_rotated.go`) and its callers in `jets/apiserver/server.go`
and `jets/apiserver/api_users.go`, and the Lambdas' connection helper
(`cdk/jetstore_one/lambdas/dbc/db_connection.go`).

## The secrets

The stack creates four secrets in Secrets Manager, with generated names. The apiserver and the Lambdas are
given a secret's **name** in their environment and read the value themselves; the scheduled detection
task is given the database secret's value, as `JETS_DSN_JSON_VALUE`, when it starts
(`stack/build_agentic_detect.go`).

| Secret (construct id) | Holds | Environment variable | Rotation's effect |
|---|---|---|---|
| `rdsSecret` | the database's `postgres` user, as JSON: host, port, user name, password | `JETS_DSN_SECRET` | `ALTER USER ... PASSWORD` in the database, then a test connection with the new value; reverted if the test fails |
| `apiSecret` | the key that signs and checks sign-in tokens (15 characters) | `AWS_API_SECRET` | none in the database |
| `adminPwdSecret` | the password of the administrator, `JETS_ADMIN_EMAIL` (15 characters) | `AWS_JETS_ADMIN_PWD_SECRET` | the administrator's password hash in `jetsapi.users` is replaced |
| `encryptionKeySecret` | the key that encrypts users' git tokens (32 characters, no punctuation) | `JETS_ENCRYPTION_KEY_SECRET` | every git token in `jetsapi.users` is decrypted with the old key and encrypted with the new one; one that cannot be decrypted is skipped and logged |

To find their names:

```bash
aws secretsmanager list-secrets --query 'SecretList[].[Name,Description,RotationEnabled]' --output table
```

Their descriptions are *API secret used for jwt token encryption*, *JetStore UI admin password* and
*JetStore Encryption Key*; the database secret is the one CDK generates for the cluster. After a
rotation, read the administrator's new password from `adminPwdSecret`.

**The restricted database logins are not among them.** Their passwords are generated once per
database and kept in `jetsapi.restricted_login` (`jets/dblogin/dblogin.go`); no rotation changes
them.

## How rotation is scheduled

`AddSecretRotationSchedules` puts all four secrets on one schedule, run by one Lambda,
`SecretRotationLambda`:

- **Every `JETS_SECRETS_ROTATION_DAYS` days**, default 30. A value that is not a number falls back to
  30, and synth prints the schedule it used (*Using secret rotation schedule of ... days*). Changing it
  takes a deploy.
- **Not on deploy**: `RotateImmediatelyOnUpdate` is false, so creating or updating a schedule does not
  rotate.

The Lambda runs Secrets Manager's four steps. `createSecret` stores a new random value as
`AWSPENDING`; `setSecret` applies it in the database, as the table above says; `testSecret` connects
with a pending database secret and runs `SELECT NOW()`; `finishSecret` makes the new value
`AWSCURRENT` and records the rotation in `jetsapi.secret_rotation` (`RecordSecretRotation`). That
last row is how the running processes find out.

## Rotate a secret now

```bash
aws secretsmanager rotate-secret --secret-id <secret name>
```

Then watch the Lambda's log group, `SecretRotationLambdaLogGroup` in the stack: each step logs
*RotateSecret Lambda Called for secret ... at step ...*, and an error names its step. Rotate one
secret at a time, and wait for `finishSecret` before the next.

## How a rotation reaches the running processes

**The apiserver** keeps the database connection, the token-signing key and the encryption key in
memory. `SecretsRotated` reloads them: it reads the current API secret and encryption key, and closes
and reopens the database pool from the database secret, then reopens the restricted logins' pools,
which copy its connection settings. It runs:

- **within the hour**, from the background task `listenAndServe` starts, which reads
  `MAX(last_update)` from `jetsapi.secret_rotation` every hour and calls `SecretsRotated` when it is
  later than the last one seen. The task does not run in dev mode.
- **at once, when a sign-in meets a rotated database password**: `Login` (`api_users.go`) calls
  `SecretsRotated` when reading the user fails with *password authentication failed*, and tries again.

So, after a rotation:

- **Database secret**: connections already open stay open; a request that needs a new one may fail
  until the next hourly check or the next sign-in.
- **API secret**: tokens signed with the old key stop validating when the apiserver reloads, so
  signed-in users must sign in again.
- **Encryption key**: a git token that fails to decrypt makes the apiserver re-read the key and try
  again (`DecryptGitToken`, `jets/user/git_profile.go`), so git operations recover without waiting.
- **Administrator password**: nothing to reload; the new password is in the database.

**The Lambdas** that open the database through `NewDbConnection` (`lambdas/dbc/db_connection.go`:
status update, run reports, register key, the cpipes starters and nodes, the API gateway Lambda)
check `jetsapi.secret_rotation` at most every five minutes, and reopen their connection when it shows
a later rotation or the check fails with *password authentication failed*.

**Tasks that start after a rotation** (a compute pipes or run-reports task, the scheduled detection
task) read the current value when they start.

## Check

- `SELECT secret, last_update FROM jetsapi.secret_rotation` names the rotated secret with the time of
  its `finishSecret`. The `secret` column is closed to the [Query Tool](/docs/user/screens/query-tool/);
  read `last_update` there, or the whole row as the database's own user.
- The UI container's log (`UiContainerLogGroup`) shows
  *Secrets have rotated, refreshed cached values in ui container* after the reload, and
  *Warning: while SecretsRotated: ...* if it failed.
- Sign in, then run the checks in [Verify a deployment](/docs/install/how-to/verify-a-deployment/).
