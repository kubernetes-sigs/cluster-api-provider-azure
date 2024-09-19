/*
Copyright 2021 The Kubernetes Authors.

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

package system

import (
	"os"
	"testing"

	"github.com/onsi/gomega"
)

func TestGetNamespace(t *testing.T) {
	cases := []struct {
		Name         string
		PodNamespace string
		Expected     string
	}{
		{
			Name:         "env var set to custom namespace",
			PodNamespace: "capz",
			Expected:     "capz",
		},
		{
			Name:         "env var empty",
			PodNamespace: "",
			Expected:     "capz-system",
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			g := gomega.NewWithT(t)
			err := os.Setenv(NamespaceEnvVarName, c.PodNamespace) //nolint:tenv // we want to use os.Setenv here instead of t.Setenv
			g.Expect(err).NotTo(gomega.HaveOccurred())
			defer func() {
				err := os.Unsetenv(NamespaceEnvVarName)
				g.Expect(err).NotTo(gomega.HaveOccurred())
			}()
			g.Expect(GetManagerNamespace()).To(gomega.Equal(c.Expected))
		})
	}
}
