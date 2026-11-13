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
	"context"
	"fmt"
	"reflect"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
)

// KubeRayAPISpecInput identifies an already-installed KubeRay environment.
type KubeRayAPISpecInput struct {
	RESTConfig  *rest.Config
	SkipCleanup bool
}

// KubeRayAPISpec verifies the ray.io/v1 API contract independently of installation
// and cluster provisioning. Deletion is an assertion, not optional cleanup.
func KubeRayAPISpec(ctx context.Context, input KubeRayAPISpecInput) {
	Expect(input.RESTConfig).NotTo(BeNil())
	clientset, err := kubernetes.NewForConfig(input.RESTConfig)
	Expect(err).NotTo(HaveOccurred())

	By("creating an isolated namespace for the KubeRay API contract")
	namespace, err := clientset.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{GenerateName: "kuberay-api-"},
	}, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	By("verifying KubeRay API discovery, schemas, CRUD, and invalid-input rejection")
	verifyKubeRayAPI(ctx, NewGomegaWithT(GinkgoT()), input.RESTConfig, namespace.Name,
		e2eConfig.GetIntervals("kuberay-api", "wait-api")...)

	// On failure, leave fixtures available for the enclosing spec's diagnostics.
	if !input.SkipCleanup {
		Expect(clientset.CoreV1().Namespaces().Delete(ctx, namespace.Name, metav1.DeleteOptions{})).To(Succeed())
	}
}

type kubeRayAPIContract struct {
	resource    schema.GroupVersionResource
	fixture     *unstructured.Unstructured
	clusterPath string
	schemaTypes map[string]string
	updatePath  string
	updateValue string
	invalidPath string
}

func kubeRayAPIContracts(namespace string) []kubeRayAPIContract {
	cluster := newRayClusterUnstructured("raycluster-api", namespace)
	cluster.Object["spec"].(map[string]interface{})["suspend"] = true
	job := newRayJobUnstructured("rayjob-api", namespace)
	job.Object["spec"].(map[string]interface{})["suspend"] = true
	serviceCluster := rayClusterSpec()
	serviceCluster["suspend"] = true
	service := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "ray.io/v1",
		"kind":       "RayService",
		"metadata":   map[string]interface{}{"name": "rayservice-api", "namespace": namespace},
		"spec": map[string]interface{}{
			"serveConfigV2":    "applications: []\n",
			"rayClusterConfig": serviceCluster,
		},
	}}
	cronJob := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "ray.io/v1",
		"kind":       "RayCronJob",
		"metadata":   map[string]interface{}{"name": "raycronjob-api", "namespace": namespace},
		"spec": map[string]interface{}{
			"schedule":    "0 * * * *",
			"suspend":     true,
			"jobTemplate": job.Object["spec"],
		},
	}}
	return []kubeRayAPIContract{
		{
			resource: rayClusterGVR, fixture: cluster, clusterPath: "spec",
			schemaTypes: map[string]string{"spec.suspend": "boolean"},
			updatePath:  "spec.headGroupSpec.rayStartParams.num-cpus", updateValue: "2",
			invalidPath: "spec.rayVersion",
		},
		{
			resource: rayJobGVR, fixture: job, clusterPath: "spec.rayClusterSpec",
			schemaTypes: map[string]string{
				"spec.entrypoint": "string", "spec.suspend": "boolean",
				"spec.shutdownAfterJobFinishes": "boolean", "spec.ttlSecondsAfterFinished": "integer",
				"spec.submitterPodTemplate": "object",
			},
			updatePath: "spec.entrypoint", updateValue: "python -c \"print('api contract')\"",
			invalidPath: "spec.entrypoint",
		},
		{
			resource: schema.GroupVersionResource{Group: "ray.io", Version: "v1", Resource: "rayservices"},
			fixture:  service, clusterPath: "spec.rayClusterConfig",
			schemaTypes: map[string]string{"spec.serveConfigV2": "string"},
			updatePath:  "spec.serveConfigV2", updateValue: "applications: []\n# API round-trip update\n",
			invalidPath: "spec.serveConfigV2",
		},
		{
			resource: schema.GroupVersionResource{Group: "ray.io", Version: "v1", Resource: "raycronjobs"},
			fixture:  cronJob, clusterPath: "spec.jobTemplate.rayClusterSpec",
			schemaTypes: map[string]string{
				"spec.schedule": "string", "spec.suspend": "boolean", "spec.jobTemplate.entrypoint": "string",
			},
			updatePath: "spec.schedule", updateValue: "30 * * * *",
			invalidPath: "spec.schedule",
		},
	}
}

func verifyKubeRayAPI(ctx context.Context, g Gomega, config *rest.Config, namespace string, intervals ...interface{}) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	g.Expect(err).NotTo(HaveOccurred())
	crdClient, err := apiextensionsclient.NewForConfig(config)
	g.Expect(err).NotTo(HaveOccurred())
	dynamicClient, err := dynamic.NewForConfig(config)
	g.Expect(err).NotTo(HaveOccurred())

	contracts := kubeRayAPIContracts(namespace)
	g.Eventually(func(g Gomega) {
		resources, err := discoveryClient.ServerResourcesForGroupVersion("ray.io/v1")
		g.Expect(err).NotTo(HaveOccurred())
		for _, contract := range contracts {
			var discovered *metav1.APIResource
			for i := range resources.APIResources {
				if resources.APIResources[i].Name == contract.resource.Resource {
					discovered = &resources.APIResources[i]
					break
				}
			}
			g.Expect(discovered).NotTo(BeNil(), "missing discovery for %s", contract.resource)
			g.Expect(discovered.Kind).To(Equal(contract.fixture.GetKind()))
			g.Expect(discovered.Namespaced).To(BeTrue())
			g.Expect([]string(discovered.Verbs)).To(ContainElements("create", "get", "update", "delete"))
		}
	}, intervals...).WithContext(ctx).Should(Succeed(), "ray.io/v1 discovery must advertise the supported API contract")

	for _, contract := range contracts {
		name := contract.resource.Resource + "." + contract.resource.Group
		crd, err := crdClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, name, metav1.GetOptions{})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(checkKubeRaySchema(crd, contract)).To(Succeed(), "schema for %s", name)
		verifyKubeRayResource(ctx, g, dynamicClient.Resource(contract.resource).Namespace(namespace), contract, intervals...)
	}
}

func checkKubeRaySchema(crd *apiextensionsv1.CustomResourceDefinition, contract kubeRayAPIContract) error {
	if crd.Spec.Group != contract.resource.Group || crd.Spec.Names.Plural != contract.resource.Resource ||
		crd.Spec.Names.Kind != contract.fixture.GetKind() || crd.Spec.Scope != apiextensionsv1.NamespaceScoped {
		return fmt.Errorf("unexpected API identity or scope for %s", crd.Name)
	}
	var root *apiextensionsv1.JSONSchemaProps
	for _, version := range crd.Spec.Versions {
		if version.Name == contract.resource.Version && version.Served && version.Schema != nil {
			root = version.Schema.OpenAPIV3Schema
			break
		}
	}
	if root == nil {
		return fmt.Errorf("%s has no served v1 OpenAPI schema", crd.Name)
	}
	fields := map[string]string{
		"": "object", "spec": "object", "status": "object",
		contract.clusterPath:                                                     "object",
		contract.clusterPath + ".rayVersion":                                     "string",
		contract.clusterPath + ".headGroupSpec":                                  "object",
		contract.clusterPath + ".headGroupSpec.rayStartParams":                   "object",
		contract.clusterPath + ".headGroupSpec.template.spec.containers":         "array",
		contract.clusterPath + ".headGroupSpec.template.spec.containers.*.image": "string",
		contract.clusterPath + ".workerGroupSpecs":                               "array",
		contract.clusterPath + ".workerGroupSpecs.*.groupName":                   "string",
		contract.clusterPath + ".workerGroupSpecs.*.replicas":                    "integer",
		contract.clusterPath + ".workerGroupSpecs.*.template":                    "object",
	}
	for path, fieldType := range contract.schemaTypes {
		fields[path] = fieldType
	}
	for path, fieldType := range fields {
		field := root
		if path != "" {
			for _, part := range strings.Split(path, ".") {
				if part == "*" {
					if field.Items == nil || field.Items.Schema == nil {
						return fmt.Errorf("%s: missing array item schema", path)
					}
					field = field.Items.Schema
				} else {
					property, found := field.Properties[part]
					if !found {
						return fmt.Errorf("%s: missing schema property %s", path, part)
					}
					field = &property
				}
			}
		}
		if field.Type != fieldType {
			return fmt.Errorf("%s: expected schema type %s, got %q", path, fieldType, field.Type)
		}
	}
	return nil
}

func verifyKubeRayResource(ctx context.Context, g Gomega, client dynamic.ResourceInterface, contract kubeRayAPIContract, intervals ...interface{}) {
	fixture := contract.fixture
	created, err := client.Create(ctx, fixture.DeepCopy(), metav1.CreateOptions{})
	g.Expect(err).NotTo(HaveOccurred(), "create %s", fixture.GetKind())
	g.Expect(created.GetUID()).NotTo(BeEmpty())
	g.Expect(checkKubeRayFields(fixture.Object["spec"], created.Object["spec"], "spec")).To(Succeed(), "create round-trip for %s", fixture.GetKind())

	read, err := client.Get(ctx, fixture.GetName(), metav1.GetOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(read.GetUID()).To(Equal(created.GetUID()))
	g.Expect(checkKubeRayFields(fixture.Object["spec"], read.Object["spec"], "spec")).To(Succeed(), "read round-trip for %s", fixture.GetKind())

	expected := fixture.DeepCopy()
	g.Expect(unstructured.SetNestedField(expected.Object, contract.updateValue, strings.Split(contract.updatePath, ".")...)).To(Succeed())
	g.Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := client.Get(ctx, fixture.GetName(), metav1.GetOptions{})
		if err != nil {
			return err
		}
		if err := unstructured.SetNestedField(current.Object, contract.updateValue, strings.Split(contract.updatePath, ".")...); err != nil {
			return err
		}
		_, err = client.Update(ctx, current, metav1.UpdateOptions{})
		return err
	})).To(Succeed(), "update %s", fixture.GetKind())
	read, err = client.Get(ctx, fixture.GetName(), metav1.GetOptions{})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(read.GetUID()).To(Equal(created.GetUID()))
	g.Expect(read.GetGeneration()).To(BeNumerically(">", created.GetGeneration()))
	g.Expect(checkKubeRayFields(expected.Object["spec"], read.Object["spec"], "spec")).To(Succeed(), "update round-trip for %s", fixture.GetKind())

	invalid := fixture.DeepCopy()
	invalid.SetName(fixture.GetName() + "-invalid")
	g.Expect(unstructured.SetNestedField(invalid.Object, true, strings.Split(contract.invalidPath, ".")...)).To(Succeed())
	_, err = client.Create(ctx, invalid, metav1.CreateOptions{})
	g.Expect(checkKubeRayInvalidError(err, contract.invalidPath)).To(Succeed(), "reject schema-invalid %s at the API server", fixture.GetKind())
	_, err = client.Get(ctx, invalid.GetName(), metav1.GetOptions{})
	g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "invalid %s must not be persisted: %v", fixture.GetKind(), err)

	g.Expect(client.Delete(ctx, fixture.GetName(), metav1.DeleteOptions{})).To(Succeed())
	g.Eventually(func() error {
		remaining, err := client.Get(ctx, fixture.GetName(), metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		return fmt.Errorf("%s %s still exists, finalizers: %v", fixture.GetKind(), fixture.GetName(), remaining.GetFinalizers())
	}, intervals...).WithContext(ctx).Should(Succeed(), "delete %s", fixture.GetKind())
}

// checkKubeRayFields permits server-defaulted fields, but no loss or mutation of
// explicitly supplied fields. Lists in these fixtures have a defined order.
func checkKubeRayFields(expected, actual interface{}, path string) error {
	switch want := expected.(type) {
	case map[string]interface{}:
		got, ok := actual.(map[string]interface{})
		if !ok {
			return fmt.Errorf("%s: expected an object, got %T", path, actual)
		}
		for key, value := range want {
			gotValue, found := got[key]
			if !found {
				return fmt.Errorf("%s.%s: field was pruned", path, key)
			}
			if err := checkKubeRayFields(value, gotValue, path+"."+key); err != nil {
				return err
			}
		}
	case []interface{}:
		got, ok := actual.([]interface{})
		if !ok || len(got) != len(want) {
			return fmt.Errorf("%s: expected a list of length %d, got %v", path, len(want), actual)
		}
		for i, value := range want {
			if err := checkKubeRayFields(value, got[i], fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	default:
		if !reflect.DeepEqual(expected, actual) {
			return fmt.Errorf("%s: expected %v (%T), got %v (%T)", path, expected, expected, actual, actual)
		}
	}
	return nil
}

func checkKubeRayInvalidError(err error, fieldPath string) error {
	if !apierrors.IsInvalid(err) {
		return fmt.Errorf("expected an Invalid API response for %s, got %v", fieldPath, err)
	}
	status, ok := err.(apierrors.APIStatus)
	if ok && status.Status().Details != nil {
		for _, cause := range status.Status().Details.Causes {
			if cause.Field == fieldPath && cause.Type == metav1.CauseType(field.ErrorTypeTypeInvalid) {
				return nil
			}
		}
		return fmt.Errorf("expected a FieldValueTypeInvalid cause for %s, got causes %+v", fieldPath, status.Status().Details.Causes)
	}
	return fmt.Errorf("expected a FieldValueTypeInvalid cause for %s, got %v", fieldPath, err)
}
