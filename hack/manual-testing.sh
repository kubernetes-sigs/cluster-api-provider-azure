#!/bin/bash
# Copyright 2018 The Kubernetes Authors.
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

if [ -z "${REGISTRY}" ] || [ -z "${MANAGER_IMAGE_TAG}" ]; then
    echo "please set environment variables REGISTRY and MANAGER_IMAGE_TAG"
    exit 1
fi

echo "================ DOCKER BUILD ==============="
make docker-build
echo "================ DOCKER PUSH ==============="
make docker-push
 
echo "================ MAKE CLEAN ==============="
make clean
 
echo "================ MAKE MANIFESTS ==============="
make generate-manifests
echo "================ MAKE BINARIES ==============="
make binaries
echo "================ KIND RESET ==============="
make kind-reset
 
echo "================ CREATE CLUSTER ==============="
make create-cluster
