cc_library(
    name = "jets_rete",
    srcs = [
        "alpha_functors.h",
        "alpha_node_impl.h",
        "alpha_node.h",
        "beta_relation.h",
        "beta_row_initializer.h",
        "beta_row_iterator.h",
        "beta_row.h",
        "expr_impl.h",
        "expr_operators.h",
        "expr.h",
        "graph_callback_mgr_impl.h",
        "node_vertex.h",
        "rete_err.h",
        "rete_meta_store.h",
        "rete_session.cc",
        "rete_session.h",
        "rete_types.h",
        "@com_google_absl//absl/hash",
    ],
    visibility = ["//visibility:public"],
    deps = [
        "//jets/rdf:jets_rdf",
        "@com_github_gflags_gflags//:gflags",
        "@com_github_google_glog//:glog",
        "@com_google_absl//absl/hash",
        "@com_google_absl//absl/container:flat_hash_set",
    ],
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
        "beta_relation_test.cc", 
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
