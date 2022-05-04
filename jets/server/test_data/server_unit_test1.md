# Server Unit Test1

Simple unit test for server process consisting of following files:

- workspace_test1.jr: workspace rule file
- code_description_test1.csv: lookup table data
- process_config_test1.sql: process config data

## Generate the workspace db using jetstore compiler

Command to execute the compiler from the directory that is the workspace
base directory (here is `server/test_data` source directory).
Run the compiler on the `workspace_test1.jr` rule file:
```
docker run --rm -v=`pwd`:/go/work -w=/usr/local/lib/jets/compiler \
  --entrypoint=python3 jetstore:bullseye jetrule_compiler.py      \
  --base_path /go/work --in_file workspace_test1.jr -d --rete_db workspace_test1.db
```

## Generate the lookup data db using lookup loader

Simularily, from the `test_data` directory where the workspace files are located:
```
docker run --rm -v=`pwd`:/go/work -w=/usr/local/lib/jets/compiler \
  --entrypoint=python3 jetstore:bullseye jetrule_lookup_loader.py      \
  --base_path /go/work  --lookup_db lookup_test1.db --rete_db workspace_test1.db
```

## Load the input csv file using jetstore loader

Load of csv file using jetstore loader into the platform postgres database.
From the `server/test_data` directory:
```
docker run --rm -v=`pwd`:/go/work -w=/go/work \
  --entrypoint=loader jetstore:bullseye    \
  -dsn="postgresql://postgres:ArtiSoft001@172.17.0.2:5432/postgres" \
  -table=test1 -in_file=input_data_test1.csv -sep '|' -d 
```

In postgres, you can query the created table to see it's schema:
```
select table_name,column_name,data_type from information_schema.columns where table_name = 'test1';
```

## Load the process config data structure into jetstore postgres database

Load the process config located in `test_data/process_config_test1.sql` into jetstore processing database. 
First copy the script into the mounted folder, from the server source folder:
```
cp -v test_data/process_config_test1.sql ~/projects/work
```
Connect into the running postgres container:
```
docker exec -it postgres /bin/bash
```
Execute the load script:
```
psql -U postgres -a -f process_config_test1.sql
```

## Update database

```
../update_db/update_db -dsn="postgresql://postgres:ArtiSoft001@172.17.0.2:5432/postgres" -drop -workspaceDb test_data/workspace_test1.db 
```

## Running the server process

Now that we have all of the parts in place, we can run the server process.
Running directly from the source directory, first build the server:
(this is for active development mode)
```
go build 
```
Execute the process:
```
./server -dsn="postgresql://postgres:ArtiSoft001@172.17.0.2:5432/postgres" -lookupDb test_data/lookup_test1.db -outTables=hc__claim -pcKey=1 -ruleset=workspace_test1.jr -sessionId=session1 -workspaceDb=test_data/workspace_test1.db -poolSize=1 -ps
```
Execute the process with c++ logging:
```
GLOG_v=1 ./server -dsn="postgresql://postgres:ArtiSoft001@172.17.0.2:5432/postgres"  -lookupDb test_data/lookup_test1.db -outTables=hc__claim -pcKey=1 -ruleset=workspace_test1.jr -sessId=sess1 -workspaceDb=test_data/workspace_test1.db -poolSize=1
```
