//go:build !race

/*
Copyright 2026 The Kubernetes Authors.

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

package v1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	crconversion "sigs.k8s.io/controller-runtime/pkg/conversion"
	"sigs.k8s.io/randfill"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta2"
)

var testScheme = func() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = AddToScheme(s)
	_ = infrav1.AddToScheme(s)
	return s
}()

func TestFuzzyConversion(t *testing.T) {
	tests := []struct {
		name  string
		hub   crconversion.Hub
		spoke crconversion.Convertible
	}{
		{
			name:  "AzureASOManagedCluster",
			hub:   &infrav1.AzureASOManagedCluster{},
			spoke: &AzureASOManagedCluster{},
		},
		{
			name:  "AzureASOManagedClusterTemplate",
			hub:   &infrav1.AzureASOManagedClusterTemplate{},
			spoke: &AzureASOManagedClusterTemplate{},
		},
		{
			name:  "AzureASOManagedControlPlane",
			hub:   &infrav1.AzureASOManagedControlPlane{},
			spoke: &AzureASOManagedControlPlane{},
		},
		{
			name:  "AzureASOManagedControlPlaneTemplate",
			hub:   &infrav1.AzureASOManagedControlPlaneTemplate{},
			spoke: &AzureASOManagedControlPlaneTemplate{},
		},
		{
			name:  "AzureASOManagedMachinePool",
			hub:   &infrav1.AzureASOManagedMachinePool{},
			spoke: &AzureASOManagedMachinePool{},
		},
		{
			name:  "AzureASOManagedMachinePoolTemplate",
			hub:   &infrav1.AzureASOManagedMachinePoolTemplate{},
			spoke: &AzureASOManagedMachinePoolTemplate{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
			Scheme:      testScheme,
			Hub:         tt.hub,
			Spoke:       tt.spoke,
			FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
		}))
	}
}

func fuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		func(in *infrav1.AzureASOManagedClusterStatus, c randfill.Continue) {
			c.FillNoCustom(in)
		},
		func(in *infrav1.AzureASOManagedControlPlaneStatus, c randfill.Continue) {
			c.FillNoCustom(in)
		},
		func(in *infrav1.AzureASOManagedMachinePoolStatus, c randfill.Continue) {
			c.FillNoCustom(in)
		},
	}
}
