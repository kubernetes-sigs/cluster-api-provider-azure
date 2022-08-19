/*
Copyright 2022 The Kubernetes Authors.

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

package services

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

func TestGetRetryAfterTime(t *testing.T) {
	defaultBackoff := 1 * time.Minute
	testcases := []struct {
		name       string
		retryAfter string
		expected   time.Time
	}{
		{
			name:       "Retry-After",
			retryAfter: "100",
			expected:   time.Now().Add(time.Duration(100) * time.Second).Truncate(time.Second),
		},
		{
			name:       "No Retry-After",
			retryAfter: "",
			expected:   time.Now().Add(defaultBackoff).Truncate(time.Second),
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			t.Parallel()
			g.Expect(GetRetryAfterTime(tc.retryAfter, defaultBackoff).Truncate(time.Second)).To(Equal(tc.expected))
		})
	}
}
