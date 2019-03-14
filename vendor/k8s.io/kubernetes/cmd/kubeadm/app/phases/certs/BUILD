package(default_visibility = ["//visibility:public"])

load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "go_default_library",
    srcs = [
        "certlist.go",
        "certs.go",
        "doc.go",
    ],
    importmap = "sigs.k8s.io/cluster-api-provider-azure/vendor/k8s.io/kubernetes/cmd/kubeadm/app/phases/certs",
    importpath = "k8s.io/kubernetes/cmd/kubeadm/app/phases/certs",
    deps = [
        "//vendor/github.com/pkg/errors:go_default_library",
        "//vendor/k8s.io/client-go/util/cert:go_default_library",
        "//vendor/k8s.io/klog:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/app/apis/kubeadm:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/app/constants:go_default_library",
        "//vendor/k8s.io/kubernetes/cmd/kubeadm/app/util/pkiutil:go_default_library",
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
    srcs = [
        ":package-srcs",
        "//cmd/kubeadm/app/phases/certs/renewal:all-srcs",
    ],
    tags = ["automanaged"],
)
