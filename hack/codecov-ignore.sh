#!/usr/bin/env bash

# Copyright 2023 The Kubernetes Authors.
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

{
  find api/v1beta1 -regex '.*_conversion\.go' -exec echo sigs.k8s.io/cluster-api-provider-azure/{} \;
  find api/v1beta1 -regex '.*zz_generated.*\.go' -exec echo sigs.k8s.io/cluster-api-provider-azure/{} \;
  find exp/api/v1beta1 -regex '.*_conversion\.go' -exec echo sigs.k8s.io/cluster-api-provider-azure/{} \;
  find exp/api/v1beta1 -regex '.*zz_generated.*\.go' -exec echo sigs.k8s.io/cluster-api-provider-azure/{} \;
} >> codecov-ignore.txt

while read -r p || [ -n "$p" ] 
do
if [[ "${OSTYPE}" == "darwin"* ]]; then
  sed -i '' "/${p//\//\\/}/d" ./coverage.out
else
  sed -i "/${p//\//\\/}/d" ./coverage.out
fi
done < ./codecov-ignore.txt
