# local_test_driver

Runs a pipeline end to end from a workstation: the real sharding and reducing starters against the
database (through the ssh tunnel), then every node of every step, in order. The environment it needs
is listed at the head of `main.go`.

**Where a node runs** is decided per step, the way the state machine decides it:

| Step | Runs on | Flag |
|---|---|---|
| `use_python_node` | a Python node served by the Lambda emulator — required | `-python_node_url` |
| any other | **this process** (the default, and the debugging loop) | — |
| any other, with the flag | the native Go node **image** served by the Lambda emulator | `-go_node_url` |

## Steps that run on the Python node

A step with `use_python_node` cannot run in this process: the Python `cp_node` is a separate
program, and the operators such a step names — a site's own, like `healthcare_corpus` — exist only
there. The reducing starter marks such a step with `UsePythonReducingTask`, the same flag the state
machine switches on (`build_cpipes_sm.go`), and for it the driver sends each node's `{id, jp, pe}`
over HTTP to a Python node given by **`-python_node_url`**. Without the flag the driver refuses the
step rather than running it on the Go node.

**The Python node is the site image itself**, run locally. Every AWS Lambda base image carries the
Runtime Interface Emulator, which serves the image's handler over HTTP when the image is started
outside Lambda. So this runs the artefact a deployment ships — the site's handler and operator
registry, the baked reference data — rather than a local approximation of it.

### Start it

Build the images the way `jets_ws_1.sh` does (no push is needed): `build_cpipes.sh`,
`build_corpus.sh`, then the workspace image, which is tagged locally as
`cpipes_python_lambda_<workspace>:latest`. Then, with the same variables exported that the driver
needs, and the ssh tunnel open:

```bash
eval "$(aws configure export-credentials --format env)"   # or export the three AWS_* yourself
docker run --rm --name cpipes-python-node --network host \
  -e _HANDLER=handler.lambda_handler \
  -e USING_SSH_TUNNEL=1 \
  -e JETS_BUCKET -e JETS_REGION -e JETS_DSN_SECRET -e JETS_S3_KMS_KEY_ARN \
  -e JETS_s3_INPUT_PREFIX -e JETS_s3_OUTPUT_PREFIX -e JETS_s3_STAGE_PREFIX -e JETS_s3_SCHEMA_TRIGGERS \
  -e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_SESSION_TOKEN -e AWS_REGION \
  --entrypoint /usr/local/bin/aws-lambda-rie \
  cpipes_python_lambda_jets_ws:latest \
  --runtime-interface-emulator-address 0.0.0.0:9123 /var/runtime/bootstrap handler.lambda_handler
```

and run the driver with

```bash
-python_node_url http://localhost:9123/2015-03-31/functions/function/invocations
```

Why each non-obvious part is there:

- **`--network host`** — the handler, given `USING_SSH_TUNNEL`, connects to the database on
  `localhost`, which has to be *your* localhost, where the tunnel is.
- **The emulator's address, `9123`** — with host networking the emulator's default `8080` is the
  host's `8080`, which a local apiserver uses. `--runtime-interface-emulator-address` moves it; that
  is why the entrypoint is spelled out rather than taken from the image's `/lambda-entrypoint.sh`,
  which starts the emulator with its defaults.
- **The four `JETS_s3_*` prefixes** — the node builds every stage and output key from them. They
  are what the deployed Lambda was missing on 2026-09-27; here the values are yours.

Nodes run one at a time, as every other node in this driver does. The node reads and writes the real
database and the real bucket, exactly as the Go nodes in this driver do.

### Faster iteration on the node's own code

A change to `tools/cpipes_node` otherwise means rebuilding two images. Mounting the working tree over
the installed package skips that — add, before the image name:

```bash
  -v "$PWD/tools/cpipes_node/cpipes_node:/var/lang/lib/python3.12/site-packages/cpipes_node:ro" \
```

and restart the container after an edit. The same works for the site's own package.

### What this does not test

**The deployed function's environment.** The container gets the variables you pass, so a variable the
CDK map forgets is invisible here; `tools/cpipes_node/tests_deployment.py` is what checks that map
against what the node reads. And the invocation is synchronous with no retry, where the state machine
has a `Map` with its own concurrency.

### How a node failure is reported

**The emulator returns HTTP 200 when the handler raises**, with `errorType`, `errorMessage` and
`stackTrace` in the body and no error header — measured 2026-09-28. `invokeLambdaNode` reads the body
for that reason and fails the run with the node's own message and stack, as a Go node's error would.

## Go nodes on the native node image

**In-process is the default and should stay the one you develop against**: breakpoints, a rebuild in
seconds, one process. `-go_node_url` exists for the check before a deploy — that the artefact that
ships behaves as the working tree does. The in-process run cannot see three ways the two differ:

- **`libjets.so`.** The driver links whichever one is in `/usr/local/lib`, from whichever of the two
  CMake trees last installed it; the image carries its own builder stage's, `ldd`-checked at build.
  This is the one that matters most, and the reason the flag targets the **native** image.
- **The environment.** The node Lambda carries the variables in its CDK block; the driver has what
  `run_env.sh` exported.
- **The image's contents** — its `bootstrap` and the workspace it carries.

Only the native node is an image; the Go-rules node Lambda is a zip bundle, so it has no container to
run and is what the in-process default already exercises.

### Start it

Build it the way `jets_ws_1.sh` does — locally it is tagged `cpipes_lambda_<workspace>:latest`. Then,
with the driver's environment exported and the tunnel open:

```bash
eval "$(aws configure export-credentials --format env)"
NODE_ENV="JETS_BUCKET JETS_DSN_SECRET JETS_INVALID_CODE CPIPES_DB_POOL_SIZE JETS_REGION
  JETS_PIVOT_YEAR_TIME_PARSING JETS_s3_INPUT_PREFIX JETS_s3_OUTPUT_PREFIX JETS_s3_STAGE_PREFIX
  JETS_s3_SCHEMA_TRIGGERS JETS_S3_KMS_KEY_ARN JETS_SENTINEL_FILE_NAME TASK_MAX_CONCURRENCY ENVIRONMENT
  JETS_DOMAIN_KEY_SEPARATOR JETS_DOMAIN_KEY_HASH_ALGO JETS_DOMAIN_KEY_HASH_SEED
  JETS_INPUT_ROW_JETS_KEY_ALGO WORKSPACE AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_SESSION_TOKEN
  AWS_REGION"
docker run --rm --name cpipes-go-node --network host \
  $(for v in $NODE_ENV; do printf -- '-e %s ' "$v"; done) \
  -e USING_SSH_TUNNEL=1 -e DEPLOY_CPIPES_NATIVE=1 \
  -e WORKSPACES_HOME=/tmp/workspaces -e LD_LIBRARY_PATH=/usr/local/lib \
  --entrypoint /usr/local/bin/aws-lambda-rie \
  cpipes_lambda_jets_ws:latest \
  --runtime-interface-emulator-address 0.0.0.0:9124 /var/runtime/bootstrap
```

and add to the driver

```bash
-go_node_url http://localhost:9124/2015-03-31/functions/function/invocations
```

`NODE_ENV` is the node Lambda's CDK block (`CpipesNativeNodeLambda`, `build_cpipes_lambdas.go`) less
the notification endpoints, which a local run should not fire, and less the four values set literally
on the command line — those are the CDK's own literals. `-e NAME` with no value forwards what the
driver's shell has, so there is no second copy of the environment to keep in step. Port `9124` so it
can run beside the Python node on `9123`.

**`USING_SSH_TUNNEL` is honoured by the node image only from 2026-09-28.** Before that,
`lambdas/dbc`'s `openDbConnection` passed `false` for it, so the node dialled the RDS host named in the
secret — unreachable from a workstation. An image built before then fails at start-up however it is
run.

### How its failure looks

**Not like the Python node's.** The Go node connects to the database in `main()`, before
`lambda.Start`, and panics when it cannot; the runtime exits and the emulator answers **502 with an
empty body**. The driver reports that as "the runtime exited before the handler returned", and the
panic itself is in `docker logs cpipes-go-node`. An error *inside* the handler comes back the Python
way: HTTP 200 with `errorType` and `errorMessage`. Both measured 2026-09-28 against
`cpipes_lambda_jets_ws`.
