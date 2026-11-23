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
# shellcheck source=scripts/kine/connections.sh
source /opt/kine-connections.sh
# shellcheck source=/dev/null
source /etc/kine/connections.env
: "${KINE_POSTGRES_MAX_CONNECTIONS:?}" "${KINE_DATASTORE_MAX_OPEN_CONNECTIONS:?}" "${KINE_DATASTORE_MAX_IDLE_CONNECTIONS:?}"
capz::kine::connection_defaults

# shellcheck source=scripts/kine/mount-datastore-disk.sh
source /opt/mount-datastore-disk.sh
systemctl stop postgresql
install -d -o postgres -g postgres "${mount}/16"
cp -a /var/lib/postgresql/16/main "${mount}/16/main"
memory_mb=$(awk '/^MemTotal:/ {print int($2 / 1024)}' /proc/meminfo)
cat > /etc/postgresql/16/main/conf.d/99-kine-performance.conf <<EOF
data_directory = '${mount}/16/main'
shared_buffers = '$((memory_mb / 4))MB'
effective_cache_size = '$((memory_mb * 3 / 4))MB'
max_connections = ${KINE_POSTGRES_MAX_CONNECTIONS}
max_wal_size = '32GB'
checkpoint_timeout = '15min'
checkpoint_completion_target = 0.9
wal_compression = on
autovacuum_vacuum_scale_factor = 0.02
autovacuum_analyze_scale_factor = 0.01
track_io_timing = on
track_wal_io_timing = on
fsync = on
synchronous_commit = on
full_page_writes = on
EOF
install -d /etc/systemd/system/postgresql@16-main.service.d /etc/systemd/system/kine.service.d
cat > /etc/systemd/system/postgresql@16-main.service.d/storage.conf <<EOF
[Unit]
RequiresMountsFor=${mount}
[Service]
ExecStartPre=/usr/bin/mountpoint -q ${mount}
EOF
cat > /etc/systemd/system/kine.service.d/postgresql.conf <<'EOF'
[Unit]
Requires=postgresql@16-main.service
After=postgresql@16-main.service
EOF
systemctl daemon-reload
systemctl start postgresql@16-main
capz::kine::verify_postgres_connections
actual_data=$(runuser -u postgres -- psql -v ON_ERROR_STOP=1 -Atc 'SHOW data_directory')
[[ "${actual_data}" == "${mount}/16/main" ]] ||
  { echo "PostgreSQL is not using the dedicated data disk." >&2; exit 1; }
durable=$(runuser -u postgres -- psql -v ON_ERROR_STOP=1 -Atc \
  "SELECT current_setting('fsync') = 'on' AND current_setting('synchronous_commit') = 'on' AND current_setting('full_page_writes') = 'on'")
[[ "${durable}" == t ]] || { echo "PostgreSQL durability settings are not enabled." >&2; exit 1; }
runuser -u postgres -- psql -v ON_ERROR_STOP=1 -Atc \
  "SELECT name, setting FROM pg_settings WHERE name IN ('data_directory','shared_buffers','max_connections','superuser_reserved_connections','reserved_connections','fsync','synchronous_commit','full_page_writes');"
