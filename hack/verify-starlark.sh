#!/usr/bin/env bash
# Copyright 2019 The Kubernetes Authors.
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

SCRIPT_DIR="$(dirname "${BASH_SOURCE[0]}")"
ROOT_PATH="$(cd "${SCRIPT_DIR}"/.. && pwd)"

VERSION="v8.0.3"

MODE="check"

if [[ "$*" == "fix" ]]; then
  MODE="fix"
fi

OS="$(uname -s)"
ARCH="$(uname -m)"

# Determine OS-specific binary name
if [[ "${OS}" == "Linux" ]]; then
  BINARY="buildifier-linux"
elif [[ "${OS}" == "Darwin" ]]; then
  BINARY="buildifier-darwin"
fi

# Append architecture suffix for the appropriate binary
if [[ "${ARCH}" == "x86_64" ]] || [[ "${ARCH}" == "amd64" ]]; then
  BINARY="${BINARY}-amd64"  # No change needed, this is the default
elif [[ "${ARCH}" == "arm64" ]] || [[ "${ARCH}" == "aarch64" ]]; then
  BINARY="${BINARY}-arm64"
fi

# create a temporary directory
TMP_DIR=$(mktemp -d)
OUT="${TMP_DIR}/out.log"

# cleanup on exit
capz::verify-starlark::cleanup() {
  ret=0
  if [[ -s "${OUT}" ]]; then
    echo "Found errors:"
    cat "${OUT}"
    echo ""
    echo "run make format-tiltfile to fix the errors"
    ret=1
  fi
  echo "Cleaning up..."
  rm -rf "${TMP_DIR}"
  exit ${ret}
}
trap capz::verify-starlark::cleanup EXIT

BUILDIFIER="${SCRIPT_DIR}/tools/bin/buildifier/${VERSION}/buildifier"

if [ ! -f "$BUILDIFIER" ]; then
  # install buildifier
  cd "${TMP_DIR}" || exit
  curl --retry 3 -L "https://github.com/bazelbuild/buildtools/releases/download/${VERSION}/${BINARY}" -o "${TMP_DIR}/buildifier"
  chmod +x "${TMP_DIR}/buildifier"
  cd "${ROOT_PATH}"
  mkdir -p "$(dirname "$0")/tools/bin/buildifier/${VERSION}"
  mv "${TMP_DIR}/buildifier" "$BUILDIFIER"
fi

echo "Running buildifier..."
cd "${ROOT_PATH}" || exit
"${BUILDIFIER}" -mode=${MODE} -v Tiltfile >> "${OUT}" 2>&1
