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

genrule(
    name = "example-out",
    srcs = glob(["*.yaml.template"]) + [
        "addons.yaml",
        "//cmd/manager:manager-amd64.digest",
    ],
    outs = [
        "out/addons.yaml",
        "out/cluster.yaml",
        "out/machines.yaml",
        "out/azure_manager_image_patch.yaml",
    ],
    cmd = " ".join([
        "ENVSUBST=$(location @com_github_a8m_envsubst//cmd/envsubst:envsubst)",
        # "MANAGER_IMAGE=$$(cat $(location //cmd/manager:manager-amd64.digest))",
        # TODO(EKF): This is a hack. Figure out why the above digest line doesn't work.
        "MANAGER_IMAGE_PULL_POLICY=Never",
        "MANAGER_IMAGE=bazel/cmd/manager:manager-amd64",
        "OUTPUT_DIR=$(@D)/out",
        "DIR=.",
        "$(location :generate-yaml.sh) -f",
    ]),
    tools = [
        ":generate-yaml.sh",
        "@com_github_a8m_envsubst//cmd/envsubst:envsubst",
    ],
    visibility = ["//visibility:public"],
)

# TODO(EKF/liztio/randomvariable): Extracted from config/build, needs real creds
genrule(
    name = "test-credentials",
    outs = ["out/credentials"],
    cmd = "mkdir -p out && touch $@",
    visibility = ["//visibility:public"],
)
