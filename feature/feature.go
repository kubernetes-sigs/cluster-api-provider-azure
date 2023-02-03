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

package feature

import (
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"
)

const (
	// nolint:godot
	// Every capz-specific feature gate should add a method here following this template:
	//
	// // MyFeature is the feature gate for my feature.
	// // owner: @username
	// // alpha: v1.X
	// MyFeature featuregate.Feature = "MyFeature"

	// AKS is the feature gate for AKS managed clusters.
	// owner: @alexeldeib
	// alpha: v0.4
	AKS featuregate.Feature = "AKS"

	// Kubelogin is the feature gate for converting kube config data to non-interactive mode using kubelogin.
	// owner: @karthikbalasub
	// alpha: v1.3
	Kubelogin featuregate.Feature = "Kubelogin"
)

func init() {
	runtime.Must(MutableGates.Add(defaultCAPZFeatureGates))
}

// defaultCAPZFeatureGates consists of all known capz-specific feature keys.
// To add a new feature, define a key for it above and add it here.
var defaultCAPZFeatureGates = map[featuregate.Feature]featuregate.FeatureSpec{
	// Every feature should be initiated here:
	AKS:       {Default: false, PreRelease: featuregate.Alpha},
	Kubelogin: {Default: false, PreRelease: featuregate.Alpha},
}
