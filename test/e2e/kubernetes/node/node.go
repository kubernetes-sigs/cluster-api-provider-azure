//go:build e2e
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

package node

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"

	"sigs.k8s.io/cluster-api-provider-azure/test/e2e/kubernetes/windows"
)

const (
	nodeOperationTimeout             = 30 * time.Second
	nodeOperationSleepBetweenRetries = 3 * time.Second
)

func GetWindowsVersion(ctx context.Context, clientset *kubernetes.Clientset) (windows.OSVersion, error) {
	options := metav1.ListOptions{
		LabelSelector: "kubernetes.io/os=windows",
	}
	var result *corev1.NodeList
	Eventually(func(g Gomega) {
		var err error
		result, err = clientset.CoreV1().Nodes().List(ctx, options)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Items).NotTo(BeEmpty())
	}, nodeOperationTimeout, nodeOperationSleepBetweenRetries).Should(Succeed())

	kernalVersion := result.Items[0].Status.NodeInfo.KernelVersion
	kernalVersions := strings.Split(kernalVersion, ".")
	if len(kernalVersions) != 4 {
		return windows.Unknown, fmt.Errorf("not a valid Windows kernel version: %s", kernalVersion)
	}

	switch kernalVersions[2] {
	case "17763":
		return windows.LTSC2019, nil
	default:
		return windows.LTSC2019, nil
	}
}

func TaintNode(clientset *kubernetes.Clientset, options metav1.ListOptions, taint *corev1.Taint) error {
	var result *corev1.NodeList
	Eventually(func(g Gomega) {
		var err error
		result, err = clientset.CoreV1().Nodes().List(context.Background(), options)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.Items).NotTo(BeEmpty())
	}, nodeOperationTimeout, nodeOperationSleepBetweenRetries).Should(Succeed())

	for i := range result.Items {
		newNode, needsUpdate := addOrUpdateTaint(&result.Items[i], taint)
		if !needsUpdate {
			continue
		}

		err := PatchNodeTaints(clientset, newNode.Name, &result.Items[i], newNode)
		if err != nil {
			return err
		}
	}

	return nil
}

// PatchNodeTaints is taken from https://github.com/kubernetes/kubernetes/blob/v1.21.1/staging/src/k8s.io/cloud-provider/node/helpers/taints.go#L91
func PatchNodeTaints(clientset *kubernetes.Clientset, nodeName string, oldNode *corev1.Node, newNode *corev1.Node) error {
	oldData, err := json.Marshal(oldNode)
	if err != nil {
		return fmt.Errorf("failed to marshal old node %#v for node %q: %w", oldNode, nodeName, err)
	}

	newTaints := newNode.Spec.Taints
	newNodeClone := oldNode.DeepCopy()
	newNodeClone.Spec.Taints = newTaints
	newData, err := json.Marshal(newNodeClone)
	if err != nil {
		return fmt.Errorf("failed to marshal new node %#v for node %q: %w", newNodeClone, nodeName, err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, corev1.Node{})
	if err != nil {
		return fmt.Errorf("failed to create patch for node %q: %w", nodeName, err)
	}

	Eventually(func(g Gomega) {
		_, err := clientset.CoreV1().Nodes().Patch(context.Background(), nodeName, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
		if err != nil {
			log.Printf("Error updating node taints on node %s:%s\n", nodeName, err.Error())
		}
		g.Expect(err).NotTo(HaveOccurred())
	}, nodeOperationTimeout, nodeOperationSleepBetweenRetries).Should(Succeed())
	return err
}

// From https://github.com/kubernetes/kubernetes/blob/v1.21.1/staging/src/k8s.io/cloud-provider/node/helpers/taints.go#L116
// addOrUpdateTaint tries to add a taint to annotations list. Returns a new copy of updated Node and true if something was updated
// false otherwise.
func addOrUpdateTaint(node *corev1.Node, taint *corev1.Taint) (*corev1.Node, bool) {
	newNode := node.DeepCopy()
	nodeTaints := newNode.Spec.Taints

	var newTaints []corev1.Taint
	updated := false
	for i := range nodeTaints {
		if taint.MatchTaint(&nodeTaints[i]) {
			if equality.Semantic.DeepEqual(*taint, nodeTaints[i]) {
				return newNode, false
			}
			newTaints = append(newTaints, *taint)
			updated = true
			continue
		}

		newTaints = append(newTaints, nodeTaints[i])
	}

	if !updated {
		newTaints = append(newTaints, *taint)
	}

	newNode.Spec.Taints = newTaints
	return newNode, true
}
