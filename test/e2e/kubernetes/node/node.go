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

package node

import (
	"fmt"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/windows"
	"strings"
)

func GetWindowsVersion(clientset *kubernetes.Clientset) (windows.OSVersion, error) {
	options := v1.ListOptions{
		LabelSelector: "kubernetes.io/os=windows",
	}
	result, err := clientset.CoreV1().Nodes().List(options)
	if err != nil {
		return windows.Unknown, err
	}

	if len(result.Items) == 0 {
		return windows.Unknown, fmt.Errorf("No Windows Nodes found.")
	}

	kernalVersion := result.Items[0].Status.NodeInfo.KernelVersion
	kernalVersions := strings.Split(kernalVersion, ".")
	if len(kernalVersions) != 4 {
		return windows.Unknown, fmt.Errorf("Not a valid Windows kernal version: %s", kernalVersion)
	}

	switch kernalVersions[2] {
	case "17763":
		return windows.LTSC2019, nil
	default:
		return windows.LTSC2019, nil
	}
}
