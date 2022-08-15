/*
Copyright 2021 The Kubernetes Authors.

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

package v1alpha3

import (
	"testing"

	fuzz "github.com/google/gofuzz"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/apitesting/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
)

func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(infrav1.AddToScheme(scheme)).To(Succeed())

	t.Run("for AzureCluster", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.AzureCluster{},
		Spoke:       &AzureCluster{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{overrideDeprecatedAndRemovedFieldsFuncs, overrideOutboundLBFunc},
	}))

	t.Run("for AzureMachine", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.AzureMachine{},
		Spoke:       &AzureMachine{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{overrideDeprecatedAndRemovedFieldsFuncs},
	}))

	t.Run("for AzureMachineTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.AzureMachineTemplate{},
		Spoke:       &AzureMachineTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{overrideDeprecatedAndRemovedFieldsFuncs},
	}))

	t.Run("for AzureClusterIdentity", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &infrav1.AzureClusterIdentity{},
		Spoke:       &AzureClusterIdentity{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{overrideDeprecatedAndRemovedFieldsFuncs},
	}))
}

func overrideDeprecatedAndRemovedFieldsFuncs(codecs runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		func(azureMachineSpec *AzureMachineSpec, c fuzz.Continue) {
			azureMachineSpec.Location = ""
		},
		func(subnetSpec *SubnetSpec, c fuzz.Continue) {
			subnetSpec.InternalLBIPAddress = ""
		},
		func(vnetSpec *VnetSpec, c fuzz.Continue) {
			vnetSpec.CidrBlock = ""
		},
		func(azureClusterIdentity *AzureClusterIdentity, c fuzz.Continue) {
			azureClusterIdentity.Spec.AllowedNamespaces = nil
		},
	}
}

func overrideOutboundLBFunc(codecs runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		func(networkSpec *infrav1.NetworkSpec, c fuzz.Continue) {
			networkSpec.ControlPlaneOutboundLB = &infrav1.LoadBalancerSpec{
				FrontendIPsCount: pointer.Int32Ptr(1),
			}
			networkSpec.NodeOutboundLB = &infrav1.LoadBalancerSpec{
				FrontendIPsCount: pointer.Int32Ptr(1),
			}
		},
	}
}
