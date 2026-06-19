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

package asomigration

import (
	"context"
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	testCRDName  = "fleetsmembers.containerservice.azure.com"
	testGroup    = "containerservice.azure.com"
	testPlural   = "fleetsmembers"
	testStale    = "v1api20230315previewstorage"
	testServed   = "v1api20230315preview"
	testNewStore = "v1api20250301"
)

func testCRD(storedVersions []string) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: testCRDName},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: testGroup,
			Names: apiextensionsv1.CustomResourceDefinitionNames{Plural: testPlural, Kind: "FleetsMember"},
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{Name: testServed, Served: true},
				{Name: testStale, Served: false, Storage: true},
				{Name: testNewStore, Served: true},
			},
		},
		Status: apiextensionsv1.CustomResourceDefinitionStatus{StoredVersions: storedVersions},
	}
}

// testDynamicClient registers list kinds for every CRD version so the fake can
// list each <plural>.<version>.<group> without erroring; instances are created
// only under the served version.
func testDynamicClient(instances int) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	gvrToListKind := map[schema.GroupVersionResource]string{}
	for _, v := range []string{testServed, testStale, testNewStore} {
		gvr := schema.GroupVersionResource{Group: testGroup, Version: v, Resource: testPlural}
		gvrToListKind[gvr] = "FleetsMemberList"
	}
	objs := make([]runtime.Object, 0, instances)
	for i := range instances {
		objs = append(objs, &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": testGroup + "/" + testServed,
			"kind":       "FleetsMember",
			"metadata": map[string]interface{}{
				"name":      "member-" + string(rune('a'+i)),
				"namespace": "default",
			},
		}})
	}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, objs...)
}

func crdExists(g *WithT, crdClient *apiextensionsfake.Clientset) bool {
	_, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), testCRDName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return false
	}
	g.Expect(err).NotTo(HaveOccurred())
	return true
}

func TestMigrateASOCRDStoredVersion(t *testing.T) {
	migration := storedVersionMigration{crd: testCRDName, stale: testStale}
	ctx := context.Background()
	logger := log.Log

	t.Run("skips when CRD is absent", func(t *testing.T) {
		g := NewWithT(t)
		crdClient := apiextensionsfake.NewSimpleClientset()
		err := migrateStoredVersion(ctx, logger, crdClient, testDynamicClient(0), migration)
		g.Expect(err).NotTo(HaveOccurred())
	})

	t.Run("skips when stale version is not stored", func(t *testing.T) {
		g := NewWithT(t)
		crdClient := apiextensionsfake.NewSimpleClientset(testCRD([]string{testNewStore}))
		err := migrateStoredVersion(ctx, logger, crdClient, testDynamicClient(0), migration)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(crdExists(g, crdClient)).To(BeTrue(), "CRD must be left intact")
	})

	t.Run("deletes the CRD when stale and empty", func(t *testing.T) {
		g := NewWithT(t)
		crdClient := apiextensionsfake.NewSimpleClientset(testCRD([]string{testStale, testNewStore}))
		err := migrateStoredVersion(ctx, logger, crdClient, testDynamicClient(0), migration)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(crdExists(g, crdClient)).To(BeFalse(), "CRD must be deleted")
	})

	t.Run("fails when stale but instances exist", func(t *testing.T) {
		g := NewWithT(t)
		crdClient := apiextensionsfake.NewSimpleClientset(testCRD([]string{testStale, testNewStore}))
		err := migrateStoredVersion(ctx, logger, crdClient, testDynamicClient(2), migration)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("2 instance(s)"))
		g.Expect(err.Error()).To(ContainSubstring("asoctl clean crds"))
		g.Expect(crdExists(g, crdClient)).To(BeTrue(), "CRD must not be deleted while it holds data")
	})

	t.Run("fails closed when listing a served version errors", func(t *testing.T) {
		g := NewWithT(t)
		crdClient := apiextensionsfake.NewSimpleClientset(testCRD([]string{testStale, testNewStore}))
		dynClient := testDynamicClient(0)
		dynClient.PrependReactor("list", testPlural, func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewInternalError(errors.New("conversion webhook down"))
		})
		err := migrateStoredVersion(ctx, logger, crdClient, dynClient, migration)
		g.Expect(err).To(HaveOccurred())
		g.Expect(crdExists(g, crdClient)).To(BeTrue(), "CRD must not be deleted when emptiness can't be confirmed")
	})
}
