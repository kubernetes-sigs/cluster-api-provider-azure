#!/usr/bin/env bash

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

set -o errexit
set -o nounset
set -o pipefail

RESOURCES_ROOT=$(dirname "${BASH_SOURCE[0]}")/resources

curl -s https://raw.githubusercontent.com/jaegertracing/jaeger-operator/master/deploy/crds/jaegertracing.io_jaegers_crd.yaml > "${RESOURCES_ROOT}/crds.yaml"
curl -s https://raw.githubusercontent.com/jaegertracing/jaeger-operator/master/deploy/service_account.yaml > "${RESOURCES_ROOT}/service_account.yaml"
curl -s https://raw.githubusercontent.com/jaegertracing/jaeger-operator/master/deploy/operator.yaml > "${RESOURCES_ROOT}/operator.yaml"
curl -s https://raw.githubusercontent.com/jaegertracing/jaeger-operator/master/deploy/role.yaml > "${RESOURCES_ROOT}/role.yaml"
curl -s https://raw.githubusercontent.com/jaegertracing/jaeger-operator/master/deploy/role_binding.yaml > "${RESOURCES_ROOT}/role_binding.yaml"
