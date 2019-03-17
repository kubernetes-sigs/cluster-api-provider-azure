package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["features.go"],
    importmap = "sigs.k8s.io/cluster-api-provider-azure/vendor/k8s.io/kubernetes/cmd/kubeadm/app/features",
    importpath = "k8s.io/kubernetes/cmd/kubeadm/app/features",
    deps = [
        "//vendor/github.com/pkg/errors:go_default_library",
        "//vendor/k8s.io/apimachinery/pkg/util/version:go_default_library",
        "//vendor/k8s.io/apiserver/pkg/util/feature:go_default_library",
    ],
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
