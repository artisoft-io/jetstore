# DEV BUILDER
# ----------------------------------------------------------------------------------------------
```
docker build --build-arg JETS_VERSION=2022.1.0 --build-arg USER_ID=`id -u` --build-arg GROUP_ID=`id -g` -t dev:latest -f Dockerfile.dev_go . 
```

# Antlr4 image
```
docker build --build-arg USER_ID=`id -u` --build-arg GROUP_ID=`id -g` -t antlr4:latest -f Dockerfile.antlr4 . 
```

## Building the image
# ----------------------------------------------------------------------------------------------
docker run -it --rm -u `id -u`:`id -g` \
    -v /home/michel/projects/repos/jetstore:/home/michel/projects/repos/jetstore \
    -v /home/michel/projects/repos/RC-Workspace:/workspaces \
    -v /home/michel/projects/work:/go/work \
    --name jets_dev \
    --entrypoint=/bin/bash dev:latest

### Running antlr4 to generate the parser class
### Run from the compiler source directory (where JetRule.g4 is located)
cd ~/projects/repos/jetstore/jets/compiler
docker run --rm -u $(id -u ${USER}):$(id -g ${USER}) -v `pwd`:/work antlr4 -Dlanguage=Python3 JetRule.g4

### Running the jetrule compiler from the source directory
From the docker dev container, in the source directory:
```
$ cd /go/jetstore/jets/compiler
$ python3 jetrule_compiler.py --help
$ python3 jetrule_compiler.py --base_path test_data --in_file test_rule_file3.jr
```

# To build
rm -rf build && mkdir build && cd build && cmake .. && make -j8 

cd build 
cmake ..

# Runtime images
## Using golang as the base for the builder
Using Dockerfile.go_bullseye as the builder image:
```
docker build --build-arg JETS_VERSION=2022.1.0 -t jetstore_builder:go-bullseye -f Dockerfile.go_bullseye .
```

## Using golang runtime base image
First attempt is using golang:1.18-bullseye as base image and copy the compiled
library to it.

The base runtime image is Dockerfile.go_base, it install python 3.9 with required packages
To build the image:
```
docker build --build-arg JETS_VERSION=2022.1.0 -t jetstore_base:go-bullseye -f Dockerfile.go_base .
```

## Putting together the jetstore runtime image frm the builder and the runtime base images
Using docker gile Dockerfile.rt_do_bullseye
```
docker build --build-arg JETS_VERSION=2022.1.0 -t jetstore:go-bullseye -f Dockerfile.rt_go_bullseye .
```
Try the image
docker run -it --rm --entrypoint /bin/bash jetstore:go-bullseye
