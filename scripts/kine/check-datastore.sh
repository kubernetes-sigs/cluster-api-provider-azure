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
case "${1:-}" in
  kine-postgres) service=kine; label=Kine; marker=KINE_READY ;;
  etcd) service=etcd; label=etcd; marker=ETCD_READY ;;
  *) echo "Specify the etcd or kine-postgres backend." >&2; exit 1 ;;
esac
export ETCDCTL_API=3
export ETCDCTL_ENDPOINTS=https://127.0.0.1:2379
export ETCDCTL_CACERT=/etc/kine/ca.crt
export ETCDCTL_CERT=/etc/kine/client.crt
export ETCDCTL_KEY=/etc/kine/client.key
echo "Checking etcdctl version:"
etcdctl version
echo "Checking ${label} service state:"
if systemctl is-active "${service}"; then
  service_status=0
else
  service_status=$?
fi
echo "${label} service check exit status: ${service_status}"
if [[ "${service_status}" != 0 ]]; then
  exit "${service_status}"
fi
key="/capz-kine-smoke/$(cat /proc/sys/kernel/random/uuid)"
# Match Kubernetes' create transaction, not arbitrary etcd transactions.
echo "Creating ${label} smoke-test key: ${key}"
if transaction_output=$(printf 'mod("%s") = "0"\n\nput %s ready\n\n\n' "$key" "$key" |
  etcdctl --command-timeout=15s txn); then
  transaction_status=0
else
  transaction_status=$?
fi
printf '%s transaction exit status: %s\n%s\n' "${label}" "${transaction_status}" "${transaction_output}"
if [[ "${transaction_status}" != 0 ]]; then
  exit "${transaction_status}"
fi
if ! grep -qx SUCCESS <<< "${transaction_output}"; then
  echo "${label} transaction did not report SUCCESS." >&2
  exit 1
fi
echo "Reading ${label} smoke-test key: ${key}"
if read_output=$(etcdctl --command-timeout=15s get "$key" --print-value-only); then
  read_status=0
else
  read_status=$?
fi
printf '%s read exit status: %s\nValue: <%s>\n' "${label}" "${read_status}" "${read_output}"
if [[ "${read_status}" != 0 ]]; then
  exit "${read_status}"
fi
if [[ "${read_output}" != ready ]]; then
  echo "${label} smoke-test value does not match expected value <ready>." >&2
  exit 1
fi
echo "${marker}"
