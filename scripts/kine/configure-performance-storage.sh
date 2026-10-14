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
device=/dev/disk/azure/data/by-lun/0
mount=/var/lib/kine-postgresql
for ((attempt=0; attempt<60; attempt++)); do
  [[ -b "${device}" ]] && break
  sleep 2
done
[[ -b "${device}" ]] || { echo "Azure data disk LUN 0 is missing; refusing OS-disk fallback." >&2; exit 1; }
[[ "$(lsblk -dn -o TRAN "${device}")" == nvme ]] ||
  { echo "Data disk LUN 0 is not NVMe." >&2; exit 1; }
if lsblk -nr -o MOUNTPOINT "${device}" | grep -q '[^[:space:]]'; then
  echo "Refusing to initialize an already mounted disk." >&2
  exit 1
fi
if [[ -n "$(wipefs --noheadings --output TYPE "${device}")" ]]; then
  echo "Refusing to format a disk with existing signatures." >&2
  exit 1
fi
mkfs.ext4 -F "${device}"
install -d "${mount}"
uuid=$(blkid -s UUID -o value "${device}")
[[ -n "${uuid}" ]] || { echo "Formatted disk has no UUID." >&2; exit 1; }
printf 'UUID=%s %s ext4 defaults,noatime 0 2\n' "${uuid}" "${mount}" >> /etc/fstab
mount "${mount}"
mountpoint -q "${mount}"
systemctl stop postgresql
install -d -o postgres -g postgres "${mount}/16"
cp -a /var/lib/postgresql/16/main "${mount}/16/main"
memory_mb=$(awk '/^MemTotal:/ {print int($2 / 1024)}' /proc/meminfo)
cat > /etc/postgresql/16/main/conf.d/99-kine-performance.conf <<EOF
data_directory = '${mount}/16/main'
shared_buffers = '$((memory_mb / 4))MB'
effective_cache_size = '$((memory_mb * 3 / 4))MB'
max_connections = 200
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
actual_data=$(runuser -u postgres -- psql -v ON_ERROR_STOP=1 -Atc 'SHOW data_directory')
[[ "${actual_data}" == "${mount}/16/main" ]] ||
  { echo "PostgreSQL is not using the dedicated data disk." >&2; exit 1; }
durable=$(runuser -u postgres -- psql -v ON_ERROR_STOP=1 -Atc \
  "SELECT current_setting('fsync') = 'on' AND current_setting('synchronous_commit') = 'on' AND current_setting('full_page_writes') = 'on'")
[[ "${durable}" == t ]] || { echo "PostgreSQL durability settings are not enabled." >&2; exit 1; }
runuser -u postgres -- psql -v ON_ERROR_STOP=1 -Atc \
  "SELECT name, setting FROM pg_settings WHERE name IN ('data_directory','shared_buffers','fsync','synchronous_commit','full_page_writes');"
