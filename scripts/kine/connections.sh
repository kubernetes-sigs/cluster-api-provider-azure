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

capz::kine::connection_defaults() {
  local headroom=64 idle_default=128 value
  export KINE_POSTGRES_MAX_CONNECTIONS="${KINE_POSTGRES_MAX_CONNECTIONS:-1024}"
  if [[ ! "${KINE_POSTGRES_MAX_CONNECTIONS}" =~ ^[1-9][0-9]{0,6}$ ]]; then
    echo "KINE_POSTGRES_MAX_CONNECTIONS must be a positive integer (at most 7 digits)." >&2
    return 1
  fi
  if (( KINE_POSTGRES_MAX_CONNECTIONS <= headroom )); then
    echo "PostgreSQL max_connections must exceed the ${headroom}-connection headroom." >&2
    return 1
  fi
  export KINE_DATASTORE_MAX_OPEN_CONNECTIONS="${KINE_DATASTORE_MAX_OPEN_CONNECTIONS:-$((KINE_POSTGRES_MAX_CONNECTIONS - headroom))}"
  if [[ ! "${KINE_DATASTORE_MAX_OPEN_CONNECTIONS}" =~ ^[1-9][0-9]{0,6}$ ]]; then
    echo "KINE_DATASTORE_MAX_OPEN_CONNECTIONS must be a positive integer; an unlimited pool is not supported." >&2
    return 1
  fi
  if (( KINE_DATASTORE_MAX_OPEN_CONNECTIONS < idle_default )); then
    idle_default="${KINE_DATASTORE_MAX_OPEN_CONNECTIONS}"
  fi
  export KINE_DATASTORE_MAX_IDLE_CONNECTIONS="${KINE_DATASTORE_MAX_IDLE_CONNECTIONS:-${idle_default}}"
  value="${KINE_DATASTORE_MAX_IDLE_CONNECTIONS}"
  if [[ ! "${value}" =~ ^[1-9][0-9]{0,6}$ ]]; then
    echo "KINE_DATASTORE_MAX_IDLE_CONNECTIONS must be a positive integer (at most 7 digits)." >&2
    return 1
  fi
  if (( KINE_DATASTORE_MAX_OPEN_CONNECTIONS > KINE_POSTGRES_MAX_CONNECTIONS - headroom ||
        KINE_DATASTORE_MAX_IDLE_CONNECTIONS > KINE_DATASTORE_MAX_OPEN_CONNECTIONS )); then
    echo "Kine's pool must leave at least ${headroom} PostgreSQL connections outside its budget, with max-idle <= max-open." >&2
    return 1
  fi
}

capz::kine::verify_postgres_connections() {
  local actual actual_max usable
  actual=$(runuser -u postgres -- psql -v ON_ERROR_STOP=1 -At -F '|' -c \
    "SELECT current_setting('max_connections')::int,
            current_setting('max_connections')::int - current_setting('superuser_reserved_connections')::int - current_setting('reserved_connections')::int") ||
    { echo "Failed to read PostgreSQL connection settings." >&2; return 1; }
  if [[ ! "${actual}" =~ ^([1-9][0-9]*)\|([0-9]+)$ ]]; then
    echo "Unexpected PostgreSQL connection settings: ${actual}" >&2
    return 1
  fi
  actual_max="${BASH_REMATCH[1]}"
  usable="${BASH_REMATCH[2]}"
  if [[ "${actual_max}" != "${KINE_POSTGRES_MAX_CONNECTIONS}" ]] ||
    (( usable > actual_max || usable <= KINE_DATASTORE_MAX_OPEN_CONNECTIONS )); then
    echo "PostgreSQL connection budget mismatch: expected max=${KINE_POSTGRES_MAX_CONNECTIONS}, observed max=${actual_max}, non-reserved=${usable}, Kine max-open=${KINE_DATASTORE_MAX_OPEN_CONNECTIONS}." >&2
    return 1
  fi
  printf 'PostgreSQL max_connections=%s, non-reserved=%s; Kine max-open=%s, max-idle=%s\n' \
    "${actual_max}" "${usable}" "${KINE_DATASTORE_MAX_OPEN_CONNECTIONS}" "${KINE_DATASTORE_MAX_IDLE_CONNECTIONS}"
}
