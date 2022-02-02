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

package reconciler_test

import (
	"testing"
	"time"

	"github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
)

func TestDefaultedTimeout(t *testing.T) {
	cases := []struct {
		Name     string
		Subject  time.Duration
		Expected time.Duration
	}{
		{
			Name:     "WithZeroValueDefaults",
			Subject:  time.Duration(0),
			Expected: reconciler.DefaultLoopTimeout,
		},
		{
			Name:     "WithRealValue",
			Subject:  2 * time.Hour,
			Expected: 2 * time.Hour,
		},
		{
			Name:     "WithNegativeValue",
			Subject:  time.Duration(-2),
			Expected: reconciler.DefaultLoopTimeout,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			g.Expect(reconciler.DefaultedLoopTimeout(c.Subject)).To(gomega.Equal(c.Expected))
		})
	}
}
