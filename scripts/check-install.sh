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

set -e
set -o errexit
set -o nounset
set -o pipefail

GOPATH="$(go env GOPATH)"
WORKINGDIR="$(pwd)"
EXPECTEDDIR="${GOPATH}/src/sigs.k8s.io/cluster-api-provider-azure"

if [ "${WORKINGDIR}" != "${EXPECTEDDIR}" ]; then
    echo "------------------------------------------------------------------------------------------------------------------"
    echo "Invalid checkout directory!"
    echo
    echo "Expected: ${EXPECTEDDIR}"
    echo "Actual:   ${WORKINGDIR}"
    echo ""
    echo "This project assumes that the repository has been checked out to \$GOPATH/src/sigs.k8s.io/cluster-api-provider-azure"
    echo "------------------------------------------------------------------------------------------------------------------"
    exit 1
fi

echo "Repository is set up correctly"

exit 0
