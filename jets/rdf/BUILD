cc_library(
    name = "jets_rdf",
    srcs = [
        "@com_google_absl//absl/hash",
        "base_graph_iterator.h",
        "base_graph.h",
        "containers_type.h",
        "graph_callback_mgr.h",
        "other/fcmp.c",
        "other/fcmp.h",
        "r_manager.cc",
        "r_manager.h",
        "rdf_ast.h",
        "rdf_err.h",
        "rdf_graph.h",
        "rdf_session_iterator.h",
        "rdf_session.h",
        "rdf_types.h",
        "uuid.cc",
        "uuid.h",
        "w_node.h",
    ],
    visibility = ["//visibility:public"],
    deps = [
        "@com_github_gflags_gflags//:gflags",
        "@com_github_google_glog//:glog",
        "@com_google_absl//absl/hash",
    ],
    copts = [
        "-Iexternal/gtest/googletest-release-1.10.0/include",
        "-Iexternal/gtest/googletest-release-1.10.0"
    ],
)

# cc_binary(
#     name = "libjets_rdf.so",
#     srcs = [
#         "rdf_err.h",
#         "base_graph.h",
#         "rdf_graph.h",
#         "rdf_graph.cc",
#         "rdf_session.h",
#         "rdf_session_iterator.h",
#         "r_manager.h",
#         "r_manager.cc", 
#         "base_graph_iterator.h",
#         "rdf_ast.h",
#         "w_node.h",
#         "rdf_types.h",
#         "@com_google_absl//absl/hash",
#     ],
#     linkshared = 1,  ## important
#     linkstatic = 1,
#     deps = [
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
    name = "jets_rdf_test",
    size = "small",
    srcs = [
        "base_graph_test.cc", 
        "rdf_ast_test.cc",
        "r_manager_test.cc",
        "rdf_graph_test.cc",
        "rdf_session_test.cc",
    ],
    deps = [
        ":jets_rdf",
        "@com_google_googletest//:gtest_main",
        "@com_google_absl//absl/hash",
    ],
)

cc_test(
    name = "jets_rdf_benchmark",
    size = "small",
    srcs = [
        "rdf_ast_benchmark.cc", 
    ],
    deps = [
        ":jets_rdf",
        "@com_github_google_benchmark//:benchmark_main",
    ],
)
