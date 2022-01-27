load("@rules_antlr//antlr:antlr4.bzl", "antlr")
load("@rules_python//python:defs.bzl", "py_binary", "py_library", "py_test")
load("@jst_deps//:requirements.bzl", "requirement")

antlr(
  name = "jetrule_g4_gen",
  srcs = ["JetRule.g4"],
  language = "Python3",
)

py_library(
  name = "jetrule_lang_lib",
  srcs = [":jetrule_g4_gen"],
)

py_binary(
  name = "jetrule_compiler",
  srcs = [
    "jetrule_compiler.py",
    "jet_listener.py",                # this produce my-rules.jr.json from input file my-rule.jr
    "jet_listener_postprocessing.py"  # this augment  my-rules.jr.json
    # next is to validate the integrity on the input my-rules.jr.json, then generate the rete network, output my-rules.rete.json
  ],
  imports = ["jetrule_g4_gen.py"],
  python_version = "PY3",
  # data = [":transform"],  # a cc_binary which we invoke at run time
  deps = [
      ":jetrule_lang_lib",
      "@antlr4_runtimes//:python",
      requirement("absl-py"),
  ],
)

py_test(
  name = "first_test",
  srcs = ["first_test.py"],
  python_version = "PY3",
  srcs_version = "PY3",
  deps = [
    ":jetrule_compiler",
    requirement("absl-py"),
  ],
  imports = ["jetrule_g4_gen.py"],
)
