#!/usr/bin/env bash

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

set -euo pipefail
repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
revision=08213f28be4c7672547ce47829d13f1f45cafa3a
checksum=750b1efa023a6255a5d384eba141ba6638ad7635851d463b8a12c272477d7dcb
cache="${repo_root}/_artifacts/perf-tests/${revision}"
report="${CL2_REPORT_DIR:-${repo_root}/_artifacts/clusterloader2}"
case "${1:-}" in
  ""|--prepare-only|--dry-run) ;;
  *) echo "Usage: bash scripts/kine/run-clusterloader.sh [--prepare-only|--dry-run]" >&2; exit 1 ;;
esac
[[ $# -le 1 ]] || { echo "Unexpected arguments." >&2; exit 1; }
mkdir -p "${cache}/source" "${cache}/workload" "${report}"
report=$(cd "${report}" && pwd)
if [[ -z "${1:-}" ]]; then
  # An earlier local run's snapshot must not satisfy this run's artifact checks.
  rm -f "${report}/prometheus_snapshot.tar.gz"
fi
if [[ ! -f "${cache}/source.tar.gz" ]]; then
  curl --fail --show-error --silent --location --retry 3 \
    "https://codeload.github.com/kubernetes/perf-tests/tar.gz/${revision}" -o "${cache}/source.tar.gz"
fi
printf '%s  %s\n' "${checksum}" "${cache}/source.tar.gz" | shasum -a 256 --check
tar -xzf "${cache}/source.tar.gz" -C "${cache}/source" --strip-components=1
if [[ ! -x "${cache}/clusterloader2" ]]; then
  (cd "${cache}/source/clusterloader2" && go build -o "${cache}/clusterloader2" ./cmd/clusterloader.go)
fi
cp -R "${cache}/source/clusterloader2/testing/write-throughput/." "${cache}/workload/"
# Replace etcd-specific/newer-version metrics with the Kubernetes 1.35 API metrics used by this fixture.
cp "${repo_root}/scripts/kine/clusterloader/measurements.yaml" "${cache}/workload/modules/measurements.yaml"
cp "${repo_root}/scripts/kine/clusterloader/writer.yaml" "${cache}/workload/modules/writer/deployment.yaml"
[[ "${1:-}" != --prepare-only ]] || exit 0

: "${KUBECONFIG:?Set the workload cluster kubeconfig}"
: "${KINE_CA_CERT_B64:?Set the public Kine CA certificate for the private metrics endpoint}"
export CL2_KINE_CA_CERT_B64="${KINE_CA_CERT_B64}"
[[ -f "${KUBECONFIG}" ]] || { echo "Workload kubeconfig does not exist." >&2; exit 1; }
KUBECONFIG=$(cd "$(dirname "${KUBECONFIG}")" && printf '%s/%s' "$PWD" "$(basename "${KUBECONFIG}")")
export KUBECONFIG
export CL2_PODS_BENCHMARK_POD_COUNT="${CL2_PODS_BENCHMARK_POD_COUNT:-10000}"
export CL2_PODS_WRITE_REPLICAS="${CL2_PODS_WRITE_REPLICAS:-10}"
export CL2_PODS_PATCH_QPS="${CL2_PODS_PATCH_QPS:-100}"
export CL2_PODS_WATCH_REPLICAS="${CL2_PODS_WATCH_REPLICAS:-10}"
export CL2_PODS_WATCHES_PER_REPLICA="${CL2_PODS_WATCHES_PER_REPLICA:-1}"
export CL2_PODS_BENCHMARK_DURATION="${CL2_PODS_BENCHMARK_DURATION:-60m}"
export CL2_PROMETHEUS_PVC_ENABLED=false
export CL2_PROMETHEUS_TOLERATE_MASTER=false
{
  printf 'perf-tests-revision=%s\n' "${revision}"
  printf 'pods=%s\nwriters=%s\nqps-per-writer=%s\nwatchers=%s\nwatches-per-watcher=%s\nduration=%s\n' \
    "${CL2_PODS_BENCHMARK_POD_COUNT}" "${CL2_PODS_WRITE_REPLICAS}" "${CL2_PODS_PATCH_QPS}" \
    "${CL2_PODS_WATCH_REPLICAS}" "${CL2_PODS_WATCHES_PER_REPLICA}" "${CL2_PODS_BENCHMARK_DURATION}"
} > "${report}/settings.txt"
args=()
if [[ "${1:-}" == --dry-run ]]; then args+=(--dry-run); fi
cd "${cache}/source/clusterloader2"
exec "${cache}/clusterloader2" --provider=skeleton --kubeconfig="${KUBECONFIG}" \
  --testconfig="${cache}/workload/config.yaml" --report-dir="${report}" --port=0 \
  --enable-exec-service=false \
  --enable-prometheus-server=true --tear-down-prometheus-server=true \
  --experimental-prometheus-snapshot-to-report-dir=true \
  --prometheus-scrape-apiserver-only=true --prometheus-apiserver-scrape-port=6443 \
  --prometheus-scrape-etcd=false --prometheus-scrape-kube-proxy=false \
  --prometheus-additional-monitors-path="${repo_root}/scripts/kine/clusterloader/prometheus" \
  --prometheus-scrape-kubelets=true "${args[@]}"
