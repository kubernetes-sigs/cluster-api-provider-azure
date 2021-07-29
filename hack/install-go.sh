#!/usr/bin/env bash

# Copyright 2021 The Kubernetes Authors.
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

echo "installing apt dependencies"
sudo apt update && sudo apt install -y curl apt-transport-https

echo "setting up work dir"
WORKDIR="$(mktemp -d)"
pushd "$WORKDIR"

trap 'rm -rf ${WORKDIR}' EXIT

GOLANG_VERSION="$(curl -sS https://golang.org/VERSION?m=text)"
echo "Downloading ${GOLANG_VERSION}"
curl -O "https://dl.google.com/go/${GOLANG_VERSION}.linux-amd64.tar.gz"

echo "unpacking go"
sudo mkdir -p /usr/local/go
sudo chown -R "$(whoami):$(whoami)" /usr/local/go 
tar -xvf "${GOLANG_VERSION}.linux-amd64.tar.gz" -C /usr/local

echo "Successfully installed go"
