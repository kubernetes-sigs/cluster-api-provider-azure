#!/bin/bash
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

# Directories.
SOURCE_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null && pwd )"
OUTPUT_DIR=${OUTPUT_DIR:-${SOURCE_DIR}/_out}

ENV_GENERATED_FILE=${OUTPUT_DIR}/.env

# shellcheck disable=SC1090
source "${ENV_GENERATED_FILE}"

ready="false"
kubeconfigPath=$(kind get kubeconfig-path --name="clusterapi")
echo "Waiting for any machine in cluster ${CLUSTER_NAME} to be in running state..."
while [ $ready == "false" ]; do
  output=$(kubectl --kubeconfig="$kubeconfigPath" get machines -o json)
  ready=$(echo "$output" | jq -r 'any(.items[]; .status.phase=="running")')
  [ "$ready" == "false" ] && echo "Waiting..." && sleep 30
done
echo 'found a running Machine'
