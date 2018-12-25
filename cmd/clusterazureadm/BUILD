load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_library(
    name = "go_default_library",
    srcs = ["main.go"],
    importpath = "sigs.k8s.io/cluster-api-provider-azure/cmd/clusterazureadm",
    visibility = ["//visibility:private"],
    deps = ["//cmd/clusterazureadm/cmd:go_default_library"],
)

go_binary(
    name = "clusterazureadm",
    embed = [":go_default_library"],
    pure = "on",
    visibility = ["//visibility:public"],
    x_defs = {
        "github.com/platform9/cluster-api-provider-azure/cmd/versioninfo.GitBranch": "{GIT_BRANCH}",
        "github.com/platform9/cluster-api-provider-azure/cmd/versioninfo.GitReleaseTag": "{GIT_RELEASE_TAG}",
        "github.com/platform9/cluster-api-provider-azure/cmd/versioninfo.GitReleaseCommit": "{GIT_RELEASE_COMMIT}",
        "github.com/platform9/cluster-api-provider-azure/cmd/versioninfo.GitCommit": "{GIT_COMMIT}",
        "github.com/platform9/cluster-api-provider-azure/cmd/versioninfo.GitTreeState": "{GIT_TREE_STATE}",
        "github.com/platform9/cluster-api-provider-azure/cmd/versioninfo.GitMajor": "{GIT_MAJOR}",
        "github.com/platform9/cluster-api-provider-azure/cmd/versioninfo.GitMinor": "{GIT_MINOR}",
    },
)
