#!/bin/bash

# Copyright 2020 The Kubernetes Authors.
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

KUBE_ROOT=$(dirname "${BASH_SOURCE[0]}")/../..
cd "${KUBE_ROOT}" || exit 1

os=$(go env GOOS)
arch=$(go env GOARCH)
mdBookVersion="v0.4.35"
genCRDAPIReferenceDocsVersion="11fe95cbdcb91e9c25446fc99e6f2cdd8cbeb91a"

# translate arch to rust's conventions (if we can)
if [[ ${arch} == "amd64" ]]; then
    arch="x86_64"
elif [[ ${arch} == "x86" ]]; then
    arch="i686"
fi

# translate os to rust's conventions (if we can)
ext="tar.gz"
cmd="tar -C /tmp -xzvf"
case ${os} in
    windows)
        target="pc-windows-msvc"
        ext="zip"
        cmd="unzip -d /tmp"
        ;;
    darwin)
        target="apple-darwin"
        arch="x86_64" # mdbook isn't packaged for arm64 darwin yet
        ;;
    linux)
        # works for linux, too
        target="unknown-${os}-gnu"
        ;;
    *)
        target="unknown-${os}"
        ;;
esac

# grab mdbook
# we hardcode linux/amd64 since rust uses a different naming scheme
echo "downloading mdBook-${mdBookVersion}-${arch}-${target}.${ext}"
set -x
curl --retry 3 -sL -o /tmp/mdbook.${ext} "https://github.com/rust-lang/mdBook/releases/download/${mdBookVersion}/mdBook-${mdBookVersion}-${arch}-${target}.${ext}"
${cmd} /tmp/mdbook.${ext}
chmod +x /tmp/mdbook

# Generate API docs
genCRDAPIReferenceDocsPath="/tmp/gen-crd-api-reference-docs-${genCRDAPIReferenceDocsVersion}"
genCRDAPIReferenceDocs="${genCRDAPIReferenceDocsPath}/gen-crd-api-reference-docs"
(
  cd /tmp
  curl --retry 3 -sL -o gen-crd-api-reference-docs.zip "https://github.com/ahmetb/gen-crd-api-reference-docs/archive/${genCRDAPIReferenceDocsVersion}.zip"
  unzip gen-crd-api-reference-docs.zip
  cd "gen-crd-api-reference-docs-${genCRDAPIReferenceDocsVersion}"
  go build .
)

${genCRDAPIReferenceDocs} -config "${genCRDAPIReferenceDocsPath}/example-config.json" -template-dir "${genCRDAPIReferenceDocsPath}/template" -api-dir ./api/v1beta1 -out-file ./docs/book/src/reference/v1beta1-api-raw.html
${genCRDAPIReferenceDocs} -config "${genCRDAPIReferenceDocsPath}/example-config.json" -template-dir "${genCRDAPIReferenceDocsPath}/template" -api-dir ./exp/api/v1beta1 -out-file ./docs/book/src/reference/v1beta1-exp-api-raw.html

# Finally build the book.
(cd docs/book && /tmp/mdbook build)
