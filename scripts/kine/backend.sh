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

capz::kine::backend_defaults() {
  local default_backend
  case "${KINE_PROFILE:-conformance}" in
    conformance) default_backend=kine-postgres ;;
    performance) default_backend=etcd ;;
    *) echo "Unknown KINE_PROFILE: ${KINE_PROFILE}" >&2; return 1 ;;
  esac
  export KINE_BACKEND="${KINE_BACKEND:-${default_backend}}"
  case "${KINE_BACKEND}" in
    kine-postgres)
      export KINE_DATASTORE_VERSION=v0.17.2
      export KINE_METRICS_JOB=kine KINE_METRICS_PREFIX=Kine
      ;;
    etcd)
      if [[ "${KINE_PROFILE:-conformance}" != performance ]]; then
        echo "KINE_BACKEND=etcd is supported only with KINE_PROFILE=performance." >&2
        return 1
      fi
      export KINE_DATASTORE_VERSION=v3.6.6
      export KINE_METRICS_JOB=etcd KINE_METRICS_PREFIX=Etcd
      ;;
    *) echo "KINE_BACKEND must be etcd or kine-postgres." >&2; return 1 ;;
  esac
  # Matched to kubeadm v1.35.5; quota is deliberately above normal etcd guidance.
  KINE_ETCD_SHA256=887afaa4a99f22d802ccdfbe65730a5e79aa5c9ce2c8799c67e9d804c50ecedb
  KINE_ETCD_QUOTA_BYTES=34359738368
  export KINE_ETCD_SHA256 KINE_ETCD_QUOTA_BYTES
}
