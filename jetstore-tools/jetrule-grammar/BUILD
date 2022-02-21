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
    "jetrule_rete_sqlite.py",
    "jet_listener.py",                # this produce my-rules.jr.json from input file my-rule.jr
    "jetrule_post_processor.py",      # this augment  my-rules.jr.json
    "jetrule_validator.py",
    "jetrule_optimizer.py",
    "jetrule_rete.py",
    "jetrule_context.py",
  ],
  imports = ["jetrule_g4_gen.py"],
  python_version = "PY3",
  data = [
    ":jetrule_main_test.jr",
    ":jetrule_main_test2.jr",
    ":jetrule_support_test.jr",
    "//jets/rete/rete_test_db:ms_factory_test1.jr",
    "//jets/rete/rete_test_db:ms_factory_test2.jr",
  ],
  # data = [":transform"],  # a cc_binary which we invoke at run time
  deps = [
      ":jetrule_lang_lib",
      "@antlr4_runtimes//:python",
      requirement("absl-py"),
      requirement("apsw"),
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
    ":jetrule_main_test.jr",
    ":jetrule_support_test.jr",
    ":jetrule_main_test.jr.json",
    ":jetrule_main_test.jrc.json",
    "//jets/rete/rete_test_db:ms_factory_test2.jr",
    "//jets/rete/rete_test_db:ms_factory_test2.jrc.json",
  ],
  imports = ["jetrule_g4_gen.py"],
)

py_test(
  name = "jetrule_post_processor_test",
  srcs = ["jetrule_post_processor_test.py"],
  python_version = "PY3",
  srcs_version = "PY3",
  deps = [
    ":jetrule_compiler",
    requirement("absl-py"),
  ],
  data = [
    ":rule5_test.json",
    ":rule6_test.json",
    ":rule7_test.json",
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
  imports = ["jetrule_g4_gen.py"],
)

py_test(
  name = "jetrule_optimizer_test",
  srcs = ["jetrule_optimizer_test.py"],
  python_version = "PY3",
  srcs_version = "PY3",
  deps = [
    ":jetrule_compiler",
    requirement("absl-py"),
  ],
  # data = [
  #   ":import_test1.jr",
  #   ":import_test2.jr",
  # ],
  imports = ["jetrule_g4_gen.py"],
)

py_test(
  name = "jetrule_compiler_test",
  srcs = ["jetrule_compiler_test.py"],
  python_version = "PY3",
  srcs_version = "PY3",
  deps = [
    ":jetrule_compiler",
    requirement("absl-py"),
  ],
  data = [
    ":import_test1.jr",
    ":import_test11.jr",
    ":import_test2.jr",
    ":import_test21.jr",
    ":import_test3.jr",
    ":import_test31.jr",
    ":import_test32.jr",
    ":import_test4.jr",
    ":import_test41.jr",
    ":import_test42.jr",
  ],
  imports = ["jetrule_g4_gen.py"],
)

py_test(
  name = "jetrule_rete_test",
  srcs = ["jetrule_rete_test.py"],
  python_version = "PY3",
  srcs_version = "PY3",
  deps = [
    ":jetrule_compiler",
    requirement("absl-py"),
  ],
  # data = [
  #   ":import_test1.jr",
  #   ":import_test2.jr",
  # ],
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
#     "jetrule_post_processor.py",  # this augment  my-rules.jr.json
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
