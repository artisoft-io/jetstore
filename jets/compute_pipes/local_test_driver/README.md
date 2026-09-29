# local_test_driver

Runs a pipeline end to end from a workstation: the real sharding and reducing starters against the
database (through the ssh tunnel), then every node of every step, in order, in this process. The
environment it needs is listed at the head of `main.go`.

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
`stackTrace` in the body and no error header — measured 2026-09-28. `invokePythonNode` reads the body
for that reason and fails the run with the node's own message and stack, as a Go node's error would.
