#!/bin/bash

# Copyright 2025 The Kubernetes Authors.
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

# kind:prepullAdditionalImages pre-pull all the additional (not Kindest/node) images that will be used in the e2e, thus making
# the actual test run less sensible to the network speed.
kind:prepullAdditionalImages () {
  # Pulling cert manager images so we can pre-load in kind nodes
  kind::prepullImage "quay.io/jetstack/cert-manager-cainjector:v1.19.1"
  kind::prepullImage "quay.io/jetstack/cert-manager-webhook:v1.19.1"
  kind::prepullImage "quay.io/jetstack/cert-manager-controller:v1.19.1"

  # Pull all images defined in DOCKER_PRELOAD_IMAGES.
  for IMAGE in $(grep DOCKER_PRELOAD_IMAGES: < "$E2E_CONF_FILE" | sed -E 's/.*\[(.*)\].*/\1/' | tr ',' ' '); do
    kind::prepullImage "${IMAGE}"
  done
}

# kind:prepullImage pre-pull a docker image if no already present locally.
# The result will be available in the retVal value which is accessible from the caller.
kind::prepullImage () {
  local image=$1
  image="${image//+/_}"

  retVal=0
  if [[ "$(docker images -q "$image" 2> /dev/null)" == "" ]]; then
    echo "+ Pulling $image"
    docker pull "$image" || retVal=$?
  else
    echo "+ image $image already present in the system, skipping pre-pull"
  fi
}
