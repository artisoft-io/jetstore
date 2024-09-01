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

Then compile the workspace with the following command:

```bash
WORKSPACES_HOME=/home/michel/projects/repos/jetstore/jets/jetrules \
WORKSPACE=test_ws \
WORKSPACE_DB_PATH=$WORKSPACES_HOME/$WORKSPACE/workspace.db \
WORKSPACE_LOOKUPS_DB_PATH=$WORKSPACES_HOME/$WORKSPACE/lookup.db \
JETRULE_COMPILER=/home/michel/projects/repos/jetstore/jets/compiler/jetrule_compiler.py \
JETRULE_LOOKUP_LOADER=/home/michel/projects/repos/jetstore/jets/compiler/jetrule_lookup_loader.py \
./compile_workspace.sh
```
