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

package v1alpha2

import (
	"testing"

	fuzz "github.com/google/gofuzz"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	v1alpha3 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
)

func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(v1alpha3.AddToScheme(scheme)).To(Succeed())

	t.Run("for AzureCluster", utilconversion.FuzzTestFunc(scheme, &v1alpha3.AzureCluster{}, &AzureCluster{}, overrideImageFuncs))
	t.Run("for AzureMachine", utilconversion.FuzzTestFunc(scheme, &v1alpha3.AzureMachine{}, &AzureMachine{}, overrideImageFuncs))
	t.Run("for AzureMachineTemplate", utilconversion.FuzzTestFunc(scheme, &v1alpha3.AzureMachineTemplate{}, &AzureMachineTemplate{}, overrideImageFuncs))
}

func overrideImageFuncs(codecs runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		func(image *v1alpha3.Image, c fuzz.Continue) {
			image.Marketplace = &v1alpha3.AzureMarketplaceImage{
				Publisher: "PUB1234",
				Offer:     "OFFER123",
				SKU:       "SKU123",
				Version:   "1.0.0",
			}
		},
	}
}
