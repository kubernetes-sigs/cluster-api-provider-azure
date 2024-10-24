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
	"regexp"
	"strings"

	"github.com/blang/semver"
	"github.com/pkg/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

/* Reference images live in an Azure community gallery with this structure:
. Gallery: "ClusterAPI-f72ceb4f-5159-4c26-a0fe-2ea738f0d019"
├── Image Definition: "capi-ubun2-2404"
│   ├── Version: "1.30.4"
│   ├── Version: "1.30.5"
│   └── Version: "1.31.1"
└── Image Definition: "capi-win-2022-containerd"
    ├── Version: "1.30.4"
    ├── Version: "1.30.5"
    └── Version: "1.31.1"
*/

// Service provides operations on Azure VM Images.
type Service struct {
}

// New creates a VM Images service.
func New(_ azure.Authorizer) (*Service, error) {
	return &Service{}, nil
}

// GetDefaultLinuxImage returns the default image spec for Ubuntu.
func (s *Service) GetDefaultLinuxImage(ctx context.Context, _, k8sVersion string) (*infrav1.Image, error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "azure.services.virtualmachineimages.GetDefaultLinuxImage")
	defer done()

	v, err := semver.ParseTolerant(k8sVersion)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to parse Kubernetes version \"%s\"", k8sVersion)
	}

	return &infrav1.Image{
		ComputeGallery: &infrav1.AzureComputeGalleryImage{
			Gallery: azure.DefaultPublicGalleryName,
			Name:    azure.DefaultLinuxGalleryImageName,
			Version: v.String(),
		},
	}, nil
}

// GetDefaultWindowsImage returns the default image spec for Windows.
func (s *Service) GetDefaultWindowsImage(ctx context.Context, _, k8sVersion, runtime, osAndVersion string) (*infrav1.Image, error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "azure.services.virtualmachineimages.GetDefaultWindowsImage")
	defer done()

	v, err := semver.ParseTolerant(k8sVersion)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to parse Kubernetes version \"%s\"", k8sVersion)
	}

	if runtime != "" && runtime != "containerd" {
		return nil, errors.Errorf("unsupported runtime %s", runtime)
	}

	// Use the Windows edition from the osAndVersion if provided.
	imageName := azure.DefaultWindowsGalleryImageName
	if osAndVersion != "" {
		match := regexp.MustCompile(`^windows-(\d{4})$`).FindStringSubmatch(osAndVersion)
		if len(match) != 2 {
			return nil, errors.Errorf("unsupported osAndVersion %s", osAndVersion)
		}
		imageName = strings.Replace(imageName, "2019", match[1], 1)
	}

	return &infrav1.Image{
		ComputeGallery: &infrav1.AzureComputeGalleryImage{
			Gallery: azure.DefaultPublicGalleryName,
			Name:    imageName,
			Version: v.String(),
		},
	}, nil
}
