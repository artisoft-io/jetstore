# Docker Notes (a bit old)

## Antlr4 image

```bash
docker build --build-arg USER_ID=`id -u` --build-arg GROUP_ID=`id -g` -t antlr4:latest -f Dockerfile.antlr4 . 
```

## Running antlr4 to generate the parser class

### Run from the compiler source directory (where JetRule.g4 is located)

```bash
cd ~/projects/repos/jetstore/jets/compiler
docker run --rm -u $(id -u ${USER}):$(id -g ${USER}) -v `pwd`:/work antlr4 -Dlanguage=Python3 JetRule.g4
```

### Running the jetrule compiler from the source directory

From the docker dev container, in the source directory:

```bash
cd /go/jetstore/jets/compiler
python3 jetrule_compiler.py --help
python3 jetrule_compiler.py --base_path test_data --in_file test_rule_file3.jr
```

## Building Runtime images

### Using golang as the base for the builder

Using Dockerfile.go_bullseye as the *builder* image:

```bash
docker pull golang:1.19-bullseye
docker build --build-arg JETS_VERSION=2022.1.0 -t jetstore_builder:go-bullseye -f Dockerfile.go_bullseye .
```

To inspect the builder image

```bash
docker run -it --rm --entrypoint=/bin/bash jetstore_builder:go-bullseye
```


### Using debian:bullseye as base runtime image (retained approach)

This is what we use.
Build the runtime base image

```bash
docker pull debian:bullseye
docker build -t jetstore_base:bullseye -f Dockerfile.bullseye_base .
```

Building the runtime image (PACKAGE THE BUILD)

```bash
docker build --build-arg JETS_VERSION=2022.1.0 -t jetstore:bullseye -f Dockerfile.rt_bullseye .
```

Testing the c++ library:

```bash
docker run --rm -w=/usr/local/bin --entrypoint=jets_test jetstore:bullseye
```

Testing the python lib:

```bash
docker run --rm -w=/usr/local/lib/jets/compiler --entrypoint=python3 jetstore:bullseye jetrule_compiler_test.py
```

Testing the go lib:

```bash
docker run --rm -w=/usr/local/bin --entrypoint=update_db jetstore:bullseye -h
```

## Generating lookup test cases

### Generate rete.db from rule file

```bash
python3 jetrule_compiler.py --base_path=/go/jetstore/jets/rete/test_data --in_file=lookup_helper_test_workspace.jr --rete_db=lookup_helper_test_workspace.db -d
```

### Generate lookup data db from workspace rete.db

```bash
python3 jetrule_lookup_loader.py --base_path=/go/jetstore/jets/rete/test_data --lookup_db=lookup_helper_test_data.db --rete_db=lookup_helper_test_workspace.db
```

## Running Postgresql Locally

### Running Postgres DB docker container locally

Pull the postgres image from docker hub and run it locally:

```bash
docker pull postgres:14-bullseye
docker run --rm --name postgres -p 5438:5432 -v /home/michel/projects/pg_work:/work -e 'POSTGRES_PASSWORD=XXXPWDXXX' -e 'POSTGRES_USER=postgres' postgres:14-bullseye
```

Get into the container:

```bash
docker exec -it postgres /bin/bash
```

Note that the dir /work is mapped to pg_work on our local workstation. To execute psql:

```bash
cd /work
psql -U postgres
\i copy_test.sql
\q
```

You can execute the script directly without going into the psql prompt:

```bash
cd /work
psql -U postgres -a -f copy_test.sql
```

Now to connect to this container from another container (e.g. PgAdmin) you need to know it's IP address that docker gave it (setting up docker compose would be better, will do that later).
Get the IP of postgres:

```bash
docker network inspect bridge
```

The connection string is now (with the correct IP of postgres) is
`postgresql://postgres:XXXPWDXXX@172.17.0.2:5432/postgres`

### Running PgAdmin docker locally

Pull the image from docker hub and run it locally:

```bash
docker pull dpage/pgadmin4
docker run --rm --name pgadmin -p 80:80 -e 'PGADMIN_DEFAULT_EMAIL=michel@artisoft.io' -e 'PGADMIN_DEFAULT_PASSWORD=XXXPWDXXX' dpage/pgadmin4
```
