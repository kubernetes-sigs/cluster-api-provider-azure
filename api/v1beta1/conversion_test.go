//go:build !race

/*
Copyright 2025 The Kubernetes Authors.

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

package v1beta1

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta2"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
)

// Test is disabled when the race detector is enabled (via "//go:build !race"
// above) because the fuzz tests run 10 000 iterations and would time out.

var testScheme = func() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = AddToScheme(s)
	_ = infrav1.AddToScheme(s)
	return s
}()

func TestFuzzyConversion(t *testing.T) {
	t.Run("for AzureCluster", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: testScheme,
		Hub:    &infrav1.AzureCluster{},
		Spoke:  &AzureCluster{},
		HubAfterMutation: func(h conversion.Hub) {
			obj := h.(*infrav1.AzureCluster)
			sortFailureDomains(obj.Spec.FailureDomains)
			sortFailureDomains(obj.Status.FailureDomains)
		},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for AzureMachine", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      testScheme,
		Hub:         &infrav1.AzureMachine{},
		Spoke:       &AzureMachine{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for AzureClusterTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: testScheme,
		Hub:    &infrav1.AzureClusterTemplate{},
		Spoke:  &AzureClusterTemplate{},
		HubAfterMutation: func(h conversion.Hub) {
			obj := h.(*infrav1.AzureClusterTemplate)
			sortFailureDomains(obj.Spec.Template.Spec.FailureDomains)
		},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for AzureMachineTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      testScheme,
		Hub:         &infrav1.AzureMachineTemplate{},
		Spoke:       &AzureMachineTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for AzureClusterIdentity", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      testScheme,
		Hub:         &infrav1.AzureClusterIdentity{},
		Spoke:       &AzureClusterIdentity{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for AzureManagedCluster", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      testScheme,
		Hub:         &infrav1.AzureManagedCluster{},
		Spoke:       &AzureManagedCluster{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for AzureManagedControlPlane", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      testScheme,
		Hub:         &infrav1.AzureManagedControlPlane{},
		Spoke:       &AzureManagedControlPlane{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for AzureManagedMachinePool", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      testScheme,
		Hub:         &infrav1.AzureManagedMachinePool{},
		Spoke:       &AzureManagedMachinePool{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for AzureManagedClusterTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      testScheme,
		Hub:         &infrav1.AzureManagedClusterTemplate{},
		Spoke:       &AzureManagedClusterTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for AzureManagedControlPlaneTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      testScheme,
		Hub:         &infrav1.AzureManagedControlPlaneTemplate{},
		Spoke:       &AzureManagedControlPlaneTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for AzureManagedMachinePoolTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      testScheme,
		Hub:         &infrav1.AzureManagedMachinePoolTemplate{},
		Spoke:       &AzureManagedMachinePoolTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for AzureASOManagedCluster", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      testScheme,
		Hub:         &infrav1.AzureASOManagedCluster{},
		Spoke:       &AzureASOManagedCluster{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for AzureASOManagedClusterTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      testScheme,
		Hub:         &infrav1.AzureASOManagedClusterTemplate{},
		Spoke:       &AzureASOManagedClusterTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for AzureASOManagedControlPlane", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      testScheme,
		Hub:         &infrav1.AzureASOManagedControlPlane{},
		Spoke:       &AzureASOManagedControlPlane{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for AzureASOManagedControlPlaneTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      testScheme,
		Hub:         &infrav1.AzureASOManagedControlPlaneTemplate{},
		Spoke:       &AzureASOManagedControlPlaneTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for AzureASOManagedMachinePool", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      testScheme,
		Hub:         &infrav1.AzureASOManagedMachinePool{},
		Spoke:       &AzureASOManagedMachinePool{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))

	t.Run("for AzureASOManagedMachinePoolTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      testScheme,
		Hub:         &infrav1.AzureASOManagedMachinePoolTemplate{},
		Spoke:       &AzureASOManagedMachinePoolTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{fuzzFuncs},
	}))
}
