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

package v1alpha4

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"

	"sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestFuzzyConversion(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(AddToScheme(scheme)).To(Succeed())
	g.Expect(v1beta1.AddToScheme(scheme)).To(Succeed())

	t.Run("for AzureCluster", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: scheme,
		Hub:    &v1beta1.AzureCluster{},
		Spoke:  &AzureCluster{},
	}))

	t.Run("for AzureMachine", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: scheme,
		Hub:    &v1beta1.AzureMachine{},
		Spoke:  &AzureMachine{},
	}))

	t.Run("for AzureMachineTemplate", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: scheme,
		Hub:    &v1beta1.AzureMachineTemplate{},
		Spoke:  &AzureMachineTemplate{},
	}))

	t.Run("for AzureClusterIdentity", utilconversion.FuzzTestFunc(utilconversion.FuzzTestFuncInput{
		Scheme: scheme,
		Hub:    &v1beta1.AzureClusterIdentity{},
		Spoke:  &AzureClusterIdentity{},
	}))

}
