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

package virtualmachineimages

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"go.uber.org/mock/gomock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/virtualmachineimages/mock_virtualmachineimages"
)

func TestCacheGet(t *testing.T) {
	cases := map[string]struct {
		location      string
		publisher     string
		offer         string
		sku           string
		have          armcompute.VirtualMachineImagesClientListResponse
		expectedError error
	}{
		"should find": {
			location: "test", publisher: "foo", offer: "bar", sku: "baz",
			have: armcompute.VirtualMachineImagesClientListResponse{
				VirtualMachineImageResourceArray: []*armcompute.VirtualMachineImageResource{
					{Name: ptr.To("foo")},
				},
			},
			expectedError: nil,
		},
		"should not find": {
			location: "test", publisher: "foo", offer: "bar", sku: "baz",
			have: armcompute.VirtualMachineImagesClientListResponse{
				VirtualMachineImageResourceArray: []*armcompute.VirtualMachineImageResource{},
			},
			expectedError: errors.New("failed to refresh VM images cache"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			mockClient := mock_virtualmachineimages.NewMockClient(mockCtrl)
			mockClient.EXPECT().List(gomock.Any(), tc.location, tc.publisher, tc.offer, tc.sku).Return(tc.have, tc.expectedError)
			c := &Cache{client: mockClient}

			g := NewWithT(t)
			val, err := c.Get(context.Background(), tc.location, tc.publisher, tc.offer, tc.sku)
			if tc.expectedError != nil {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tc.expectedError.Error()))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(val).To(Equal(tc.have))
			}
		})
	}
}
