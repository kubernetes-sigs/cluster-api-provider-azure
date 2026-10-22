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
