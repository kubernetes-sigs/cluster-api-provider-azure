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
	"testing"

	. "github.com/onsi/gomega"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
)

func TestGetDefaultLinuxImage(t *testing.T) {
	tests := []struct {
		k8sVersion string
		expected   *infrav1.Image
		expectErr  bool
	}{
		{
			k8sVersion: "v1.31.1",
			expected: &infrav1.Image{
				ComputeGallery: &infrav1.AzureComputeGalleryImage{
					Gallery: azure.DefaultPublicGalleryName,
					Name:    azure.DefaultLinuxGalleryImageName,
					Version: "1.31.1",
				},
			},
			expectErr: false,
		},
		{
			k8sVersion: " 1.31",
			expected: &infrav1.Image{
				ComputeGallery: &infrav1.AzureComputeGalleryImage{
					Gallery: azure.DefaultPublicGalleryName,
					Name:    azure.DefaultLinuxGalleryImageName,
					Version: "1.31.0",
				},
			},
			expectErr: false,
		},
		{
			k8sVersion: "1.31.1.2",
			expectErr:  true,
		},
		{
			k8sVersion: "v1.31.1+1234  ",
			expected: &infrav1.Image{
				ComputeGallery: &infrav1.AzureComputeGalleryImage{
					Gallery: azure.DefaultPublicGalleryName,
					Name:    azure.DefaultLinuxGalleryImageName,
					Version: "1.31.1+1234",
				},
			},
			expectErr: false,
		},
		{
			k8sVersion: "1.28.12",
			expected: &infrav1.Image{
				Marketplace: &infrav1.AzureMarketplaceImage{
					ImagePlan: infrav1.ImagePlan{
						Publisher: "cncf-upstream",
						Offer:     "capi",
						SKU:       "ubuntu-2204-gen1",
					},
					Version: "128.12.20240717",
				},
			},
		},
	}

	location := "unused"
	for _, test := range tests {
		t.Run(test.k8sVersion, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)
			svc := Service{}
			image, err := svc.GetDefaultLinuxImage(t.Context(), location, test.k8sVersion)
			if test.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(image).To(Equal(test.expected))
			}
		})
	}
}

func TestGetDefaultWindowsImage(t *testing.T) {
	tests := []struct {
		k8sVersion   string
		runtime      string
		osAndVersion string
		expected     *infrav1.Image
		expectErr    bool
	}{
		{
			k8sVersion:   "v1.31.1",
			runtime:      "containerd",
			osAndVersion: "windows-2019",
			expected: &infrav1.Image{
				ComputeGallery: &infrav1.AzureComputeGalleryImage{
					Gallery: azure.DefaultPublicGalleryName,
					Name:    "capi-win-2019-containerd",
					Version: "1.31.1",
				},
			},
			expectErr: false,
		},
		{
			k8sVersion:   "v1.31.1",
			runtime:      "containerd",
			osAndVersion: "windows-2022",
			expected: &infrav1.Image{
				ComputeGallery: &infrav1.AzureComputeGalleryImage{
					Gallery: azure.DefaultPublicGalleryName,
					Name:    "capi-win-2022-containerd",
					Version: "1.31.1",
				},
			},
			expectErr: false,
		},
		{
			k8sVersion:   " 1.31",
			runtime:      "containerd",
			osAndVersion: "windows-2022",
			expected: &infrav1.Image{
				ComputeGallery: &infrav1.AzureComputeGalleryImage{
					Gallery: azure.DefaultPublicGalleryName,
					Name:    "capi-win-2022-containerd",
					Version: "1.31.0",
				},
			},
			expectErr: false,
		},
		{
			k8sVersion: "1.31.1.2",
			expectErr:  true,
		},
		{
			k8sVersion:   "v1.31.1+1234  ",
			runtime:      "containerd",
			osAndVersion: "windows-2022",
			expected: &infrav1.Image{
				ComputeGallery: &infrav1.AzureComputeGalleryImage{
					Gallery: azure.DefaultPublicGalleryName,
					Name:    "capi-win-2022-containerd",
					Version: "1.31.1+1234",
				},
			},
			expectErr: false,
		},
		{
			k8sVersion: "v1.31.1",
			runtime:    "docker",
			expectErr:  true,
		},
		{
			k8sVersion:   "v1.31.1",
			osAndVersion: "windows-abcd",
			expectErr:    true,
		},
		{
			k8sVersion: "1.31.1",
			expected: &infrav1.Image{
				ComputeGallery: &infrav1.AzureComputeGalleryImage{
					Gallery: azure.DefaultPublicGalleryName,
					Name:    azure.DefaultWindowsGalleryImageName,
					Version: "1.31.1",
				},
			},
			expectErr: false,
		},
		{
			k8sVersion: "1.28.12",
			expected: &infrav1.Image{
				Marketplace: &infrav1.AzureMarketplaceImage{
					ImagePlan: infrav1.ImagePlan{
						Publisher: "cncf-upstream",
						Offer:     "capi-windows",
						SKU:       "windows-2019-containerd-gen1",
					},
					Version: "128.12.20240717",
				},
			},
		},
	}

	location := "unused"
	for _, test := range tests {
		t.Run(test.k8sVersion, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)
			svc := Service{}
			image, err := svc.GetDefaultWindowsImage(t.Context(), location, test.k8sVersion, test.runtime, test.osAndVersion)
			if test.expectErr {
				g.Expect(err).To(HaveOccurred())
			} else {
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(image).To(Equal(test.expected))
			}
		})
	}
}
