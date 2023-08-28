#!/bin/bash
set -e

. /go/work/cloud9_jetstore_env.sh 

# Run when the data model has changed
cd ${WORKSPACES_HOME}/${WORKSPACE}
update_db -workspaceDb workspace.db -migrateDb
