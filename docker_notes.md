# ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### #
# DEV BUILDER
# ----------------------------------------------------------------------------------------------
docker build --build-arg JETS_VERSION=2021.1.0 -t dev:latest -f Dockerfile.dev_builder . 

# ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### # ### #
# RUN DEV BUILDER
# ----------------------------------------------------------------------------------------------
docker run -it --rm -p 8080:8080 -u `id -u`:`id -g` \
    -v /home/michel/projects/repos/jetstore:/home/michel/projects/repos/jetstore \
    -v /home/michel/projects/workspaces:/workspaces \
    -v /home/michel/projects/test_work:/work \
    --name jets_dev \
    --entrypoint=/bin/bash dev:latest

${workspaceFolder}/**

# To build
bazel build --sandbox_debug  //jets:jetstore

# To build with large output
bazel build //jets/rete:jets_rete_test --verbose_failures --experimental_ui_max_stdouterr_bytes=5000000

# Need this
BAZEL_CXXOPTS="-std=c++17:-O3" bazel build --verbose_failures //jets/rdf:jets_rdf_test
BAZEL_CXXOPTS="-std=c++17:-O3" bazel build --verbose_failures //jets/rdf:jets_rdf_benchmark

# To Run with log to stderr
GLOG_logtostderr=1 bazel-bin/jets/jetstore 
GLOG_logtostderr=1 bazel-bin/jets/jetstore --languages=francais,english

# Running tests
bazel test --test_output=all //jets/rdf:jets_rdf_test
BAZEL_CXXOPTS="-std=c++17:-O3" bazel test --test_output=all //jets/rdf:jets_rdf_benchmark

BAZEL_CXXOPTS="-std=c++17:-O3" bazel build --test_output=all //jets/rdf:jets_rdf_benchmark

BAZEL_CXXOPTS="-std=c++17:-O3" bazel build --test_output=all //jets/rete:jets_rete_test

# To generate compile_commands.json:
INSTALL_DIR="/usr/local/bin"
VERSION="0.5.2"

# Download and symlink.
(
  cd "${INSTALL_DIR}" \
  && curl -L "https://github.com/grailbio/bazel-compilation-database/archive/0.5.2.tar.gz" | tar -xz \
  && ln -f -s "${INSTALL_DIR}/bazel-compilation-database-${VERSION}/generate.py" bazel-compdb
)

bazel-compdb # This will generate compile_commands.json in your workspace root.

# To pass additional flags to bazel, pass the flags as arguments after --
bazel-compdb -- [additional flags for bazel]

# You can tweak some behavior with flags:
# 1. To use the source dir instead of bazel-execroot for directory in which clang commands are run.
bazel-compdb -s
bazel-compdb -s -- [additional flags for bazel]
# 2. To consider only targets given by a specific query pattern, say `//cc/...`. Also see below section for another way.
bazel-compdb -q //cc/...
bazel-compdb -q //cc/... -- [additional flags for bazel]