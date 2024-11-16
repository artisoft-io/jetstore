# Test Workspace

## Compiling Workspace

To compile the JetStore Workspace into the workspace.db and lookup.db, start by creating a python env
in the Workspace folder (where `compile_workspace.sh` is located):

```bash
python3 -m venv env
source env/bin/activate
pip3 install --no-input \
    absl-py \
    aiocsv \
    aiofiles \
    antlr4-python3-runtime \
    apsw \
    argo-workflows \
    bcrypt \
    boto3 \
    certifi \
    frozendict \
    openpyxl \
    pandas \
    prometheus_client \
    protobuf \
    psycopg[binary,pool] \
    PyYAML \
    yamlconf \
    requests \
    && pip3 list
```

Then compile the workspace with the following command
in the Workspace folder (where `compile_workspace.sh` is located):

```bash
WORKSPACES_HOME=/home/michel/projects/repos/jetstore/jets/jetrules \
WORKSPACE=test_ws \
WORKSPACE_DB_PATH=$WORKSPACES_HOME/$WORKSPACE/workspace.db \
WORKSPACE_LOOKUPS_DB_PATH=$WORKSPACES_HOME/$WORKSPACE/lookup.db \
JETRULE_COMPILER=/home/michel/projects/repos/jetstore/jets/compiler/jetrule_compiler.py \
JETRULE_LOOKUP_LOADER=/home/michel/projects/repos/jetstore/jets/compiler/jetrule_lookup_loader.py \
./compile_workspace.sh
```

## Running apiserver with test_ws

```bash
JETS_DSN_SECRET=rdsSecret9F108BD1-nckPIWNHV2Pz AWS_API_SECRET=apiSecret6F8DC95D-Jv03ulNqRYU3 JETS_ENCRYPTION_KEY=1rKVXjy6sJkZpcw0QQwBle8mhtLkFDAr JETS_ADMIN_EMAIL=admin AWS_JETS_ADMIN_PWD_SECRET=adminPwdSecret76988700-IAe7o9XxEr58 JETS_REGION='us-east-1' JETS_BUCKET=bucket.jetstore.io WORKSPACES_HOME=/home/michel/projects/repos/jetstore/jets/jetrules JETS_LOADER_SM_ARN='arn:aws:states:us-east-1:470601442608:stateMachine:loaderSM' JETS_SERVER_SM_ARN='arn:aws:states:us-east-1:470601442608:stateMachine:serverSM' JETRULE_COMPILER=/home/michel/projects/repos/jetstore/jets/compiler/jetrule_compiler.py JETRULE_LOOKUP_LOADER=/home/michel/projects/repos/jetstore/jets/compiler/jetrule_lookup_loader.py JETS_VERSION=1672973310850 JETSTORE_DEV_MODE=1 JETS_s3_INPUT_PREFIX='jetstore/input' JETS_s3_OUTPUT_PREFIX='jetstore/output' JETS_s3_STAGE_PREFIX='jetstore/stage' JETS_s3_SCHEMA_TRIGGERS='jetstore/schema_triggers' JETS_DOMAIN_KEY_HASH_SEED='6ba7b810-9dad-11d1-80b4-00c04fd430c8' JETS_INPUT_ROW_JETS_KEY_ALGO='row_hash' JETS_INVALID_CODE='NOT VALID' JETS_SCHEMA_FILE=~/projects/repos/jetstore/jets/serverv2/jets_schema.json JETS_INIT_DB_SCRIPT=~/projects/repos/jetstore/jets/serverv2/jets_init_db.sql NBR_SHARDS=1 JETS_DOMAIN_KEY_HASH_ALGO=none JETS_LOG_DEBUG=2 JETS_DOMAIN_KEY_SEPARATOR=':' WORKSPACE=test_ws WORKSPACE_BRANCH=main WORKSPACE_FILE_KEY_LABEL_RE='processing_ticket=(.*?)\/' ACTIVE_WORKSPACE_URI='https://github.com/artisoft-io/cedargate_ws' WORKSPACE_URI='https://github.com/artisoft-io/cedargate_ws' WORKSPACE_DB_PATH=$WORKSPACES_HOME/$WORKSPACE/workspace.db WORKSPACE_LOOKUPS_DB_PATH=$WORKSPACES_HOME/$WORKSPACE/lookup.db ./apiserver -tokenExpiration 60 -usingSshTunnel   -WEB_APP_DEPLOYMENT_DIR /home/michel/projects/repos/jetstore/jetsclient/build/web
```

Making the new `workspace.tgz`:

```bash
 tar cfvz workspace.tgz --exclude '*.jr' workspace_control.json jet_rules/
```
