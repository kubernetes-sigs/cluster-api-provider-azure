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
	"sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
)

func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(v1alpha4.AddToScheme(scheme)).To(Succeed())

	t.Run("for AzureCluster", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1alpha4.AzureCluster{},
		Spoke:       &AzureCluster{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{overrideImageFuncs, overrideDeprecatedFieldsFuncs},
	}))

	t.Run("for AzureMachine", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1alpha4.AzureMachine{},
		Spoke:       &AzureMachine{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{overrideImageFuncs, overrideDeprecatedFieldsFuncs},
	}))

	t.Run("for AzureMachineTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme:      scheme,
		Hub:         &v1alpha4.AzureMachineTemplate{},
		Spoke:       &AzureMachineTemplate{},
		FuzzerFuncs: []fuzzer.FuzzerFuncs{overrideImageFuncs, overrideDeprecatedFieldsFuncs},
	}))

}

func overrideImageFuncs(codecs runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		func(image *v1alpha4.Image, c fuzz.Continue) {
			image.Marketplace = &v1alpha4.AzureMarketplaceImage{
				Publisher: "PUB1234",
				Offer:     "OFFER123",
				SKU:       "SKU123",
				Version:   "1.0.0",
			}
		},
	}
}

func overrideDeprecatedFieldsFuncs(codecs runtimeserializer.CodecFactory) []interface{} {
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
	}
}
