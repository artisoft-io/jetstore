# Logger Replacement — Mitigating CWE-117 (Improper Output Neutralization for Logs)

## The Vulnerability

CWE-117 (*Improper Output Neutralization for Logs*), also known as **log injection** or
**log forging**, occurs when untrusted data is written to a log without neutralizing
control characters. The Go standard library `log` package writes messages verbatim as
plain text, one line per entry. An attacker who controls any value that reaches a log
statement — a file name, a user-supplied form field, an HTTP header, a column name in an
input file — can embed newline characters and fabricate entirely synthetic log records:

```go
// user supplies: "alice\n2026-07-31 12:00:00 user admin granted role SUPERUSER"
log.Printf("login attempt for user %s", userName)
```

The resulting log file contains two lines that are indistinguishable from genuine entries.
This defeats forensic analysis, can be used to hide malicious activity, and — when logs
are ingested by a downstream viewer — can carry escape sequences or markup into that tool.

## The Approach: Structured JSON Logging via `go.uber.org/zap`

JetStore addresses CWE-117 by **replacing plain-text logging with structured JSON
logging** using [`go.uber.org/zap`](https://github.com/uber-go/zap) (currently `v1.28.0`,
declared in `go.mod`).

The core property being relied upon is that **a JSON encoder must escape its string
values**. When zap's JSON encoder writes a log message, newlines become `\n`, carriage
returns become `\r`, tabs become `\t`, quotes become `\"`, and other control characters
become `\uXXXX` escapes — all *inside* a single JSON string value. The neutralization is
performed by the encoder itself, unconditionally, for every value it writes. There is no
call site that can forget to sanitize, and no format string that can bypass it.

The forged entry above becomes a single, harmless record:

```json
{"level":"info","ts":"2026-07-31T12:00:00.000Z","msg":"login attempt for user alice\n2026-07-31 12:00:00 user admin granted role SUPERUSER"}
```

One JSON object on one line — one log record. The injected content is visibly *data*
inside the `msg` field, not a record of its own.

## Implementation

### Central setup function

The configuration lives in a single place, `jets/utils/use_zap_logger.go`:

```go
// UseJetStoreLogger sets up a zap logger and redirects the standard library log output to it.
func UseJetStoreLogger() {
	// ...
	EncoderCfg := zap.NewProductionEncoderConfig()
	EncoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(zap.InfoLevel),
		Development:      false,
		DisableCaller:    true,
		Encoding:         "json",
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
		EncoderConfig:    EncoderCfg,
	}
	logger := zap.Must(cfg.Build())
	defer logger.Sync()

	// Replace the system logger with our new logger.
	zap.RedirectStdLog(logger)
}
```

The essential settings are `Encoding: "json"` (the escaping encoder) and `OutputPaths:
["stdout"]`, which suits the containerized deployment model where ECS/Lambda forward
stdout to CloudWatch Logs.

### Redirecting existing `log` calls — `zap.RedirectStdLog`

The key design decision is the call to `zap.RedirectStdLog(logger)`. Rather than rewriting
the thousands of existing `log.Printf` / `log.Println` call sites across the codebase,
this installs zap as the output destination of the standard library's global logger. Every
pre-existing `log.*` call in the Go code is transparently routed through zap's JSON
encoder and is therefore neutralized.

This yields the mitigation with:

- **No call-site changes.** Existing code keeps using `log.Printf`; there is no risk of a
  missed or partially migrated file.
- **No regression path.** New code written with the familiar `log` package is protected by
  default. A developer cannot reintroduce the flaw by forgetting to use the "safe" logger.
- **A single point of policy.** Log level, format, and destination are changed in one file.

### Invocation

`utils.UseJetStoreLogger()` is called as one of the first statements in the `main()`
function of every Go binary, so that the redirection is in place before any user-supplied
data can be logged. This covers the service and CLI binaries under `jets/` — `apiserver`,
`cbooter`, `cpipes_server`, `cpipes_native_server`, `compile_workspace`, `compilerv2`,
`update_db`, `run_reports`, `purge_database` — as well as the AWS Lambda handlers and CDK
programs under `cdk/` — `cp_node`, `cp_node_native`, `cp_sharding_starter`,
`cp_reducing_starter`, `api_gateway`, `register_keys_v2`, `sqs_register_keys`,
`status_update`, `run_reports`, `purge_data`, `rotate_secret`, `bootstrap_aws`, and
`vpc_peering`.

For example, `jets/apiserver/main.go`:

```go
func main() {
	utils.UseJetStoreLogger()
	// ...
}
```

**Any new `main()` added to the repository must include this call.**

### Local development escape hatch

The function returns early — leaving the standard text logger in place — when the
`JETSTORE_DEV_MODE` environment variable is set:

```go
_, globalDevMode := os.LookupEnv("JETSTORE_DEV_MODE")
if globalDevMode {
	log.Print("JETSTORE_DEV_MODE is set, using standard library logger instead of zap logger")
	return
}
```

This keeps console output human-readable while developing locally. `JETSTORE_DEV_MODE` is
a developer-workstation setting only and is **not** set in any deployed environment, so
production, staging, and all CI/CD-deployed containers always run with JSON encoding
active.

## Structured Audit Logging

Beyond the global redirection, the API server maintains a dedicated audit logger
(`server.AuditLogger`, configured in `jets/apiserver/server.go`) that is also a zap JSON
logger, tagged with the initial field `{"logger_type": "audit_log"}` so audit records can
be filtered out of the general log stream.

Security-relevant events are recorded using zap's **typed structured fields** rather than
string formatting:

```go
server.AuditLogger.Info("user login",
	zap.String("user", jetsUser.Email),
	zap.String("time", time.Now().Format(time.RFC3339)))
```

This is the strongest form of the mitigation. The untrusted value (`jetsUser.Email`) never
touches the message text; it is carried as a distinct, individually escaped JSON field.
The message key stays a fixed constant, which makes the records reliably parseable and
queryable in CloudWatch Logs Insights and leaves no ambiguity about which part of the
record came from the user.

## Complementary Practices

Structured logging neutralizes the *encoding* of untrusted data, but it does not decide
*what* is appropriate to log. It is paired with the practice of not writing sensitive
values to logs at all — secret names and values are masked or omitted at the call site
(see, for example, the recent commits `Masking secret name in log` and `remove sensitive
log stmt`). CWE-117 mitigation and sensitive-data-exposure mitigation are separate
concerns and are both required.

## Summary

| Aspect | Approach |
|---|---|
| **Weakness** | CWE-117 — log injection via unescaped newlines/control characters in untrusted data |
| **Mitigation** | Structured JSON logging with `go.uber.org/zap`; the JSON encoder escapes all control characters |
| **Mechanism** | `zap.RedirectStdLog()` reroutes every standard-library `log.*` call through zap |
| **Central location** | `jets/utils/use_zap_logger.go` — `UseJetStoreLogger()` |
| **Coverage** | Called at the top of `main()` in every Go binary (services, CLIs, Lambda handlers, CDK apps) |
| **Exception** | `JETSTORE_DEV_MODE` env var restores plain-text logging for local development only |
| **Reinforcement** | Audit events use typed `zap.String(...)` fields, keeping untrusted values out of the message text |
