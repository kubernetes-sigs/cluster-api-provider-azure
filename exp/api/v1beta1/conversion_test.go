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
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/randfill"

	infrav1beta1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta2"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta2"
)

// Test is disabled when the race detector is enabled (via "//go:build !race"
// above) because the fuzz tests run 10 000 iterations and would time out.

var expTestScheme = func() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = infrav1beta1.AddToScheme(s)
	_ = infrav1.AddToScheme(s)
	_ = AddToScheme(s)
	_ = infrav1exp.AddToScheme(s)
	return s
}()

func TestFuzzyConversion(t *testing.T) {
	t.Run("for AzureMachinePool", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      expTestScheme,
		Hub:         &infrav1exp.AzureMachinePool{},
		Spoke:       &AzureMachinePool{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{expFuzzFuncs},
	}))

	t.Run("for AzureMachinePoolMachine", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      expTestScheme,
		Hub:         &infrav1exp.AzureMachinePoolMachine{},
		Spoke:       &AzureMachinePoolMachine{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{expFuzzFuncs},
	}))
}

func expFuzzFuncs(_ runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		hubAzureMachinePoolStatus,
		hubAzureMachinePoolMachineStatus,
		expSpokeCondition,
	}
}

func hubAzureMachinePoolStatus(in *infrav1exp.AzureMachinePoolStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	in.Conditions = nil
	in.Initialization = infrav1exp.AzureMachinePoolInitializationStatus{}
	in.Deprecated = nil
}

func hubAzureMachinePoolMachineStatus(in *infrav1exp.AzureMachinePoolMachineStatus, c randfill.Continue) {
	c.FillNoCustom(in)
	in.Conditions = nil
	in.Initialization = infrav1exp.AzureMachinePoolMachineInitializationStatus{}
	in.Deprecated = nil
}

// expSpokeCondition ensures v1beta1 conditions have non-empty Type so they
// survive the hasNonEmptyConditions gate in spoke→hub conversion.
func expSpokeCondition(in *clusterv1beta1.Condition, c randfill.Continue) {
	c.FillNoCustom(in)
	if in.Type == "" {
		in.Type = clusterv1beta1.ConditionType(fmt.Sprintf("FuzzCondition_%s", c.String(10)))
	}
	in.Status = corev1.ConditionStatus([]corev1.ConditionStatus{
		corev1.ConditionTrue,
		corev1.ConditionFalse,
		corev1.ConditionUnknown,
	}[c.Intn(3)])
}
