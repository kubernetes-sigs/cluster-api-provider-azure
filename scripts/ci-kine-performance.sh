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
set +x

export KINE_PROFILE=performance
export KINE_PREFLIGHT_ONLY=false
case "${1:-}" in
  "") ;;
  --preflight-only) export KINE_PREFLIGHT_ONLY=true ;;
  *) echo "Usage: bash scripts/ci-kine-performance.sh [--preflight-only]" >&2; exit 1 ;;
esac
[[ $# -le 1 ]] || { echo "Unexpected arguments." >&2; exit 1; }
# shellcheck source=scripts/ci-kine-conformance.sh
source "$(dirname "${BASH_SOURCE[0]}")/ci-kine-conformance.sh"
capz::kine::main
