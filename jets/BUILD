cc_binary(
    name = "jetstore",
    srcs = [
        "jets_err.h",
        "jets_main.cc",
    ],
    linkstatic = True,
    deps = [
        "//jets/rdf:jets_rdf",
        "//jets/rete:jets_rete",
        "@com_github_gflags_gflags//:gflags",
        "@com_github_google_glog//:glog",
        "@com_google_absl//absl/hash",
    ],
)
