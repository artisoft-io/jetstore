workspace(name = "io_artisoft_jetstore")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")

http_archive(
  name = "com_google_absl",
  urls = ["https://github.com/abseil/abseil-cpp/archive/98eb410c93ad059f9bba1bf43f5bb916fc92a5ea.zip"],
  strip_prefix = "abseil-cpp-98eb410c93ad059f9bba1bf43f5bb916fc92a5ea",
)

# GoogleTest/GoogleMock framework. Used by most unit-tests.
http_archive(
    name = "com_google_googletest",  # 2021-07-09T13:28:13Z
    sha256 = "12ef65654dc01ab40f6f33f9d02c04f2097d2cd9fbe48dc6001b29543583b0ad",
    strip_prefix = "googletest-8d51ffdfab10b3fba636ae69bc03da4b54f8c235",
    urls = ["https://github.com/google/googletest/archive/8d51ffdfab10b3fba636ae69bc03da4b54f8c235.zip"],
)

# Google benchmark.
http_archive(
    name = "com_github_google_benchmark",  # 2021-09-20T09:19:51Z
    sha256 = "62e2f2e6d8a744d67e4bbc212fcfd06647080de4253c97ad5c6749e09faf2cb0",
    strip_prefix = "benchmark-0baacde3618ca617da95375e0af13ce1baadea47",
    urls = ["https://github.com/google/benchmark/archive/0baacde3618ca617da95375e0af13ce1baadea47.zip"],
)

# Bazel platform rules.
http_archive(
    name = "platforms",
    sha256 = "b601beaf841244de5c5a50d2b2eddd34839788000fa1be4260ce6603ca0d8eb7",
    strip_prefix = "platforms-98939346da932eef0b54cf808622f5bb0928f00b",
    urls = ["https://github.com/bazelbuild/platforms/archive/98939346da932eef0b54cf808622f5bb0928f00b.zip"],
)

# Bazel python rules
http_archive(
    name = "rules_python",
    sha256 = "a30abdfc7126d497a7698c29c46ea9901c6392d6ed315171a6df5ce433aa4502",
    strip_prefix = "rules_python-0.6.0",
    url = "https://github.com/bazelbuild/rules_python/archive/0.6.0.tar.gz",
)

load("@rules_python//python:pip.bzl", "pip_install")

# Create a central external repo, @jst_deps, that contains Bazel targets for all the
# third-party packages specified in the requirements.txt file.
pip_install(
   name = "jst_deps",
   requirements = "//jetstore-tools:requirements.txt",
)

# # Adding support for antlr4, using jar
# http_file(
#     name = "antlr4_jar",
#     sha256 = "bd11b2464bc8aee5f51b119dff617101b77fa729540ee7f08241a6a672e6bc81",
#     urls = ["https://www.antlr.org/download/antlr-4.9-complete.jar"],
# )

# Adding support for antlr4 via bazel rules
http_archive(
    name = "rules_antlr",
    sha256 = "234c401cfabab78f2d7f5589239d98f16f04338768a72888f660831964948ab1",
    strip_prefix = "rules_antlr-0.6.0",
    urls = ["https://github.com/artisoft-io/rules_antlr/archive/refs/tags/0.6.0.tar.gz"],
)

load("@rules_antlr//antlr:repositories.bzl", "rules_antlr_dependencies")
load("@rules_antlr//antlr:lang.bzl", "PYTHON")
rules_antlr_dependencies("4.9.3", PYTHON)

load("@bazel_tools//tools/build_defs/repo:git.bzl", "git_repository")

git_repository(
    name = "com_github_nelhage_rules_boost",
    commit = "fce83babe3f6287bccb45d2df013a309fa3194b8",
    remote = "https://github.com/nelhage/rules_boost",
    shallow_since = "1591047380 -0700",
)

load("@com_github_nelhage_rules_boost//:boost/boost.bzl", "boost_deps")
boost_deps()

# Command line arguments
http_archive(
    name = "com_github_gflags_gflags",
    sha256 = "34af2f15cf7367513b352bdcd2493ab14ce43692d2dcd9dfc499492966c64dcf",
    strip_prefix = "gflags-2.2.2",
    urls = ["https://github.com/gflags/gflags/archive/v2.2.2.tar.gz"],
)

# Logging
http_archive(
    name = "com_github_google_glog",
    sha256 = "21bc744fb7f2fa701ee8db339ded7dce4f975d0d55837a97be7d46e8382dea5a",
    strip_prefix = "glog-0.5.0",
    urls = ["https://github.com/google/glog/archive/v0.5.0.zip"],
)

# Add SQLite
# Then sqlite is available as @com_github_rockwotj_sqlite_bazel//:sqlite3.
SQLITE_BAZEL_COMMIT = "1e793d9f9350de0fce449b5186fea46693c8622e"

http_archive(
    name = "com_github_rockwotj_sqlite_bazel",
    strip_prefix = "sqlite-bazel-" + SQLITE_BAZEL_COMMIT,
    urls = ["https://github.com/rockwotj/sqlite-bazel/archive/%s.zip" % SQLITE_BAZEL_COMMIT],
)

# # To generate compile_commands.json
# http_archive(
#     name = "com_grail_bazel_compdb",
#     strip_prefix = "bazel-compilation-database-0.5.2",
#     urls = ["https://github.com/grailbio/bazel-compilation-database/archive/0.5.2.tar.gz"],
# )

# load("@com_grail_bazel_compdb//:deps.bzl", "bazel_compdb_deps")
# bazel_compdb_deps()

# The Python toolchain must be registered ALWAYS at the end of the file
register_toolchains("//jetstore-tools:py_3_toolchain")