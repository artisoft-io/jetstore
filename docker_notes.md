# DEV BUILDER
# ----------------------------------------------------------------------------------------------
docker build --build-arg JETS_VERSION=2022.1.0 --build-arg USER_ID=`id -u` --build-arg GROUP_ID=`id -g` -t dev:latest -f Dockerfile.dev_go . 

# Antlr4 image
docker build --build-arg USER_ID=`id -u` --build-arg GROUP_ID=`id -g` -t antlr4:latest -f Dockerfile.antlr4 . 

## Building the image
# ----------------------------------------------------------------------------------------------
docker run -it --rm -u `id -u`:`id -g` \
    -v /home/michel/projects/repos/jetstore:/home/michel/projects/repos/jetstore \
    -v /home/michel/projects/workspaces:/workspaces \
    -v /home/michel/projects/work:/go/work \
    --name jets_dev \
    --entrypoint=/bin/bash dev:latest

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


