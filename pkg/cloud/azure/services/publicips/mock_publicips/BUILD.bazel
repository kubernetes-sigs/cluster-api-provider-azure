load("@io_bazel_rules_go//go:def.bzl", "go_library")
load("@bazel_gomock//:gomock.bzl", "gomock")

gomock(
    name = "mocks",
    out = "publicips_mock.go",
    interfaces = [
        "PublicIPAddressesClientAPI",
    ],
    library = "//vendor/github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-12-01/network/networkapi:go_default_library",
    package = "mock_publicips",
    visibility = ["//visibility:public"],
)

go_library(
    name = "go_default_library",
    srcs = ["publicips_mock.go"],
    importpath = "sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/publicips/mock_publicips",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-12-01/network:go_default_library",
        "//vendor/github.com/golang/mock/gomock:go_default_library",
    ],
)
