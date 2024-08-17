# Server Unit Test1

Simple unit test for server process consisting of following files:

- ruleset1_test1.jr: main rule file
- ruleset2_test1.jr: main rule file
- support_test1.jr: common components file
- code_description_test1.csv: lookup table data
- process_config_test1.sql: process config data

## Generate the workspace db using jetstore compiler

Command to execute the compiler from the jetserver source directory `server/`
(the workspace base directory is `server/test_data`.)
Run the compiler on the main rule files:`ruleset1_test1.jr` and `ruleset2_test1.jr`

```bash
docker run --rm -v=`pwd`/test_data:/go/work -w=/usr/local/lib/jets/compiler \
  --entrypoint=python3 jetstore:bullseye jetrule_compiler.py      \
  --base_path /go/work --in_file ruleset1_test1.jr -d --rete_db workspace_test1.db

docker run --rm -v=`pwd`/test_data:/go/work -w=/usr/local/lib/jets/compiler \
  --entrypoint=python3 jetstore:bullseye jetrule_compiler.py      \
  --base_path /go/work --in_file ruleset2_test1.jr --rete_db workspace_test1.db
```

Running `test2`

```bash
docker run --rm -v=`pwd`/test_data:/go/work -w=/usr/local/lib/jets/compiler \
  --entrypoint=python3 jetstore:bullseye jetrule_compiler.py      \
  --base_path /go/work --in_file ruleset1_test2.jr -d --rete_db workspace_test2.db

docker run --rm -v=`pwd`/test_data:/go/work -w=/usr/local/lib/jets/compiler \
  --entrypoint=python3 jetstore:bullseye jetrule_compiler.py      \
  --base_path /go/work --in_file ruleset2_test2.jr --rete_db workspace_test2.db

docker run --rm -v=`pwd`/test_data:/go/work -w=/usr/local/lib/jets/compiler \
  --entrypoint=python3 jetstore:bullseye jetrule_compiler.py      \
  --base_path /go/work --in_file ruleseq_test2.jr --rete_db workspace_test2.db
```

## Generate the lookup data db using lookup loader

Simularily, from the `test_data` directory where the workspace files are located:

```bash
docker run --rm -v=`pwd`:/go/work -w=/usr/local/lib/jets/compiler \
  --entrypoint=python3 jetstore:bullseye jetrule_lookup_loader.py      \
  --base_path /go/work  --lookup_db lookup_test1.db --rete_db workspace_test1.db
```

## Load the input csv file using jetstore loader

Load of csv file using jetstore loader into the platform postgres database.
From the `server/` directory:

```bash
docker run --rm -v=`pwd`:/go/work -w=/go/work \
  --entrypoint=loader jetstore:bullseye    \
  -dsn="postgresql://postgres:<PWD>@<IP>:5432/postgres" \
  -table=test1 -in_file=test_data/input_data_test1.csv -sep '|' -d 
```

In postgres, you can query the created table to see it's schema:

```bash
select table_name,column_name,data_type from information_schema.columns where table_name = 'test1';
```

## Load the process config data structure into jetstore postgres database

Load the process config located in `test_data/process_config_test1.sql` into jetstore processing database. 
First copy the script into the mounted folder, from the server source folder:

```bash
cp -v test_data/process_config_test1.sql ~/projects/work
```

Connect into the running postgres container:

```bash
docker exec -it postgres /bin/bash
```

Execute the load script:

```bash
psql -U postgres -a -f process_config_test1.sql
```

## Update database

```bash
../update_db/update_db -dsn="postgresql://postgres:<PWD>@<IP>:5432/postgres" -drop -workspaceDb test_data/workspace_test1.db 
```

## Running the server process

Now that we have all of the parts in place, we can run the server process.
Running directly from the source directory, first build the server:
(this is for active development mode) using the command `go build`
Execute the process:

```bash
./server -dsn="postgresql://postgres:<PWD>@<IP>:5432/postgres" -lookupDb test_data/lookup_test1.db -outTables=hc__claim -pcKey=1 -ruleseq=step1 -sessionId=session1 -workspaceDb=test_data/workspace_test1.db -poolSize=1
```

Execute the process with c++ logging:

```bash
GLOG_v=1 ./server -dsn="postgresql://postgres:<PWD>@<IP>:5432/postgres"  -lookupDb test_data/lookup_test1.db -outTables=hc__claim -pcKey=1 -ruleset=workspace_test1.jr -sessionId=sess1 -workspaceDb=test_data/workspace_test1.db -poolSize=1
```

Running `test2` with logging:

```bash
GLOG_v=4 ./server -dsn="postgresql://postgres:<PWD>@<IP>:5432/postgres"  -outTables=hc__zipclaim -pcKey=201 -ruleseq=step1 -sessionId=sess1 -workspaceDb=test_data/workspace_test2.db -poolSize=1
```
