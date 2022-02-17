# USING NIX ATTEMPT
# cc_import(
#   name = "pqxx",
#   includes = ["@libpqxx//:include"],
#   # hdrs = ["@libpqxx//:include"],
#   # static_library = "@libpqxx//:libpqxx.a",
#   static_library = "libpqxx.a",
#   # If alwayslink is turned on,
#   # libmylib.a will be forcely linked into any binary that depends on it.
#   # alwayslink = 1,
# )
# USING NIX ATTEMPT

# USING FOREIGN_CC_DEPEDENCY
load("@rules_foreign_cc//foreign_cc:defs.bzl", "cmake")
cmake(
    name = "libpqxx",
    # cache_entries = {
    #     "CMAKE_C_FLAGS": "-fPIC",
    # },
    env = {
      "SKIP_BUILD_TEST": "ON",
      "BUILD_SHARED_LIBS": "OFF",
    },
    lib_source = "@com_github_jtv_libpqxx//:all_srcs",
    out_static_libs = ["libpqxx.a"],
)
# USING FOREIGN_CC_DEPEDENCY

cc_library(
    name = "jets_rete",
    srcs = [
        "antecedent_query_visitor.h",
        "alpha_functors.h",
        "alpha_node_impl.h",
        "alpha_node.h",
        "beta_relation.h",
        "beta_row_initializer.h",
        "beta_row_iterator.h",
        "beta_row.h",
        "expr_impl.h",
        "expr_operators.h",
        "expr_operator_factory.h",
        "expr.h",
        "graph_callback_mgr_impl.h",
        "node_vertex.h",
        "rete_err.h",
        "rete_meta_store.h",
        "rete_session.cc",
        "rete_session.h",
        "rete_meta_store_factory.h",
        "rete_types.h",
        "rete_types_impl.h",
        "@com_google_absl//absl/hash",
    ],
    visibility = ["//visibility:public"],
    deps = [
        "//jets/rdf:jets_rdf",
        "@com_github_gflags_gflags//:gflags",
        "@com_github_google_glog//:glog",
        "@com_google_absl//absl/hash",
        "@com_google_absl//absl/container:flat_hash_set",
        "@com_github_rockwotj_sqlite_bazel//:sqlite3",
        ":libpqxx"
        # "@libpqxx//:lib",
    ],
    # linkopts = ["-lpqxx"],
    # includes = ["@libpqxx//:include"],
    copts = [
        "-Iexternal/gtest/googletest-release-1.10.0/include",
        "-Iexternal/gtest/googletest-release-1.10.0"
    ],
)

# cc_binary(
#     name = "libjets_rete.so",
#     srcs = [
#         "node_vertex.h",
#         "beta_relation.h",
#         "beta_relation.cc",
#         "beta_row_initializer.h",
#         "beta_row.h",
#         # "beta_relation_iterator.h",
#         "@com_google_absl//absl/hash",
#     ],
#     linkshared = 1,  ## important
#     linkstatic = 1,
#     deps = [
#         "//jets/rdf:jets_rdf",
#         "@com_github_gflags_gflags//:gflags",
#         "@com_github_google_glog//:glog",
#         "@com_google_absl//absl/hash",
#     ],
#     copts = [
#         "-Iexternal/gtest/googletest-release-1.10.0/include",
#         "-Iexternal/gtest/googletest-release-1.10.0"
#     ],
# )

cc_test(
    name = "jets_rete_test",
    size = "small",
    srcs = [
        "beta_row_initializer_test.cc", 
        "beta_row_test.cc", 
        "expr_test.cc", 
        "node_vertex_test.cc", 
        "rete_meta_store_test.cc", 
        "rete_session_test.cc", 
    ],
    deps = [
        "//jets/rdf:jets_rdf",
        ":jets_rete",
        "@com_google_googletest//:gtest_main",
        "@com_google_absl//absl/container:flat_hash_set",
    ],
)

cc_test(
    name = "rete_meta_store_factory_test",
    size = "small",
    srcs = [
        "rete_meta_store_factory_test.cc", 
    ],
    data = [":rete_meta_store_test.db"],
    deps = [
        "//jets/rdf:jets_rdf",
        ":jets_rete",
        "@com_google_googletest//:gtest_main",
        "@com_google_absl//absl/container:flat_hash_set",
    ],
)

cc_test(
    name = "jets_rete_benchmark",
    size = "small",
    srcs = [
        "beta_row_benchmark.cc", 
        "expr_benchmark.cc", 
    ],
    deps = [
        "//jets/rdf:jets_rdf",
        ":jets_rete",
        "@com_github_google_benchmark//:benchmark_main",
    ],
)
