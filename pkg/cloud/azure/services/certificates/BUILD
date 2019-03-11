load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "go_default_library",
    srcs = [
        "certificates.go",
        "service.go",
    ],
    importpath = "sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/certificates",
    visibility = ["//visibility:public"],
    deps = [
        "//pkg/apis/azureprovider/v1alpha1:go_default_library",
        "//pkg/cloud/azure/actuators:go_default_library",
        "//vendor/github.com/pkg/errors:go_default_library",
        "//vendor/k8s.io/client-go/tools/clientcmd/api:go_default_library",
        "//vendor/k8s.io/klog:go_default_library",
    ],
)

go_test(
    name = "go_default_test",
    srcs = ["certificates_test.go"],
    embed = [":go_default_library"],
    deps = [
        "//pkg/apis/azureprovider/v1alpha1:go_default_library",
        "//vendor/github.com/pkg/errors:go_default_library",
    ],
)
