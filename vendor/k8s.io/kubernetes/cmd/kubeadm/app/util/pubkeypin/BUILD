package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["pubkeypin.go"],
    importmap = "sigs.k8s.io/cluster-api-provider-azure/vendor/k8s.io/kubernetes/cmd/kubeadm/app/util/pubkeypin",
    importpath = "k8s.io/kubernetes/cmd/kubeadm/app/util/pubkeypin",
    deps = ["//vendor/github.com/pkg/errors:go_default_library"],
)

filegroup(
    name = "package-srcs",
    srcs = glob(["**"]),
    tags = ["automanaged"],
    visibility = ["//visibility:private"],
)

filegroup(
    name = "all-srcs",
    srcs = [":package-srcs"],
    tags = ["automanaged"],
)
