// +build e2e

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

package e2e

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	e2e_pod "sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/pod"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kinderrors "sigs.k8s.io/kind/pkg/errors"
	"sigs.k8s.io/yaml"
)

// AzureTimeSyncSpecInput is the input for AzureTimeSyncSpec.
type AzureTimeSyncSpecInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Namespace             *corev1.Namespace
	ClusterName           string
}

// AzureTimeSyncSpec implements a test that verifies time synchronization is healthy for
// the nodes in a cluster.
func AzureTimeSyncSpec(ctx context.Context, inputGetter func() AzureTimeSyncSpecInput) {
	var (
		specName = "azure-timesync"
		input    AzureTimeSyncSpecInput
		thirty   = 30 * time.Second
	)

	input = inputGetter()
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	namespace, clusterName := input.Namespace.Name, input.ClusterName
	Eventually(func() error {
		sshInfo, err := getClusterSSHInfo(ctx, input.BootstrapClusterProxy, namespace, clusterName)
		if err != nil {
			return err
		}

		if len(sshInfo) <= 0 {
			return errors.New("sshInfo did not contain any machines")
		}

		var testFuncs []func() error
		for _, s := range sshInfo {
			Byf("checking that time synchronization is healthy on %s", s.Hostname)
			// must capture for closure
			machineInfo := s
			execToStringFn := func(expected, command string, args ...string) func() error {
				// don't assert in this test func, just return errors
				return func() error {
					f := &strings.Builder{}
					if err := execOnHost(machineInfo.Endpoint, machineInfo.Hostname, machineInfo.Port, f, command, args...); err != nil {
						return err
					}
					if !strings.Contains(f.String(), expected) {
						return fmt.Errorf("expected \"%s\" in command output:\n%s", expected, f.String())
					}
					return nil
				}
			}

			// May need to break this up on clusters with larger number of nodes.  There is a 10 ssh connection default.
			if machineInfo.IsWindows {
				//skip for now
				Logf("Skipping windows time sync check.  TODO: re-enable and check w32t service is running. Issue #1782")
			} else {
				testFuncs = append(testFuncs,
					execToStringFn(
						"✓ chronyd is active",
						"systemctl", "is-active", "chronyd", "&&",
						"echo", "✓ chronyd is active",
					),
					execToStringFn(
						"Reference ID",
						"chronyc", "tracking",
					),
				)
			}
		}

		return kinderrors.AggregateConcurrent(testFuncs)
	}, thirty, thirty).Should(Succeed())
}

const (
	nsenterWorkloadFile = "workloads/nsenter/daemonset.yaml"
)

// AzureDaemonsetTimeSyncSpec implements a test that verifies time synchronization is healthy for
// the nodes in a cluster. It uses a privileged daemonset and nsenter instead of SSH.
func AzureDaemonsetTimeSyncSpec(ctx context.Context, inputGetter func() AzureTimeSyncSpecInput) {
	var (
		specName = "azure-timesync"
		input    AzureTimeSyncSpecInput
		sixty    = 60 * time.Second
		five     = 5 * time.Second
	)

	input = inputGetter()
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
	namespace, clusterName := input.Namespace.Name, input.ClusterName
	workloadCluster := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, namespace, clusterName)
	kubeclient := workloadCluster.GetClient()
	clientset := workloadCluster.GetClientSet()
	config := workloadCluster.GetRESTConfig()
	var nsenterDs unstructured.Unstructured

	yamlData, err := ioutil.ReadFile(nsenterWorkloadFile)
	Expect(err).NotTo(HaveOccurred(), "failed to unmarshal nsenter daemonset from yaml")

	var jsonData []byte
	jsonData, err = yaml.YAMLToJSON(yamlData)
	Expect(err).NotTo(HaveOccurred(), "failed to convert nsenter yaml to json")

	err = nsenterDs.UnmarshalJSON(jsonData)
	Expect(err).NotTo(HaveOccurred(), "failed to unmarshal nsenter json bytes to unstructured")

	nsenterDs.SetNamespace("default")

	err = kubeclient.Create(ctx, &nsenterDs)
	Expect(err).NotTo(HaveOccurred(), "failed to create daemonset for time sync check")

	var nodes corev1.NodeList
	kubeclient.List(ctx, &nodes)
	Expect(err).NotTo(HaveOccurred(), "failed to list nodes for daemonset timesync check")
	Expect(len(nodes.Items)).Should(BeNumerically(">", 1), "expected to find >= 1 node for timesync check")
	desired := int32(len(nodes.Items))

	Eventually(func() error {
		var ds appsv1.DaemonSet
		if err := kubeclient.Get(ctx, types.NamespacedName{"default", "nsenter"}, &ds); err != nil {
			Logf("failed to get nsenter ds: %s", err)
			return err
		}
		ready := ds.Status.NumberReady
		available := ds.Status.NumberAvailable
		allReadyAndAvailable := desired == ready && desired == available
		generationOk := ds.ObjectMeta.Generation == ds.Status.ObservedGeneration

		msg := fmt.Sprintf("want %d instances, found %d ready and %d available. generation: %d, observedGeneration: %d", desired, ready, available, ds.ObjectMeta.Generation, ds.Status.ObservedGeneration)
		Logf(msg)
		if allReadyAndAvailable && generationOk {
			return nil
		}
		return fmt.Errorf(msg)
	}, sixty, five).Should(Succeed())

	var podList corev1.PodList
	err = kubeclient.List(ctx, &podList, client.MatchingLabels(map[string]string{"app": "nsenter"}))
	Expect(err).NotTo(HaveOccurred(), "failed to list pods for daemonset timesync check")

	Logf("mapping nsenter pods to hostnames for host-by-host execution")
	podMap := map[string]corev1.Pod{}
	for _, pod := range podList.Items {
		podMap[pod.Spec.NodeName] = pod
	}

	for k, v := range podMap {
		Logf("found host %s with pod %s", k, v.Name)
	}

	Eventually(func() error {
		execInfo, err := getClusterSSHInfo(ctx, input.BootstrapClusterProxy, namespace, clusterName)
		if err != nil {
			return err
		}

		if len(execInfo) <= 0 {
			return errors.New("execInfo did not contain any machines")
		}

		nsenterCmd := []string{"nsenter", "--target", "1", "--mount=/proc/1/ns/mnt", "--", "/bin/sh", "-c"}
		var testFuncs []func() error
		for key := range execInfo {
			key := key
			s := execInfo[key]
			Byf("checking that time synchronization is healthy on %s", s.Hostname)
			execToStringFn := func(expected, command string) func() error {
				// don't assert in this test func, just return errors
				return func() error {
					pod, exists := podMap[s.Hostname]
					if !exists {
						return fmt.Errorf("failed to find pod matching host %s", s.Hostname)
					}

					fullCommand := make([]string, 0, len(nsenterCmd)+1)
					fullCommand = append(fullCommand, nsenterCmd...)
					fullCommand = append(fullCommand, command)
					stdout, err := e2e_pod.ExecWithOutput(clientset, config, pod, fullCommand)
					if err != nil {
						return fmt.Errorf("failed to nsenter host %s, error: '%s', stdout:  '%s'", s.Hostname, err, stdout.String())
					}

					if !strings.Contains(stdout.String(), expected) {
						return fmt.Errorf("expected \"%s\" in command output:\n%s", expected, stdout.String())
					}

					Byf("time sync OK for host %s", s.Hostname)

					return nil
				}
			}

			testFuncs = append(testFuncs,
				execToStringFn(
					"chronyd is active",
					"systemctl is-active chronyd && echo chronyd is active",
				),
				execToStringFn(
					"Reference ID",
					"chronyc tracking",
				),
			)
		}

		return kinderrors.AggregateConcurrent(testFuncs)
	}, sixty, five).Should(Succeed())
}
