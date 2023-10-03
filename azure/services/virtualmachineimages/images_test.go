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
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/mock_azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/virtualmachineimages/mock_virtualmachineimages"
)

func TestGetDefaultUbuntuImage(t *testing.T) {
	tests := []struct {
		k8sVersion      string
		expectedSKU     string
		expectedVersion string
		versions        armcompute.VirtualMachineImagesClientListResponse
	}{
		{
			k8sVersion:      "v1.15.6",
			expectedSKU:     "k8s-1dot15dot6-ubuntu-1804",
			expectedVersion: "latest",
		},
		{
			k8sVersion:      "v1.17.11",
			expectedSKU:     "k8s-1dot17dot11-ubuntu-1804",
			expectedVersion: "latest",
		},
		{
			k8sVersion:      "v1.18.19",
			expectedSKU:     "k8s-1dot18dot19-ubuntu-1804",
			expectedVersion: "latest",
		},
		{
			k8sVersion:      "v1.18.20",
			expectedSKU:     "k8s-1dot18dot20-ubuntu-2004",
			expectedVersion: "latest",
		},
		{
			k8sVersion:      "v1.19.11",
			expectedSKU:     "k8s-1dot19dot11-ubuntu-1804",
			expectedVersion: "latest",
		},
		{
			k8sVersion:      "v1.19.12",
			expectedSKU:     "k8s-1dot19dot12-ubuntu-2004",
			expectedVersion: "latest",
		},
		{
			k8sVersion:      "v1.21.1",
			expectedSKU:     "k8s-1dot21dot1-ubuntu-1804",
			expectedVersion: "latest",
		},
		{
			k8sVersion:      "v1.21.2",
			expectedSKU:     "k8s-1dot21dot2-ubuntu-2004",
			expectedVersion: "latest",
		},
		{
			k8sVersion:      "v1.21.12",
			expectedSKU:     "k8s-1dot21dot12-ubuntu-2004",
			expectedVersion: "latest",
		},
		{
			k8sVersion:      "v1.21.13",
			expectedSKU:     "ubuntu-2004-gen1",
			expectedVersion: "121.13.20220613",
			versions: armcompute.VirtualMachineImagesClientListResponse{
				VirtualMachineImageResourceArray: []*armcompute.VirtualMachineImageResource{
					{Name: ptr.To("121.13.20220526")},
					{Name: ptr.To("121.13.20220613")},
					{Name: ptr.To("121.13.20220524")},
				},
			},
		},
		{
			k8sVersion:      "v1.22.0",
			expectedSKU:     "k8s-1dot22dot0-ubuntu-2004",
			expectedVersion: "latest",
		},
		{
			k8sVersion:      "v1.22.9",
			expectedSKU:     "k8s-1dot22dot9-ubuntu-2004",
			expectedVersion: "latest",
		},
		{
			k8sVersion:      "v1.22.10",
			expectedSKU:     "ubuntu-2004-gen1",
			expectedVersion: "122.10.20220613",
			versions: armcompute.VirtualMachineImagesClientListResponse{
				VirtualMachineImageResourceArray: []*armcompute.VirtualMachineImageResource{
					{Name: ptr.To("122.10.20220524")},
					{Name: ptr.To("122.10.20220613")},
				},
			},
		},
		{
			k8sVersion:      "v1.22.16",
			expectedSKU:     "ubuntu-2004-gen1",
			expectedVersion: "122.16.20221117",
			versions: armcompute.VirtualMachineImagesClientListResponse{
				VirtualMachineImageResourceArray: []*armcompute.VirtualMachineImageResource{
					{Name: ptr.To("122.16.20221117")},
				},
			},
		},
		{
			k8sVersion:      "v1.23.6",
			expectedSKU:     "k8s-1dot23dot6-ubuntu-2004",
			expectedVersion: "latest",
		},
		{
			k8sVersion:      "v1.23.7",
			expectedSKU:     "ubuntu-2004-gen1",
			expectedVersion: "123.7.20231231",
			versions: armcompute.VirtualMachineImagesClientListResponse{
				VirtualMachineImageResourceArray: []*armcompute.VirtualMachineImageResource{
					{Name: ptr.To("123.7.20221124")},
					{Name: ptr.To("123.7.20220524")},
					{Name: ptr.To("123.7.20231231")},
					{Name: ptr.To("123.7.20220818")},
				},
			},
		},
		{
			k8sVersion:      "v1.24.0",
			expectedSKU:     "ubuntu-2004-gen1",
			expectedVersion: "124.0.20220512",
			versions: armcompute.VirtualMachineImagesClientListResponse{
				VirtualMachineImageResourceArray: []*armcompute.VirtualMachineImageResource{
					{Name: ptr.To("124.0.20220512")},
				},
			},
		},
		{
			k8sVersion:      "v1.23.12",
			expectedSKU:     "ubuntu-2004-gen1",
			expectedVersion: "123.12.20220921",
			versions: armcompute.VirtualMachineImagesClientListResponse{
				VirtualMachineImageResourceArray: []*armcompute.VirtualMachineImageResource{
					{Name: ptr.To("123.12.20220921")},
				},
			},
		},
		{
			k8sVersion:      "v1.23.13",
			expectedSKU:     "ubuntu-2004-gen1",
			expectedVersion: "123.13.20221014",
			versions: armcompute.VirtualMachineImagesClientListResponse{
				VirtualMachineImageResourceArray: []*armcompute.VirtualMachineImageResource{
					{Name: ptr.To("123.13.20221014")},
				},
			},
		},
		{
			k8sVersion:      "v1.24.6",
			expectedSKU:     "ubuntu-2004-gen1",
			expectedVersion: "124.6.20220921",
			versions: armcompute.VirtualMachineImagesClientListResponse{
				VirtualMachineImageResourceArray: []*armcompute.VirtualMachineImageResource{
					{Name: ptr.To("124.6.20220921")},
				},
			},
		},
		{
			k8sVersion:      "v1.24.7",
			expectedSKU:     "ubuntu-2004-gen1",
			expectedVersion: "124.7.20221014",
			versions: armcompute.VirtualMachineImagesClientListResponse{
				VirtualMachineImageResourceArray: []*armcompute.VirtualMachineImageResource{
					{Name: ptr.To("124.7.20221014")},
				},
			},
		},
		{
			k8sVersion:      "v1.25.2",
			expectedSKU:     "ubuntu-2004-gen1",
			expectedVersion: "125.2.20220921",
			versions: armcompute.VirtualMachineImagesClientListResponse{
				VirtualMachineImageResourceArray: []*armcompute.VirtualMachineImageResource{
					{Name: ptr.To("125.2.20220921")},
				},
			},
		},
		{
			k8sVersion:      "v1.25.3",
			expectedSKU:     "ubuntu-2204-gen1",
			expectedVersion: "125.3.20221014",
			versions: armcompute.VirtualMachineImagesClientListResponse{
				VirtualMachineImageResourceArray: []*armcompute.VirtualMachineImageResource{
					{Name: ptr.To("125.3.20221014")},
				},
			},
		},
	}

	location := "westus3"
	for _, test := range tests {
		test := test
		t.Run(test.k8sVersion, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			mockAuth := mock_azure.NewMockAuthorizer(mockCtrl)
			mockAuth.EXPECT().HashKey().Return(t.Name()).AnyTimes()
			mockAuth.EXPECT().SubscriptionID().AnyTimes()
			mockAuth.EXPECT().CloudEnvironment().AnyTimes()
			mockAuth.EXPECT().Token().Return(&azidentity.DefaultAzureCredential{}).AnyTimes()
			mockClient := mock_virtualmachineimages.NewMockClient(mockCtrl)
			svc := Service{Client: mockClient, Authorizer: mockAuth}

			if test.versions.VirtualMachineImageResourceArray != nil {
				mockClient.EXPECT().
					List(gomock.Any(), location, azure.DefaultImagePublisherID, azure.DefaultImageOfferID, gomock.Any()).
					Return(test.versions, nil)
			}
			image, err := svc.GetDefaultUbuntuImage(context.TODO(), location, test.k8sVersion)

			g := NewWithT(t)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(image.Marketplace.Version).To(Equal(test.expectedVersion))
			g.Expect(image.Marketplace.SKU).To(Equal(test.expectedSKU))
		})
	}
}

func TestGetDefaultWindowsImage(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	mockClient := mock_virtualmachineimages.NewMockClient(mockCtrl)
	svc := Service{Client: mockClient}

	var tests = []struct {
		name        string
		k8sVersion  string
		runtime     string
		osVersion   string
		expectedSKU string
		expectedErr string
	}{
		{
			name:        "no k8sVersion",
			k8sVersion:  "1.1.1.1.1.1",
			runtime:     "",
			osVersion:   "",
			expectedSKU: "",
			expectedErr: "unable to parse Kubernetes version \"1.1.1.1.1.1\": Invalid character(s) found in patch number \"1.1.1.1\"",
		},
		{
			name:        "1.21.* - default runtime - default osVersion",
			k8sVersion:  "v1.21.4",
			runtime:     "",
			osVersion:   "",
			expectedSKU: "k8s-1dot21dot4-windows-2019",
			expectedErr: "",
		},
		{
			name:        "1.21.* - dockershim runtime - default osVersion",
			k8sVersion:  "v1.21.4",
			runtime:     "dockershim",
			osVersion:   "",
			expectedSKU: "k8s-1dot21dot4-windows-2019",
			expectedErr: "",
		},
		{
			name:        "1.21.* - containerd runtime - default osVersion",
			k8sVersion:  "v1.21.4",
			runtime:     "containerd",
			osVersion:   "",
			expectedSKU: "",
			expectedErr: "containerd image only supported in 1.22+",
		},
		{
			name:        "1.23.* - containerd runtime - default osVersion",
			k8sVersion:  "v1.23.2",
			runtime:     "containerd",
			osVersion:   "",
			expectedSKU: "k8s-1dot23dot2-windows-2019-containerd",
			expectedErr: "",
		},
		{
			name:        "1.23.* - default runtime - 2019 osVersion",
			k8sVersion:  "v1.23.2",
			runtime:     "",
			osVersion:   "windows-2019",
			expectedSKU: "k8s-1dot23dot2-windows-2019-containerd",
			expectedErr: "",
		},
		{
			name:        "1.23.* - default runtime - 2022 osVersion",
			k8sVersion:  "v1.23.2",
			runtime:     "",
			osVersion:   "windows-2022",
			expectedSKU: "k8s-1dot23dot2-windows-2022-containerd",
			expectedErr: "",
		},
		{
			name:        "1.23.* - containerd runtime - 2022 osVersion",
			k8sVersion:  "v1.23.2",
			runtime:     "containerd",
			osVersion:   "windows-2022",
			expectedSKU: "k8s-1dot23dot2-windows-2022-containerd",
			expectedErr: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			g := NewWithT(t)
			image, err := svc.GetDefaultWindowsImage(context.TODO(), "", test.k8sVersion, test.runtime, test.osVersion)
			if test.expectedErr != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(Equal(test.expectedErr))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(image.Marketplace.SKU).To(Equal(test.expectedSKU))
			}
		})
	}
}

func TestGetDefaultImageSKUID(t *testing.T) {
	var tests = []struct {
		k8sVersion      string
		osAndVersion    string
		expectedSKU     string
		expectedVersion string
		expectedError   bool
		versions        armcompute.VirtualMachineImagesClientListResponse
	}{
		{
			k8sVersion:      "v1.14.9",
			expectedSKU:     "k8s-1dot14dot9-ubuntu-1804",
			expectedVersion: "latest",
			expectedError:   false,
			osAndVersion:    "ubuntu-1804",
		},
		{
			k8sVersion:      "v1.14.10",
			expectedSKU:     "k8s-1dot14dot10-ubuntu-1804",
			expectedError:   false,
			expectedVersion: "latest",
			osAndVersion:    "ubuntu-1804",
		},
		{
			k8sVersion:      "v1.15.6",
			expectedSKU:     "k8s-1dot15dot6-ubuntu-1804",
			expectedError:   false,
			expectedVersion: "latest",
			osAndVersion:    "ubuntu-1804",
		},
		{
			k8sVersion:      "v1.15.7",
			expectedSKU:     "k8s-1dot15dot7-ubuntu-1804",
			expectedError:   false,
			expectedVersion: "latest",
			osAndVersion:    "ubuntu-1804",
		},
		{
			k8sVersion:      "v1.16.3",
			expectedSKU:     "k8s-1dot16dot3-ubuntu-1804",
			expectedError:   false,
			expectedVersion: "latest",
			osAndVersion:    "ubuntu-1804",
		},
		{
			k8sVersion:      "v1.16.4",
			expectedSKU:     "k8s-1dot16dot4-ubuntu-1804",
			expectedError:   false,
			expectedVersion: "latest",
			osAndVersion:    "ubuntu-1804",
		},
		{
			k8sVersion:      "1.12.0",
			expectedSKU:     "k8s-1dot12dot0-ubuntu-1804",
			expectedError:   false,
			expectedVersion: "latest",
			osAndVersion:    "ubuntu-1804",
		},
		{
			k8sVersion:    "1.1.notvalid.semver",
			expectedSKU:   "",
			expectedError: true,
		},
		{
			k8sVersion:      "v1.19.3",
			expectedSKU:     "k8s-1dot19dot3-windows-2019",
			expectedVersion: "latest",
			expectedError:   false,
			osAndVersion:    "windows-2019",
		},
		{
			k8sVersion:      "v1.20.8",
			expectedSKU:     "k8s-1dot20dot8-windows-2019",
			expectedVersion: "latest",
			expectedError:   false,
			osAndVersion:    "windows-2019",
		},
		{
			k8sVersion:      "v1.21.2",
			expectedSKU:     "k8s-1dot21dot2-windows-2019",
			expectedVersion: "latest",
			expectedError:   false,
			osAndVersion:    "windows-2019",
		},
		{
			k8sVersion:      "v1.21.13",
			expectedSKU:     "windows-2022-gen1",
			expectedVersion: "121.13.20300524",
			expectedError:   false,
			osAndVersion:    "windows-2022",
			versions: armcompute.VirtualMachineImagesClientListResponse{
				VirtualMachineImageResourceArray: []*armcompute.VirtualMachineImageResource{
					{Name: ptr.To("121.13.20220524")},
					{Name: ptr.To("124.0.20220512")},
					{Name: ptr.To("121.13.20220524")},
					{Name: ptr.To("123.13.20220524")},
					{Name: ptr.To("121.13.20220619")},
					{Name: ptr.To("121.13.20300524")},
					{Name: ptr.To("121.14.20220524")},
					{Name: ptr.To("121.12.20220524")},
					{Name: ptr.To("121.13.20220101")},
					{Name: ptr.To("121.13.20231231")},
					{Name: ptr.To("121.13.19991231")},
				},
			},
		},
		{
			k8sVersion:      "v1.20.8",
			expectedSKU:     "k8s-1dot20dot8-ubuntu-2004",
			expectedVersion: "latest",
			expectedError:   false,
			osAndVersion:    "ubuntu-2004",
		},
		{
			k8sVersion:      "v1.21.2",
			expectedSKU:     "k8s-1dot21dot2-ubuntu-2004",
			expectedVersion: "latest",
			expectedError:   false,
			osAndVersion:    "ubuntu-2004",
		},
		{
			k8sVersion:      "v1.22.0",
			expectedSKU:     "k8s-1dot22dot0-ubuntu-2004",
			expectedVersion: "latest",
			expectedError:   false,
			osAndVersion:    "ubuntu-2004",
		},
		{
			k8sVersion:      "v1.22.9",
			expectedSKU:     "k8s-1dot22dot9-ubuntu-2004",
			expectedVersion: "latest",
			expectedError:   false,
			osAndVersion:    "ubuntu-2004",
		},
		{
			k8sVersion:      "v1.23.12",
			expectedSKU:     "ubuntu-2004-gen1",
			expectedVersion: "123.12.20220921",
			expectedError:   false,
			osAndVersion:    "ubuntu-2004",
			versions: armcompute.VirtualMachineImagesClientListResponse{
				VirtualMachineImageResourceArray: []*armcompute.VirtualMachineImageResource{
					{Name: ptr.To("123.12.20220921")},
				},
			},
		},
		{
			k8sVersion:      "v1.23.13",
			expectedSKU:     "ubuntu-2204-gen1",
			expectedVersion: "123.13.20220524",
			expectedError:   false,
			osAndVersion:    "ubuntu-2204",
			versions: armcompute.VirtualMachineImagesClientListResponse{
				VirtualMachineImageResourceArray: []*armcompute.VirtualMachineImageResource{
					{Name: ptr.To("123.13.20220524")},
				},
			},
		},
		{
			k8sVersion:    "v1.23.13",
			expectedError: true,
			osAndVersion:  "ubuntu-2004",
			versions: armcompute.VirtualMachineImagesClientListResponse{
				VirtualMachineImageResourceArray: []*armcompute.VirtualMachineImageResource{},
			},
		},
		{
			k8sVersion:      "v1.24.0",
			expectedSKU:     "ubuntu-2004-gen1",
			expectedVersion: "124.0.20220512",
			expectedError:   false,
			osAndVersion:    "ubuntu-2004",
			versions: armcompute.VirtualMachineImagesClientListResponse{
				VirtualMachineImageResourceArray: []*armcompute.VirtualMachineImageResource{
					{Name: ptr.To("124.0.20220512")},
				},
			},
		},
		{
			k8sVersion:      "v1.24.0",
			expectedSKU:     "windows-2022-containerd-gen1",
			expectedVersion: "124.0.20220606",
			expectedError:   false,
			osAndVersion:    "windows-2022-containerd",
			versions: armcompute.VirtualMachineImagesClientListResponse{
				VirtualMachineImageResourceArray: []*armcompute.VirtualMachineImageResource{
					{Name: ptr.To("124.0.20220606")},
				},
			},
		},
		{
			k8sVersion:    "v1.24.1",
			expectedError: true,
			osAndVersion:  "windows-2022-containerd",
			versions: armcompute.VirtualMachineImagesClientListResponse{
				VirtualMachineImageResourceArray: []*armcompute.VirtualMachineImageResource{
					{Name: ptr.To("124.0.20220606")},
				},
			},
		},
		{
			k8sVersion:      "v1.25.4",
			expectedSKU:     "ubuntu-2204-gen1",
			expectedVersion: "125.4.20221011",
			expectedError:   false,
			osAndVersion:    "ubuntu-2204",
			versions: armcompute.VirtualMachineImagesClientListResponse{
				VirtualMachineImageResourceArray: []*armcompute.VirtualMachineImageResource{
					{Name: ptr.To("125.4.20221011")},
				},
			},
		},
	}

	location := "francesouth"
	for _, test := range tests {
		test := test
		t.Run(test.k8sVersion, func(t *testing.T) {
			t.Parallel()

			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			mockAuth := mock_azure.NewMockAuthorizer(mockCtrl)
			mockAuth.EXPECT().HashKey().Return(t.Name()).AnyTimes()
			mockAuth.EXPECT().SubscriptionID().AnyTimes()
			mockAuth.EXPECT().CloudEnvironment().AnyTimes()
			mockAuth.EXPECT().Token().Return(&azidentity.DefaultAzureCredential{}).AnyTimes()
			mockClient := mock_virtualmachineimages.NewMockClient(mockCtrl)
			svc := Service{Client: mockClient, Authorizer: mockAuth}

			offer := azure.DefaultImageOfferID
			if strings.HasPrefix(test.osAndVersion, "windows") {
				offer = azure.DefaultWindowsImageOfferID
			}
			if test.versions.VirtualMachineImageResourceArray != nil {
				mockClient.EXPECT().
					List(gomock.Any(), location, azure.DefaultImagePublisherID, offer, gomock.Any()).
					Return(test.versions, nil)
			}
			id, version, err := svc.getSKUAndVersion(context.TODO(), location, azure.DefaultImagePublisherID,
				offer, test.k8sVersion, test.osAndVersion)

			g := NewWithT(t)
			if test.expectedError {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			g.Expect(id).To(Equal(test.expectedSKU))
			g.Expect(version).To(Equal(test.expectedVersion))
		})
	}
}
