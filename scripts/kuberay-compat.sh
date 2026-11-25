#!/bin/bash

# Copyright 2026 The Kubernetes Authors.
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

###############################################################################

# Runs the upstream KubeRay e2e suites (the jobs in kuberay-project/kuberay
# .buildkite/test-e2e.yml) against the cluster in KUBECONFIG, using the released
# kuberay-operator Helm chart. Writes one JUnit file per suite to ARTIFACTS and
# exits non-zero if any suite fails.
#
# Called by the [KubeRay-Compat] e2e spec (test/e2e/azure_kuberay_compat.go). It can
# also be run directly against an existing cluster:
#   KUBECONFIG=<path> KUBERAY_COMPAT_KUBERAY_VERSION=1.7.0 \
#   KUBERAY_COMPAT_RAY_IMAGE=rayproject/ray:2.58.0 ./scripts/kuberay-compat.sh
#
# Patches in scripts/kuberay-compat-patches/v<version>/ are applied to the upstream checkout
# before the suites run; each patch header explains why it's needed. Set
# KUBERAY_COMPAT_APPLY_PATCHES=false to run the unmodified upstream tests.

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

: "${KUBECONFIG:?KUBECONFIG must point at the cluster to test}"
KUBERAY_VERSION="${KUBERAY_COMPAT_KUBERAY_VERSION:?KUBERAY_COMPAT_KUBERAY_VERSION must be set, e.g. 1.7.0}"
RAY_IMAGE="${KUBERAY_COMPAT_RAY_IMAGE:-}"
SUITES="${KUBERAY_COMPAT_SUITES:-}"
ARTIFACTS="${ARTIFACTS:-${REPO_ROOT}/_artifacts}"
LOG_DIR="${KUBERAY_COMPAT_LOG_DIR:-${ARTIFACTS}/kuberay-compat}"
KUBERAY_REPO="${KUBERAY_COMPAT_REPO:-https://github.com/kuberay-project/kuberay.git}"
KUBERAY_HELM_REPO="${KUBERAY_COMPAT_HELM_REPO:-https://ray-project.github.io/kuberay-helm/}"
APPLY_PATCHES="${KUBERAY_COMPAT_APPLY_PATCHES:-true}"
PATCH_DIR="${REPO_ROOT}/scripts/kuberay-compat-patches/v${KUBERAY_VERSION}"
OPERATOR_NAMESPACE="default"

# name|go test timeout|go test args, mirroring the jobs in .buildkite/test-e2e.yml at v1.7.x.
# Upstream runs every job on its own Kind cluster; here all suites share one cluster, so
# "raycluster" skips the TLS tests that "mtls" runs once cert-manager is installed.
ALL_SUITES=(
  "raycluster|30m|-skip TLS ./test/e2e"
  "rayjob|60m|./test/e2erayjob"
  "rayservice|30m|-skip Suspend ./test/e2erayservice"
  "rayservice-suspend|30m|-run Suspend ./test/e2erayservice"
  "autoscaler-part1|60m|./test/e2eautoscaler/raycluster_autoscaler_test.go ./test/e2eautoscaler/support.go"
  "autoscaler-part2|60m|./test/e2eautoscaler/raycluster_autoscaler_part2_test.go ./test/e2eautoscaler/support.go"
  "mtls|30m|-run TLS ./test/e2e"
  "raycronjob|40m|./test/e2eraycronjob"
)

suite_selected() {
  [[ -z "${SUITES}" || ",${SUITES}," == *",$1,"* ]]
}

selected=()
for entry in "${ALL_SUITES[@]}"; do
  if suite_selected "${entry%%|*}"; then
    selected+=("${entry}")
  fi
done
IFS=',' read -r -a requested <<<"${SUITES}"
for name in "${requested[@]}"; do
  if ! printf '%s\n' "${ALL_SUITES[@]}" | grep -q "^${name}|"; then
    echo "Unknown suite '${name}' in KUBERAY_COMPAT_SUITES; known suites: $(printf '%s\n' "${ALL_SUITES[@]}" | cut -d'|' -f1 | paste -sd, -)"
    exit 1
  fi
done

make --directory="${REPO_ROOT}" kubectl helm gotestsum
export PATH="${REPO_ROOT}/hack/tools/bin:${PATH}"

WORK_DIR="$(mktemp -d)"
mkdir -p "${ARTIFACTS}" "${LOG_DIR}"

collect_logs() {
  kubectl logs --namespace "${OPERATOR_NAMESPACE}" deployment/kuberay-operator --all-containers --tail=-1 > "${LOG_DIR}/kuberay-operator.log" 2>&1 || true
  kubectl get pods --all-namespaces -o wide > "${LOG_DIR}/pods.txt" 2>&1 || true
  rm -rf "${WORK_DIR}"
}
trap collect_logs EXIT

# Upstream tests delete their test-ns-* namespaces in t.Cleanup, which doesn't run when a suite
# hits its go test timeout or panics. Save what was left behind, then delete it so crash-looping
# Ray pods don't take capacity from the next suite.
cleanup_test_namespaces() {
  local dir=$1 leftovers ns
  leftovers=$(kubectl get namespaces -o jsonpath='{range .items[?(@.status.phase=="Active")]}{.metadata.name}{"\n"}{end}' | grep '^test-ns-' || true)
  [[ -z "${leftovers}" ]] && return 0
  for ns in ${leftovers}; do
    echo "Deleting leftover namespace ${ns}"
    {
      kubectl get rayclusters,rayjobs,rayservices,pods --namespace "${ns}" -o wide
      kubectl describe pods --namespace "${ns}"
      kubectl get events --namespace "${ns}" --sort-by=.lastTimestamp
    } > "${dir}/leftover-${ns}.txt" 2>&1 || true
  done
  # shellcheck disable=SC2086 # leftovers holds one namespace name per line
  kubectl delete namespace ${leftovers} --timeout=10m || echo "Warning: leftover namespaces were not fully deleted"
}

echo "KubeRay ${KUBERAY_VERSION}, Ray image ${RAY_IMAGE:-<upstream default>}, suites: $(printf '%s\n' "${selected[@]}" | cut -d'|' -f1 | paste -sd, -)"
kubectl version

git clone --quiet --depth 1 --branch "v${KUBERAY_VERSION}" "${KUBERAY_REPO}" "${WORK_DIR}/kuberay"
SRC="${WORK_DIR}/kuberay/ray-operator"

if [[ "${APPLY_PATCHES}" == "true" && -d "${PATCH_DIR}" ]]; then
  for patch in "${PATCH_DIR}"/*.patch; do
    [[ -e "${patch}" ]] || continue
    echo "Applying ${patch#"${REPO_ROOT}/"} to the upstream tests"
    git -C "${WORK_DIR}/kuberay" apply --verbose "${patch}"
  done
  git -C "${WORK_DIR}/kuberay" diff > "${LOG_DIR}/upstream-patches.diff"
fi

# Upstream installs cert-manager before the operator for the mTLS job.
if suite_selected mtls; then
  make --directory="${SRC}" certmanager
fi

helm upgrade --install kuberay-operator kuberay-operator \
  --repo "${KUBERAY_HELM_REPO}" \
  --version "${KUBERAY_VERSION}" \
  --namespace "${OPERATOR_NAMESPACE}" \
  --values "${WORK_DIR}/kuberay/.buildkite/values-kuberay-operator-override.yaml" \
  --wait --timeout 10m

if [[ -n "${RAY_IMAGE}" ]]; then
  export KUBERAY_TEST_RAY_IMAGE="${RAY_IMAGE}"
  ray_tag="${RAY_IMAGE##*:}"
  export KUBERAY_TEST_RAY_VERSION="${KUBERAY_COMPAT_RAY_VERSION:-${ray_tag%%-*}}"

  # Pull the multi-GB Ray image onto every node up front so the first test in each suite
  # doesn't spend its (Kind-tuned) timeouts on the image pull.
  kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kuberay-compat-prepull
  namespace: ${OPERATOR_NAMESPACE}
spec:
  selector:
    matchLabels:
      app: kuberay-compat-prepull
  template:
    metadata:
      labels:
        app: kuberay-compat-prepull
    spec:
      initContainers:
      - name: pull
        image: ${RAY_IMAGE}
        command: ["true"]
      containers:
      - name: pause
        image: registry.k8s.io/pause:3.10
EOF
  kubectl rollout status --namespace "${OPERATOR_NAMESPACE}" daemonset/kuberay-compat-prepull --timeout=20m
  kubectl delete --namespace "${OPERATOR_NAMESPACE}" daemonset/kuberay-compat-prepull
fi

failed=()
for entry in "${selected[@]}"; do
  IFS='|' read -r name timeout args <<<"${entry}"
  suite_log_dir="${LOG_DIR}/${name}"
  mkdir -p "${suite_log_dir}"
  echo "--- Running KubeRay suite ${name}: go test -timeout ${timeout} ${args}"
  # shellcheck disable=SC2086 # args holds multiple go test arguments
  if ! (cd "${SRC}" &&
    KUBERAY_TEST_OUTPUT_DIR="${suite_log_dir}" \
    KUBERAY_TEST_TIMEOUT_SHORT="${KUBERAY_TEST_TIMEOUT_SHORT:-1m}" \
    KUBERAY_TEST_TIMEOUT_MEDIUM="${KUBERAY_TEST_TIMEOUT_MEDIUM:-5m}" \
    KUBERAY_TEST_TIMEOUT_LONG="${KUBERAY_TEST_TIMEOUT_LONG:-10m}" \
    gotestsum --format testname \
      --junitfile "${ARTIFACTS}/junit.kuberay-compat.${name}.xml" \
      --jsonfile "${suite_log_dir}/gotest.json" \
      -- -count=1 -timeout "${timeout}" ${args}); then
    failed+=("${name}")
  fi
  cleanup_test_namespaces "${suite_log_dir}"
done

if [[ ${#failed[@]} -gt 0 ]]; then
  echo "KubeRay suites failed: ${failed[*]}"
  exit 1
fi
echo "All KubeRay suites passed"
