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

CHART_RELEASE=${CHART_RELEASE:-0.1.11}
JAEGER_ROOT=$(dirname "${BASH_SOURCE[0]}")
CHART_ROOT=$JAEGER_ROOT/chart

rm -rf "$CHART_ROOT"
# "tar" has no POSIX standard, so use only basic options and test with both BSD and GNU.
wget -qO- https://github.com/hansehe/jaeger-all-in-one/raw/master/helm/charts/jaeger-all-in-one-"$CHART_RELEASE".tgz \
    | tar xvz -C "$JAEGER_ROOT"
mv "$JAEGER_ROOT"/jaeger-all-in-one "$CHART_ROOT"
