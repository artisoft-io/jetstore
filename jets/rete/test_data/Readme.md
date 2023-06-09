# To update workspace and lookup db of test cases

From the `rete` source directory, compile the workspace db:

```bash
docker run --rm -v=`pwd`/test_data:/go/work -w=/usr/local/lib/jets/compiler   --entrypoint=python3 jetstore:bullseye jetrule_compiler.py --base_path /go/work --in_file lookup_helper_test_workspace.jr -d --rete_db lookup_helper_test_workspace.db
```

Alternative if you have a python env, from the `test_data` directory:

```bash
python3 ../../compiler/jetrule_compiler.py --base_path . --in_file lookup_helper_test_workspace.jr -d --rete_db lookup_helper_test_workspace.db
```

Compile the lookup table:

```bash
docker run --rm -v=`pwd`/test_data:/go/work -w=/usr/local/lib/jets/compiler --entrypoint=python3 jetstore:bullseye jetrule_lookup_loader.py --base_path /go/work --rete_db lookup_helper_test_workspace.db --lookup_db lookup_helper_test_data.db
```

Alternative if you have a python env, from `test_data` directory:

```bash
python3 ../../compiler/jetrule_lookup_loader.py --base_path . --rete_db lookup_helper_test_workspace.db  --lookup_db lookup_helper_test_data.db
```
