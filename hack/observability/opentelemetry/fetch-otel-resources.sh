#!/usr/bin/env bash

# Copyright 2021 The Kubernetes Authors.
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

CHART_RELEASE=${CHART_RELEASE:-0.13.1}
OTEL_ROOT=$(dirname "${BASH_SOURCE[0]}")
CHART_ROOT=$OTEL_ROOT/chart

rm -rf "$CHART_ROOT"
# "tar" has no POSIX standard, so use only basic options and test with both BSD and GNU.
wget -qO- https://github.com/open-telemetry/opentelemetry-helm-charts/releases/download/opentelemetry-collector-"$CHART_RELEASE"/opentelemetry-collector-"$CHART_RELEASE".tgz \
  | tar xvz -C "$OTEL_ROOT" --exclude "ci" --exclude "examples" -
mv "$OTEL_ROOT"/opentelemetry-collector "$CHART_ROOT"
wget -q https://raw.githubusercontent.com/open-telemetry/opentelemetry-helm-charts/main/LICENSE -P "$CHART_ROOT"
