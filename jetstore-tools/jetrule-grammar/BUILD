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
  name = "jet_listener_test",
  srcs = ["jet_listener_test.py"],
  python_version = "PY3",
  srcs_version = "PY3",
  deps = [
    ":jetrule_compiler",
    requirement("absl-py"),
  ],
  imports = ["jetrule_g4_gen.py"],
)

py_test(
  name = "jet_listener_test2",
  srcs = ["jet_listener_test2.py"],
  python_version = "PY3",
  srcs_version = "PY3",
  deps = [
    ":jetrule_compiler",
    requirement("absl-py"),
  ],
  data = [
    ":jet_listerner_test_data.jr",
    ":jet_listerner_test_data.jr.json",
  ],
  imports = ["jetrule_g4_gen.py"],
)

py_test(
  name = "jet_listener_postprocessing_test",
  srcs = ["jet_listener_postprocessing_test.py"],
  python_version = "PY3",
  srcs_version = "PY3",
  deps = [
    ":jetrule_compiler",
    requirement("absl-py"),
  ],
  data = [
    ":rule6_test.json",
    ":rule7_test.json",
  ],
  imports = ["jetrule_g4_gen.py"],
)

py_test(
  name = "jet_listener_postprocessing_test2",
  srcs = ["jet_listener_postprocessing_test2.py"],
  python_version = "PY3",
  srcs_version = "PY3",
  deps = [
    ":jetrule_compiler",
    requirement("absl-py"),
  ],
  data = [
    ":jet_listener_postprocessing_test_data.jr",
    ":jet_listener_postprocessing_test_data.jr.json",
  ],
  imports = ["jetrule_g4_gen.py"],
)

py_test(
  name = "jetrule_validator_test",
  srcs = ["jetrule_validator_test.py"],
  python_version = "PY3",
  srcs_version = "PY3",
  deps = [
    ":jetrule_compiler",
    requirement("absl-py"),
  ],
  data = [
    ":import_test1.jr",
    ":import_test2.jr",
  ],
  imports = ["jetrule_g4_gen.py"],
)

# genrule(
#     name = "gen_jetrule_parser",
#     srcs = [
#       "JetRule.g4",
#       "@antlr4_jar//file",
#     ],
#     outs = [
#       # "envinfo22.txt",
#       "JetRule.interp",
#       "JetRule.tokens",
#       "JetRuleLexer.interp",
#       "JetRuleLexer.py",
#       "JetRuleLexer.tokens",
#       "JetRuleListener.py",
#       "JetRuleParser.py",
#     ],
#     local = True,
#     cmd = """java -Xmx500M -cp $(location @antlr4_jar//file) org.antlr.v4.Tool -Dlanguage=Python3 $(location JetRule.g4) -Xexact-output-dir -o py \
#       && mv py/JetRule.interp $(location JetRule.interp)               \
#       && mv py/JetRule.tokens $(location JetRule.tokens)              \
#       && mv py/JetRuleLexer.interp $(location JetRuleLexer.interp)              \
#       && mv py/JetRuleLexer.py $(location JetRuleLexer.py)              \
#       && mv py/JetRuleLexer.tokens $(location JetRuleLexer.tokens)              \
#       && mv py/JetRuleListener.py $(location JetRuleListener.py)              \
#       && mv py/JetRuleParser.py $(location JetRuleParser.py)              \
#       """,
#     # cmd_bash = """( env; echo "-----"; pwd; ls -l; echo "-----"; whoami; echo "outs = $@" ) | tee $@""",
#     tools = ["@antlr4_jar//file"],
# )

# py_binary(
#   name = "jetrule_compiler",
#   srcs = [
#     "jetrule_compiler.py",
#     "jet_listener.py",                 # this produce my-rules.jr.json from input file my-rule.jr
#     "jet_listener_postprocessing.py",  # this augment  my-rules.jr.json
#     "JetRuleLexer.py",
#     "JetRuleListener.py",
#     "JetRuleParser.py",
#     # next is to validate the integrity on the input my-rules.jr.json, then generate the rete network, output my-rules.rete.json
#   ],
#   # imports = ["jetrule_g4_gen.py"],
#   python_version = "PY3",
#   data = [":gen_jetrule_parser"], 
#   deps = [
#       # ":gen_jetrule_parser",
#       requirement("absl-py"),
#       requirement("antlr4-python3-runtime"),
#   ],
# )
