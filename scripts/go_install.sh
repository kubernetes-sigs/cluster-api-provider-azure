#!/usr/bin/env bash
# Copyright 2020 The Kubernetes Authors.
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

if [ -z "${1}" ]; then
  echo "must provide module as first parameter"
  exit 1
fi

if [ -z "${2}" ]; then
  echo "must provide binary name as second parameter"
  exit 1
fi

if [ -z "${3}" ]; then
  echo "must provide version as third parameter"
  exit 1
fi

if [ -z "${GOBIN}" ]; then
  echo "GOBIN is not set. Must set GOBIN to install the bin in a specified directory."
  exit 1
fi

rm "${GOBIN}/${2}"* 2> /dev/null || true

# Ensure tools are built with the project's Go toolchain version.
# CI images may have an older Go as the default, and `go install module@version`
# uses the module's own go.mod for toolchain selection, which may not require
# the newer Go version needed to process this project's source files.
if [ -f go.mod ]; then
  toolchain=$(sed -n 's/^toolchain //p' go.mod)
  if [ -n "${toolchain}" ]; then
    export GOTOOLCHAIN="${toolchain}"
  fi
fi

# install the golang module specified as the first argument
go install -tags capztools "${1}@${3}"
mv "${GOBIN}/${2}" "${GOBIN}/${2}-${3}"
ln -sf "${GOBIN}/${2}-${3}" "${GOBIN}/${2}"
