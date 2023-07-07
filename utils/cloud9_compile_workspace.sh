#!/bin/bash
set -e

. /go/work/cloud9_jetstore_env.sh 

cd ${WORKSPACES_HOME}/${WORKSPACE}
./compile_workspace.sh 
