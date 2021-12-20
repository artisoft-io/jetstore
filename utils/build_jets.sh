#!/bin/bash
set -e
echo "BUILD JETS SERVER ARGUMENTS:  $@"

cd ${JETS_SOURCE_DIR}
mkdir -p build
cd build
cmake -DCMAKE_BUILD_TYPE=Release -DJETS_VERSION=$JETS_VERSION ..
make clean 
make -j12
cd ..
