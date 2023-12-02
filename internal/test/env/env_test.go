/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package env

import (
	"path"
	goruntime "runtime"
	"testing"

	"github.com/onsi/gomega"
)

func TestGetFilePathToCAPICRDs(t *testing.T) {
	_, filename, _, _ := goruntime.Caller(0)
	root := path.Join(path.Dir(filename), "..", "..", "..")
	g := gomega.NewWithT(t)
	g.Expect(getFilePathToCAPICRDs(root)).To(gomega.MatchRegexp(`(.+)/pkg/mod/sigs\.k8s\.io/cluster-api@v(.+)/config/crd/bases`))
}
