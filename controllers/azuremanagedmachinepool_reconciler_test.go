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

package controllers

import (
	"testing"

	"github.com/onsi/gomega"
	"github.com/pkg/errors"
)

func TestIsAgentPoolVMSSNotFoundError(t *testing.T) {
	cases := []struct {
		Name     string
		Err      error
		Expected bool
	}{
		{
			Name:     "WithANotFoundError",
			Err:      NewAgentPoolVMSSNotFoundError("foo", "baz"),
			Expected: true,
		},
		{
			Name:     "WithAWrappedNotFoundError",
			Err:      errors.Wrap(NewAgentPoolVMSSNotFoundError("foo", "baz"), "boom"),
			Expected: true,
		},
		{
			Name:     "NotTheRightKindOfError",
			Err:      errors.New("foo"),
			Expected: false,
		},
		{
			Name:     "NilError",
			Err:      nil,
			Expected: false,
		},
	}

	for _, c := range cases {
		c := c
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			g.Expect(errors.Is(c.Err, NewAgentPoolVMSSNotFoundError("foo", "baz"))).To(gomega.Equal(c.Expected))
		})
	}
}
