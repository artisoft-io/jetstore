load("@rules_python//python:defs.bzl", "py_binary", "py_library", "py_test")
load("@jst_deps//:requirements.bzl", "requirement")

py_library(
  name = "jetstore_py",
  srcs = ["jetstore_api.py"],
  visibility = ["//visibility:public"],
  # imports = ["jets/rete"],
  deps = [
    "//jets/rete:jetstore_binding",
    # "//jets/rete:libjets_rete.so",
  ],
  data = [
    '//jets/rete:libjets_rete.so',
    "//jets/rete/rete_test_db:jetrule_rete_test.db",
  ],
)

py_test(
  name = "jetstore_test",
  srcs = ["jetstore_test.py"],
  python_version = "PY3",
  srcs_version = "PY3",
  deps = [
    "//jets/rete:jetstore_binding",
    ":jetstore_py",
    requirement("absl-py"),
  ],
  imports = ["jetstore_py"],
  data = [
    '//jets/rete:libjets_rete.so',
    "//jets/rete/rete_test_db:jetrule_rete_test.db",
  ],
)
