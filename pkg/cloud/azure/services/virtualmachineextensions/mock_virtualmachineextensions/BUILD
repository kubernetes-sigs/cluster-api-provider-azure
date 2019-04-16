load("@io_bazel_rules_go//go:def.bzl", "go_library")
load("@bazel_gomock//:gomock.bzl", "gomock")

gomock(
    name = "mocks",
    out = "virtualmachineextensions_mock.go",
    interfaces = [
        "VirtualMachineExtensionsClientAPI",
    ],
    library = "//vendor/github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-03-01/compute/computeapi:go_default_library",
    package = "mock_virtualmachineextensions",
    visibility = ["//visibility:public"],
)

go_library(
    name = "go_default_library",
    srcs = ["virtualmachineextensions_mock.go"],
    importpath = "sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/virtualmachineextensions/mock_virtualmachineextensions",
    visibility = ["//visibility:public"],
    deps = [
        "//vendor/github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2019-03-01/compute:go_default_library",
        "//vendor/github.com/golang/mock/gomock:go_default_library",
    ],
)
