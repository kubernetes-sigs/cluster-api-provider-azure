/*
Copyright 2019 The Kubernetes Authors.

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

package groups

import (
	"context"
	"testing"

	//"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-05-01/resources"
	//"github.com/Azure/go-autorest/autorest/to"
	"github.com/golang/mock/gomock"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/mocks"
	//"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/groups/mock_groups"
)

func TestReconcile(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	testCases := []struct {
		name   string
		input  *v1alpha1.ResourceGroup
		expect func(m *mocks.MockServiceMockRecorder)
	}{
		{
			name: "managed",
			input: &v1alpha1.ResourceGroup{
				Name:    "managed-resource-group",
				Managed: "yes",
			},
			expect: func(m *mocks.MockServiceMockRecorder) {
				m.Reconcile(gomock.AssignableToTypeOf(context.Background()), gomock.AssignableToTypeOf(&v1alpha1.ResourceGroup{})).Return(nil)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			serviceMock := mocks.NewMockService(mockCtrl)

			tc.expect(serviceMock.EXPECT())

			err := serviceMock.Reconcile(context.Background(), tc.input)
			if err != nil {
				t.Fatalf("did not expect error calling a mock: %v", err)
			}
		})
	}
}

func TestDelete(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	testCases := []struct {
		name   string
		input  *v1alpha1.ResourceGroup
		expect func(m *mocks.MockServiceMockRecorder)
	}{
		{
			name: "managed",
			input: &v1alpha1.ResourceGroup{
				Name:    "managed-resource-group",
				Managed: "yes",
			},
			expect: func(m *mocks.MockServiceMockRecorder) {
				m.Delete(gomock.AssignableToTypeOf(context.Background()), gomock.AssignableToTypeOf(&v1alpha1.ResourceGroup{})).Return(nil)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			serviceMock := mocks.NewMockService(mockCtrl)

			tc.expect(serviceMock.EXPECT())

			err := serviceMock.Delete(context.Background(), tc.input)
			if err != nil {
				t.Fatalf("did not expect error calling a mock: %v", err)
			}
		})
	}
}

/*
func TestGet(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	testCases := []struct {
		name   string
		input  *v1alpha1.ResourceGroup
		expect func(m *mock_groups.MockGroupsClientAPIMockRecorder)
	}{
		{
			name: "managed",
			input: &v1alpha1.ResourceGroup{
				Name:    "managed-resource-group",
				Managed: "yes",
			},
			expect: func(m *mock_groups.MockGroupsClientAPIMockRecorder) {
				m.Get(gomock.AssignableToTypeOf(context.Background()), gomock.AssignableToTypeOf(&v1alpha1.ResourceGroup{})).Return(resources.Group{}, nil)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			groupsMock := mock_groups.NewMockGroupsClientAPI(mockCtrl)

			tc.expect(groupsMock.EXPECT())

			rg, err := groupsMock.Get(context.Background(), tc.input.Name)
			if err != nil {
				t.Fatalf("did not expect error calling a mock: %v", err)
			}

			if to.String(rg.Name) != tc.input.Name {
				t.Fatalf("could not rg")
			}
		})
	}
}
*/
