//go:build e2e
// +build e2e

/*
Copyright 2022 The Kubernetes Authors.

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
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	asocontainerservicev1 "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20231001"
	asocontainerservicev1preview "github.com/Azure/azure-service-operator/v2/api/containerservice/v1api20231102preview"
	asoresourcesv1 "github.com/Azure/azure-service-operator/v2/api/resources/v1api20200601"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/kubectl/pkg/describe"
	"k8s.io/utils/ptr"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
)

type (
	AzureClusterProxy struct {
		framework.ClusterProxy
	}
	// myEventData is used to be able to Marshal armmonitor.EventData into JSON
	// see https://github.com/Azure/azure-sdk-for-go/issues/8224#issuecomment-614777550
	myEventData armmonitor.EventData
)

func NewAzureClusterProxy(name string, kubeconfigPath string, options ...framework.Option) *AzureClusterProxy {
	proxy := framework.NewClusterProxy(name, kubeconfigPath, initScheme(), options...)
	return &AzureClusterProxy{
		ClusterProxy: proxy,
	}
}

func initScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	framework.TryAddDefaultSchemes(scheme)
	Expect(infrav1.AddToScheme(scheme)).To(Succeed())
	Expect(infrav1exp.AddToScheme(scheme)).To(Succeed())
	Expect(expv1.AddToScheme(scheme)).To(Succeed())
	Expect(asoresourcesv1.AddToScheme(scheme)).To(Succeed())
	Expect(asocontainerservicev1.AddToScheme(scheme)).To(Succeed())
	Expect(asocontainerservicev1preview.AddToScheme(scheme)).To(Succeed())
	return scheme
}

func (acp *AzureClusterProxy) CollectWorkloadClusterLogs(ctx context.Context, namespace, name, outputPath string) {
	Logf("Dumping workload cluster %s/%s logs", namespace, name)
	acp.ClusterProxy.CollectWorkloadClusterLogs(ctx, namespace, name, outputPath)

	aboveMachinesPath := strings.Replace(outputPath, "/machines", "", 1)

	Logf("Dumping workload cluster %s/%s nodes", namespace, name)
	start := time.Now()
	acp.collectNodes(ctx, namespace, name, aboveMachinesPath)
	Logf("Fetching nodes took %s", time.Since(start).String())

	Logf("Dumping workload cluster %s/%s pod logs", namespace, name)
	start = time.Now()
	acp.collectPodLogs(ctx, namespace, name, aboveMachinesPath)
	Logf("Fetching pod logs took %s", time.Since(start).String())

	Logf("Dumping workload cluster %s/%s Azure activity log", namespace, name)
	start = time.Now()
	acp.collectActivityLogs(ctx, namespace, name, aboveMachinesPath)
	Logf("Fetching activity logs took %s", time.Since(start).String())
}

func (acp *AzureClusterProxy) collectPodLogs(ctx context.Context, namespace string, name string, aboveMachinesPath string) {
	workload := acp.GetWorkloadCluster(ctx, namespace, name)
	pods := &corev1.PodList{}

	Expect(workload.GetClient().List(ctx, pods)).To(Succeed())

	var err error
	var podDescribe string

	podDescriber, ok := describe.DescriberFor(schema.GroupKind{Group: corev1.GroupName, Kind: "Pod"}, workload.GetRESTConfig())
	if !ok {
		Logf("failed to get pod describer")
	}

	for _, pod := range pods.Items {
		podNamespace := pod.GetNamespace()

		// Describe the pod.
		podDescribe, err = podDescriber.Describe(podNamespace, pod.GetName(), describe.DescriberSettings{ShowEvents: true})
		if err != nil {
			Logf("failed to describe pod %s/%s: %v", podNamespace, pod.GetName(), err)
		}

		// collect the init container logs
		for _, container := range pod.Spec.InitContainers {
			// Watch each init container's logs in a goroutine, so we can stream them all concurrently.
			go collectContainerLogs(ctx, pod, container, aboveMachinesPath, workload)
		}

		for _, container := range pod.Spec.Containers {
			// Watch each container's logs in a goroutine, so we can stream them all concurrently.
			go collectContainerLogs(ctx, pod, container, aboveMachinesPath, workload)
		}

		Logf("Describing Pod %s/%s", podNamespace, pod.Name)
		describeFile := path.Join(aboveMachinesPath, podNamespace, pod.Name, "pod-describe.txt")
		writeLogFile(describeFile, podDescribe)
	}
}

func collectContainerLogs(ctx context.Context, pod corev1.Pod, container corev1.Container, aboveMachinesPath string, workload framework.ClusterProxy) {
	defer GinkgoRecover()

	podNamespace := pod.GetNamespace()

	Logf("Creating log watcher for controller %s/%s, container %s", podNamespace, pod.Name, container.Name)
	logFile := path.Join(aboveMachinesPath, podNamespace, pod.Name, container.Name+".log")
	if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
		// Failing to mkdir should not cause the test to fail
		Logf("Error mkdir: %v", err)
		return
	}

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		// Failing to fetch logs should not cause the test to fail
		Logf("Error opening file to write pod logs: %v", err)
		return
	}
	defer f.Close()

	opts := &corev1.PodLogOptions{
		Container: container.Name,
		Follow:    true,
	}

	podLogs, err := workload.GetClientSet().CoreV1().Pods(podNamespace).GetLogs(pod.Name, opts).Stream(ctx)
	if err != nil {
		// Failing to stream logs should not cause the test to fail
		Logf("Error starting logs stream for pod %s/%s, container %s: %v", podNamespace, pod.Name, container.Name, err)
		return
	}
	defer podLogs.Close()

	out := bufio.NewWriter(f)
	defer out.Flush()
	_, err = out.ReadFrom(podLogs)
	if errors.Is(err, io.ErrUnexpectedEOF) {
		// Failing to stream logs should not cause the test to fail
		Logf("Got error while streaming logs for pod %s/%s, container %s: %v", podNamespace, pod.Name, container.Name, err)
	}
}

func (acp *AzureClusterProxy) collectNodes(ctx context.Context, namespace string, name string, aboveMachinesPath string) {
	workload := acp.GetWorkloadCluster(ctx, namespace, name)
	nodes := &corev1.NodeList{}

	Expect(workload.GetClient().List(ctx, nodes)).To(Succeed())

	var err error
	var nodeDescribe string

	nodeDescriber, ok := describe.DescriberFor(schema.GroupKind{Group: corev1.GroupName, Kind: "Node"}, workload.GetRESTConfig())
	if !ok {
		Logf("failed to get node describer")
	}

	for _, node := range nodes.Items {
		// Describe the node.
		Logf("Describing Node %s", node.GetName())
		nodeDescribe, err = nodeDescriber.Describe(node.GetNamespace(), node.GetName(), describe.DescriberSettings{ShowEvents: true})
		if err != nil {
			Logf("failed to describe node %s: %v", node.GetName(), err)
		}

		describeFile := path.Join(aboveMachinesPath, nodesDir, node.GetName(), "node-describe.txt")
		writeLogFile(describeFile, nodeDescribe)
	}
}

func (acp *AzureClusterProxy) collectActivityLogs(ctx context.Context, namespace, name, aboveMachinesPath string) {
	timeoutctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cred, err := azidentity.NewDefaultAzureCredential(nil)
	Expect(err).NotTo(HaveOccurred())
	activityLogsClient, err := armmonitor.NewActivityLogsClient(getSubscriptionID(Default), cred, nil)
	Expect(err).NotTo(HaveOccurred())

	clusterClient := acp.GetClient()
	cluster := framework.GetClusterByName(ctx, framework.GetClusterByNameInput{
		Getter:    clusterClient,
		Name:      name,
		Namespace: namespace,
	})
	if cluster.Spec.InfrastructureRef == nil {
		Logf("No infrastructure for cluster %s/%s", namespace, name)
		return
	}

	var groupName string
	switch cluster.Spec.InfrastructureRef.Kind {
	case infrav1.AzureClusterKind:
		workloadCluster, err := getAzureCluster(timeoutctx, clusterClient, cluster.Spec.InfrastructureRef.Namespace, cluster.Spec.InfrastructureRef.Name)
		if err != nil {
			// Failing to fetch logs should not cause the test to fail
			Logf("Error fetching activity logs for cluster %s in namespace %s.  Not able to find the workload cluster on the management cluster: %v", name, namespace, err)
			return
		}
		groupName = workloadCluster.Spec.ResourceGroup
	case infrav1.AzureManagedClusterKind:
		if cluster.Spec.ControlPlaneRef == nil {
			Logf("No control plane for cluster %s/%s", namespace, name)
			return
		}
		controlPlane, err := getAzureManagedControlPlane(timeoutctx, clusterClient, cluster.Spec.ControlPlaneRef.Namespace, cluster.Spec.ControlPlaneRef.Name)
		if err != nil {
			// Failing to fetch logs should not cause the test to fail
			Logf("Error fetching activity logs for cluster %s in namespace %s.  Not able to find the AzureManagedControlPlane on the management cluster: %v", name, namespace, err)
			return
		}
		groupName = controlPlane.Spec.ResourceGroupName
	case infrav1.AzureASOManagedClusterKind:
		asoCluster, err := getAzureASOManagedCluster(timeoutctx, clusterClient, cluster.Spec.InfrastructureRef.Namespace, cluster.Spec.InfrastructureRef.Name)
		if err != nil {
			// Failing to fetch logs should not cause the test to fail
			Logf("Error fetching activity logs for cluster %s in namespace %s.  Not able to find the AzureASOManagedCluster on the management cluster: %v", name, namespace, err)
			return
		}
		for _, resource := range asoCluster.Spec.Resources {
			u := &unstructured.Unstructured{}
			Expect(u.UnmarshalJSON(resource.Raw)).To(Succeed())
			if u.GroupVersionKind().Kind != "ResourceGroup" {
				continue
			}
			// AzureName might not be specified in the CAPZ resource. GET the rg to make sure we have it.
			rg := &asoresourcesv1.ResourceGroup{}
			Expect(clusterClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: u.GetName()}, rg)).To(Succeed())
			groupName = rg.AzureName()
			break
		}
	}

	start := time.Now().Add(-2 * time.Hour).UTC().Format(time.RFC3339)
	end := time.Now().UTC().Format(time.RFC3339)

	filter := fmt.Sprintf("eventTimestamp ge '%s' and eventTimestamp le '%s' and resourceGroupName eq '%s'", start, end, groupName)
	pager := activityLogsClient.NewListPager(filter, nil)

	logFile := path.Join(aboveMachinesPath, activitylog, groupName+".log")
	Expect(os.MkdirAll(filepath.Dir(logFile), 0o755)).To(Succeed())

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		// Failing to fetch logs should not cause the test to fail
		Logf("Error opening file to write activity logs: %v", err)
		return
	}
	defer f.Close()
	out := bufio.NewWriter(f)
	defer out.Flush()

	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			// Failing to fetch logs should not cause the test to fail
			Logf("Error getting pager for activity logs in resource group %s: %v", groupName, err)
			return
		}
		for _, event := range page.Value {
			if ptr.Deref(event.Category.Value, "") != "Policy" {
				b, err := json.MarshalIndent(myEventData(*event), "", "    ")
				if err != nil {
					Logf("Got error converting activity logs data to json: %v", err)
				}
				if _, err = out.WriteString(string(b) + "\n"); err != nil {
					Logf("Got error while writing activity logs for resource group %s: %v", groupName, err)
				}
			}
		}
	}
}

func writeLogFile(logFilepath string, logData string) {
	go func() {
		defer GinkgoRecover()

		if err := os.MkdirAll(filepath.Dir(logFilepath), 0o755); err != nil {
			// Failing to mkdir should not cause the test to fail
			Logf("Error mkdir: %v", err)
			return
		}

		f, err := os.OpenFile(logFilepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			// Failing to open the file should not cause the test to fail
			Logf("Error opening file %s to write logs: %v", logFilepath, err)
			return
		}
		defer f.Close()

		out := bufio.NewWriter(f)
		defer out.Flush()
		_, err = out.WriteString(logData)
		if err != nil {
			// Failing to write a log file should not cause the test to fail
			Logf("failed to write logFile %s: %v", logFilepath, err)
		}
	}()
}
