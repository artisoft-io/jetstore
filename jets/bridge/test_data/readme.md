# Compiling JetRule Files
From the directory that is the workspace base directory 
you can do using the jetstore docker image:
```
docker run --rm -v=`pwd`:/go/work -w=/usr/local/lib/jets/compiler \
  --entrypoint=python3 jetstore:bullseye jetrule_compiler.py      \
  --base_path /go/work --in_file bridge_test1.jr -d --rete_db bridge_test1.db
```