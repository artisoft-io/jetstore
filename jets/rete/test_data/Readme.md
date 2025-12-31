# Test Data

This directory contains test data files for the rete unit tests.

## `rete_meta_store_factory_test1.cc` and `rete_meta_store_factory_test2.cc`

- `rete_meta_store_factory_test0.cc` unit test is a basic sqlite 3 test.
- `rete_meta_store_factory_test1.cc` unit test is to load a complete
workspace db built using compiler v1.
It uses `usi_worksdpace_v1.db` and `usi_lookupdb_v1.db` test data files.
- `rete_meta_store_factory_test2.cc` unit test is to load a complete
workspace db built using compiler v2.
It uses `usi_worksdpace_v2.db` and `usi_lookupdb_v2.db` test data files.

## To update workspace and lookup db of test cases

This is based on the compiler v1 (python-based) and will need to be
updated to use compiler v2.

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
