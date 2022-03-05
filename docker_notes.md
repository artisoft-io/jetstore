# ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### #
# DEV BUILDER
# ----------------------------------------------------------------------------------------------
docker build --build-arg JETS_VERSION=2022.1.0 --build-arg USER_ID=`id -u` --build-arg GROUP_ID=`id -g` -t dev:latest -f Dockerfile.dev_go . 

# ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### #
# RUN DEV BUILDER
# ----------------------------------------------------------------------------------------------
docker run -it --rm -u `id -u`:`id -g` \
    -v /home/michel/projects/repos/jetstore:/home/michel/projects/repos/jetstore \
    -v /home/michel/projects/workspaces:/workspaces \
    -v /home/michel/projects/work:/go/work \
    --name jets_dev \
    --entrypoint=/bin/bash dev:latest

${workspaceFolder}/**

# To build
rm -rf build && mkdir build && cd build && cmake .. && make -j8 

cd build 
cmake ..


