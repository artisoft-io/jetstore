#!/bin/bash
set -e
echo "Compile Workspace $WORKSPACE"
echo "WORKSPACE: $WORKSPACE"
echo "WORKSPACES_HOME: $WORKSPACES_HOME"
echo "JETRULE_COMPILER: $JETRULE_COMPILER"
echo "JETRULE_LOOKUP_LOADER: $JETRULE_LOOKUP_LOADER"

cd ${WORKSPACES_HOME}/${WORKSPACE}

# Test Lookup
python3 $JETRULE_COMPILER -s --base_path ${WORKSPACES_HOME}/${WORKSPACE} --in_file jet_rules/test_lookup_main.jr     --rete_db workspace.db -d
# Test Looping
python3 $JETRULE_COMPILER -s --base_path ${WORKSPACES_HOME}/${WORKSPACE} --in_file jet_rules/test_looping_main.jr    --rete_db workspace.db

# Lookup tables
python3 $JETRULE_LOOKUP_LOADER --base_path ${WORKSPACES_HOME}/${WORKSPACE} --rete_db workspace.db --lookup_db lookup.db 
