workspace(name = "io_artisoft_jetstore")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")

http_archive(                                         # Updated on 2022-02-12
  name = "com_google_absl",
  urls = ["https://github.com/abseil/abseil-cpp/archive/73316fc3c565e5998983b0fb502d938ccddcded2.zip"],
  strip_prefix = "abseil-cpp-73316fc3c565e5998983b0fb502d938ccddcded2",
)

# google/googletest framework. Used by most unit-tests.
http_archive(
    name = "com_google_googletest",                   # Updated on 2022-02-12
    # sha256 = "12ef65654dc01ab40f6f33f9d02c04f2097d2cd9fbe48dc6001b29543583b0ad",
    strip_prefix = "googletest-0e402173c97aea7a00749e825b194bfede4f2e45",
    urls = ["https://github.com/google/googletest/archive/0e402173c97aea7a00749e825b194bfede4f2e45.zip"],
)

# google/benchmark
http_archive(
    name = "com_github_google_benchmark",             # Updated on 2022-02-12
    # sha256 = "62e2f2e6d8a744d67e4bbc212fcfd06647080de4253c97ad5c6749e09faf2cb0",
    strip_prefix = "benchmark-6e51dcbcc3965b3f4b13d4bab5e43895c1a73290",
    urls = ["https://github.com/google/benchmark/archive/6e51dcbcc3965b3f4b13d4bab5e43895c1a73290.zip"],
)

# bazelbuild/platforms -- Bazel platform rules.
http_archive(
    name = "platforms",                               # Updated on 2022-02-12
    # sha256 = "b601beaf841244de5c5a50d2b2eddd34839788000fa1be4260ce6603ca0d8eb7",
    strip_prefix = "platforms-fbd0d188dac49fbcab3d2876a2113507e6fc68e9",
    urls = ["https://github.com/bazelbuild/platforms/archive/fbd0d188dac49fbcab3d2876a2113507e6fc68e9.zip"],
)

# bazelbuild/rules_python  -- Bazel python rules
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

# NIX
# # Add nix packages for external dependencies
# # Import and load the Bazel rules to build Nix packages.
# http_archive(
#     name = "io_tweag_rules_nixpkgs",
#     sha256 = "7aee35c95251c1751e765f7da09c3bb096d41e6d6dca3c72544781a5573be4aa",
#     strip_prefix = "rules_nixpkgs-0.8.0",
#     urls = ["https://github.com/tweag/rules_nixpkgs/archive/v0.8.0.tar.gz"],
# )
# load("@io_tweag_rules_nixpkgs//nixpkgs:repositories.bzl", "rules_nixpkgs_dependencies")
# rules_nixpkgs_dependencies()
# load("@io_tweag_rules_nixpkgs//nixpkgs:nixpkgs.bzl", "nixpkgs_git_repository", "nixpkgs_package")
# nixpkgs_package(name = "postgresql")
# nixpkgs_package(name = "libpqxx")
# NIX

# FOREIGN RULE
http_archive(
    name = "rules_foreign_cc",
    sha256 = "bcd0c5f46a49b85b384906daae41d277b3dc0ff27c7c752cc51e43048a58ec83",
    strip_prefix = "rules_foreign_cc-0.7.1",
    url = "https://github.com/bazelbuild/rules_foreign_cc/archive/0.7.1.tar.gz",
)

load("@rules_foreign_cc//foreign_cc:repositories.bzl", "rules_foreign_cc_dependencies")

# This sets up some common toolchains for building targets. For more details, please see
# https://bazelbuild.github.io/rules_foreign_cc/0.7.1/flatten.html#rules_foreign_cc_dependencies
rules_foreign_cc_dependencies()
# FOREIGN RULE

# Add libpqxx repo -- will use a foreign rule to compile it with cmake
_ALL_CONTENT = """\
filegroup(
    name = "all_srcs",
    srcs = glob(["**"]),
    visibility = ["//visibility:public"],
)
"""
# libpqxx source code repository
http_archive(
    name = "com_github_jtv_libpqxx",
    build_file_content = _ALL_CONTENT,
    strip_prefix = "libpqxx-7.7.0",
    url = "https://github.com/jtv/libpqxx/archive/refs/tags/7.7.0.tar.gz",
)
# Add libpqxx repo -- will use a foreign rule to compile it with cmake


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