//go:build e2e
// +build e2e

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

package e2e

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"text/template"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"

	deploymentBuilder "sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/deployment"
)

//go:embed data/cluster-autoscaler/rbac.yaml
var clusterAutoscalerRBAC string

//go:embed data/cluster-autoscaler/deployment.yaml.tmpl
var clusterAutoscalerDeploymentTemplate string

const (
	AutoscalingFromZeroSpecName = "autoscale-from-zero"
)

// AutoscalingFromZeroSpecInput defines the input for AutoscalingFromZeroSpec.
type AutoscalingFromZeroSpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
	Cluster               *clusterv1.Cluster
	WaitIntervals         []interface{}
}

// AutoscalingFromZeroSpec implements a test that verifies autoscaling from zero functionality.
// It adds autoscaler annotations to an existing MachineDeployment, deploys Cluster Autoscaler,
// triggers scale-up with a workload, and verifies machines can scale from 0 to 1+.
func AutoscalingFromZeroSpec(ctx context.Context, inputGetter func() AutoscalingFromZeroSpecInput) {
	input := inputGetter()
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil")
	Expect(input.Namespace).NotTo(BeNil(), "Invalid argument. input.Namespace can't be nil")
	Expect(input.ClusterName).NotTo(BeEmpty(), "Invalid argument. input.ClusterName can't be empty")
	Expect(input.Cluster).NotTo(BeNil(), "Invalid argument. input.Cluster can't be nil")
	Expect(input.WaitIntervals).NotTo(BeEmpty(), "Invalid argument. input.WaitIntervals can't be empty")

	var (
		bootstrapClusterProxy = input.BootstrapClusterProxy
		workloadClusterProxy  = bootstrapClusterProxy.GetWorkloadCluster(ctx, input.Namespace.Name, input.ClusterName)
		mgmtClient            = bootstrapClusterProxy.GetClient()
		workloadClientset     *kubernetes.Clientset
		autoscaleMD           *clusterv1.MachineDeployment
	)

	Expect(workloadClusterProxy).NotTo(BeNil())
	Expect(mgmtClient).NotTo(BeNil())

	By("Getting workload cluster clientset")
	workloadClientset = workloadClusterProxy.GetClientSet()
	Expect(workloadClientset).NotTo(BeNil())

	// Step 1: Add autoscaler annotations to existing MachineDeployment
	By("Adding autoscaler annotations to existing MachineDeployment")
	autoscaleMD = addAutoscalerAnnotationsToMachineDeployment(ctx, mgmtClient, input.Namespace.Name, input.ClusterName, input.WaitIntervals)

	// Step 2: Wait for MachineDeployment to stabilize at 0 replicas
	By("Waiting for MachineDeployment to stabilize at 0 replicas")
	Eventually(func(g Gomega) {
		md := &clusterv1.MachineDeployment{}
		err := mgmtClient.Get(ctx, client.ObjectKeyFromObject(autoscaleMD), md)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(md.Status.Replicas).To(HaveValue(Equal(int32(0))))
		g.Expect(md.Status.ReadyReplicas).To(HaveValue(Equal(int32(0))))
	}, input.WaitIntervals...).Should(Succeed())

	Byf("MachineDeployment %s is stable at 0 replicas", autoscaleMD.Name)

	// Step 3: Deploy Cluster Autoscaler RBAC to management cluster
	By("Deploying Cluster Autoscaler RBAC to management cluster")
	err := deployClusterAutoscalerRBAC(ctx, mgmtClient, input.Namespace.Name, input.ClusterName)
	Expect(err).NotTo(HaveOccurred())

	// Step 4: Deploy Cluster Autoscaler Deployment to management cluster
	By("Deploying Cluster Autoscaler to management cluster")
	err = deployClusterAutoscaler(ctx, mgmtClient, input.Namespace.Name, input.ClusterName)
	Expect(err).NotTo(HaveOccurred())

	// Step 5: Wait for CA pod to be ready in management cluster
	By("Waiting for Cluster Autoscaler pod to be ready in management cluster")
	Eventually(func(g Gomega) {
		pods, err := bootstrapClusterProxy.GetClientSet().CoreV1().Pods(input.Namespace.Name).
			List(ctx, metav1.ListOptions{LabelSelector: "app=cluster-autoscaler"})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(pods.Items).To(HaveLen(1))

		pod := pods.Items[0]
		g.Expect(pod.Status.Phase).To(Equal(corev1.PodRunning))
		g.Expect(pod.Status.Conditions).To(ContainElement(gstruct.MatchFields(gstruct.IgnoreExtras, gstruct.Fields{
			"Type":   Equal(corev1.PodReady),
			"Status": Equal(corev1.ConditionTrue),
		})))
	}, input.WaitIntervals...).Should(Succeed())

	Byf("Cluster Autoscaler is running and ready in management cluster")

	// Step 6: Create nginx deployment to trigger scale-up in workload cluster
	By("Creating nginx deployment to trigger scale-up")
	deployment, err := deploymentBuilder.Create("nginx:1.21", "autoscale-trigger", corev1.NamespaceDefault).
		SetReplicas(2).
		SetResourceRequests("100m", "128Mi").
		Deploy(ctx, workloadClientset)
	Expect(err).NotTo(HaveOccurred())

	// Clean up the deployment after the test
	defer func() {
		By("Cleaning up nginx deployment")
		err := workloadClientset.AppsV1().Deployments(corev1.NamespaceDefault).
			Delete(ctx, deployment.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			Logf("Failed to delete deployment %s: %v", deployment.Name, err)
		}
	}()

	// Step 7: Wait for CA to scale MachineDeployment from 0 to 1+
	By("Waiting for Cluster Autoscaler to scale MachineDeployment to 1+")
	Eventually(func(g Gomega) {
		md := &clusterv1.MachineDeployment{}
		err := mgmtClient.Get(ctx, client.ObjectKeyFromObject(autoscaleMD), md)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(md.Spec.Replicas).NotTo(BeNil())
		g.Expect(*md.Spec.Replicas).To(BeNumerically(">=", 1))
		g.Expect(md.Status.Replicas).To(HaveValue(BeNumerically(">=", 1)))
		g.Expect(md.Status.ReadyReplicas).To(HaveValue(BeNumerically(">=", 1)))
	}, input.WaitIntervals...).Should(Succeed())

	Byf("Cluster Autoscaler scaled MachineDeployment from 0 to 1+")

	// Step 8: Verify nginx deployment is ready
	By("Verifying nginx deployment is ready in workload cluster")
	Eventually(func(g Gomega) {
		nginxDeployment, err := workloadClientset.AppsV1().Deployments(corev1.NamespaceDefault).
			Get(ctx, deployment.Name, metav1.GetOptions{})
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(nginxDeployment.Status.ReadyReplicas).To(Equal(int32(2)))
	}, input.WaitIntervals...).Should(Succeed())

	Byf("Nginx deployment is ready - scale-up from zero test PASSED!")
}

// addAutoscalerAnnotationsToMachineDeployment adds autoscaler annotations to the existing MachineDeployment.
func addAutoscalerAnnotationsToMachineDeployment(ctx context.Context, mgmtClient client.Client, namespace, clusterName string, waitIntervals []interface{}) *clusterv1.MachineDeployment {
	mdList := &clusterv1.MachineDeploymentList{}
	err := mgmtClient.List(ctx, mdList,
		client.InNamespace(namespace),
		client.MatchingLabels{
			clusterv1.ClusterNameLabel: clusterName,
		})
	Expect(err).NotTo(HaveOccurred())
	Expect(mdList.Items).To(HaveLen(1), "Expected exactly one MachineDeployment for cluster %s, found %d", clusterName, len(mdList.Items))

	mdKey := client.ObjectKey{
		Namespace: mdList.Items[0].Namespace,
		Name:      mdList.Items[0].Name,
	}

	var updatedMD *clusterv1.MachineDeployment

	Eventually(func(g Gomega) {
		md := &clusterv1.MachineDeployment{}
		err := mgmtClient.Get(ctx, mdKey, md)
		g.Expect(err).NotTo(HaveOccurred())

		// Add autoscaler annotations
		if md.Annotations == nil {
			md.Annotations = make(map[string]string)
		}
		md.Annotations["cluster.x-k8s.io/cluster-api-autoscaler-node-group-min-size"] = "0"
		md.Annotations["cluster.x-k8s.io/cluster-api-autoscaler-node-group-max-size"] = "3"

		// Update the MachineDeployment
		err = mgmtClient.Update(ctx, md)
		g.Expect(err).NotTo(HaveOccurred())

		updatedMD = md
	}, waitIntervals...).Should(Succeed())

	return updatedMD
}

// deployClusterAutoscalerRBAC deploys the RBAC resources for Cluster Autoscaler to management cluster.
func deployClusterAutoscalerRBAC(ctx context.Context, mgmtClient client.Client, namespace, clusterName string) error {
	// Parse RBAC template
	tmpl, err := template.New("cluster-autoscaler-rbac").Parse(clusterAutoscalerRBAC)
	if err != nil {
		return fmt.Errorf("failed to parse RBAC template: %w", err)
	}

	// Execute template
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]string{
		"Namespace":   namespace,
		"ClusterName": clusterName,
	})
	if err != nil {
		return fmt.Errorf("failed to execute RBAC template: %w", err)
	}

	// Parse and apply RBAC YAML
	codecs := serializer.NewCodecFactory(scheme.Scheme)
	decoder := codecs.UniversalDeserializer()
	objects := []runtime.Object{}

	// Split YAML by "---" and decode each document
	docs := bytes.Split(buf.Bytes(), []byte("\n---\n"))
	for _, doc := range docs {
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}

		obj, _, err := decoder.Decode(doc, nil, nil)
		if err != nil {
			return fmt.Errorf("failed to decode RBAC YAML: %w", err)
		}
		objects = append(objects, obj)
	}

	// Apply each object using controller-runtime client
	for _, obj := range objects {
		// Convert runtime.Object to client.Object
		clientObj, ok := obj.(client.Object)
		if !ok {
			return fmt.Errorf("object is not a client.Object: %T", obj)
		}

		Logf("Creating %s %s", obj.GetObjectKind().GroupVersionKind().Kind, clientObj.GetName())
		err := mgmtClient.Create(ctx, clientObj)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create %s %s: %w",
				obj.GetObjectKind().GroupVersionKind().Kind, clientObj.GetName(), err)
		}
	}

	return nil
}

// deployClusterAutoscaler deploys the Cluster Autoscaler deployment to management cluster.
func deployClusterAutoscaler(ctx context.Context, mgmtClient client.Client, namespace, clusterName string) error {
	// Parse template
	tmpl, err := template.New("cluster-autoscaler").Parse(clusterAutoscalerDeploymentTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse deployment template: %w", err)
	}

	// Execute template
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]string{
		"Namespace":   namespace,
		"ClusterName": clusterName,
	})
	if err != nil {
		return fmt.Errorf("failed to execute deployment template: %w", err)
	}

	// Decode deployment
	codecs := serializer.NewCodecFactory(scheme.Scheme)
	decoder := codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode(buf.Bytes(), nil, nil)
	if err != nil {
		return fmt.Errorf("failed to decode deployment YAML: %w", err)
	}

	deployment, ok := obj.(*appsv1.Deployment)
	if !ok {
		return fmt.Errorf("decoded object is not a Deployment")
	}

	// Create deployment using controller-runtime client
	Logf("Creating Cluster Autoscaler Deployment %s/%s", deployment.Namespace, deployment.Name)
	err = mgmtClient.Create(ctx, deployment)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create Cluster Autoscaler deployment %s: %w", deployment.Name, err)
	}

	return nil
}
