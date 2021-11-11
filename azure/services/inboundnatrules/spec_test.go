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

package inboundnatrules

import (
	"testing"

	. "github.com/onsi/gomega"
)

func TestGetAvailablePort(t *testing.T) {
	testcases := []struct {
		name               string
		portsInput         map[int32]struct{}
		expectedError      string
		expectedPortResult int32
	}{
		{
			name:               "Empty ports",
			portsInput:         map[int32]struct{}{},
			expectedError:      "",
			expectedPortResult: 22,
		},
		{
			name: "22 taken",
			portsInput: map[int32]struct{}{
				22: {},
			},
			expectedError:      "",
			expectedPortResult: 2201,
		},
		{
			name: "Existing ports",
			portsInput: map[int32]struct{}{
				22:   {},
				2201: {},
				2202: {},
				2204: {},
			},
			expectedError:      "",
			expectedPortResult: 2203,
		},
		{
			name:               "No ports available",
			portsInput:         getFullPortsMap(),
			expectedError:      "No available SSH Frontend ports",
			expectedPortResult: 0,
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			t.Parallel()

			res, err := getAvailablePort(tc.portsInput)
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(res).To(Equal(tc.expectedPortResult))
			}
		})
	}
}

func getFullPortsMap() map[int32]struct{} {
	res := map[int32]struct{}{
		22: {},
	}
	for i := 2201; i < 2220; i++ {
		res[int32(i)] = struct{}{}
	}
	return res
}
