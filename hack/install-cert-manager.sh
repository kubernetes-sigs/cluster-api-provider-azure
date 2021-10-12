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

TEST_RESOURCE=$(cat <<-END
apiVersion: v1
kind: Namespace
metadata:
  name: cert-manager-test
---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: test-selfsigned
  namespace: cert-manager-test
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: selfsigned-cert
  namespace: cert-manager-test
spec:
  dnsNames:
    - example.com
  secretName: selfsigned-cert-tls
  issuerRef:
    name: test-selfsigned
END
)

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd "${REPO_ROOT}" || exit 1

# Installation of kubectl
mkdir -p "${REPO_ROOT}/hack/tools/bin"
KUBECTL=$(realpath hack/tools/bin/kubectl)
make "${KUBECTL}" &>/dev/null

## Install cert manager and wait for availability
"${KUBECTL}" apply -f https://github.com/jetstack/cert-manager/releases/download/v1.5.0/cert-manager.yaml
"${KUBECTL}" wait --for=condition=Available --timeout=5m -n cert-manager deployment/cert-manager
"${KUBECTL}" wait --for=condition=Available --timeout=5m -n cert-manager deployment/cert-manager-cainjector
"${KUBECTL}" wait --for=condition=Available --timeout=5m -n cert-manager deployment/cert-manager-webhook

for _ in {1..6}; do
  (echo "$TEST_RESOURCE" | ${KUBECTL} apply -f -) && break
  sleep 15
done

"${KUBECTL}" wait --for=condition=Ready --timeout=300s -n cert-manager-test certificate/selfsigned-cert
echo "$TEST_RESOURCE" | "${KUBECTL}" delete -f -
