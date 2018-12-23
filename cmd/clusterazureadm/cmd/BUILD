load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["root.go"],
    importpath = "sigs.k8s.io/cluster-api-provider-azure/cmd/clusterazureadm/cmd",
    visibility = ["//visibility:public"],
    deps = [
        "//cmd/clusterazureadm/cmd/alpha:go_default_library",
        "//cmd/versioninfo:go_default_library",
        "//vendor/github.com/spf13/cobra:go_default_library",
        "//vendor/github.com/spf13/pflag:go_default_library",
    ],
)
