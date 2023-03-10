//go:build e2e
// +build e2e

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

package windows

import (
	"fmt"
	"log"
)

type OSVersion string
type WindowsTestImages string

const (
	Unknown  = OSVersion("")
	LTSC2019 = OSVersion("2019")
)

const (
	IIS   = WindowsTestImages("IIS")
	Httpd = WindowsTestImages("httpd")
)

type WindowsImage struct {
	BaseImage string
	Tags      map[OSVersion]string
}

func (i *WindowsImage) GetImage(version OSVersion) string {
	tag, ok := i.Tags[version]
	if !ok {
		log.Printf("Warning: Tag for version %s not found for image %s", version, i.BaseImage)
	}
	return fmt.Sprintf("%s:%s", i.BaseImage, tag)
}

func GetWindowsImage(testImage WindowsTestImages, version OSVersion) string {
	httpd := WindowsImage{
		BaseImage: "registry.k8s.io/e2e-test-images/httpd",
		Tags: map[OSVersion]string{
			LTSC2019: "2.4.39-alpine",
		},
	}

	return httpd.GetImage(version)
}
