# Copyright 2019 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    sha256 = "f04d2373bcaf8aa09bccb08a98a57e721306c8f6043a2a0ee610fd6853dcde3d",
    urls = [
        "https://storage.googleapis.com/bazel-mirror/github.com/bazelbuild/rules_go/releases/download/0.18.6/rules_go-0.18.6.tar.gz",
        "https://github.com/bazelbuild/rules_go/releases/download/0.18.6/rules_go-0.18.6.tar.gz",
    ],
)

http_archive(
    name = "io_bazel_rules_docker",
    sha256 = "aed1c249d4ec8f703edddf35cbe9dfaca0b5f5ea6e4cd9e83e99f3b0d1136c3d",
    strip_prefix = "rules_docker-0.7.0",
    urls = ["https://github.com/bazelbuild/rules_docker/archive/v0.7.0.tar.gz"],
)

http_archive(
    name = "bazel_gazelle",
    sha256 = "3c681998538231a2d24d0c07ed5a7658cb72bfb5fd4bf9911157c0e9ac6a2687",
    urls = ["https://github.com/bazelbuild/bazel-gazelle/releases/download/0.17.0/bazel-gazelle-0.17.0.tar.gz"],
)

http_archive(
    name = "io_kubernetes_build",
    sha256 = "4a8384320fba401cbf21fef177aa113ed8fe35952ace98e00b796cac87ae7868",
    strip_prefix = "repo-infra-df02ded38f9506e5bbcbf21702034b4fef815f2f",
    urls = ["https://github.com/kubernetes/repo-infra/archive/df02ded38f9506e5bbcbf21702034b4fef815f2f.tar.gz"],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")

go_rules_dependencies()

go_register_toolchains(
    go_version = "1.12.6",
)

load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")

gazelle_dependencies()

load(
    "@io_bazel_rules_docker//repositories:repositories.bzl",
    container_repositories = "repositories",
)

container_repositories()

load(
    "@io_bazel_rules_docker//go:image.bzl",
    _go_image_repos = "repositories",
)

_go_image_repos()

go_repository(
    name = "com_github_golangci_golangci-lint",
    build_file_generation = "on",
    importpath = "github.com/golangci/golangci-lint",
    tag = "v1.16.0",
)

go_repository(
    name = "com_github_golang_mock",
    build_file_generation = "on",
    importpath = "github.com/golang/mock",
    tag = "v1.2.0",
)

go_repository(
    name = "io_k8s_sigs_kind",
    commit = "161151a26faf0dbe962ac9f323cc0cdebac79ba8",
    importpath = "sigs.k8s.io/kind",
)

go_repository(
    name = "io_k8s_sigs_kustomize",
    importpath = "sigs.k8s.io/kustomize",
    tag = "v1.0.11",
)

go_repository(
    name = "io_k8s_kubernetes",
    importpath = "k8s.io/kubernetes",
    tag = "v1.15.1",
)

go_repository(
    name = "com_github_a8m_envsubst",
    importpath = "github.com/a8m/envsubst",
    tag = "v1.1.0",
)

# Use a fork until https://github.com/jmhodges/bazel_gomock/pull/19
# has merged
go_repository(
    name = "bazel_gomock",
    commit = "d5cc12f6eca65d5b6b99f88b5551c37a3a47a65b",
    importpath = "github.com/jmhodges/bazel_gomock",
    remote = "https://github.com/detiber/bazel_gomock",
    vcs = "git",
)
