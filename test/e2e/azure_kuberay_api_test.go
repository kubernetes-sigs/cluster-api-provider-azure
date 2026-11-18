//go:build e2e
// +build e2e

/*
Copyright The Kubernetes Authors.

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

package e2e

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestKubeRayHeadRuntimeEnvAgentPort(t *testing.T) {
	contracts := append([]kubeRayAPIContract{
		{fixture: newRayClusterUnstructured("raycluster-e2e", "default"), clusterPath: "spec"},
		{fixture: newRayJobUnstructured("rayjob-e2e", "default"), clusterPath: "spec.rayClusterSpec"},
	}, kubeRayAPIContracts("kuberay-api")...)

	for _, contract := range contracts {
		t.Run(contract.fixture.GetName(), func(t *testing.T) {
			g := NewWithT(t)
			path := append(strings.Split(contract.clusterPath, "."), "headGroupSpec", "rayStartParams", "runtime-env-agent-port")
			port, found, err := unstructured.NestedString(contract.fixture.Object, path...)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(found).To(BeTrue(), "the head must use a fixed runtime-env agent port")
			g.Expect(port).To(Equal("52366"), "Ray Client must not depend on raylet registration to discover the agent port")
		})
	}
}

func TestKubeRayFields(t *testing.T) {
	for _, tt := range []struct {
		name     string
		expected interface{}
		actual   interface{}
		wantErr  bool
	}{
		{
			name:     "server defaults are allowed",
			expected: map[string]interface{}{"replicas": int64(1)},
			actual:   map[string]interface{}{"replicas": int64(1), "suspend": false},
		},
		{
			name:     "nested defaults are allowed",
			expected: []interface{}{map[string]interface{}{"name": "worker"}},
			actual:   []interface{}{map[string]interface{}{"name": "worker", "replicas": int64(1)}},
		},
		{
			name:     "pruned fields fail",
			expected: map[string]interface{}{"replicas": int64(1)},
			actual:   map[string]interface{}{}, wantErr: true,
		},
		{name: "changed values fail", expected: int64(1), actual: int64(2), wantErr: true},
		{name: "changed types fail", expected: int64(1), actual: "1", wantErr: true},
		{name: "non-object fails", expected: map[string]interface{}{}, actual: nil, wantErr: true},
		{name: "non-array fails", expected: []interface{}{}, actual: nil, wantErr: true},
		{name: "missing array entries fail", expected: []interface{}{"a"}, actual: []interface{}{}, wantErr: true},
		{name: "extra array entries fail", expected: []interface{}{"a"}, actual: []interface{}{"a", "b"}, wantErr: true},
		{name: "reordered array entries fail", expected: []interface{}{"a", "b"}, actual: []interface{}{"b", "a"}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := checkKubeRayFields(tt.expected, tt.actual, "spec")
			if (err != nil) != tt.wantErr {
				t.Fatalf("checkKubeRayFields() = %v, want error: %v", err, tt.wantErr)
			}
		})
	}
}

func TestKubeRayInvalidError(t *testing.T) {
	kind := schema.GroupKind{Group: "ray.io", Kind: "RayCluster"}
	path := field.NewPath("spec", "rayVersion")
	invalid := apierrors.NewInvalid(kind, "invalid", field.ErrorList{field.TypeInvalid(path, true, "must be a string")})
	for _, tt := range []struct {
		name    string
		err     error
		wantErr bool
	}{
		{name: "schema validation rejection", err: invalid},
		{name: "accepted invalid object", wantErr: true},
		{name: "transport failure", err: errors.New("connection refused"), wantErr: true},
		{name: "unauthorized", err: apierrors.NewUnauthorized("unauthorized"), wantErr: true},
		{name: "not found", err: apierrors.NewNotFound(schema.GroupResource{Group: "ray.io", Resource: "rayclusters"}, "invalid"), wantErr: true},
		{name: "unrelated invalid field", err: apierrors.NewInvalid(kind, "invalid", field.ErrorList{field.TypeInvalid(field.NewPath("metadata", "name"), true, "must be a string")}), wantErr: true},
		{name: "wrong validation cause", err: apierrors.NewInvalid(kind, "invalid", field.ErrorList{field.Required(path, "required")}), wantErr: true},
		{name: "missing status details", err: &apierrors.StatusError{ErrStatus: metav1.Status{Reason: metav1.StatusReasonInvalid}}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := checkKubeRayInvalidError(tt.err, path.String())
			if (err != nil) != tt.wantErr {
				t.Fatalf("checkKubeRayInvalidError() = %v, want error: %v", err, tt.wantErr)
			}
		})
	}
}

// TestKubeRayAPIContract runs the same assertions as the Azure spec against a
// real API server. Supply CRDs from the pinned chart; no operator or Azure is needed.
func TestKubeRayAPIContract(t *testing.T) {
	crdPath := os.Getenv("KUBERAY_CRD_PATH")
	if crdPath == "" {
		t.Skip("set KUBERAY_CRD_PATH to the pinned Helm chart's CRDs to run the API-server contract test")
	}
	g := NewWithT(t)
	file, err := os.Open(crdPath)
	g.Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() { g.Expect(file.Close()).To(Succeed()) })

	var crds []*apiextensionsv1.CustomResourceDefinition
	decoder := yaml.NewYAMLOrJSONDecoder(file, 4096)
	for {
		crd := &apiextensionsv1.CustomResourceDefinition{}
		err := decoder.Decode(crd)
		if errors.Is(err, io.EOF) {
			break
		}
		g.Expect(err).NotTo(HaveOccurred())
		if crd.Name != "" {
			crds = append(crds, crd)
		}
	}
	for _, contract := range kubeRayAPIContracts("kuberay-api") {
		t.Run(contract.fixture.GetKind()+" schema", func(t *testing.T) {
			g := NewWithT(t)
			var crd *apiextensionsv1.CustomResourceDefinition
			for _, candidate := range crds {
				if candidate.Name == contract.resource.Resource+"."+contract.resource.Group {
					crd = candidate
					break
				}
			}
			g.Expect(crd).NotTo(BeNil())
			g.Expect(checkKubeRaySchema(crd, contract)).To(Succeed())
			for _, mutation := range []struct {
				name   string
				change func(*apiextensionsv1.CustomResourceDefinition)
			}{
				{"wrong scope", func(crd *apiextensionsv1.CustomResourceDefinition) { crd.Spec.Scope = apiextensionsv1.ClusterScoped }},
				{"wrong kind", func(crd *apiextensionsv1.CustomResourceDefinition) { crd.Spec.Names.Kind = "Other" }},
				{"unserved v1", func(crd *apiextensionsv1.CustomResourceDefinition) {
					for i := range crd.Spec.Versions {
						crd.Spec.Versions[i].Served = false
					}
				}},
				{"missing schema", func(crd *apiextensionsv1.CustomResourceDefinition) {
					for i := range crd.Spec.Versions {
						crd.Spec.Versions[i].Schema = nil
					}
				}},
				{"missing spec", func(crd *apiextensionsv1.CustomResourceDefinition) {
					for i := range crd.Spec.Versions {
						delete(crd.Spec.Versions[i].Schema.OpenAPIV3Schema.Properties, "spec")
					}
				}},
				{"wrong field type", func(crd *apiextensionsv1.CustomResourceDefinition) {
					for i := range crd.Spec.Versions {
						crd.Spec.Versions[i].Schema.OpenAPIV3Schema.Properties["status"] = apiextensionsv1.JSONSchemaProps{Type: "string"}
					}
				}},
			} {
				t.Run(mutation.name, func(t *testing.T) {
					broken := crd.DeepCopy()
					mutation.change(broken)
					NewWithT(t).Expect(checkKubeRaySchema(broken, contract)).NotTo(Succeed())
				})
			}
		})
	}
	if t.Failed() {
		t.Fatal("CRD schema checks failed")
	}

	environment := &envtest.Environment{CRDs: crds}
	config, err := environment.Start()
	g.Expect(err).NotTo(HaveOccurred())
	t.Cleanup(func() { g.Expect(environment.Stop()).To(Succeed()) })

	ctx := t.Context()
	clientset, err := kubernetes.NewForConfig(config)
	g.Expect(err).NotTo(HaveOccurred())
	namespace, err := clientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "kuberay-api"},
	}, metav1.CreateOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	verifyKubeRayAPI(ctx, g, config, namespace.Name, "30s", "100ms")

	dynamicClient, err := dynamic.NewForConfig(config)
	g.Expect(err).NotTo(HaveOccurred())
	for _, contract := range kubeRayAPIContracts(namespace.Name) {
		remaining, err := dynamicClient.Resource(contract.resource).Namespace(namespace.Name).List(ctx, metav1.ListOptions{})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(remaining.Items).To(BeEmpty(), "all %s fixtures must be deleted or rejected", contract.resource.Resource)
	}
}
