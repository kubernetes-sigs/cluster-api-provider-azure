#!/usr/bin/env bash

# Copyright 2022 The Kubernetes Authors.
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

CHART_RELEASE=${CHART_RELEASE:-1.4.0}
VISUALIZER_ROOT=$(dirname "${BASH_SOURCE[0]}")
CHART_ROOT=$VISUALIZER_ROOT/chart

# "tar" has no POSIX standard, so use only basic options and test with both BSD and GNU.
rm -rf "$CHART_ROOT"
wget -qO- https://jont828.github.io/cluster-api-visualizer/charts/cluster-api-visualizer-"$CHART_RELEASE".tgz \
  | tar xvz -C "$VISUALIZER_ROOT"
mv "$VISUALIZER_ROOT/cluster-api-visualizer" "$CHART_ROOT"
