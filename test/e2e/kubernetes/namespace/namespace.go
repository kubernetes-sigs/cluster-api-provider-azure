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

// Package namespace provides utilities for working with Kubernetes Namespace resources.
package namespace

import (
	"context"
	"log"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	namespaceOperationTimeout             = 30 * time.Second
	namespaceOperationSleepBetweenRetries = 3 * time.Second
)

// Create a namespace with the given name
func Create(ctx context.Context, clientset *kubernetes.Clientset, name string, labels map[string]string) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}

	var namespace *corev1.Namespace
	Eventually(func(g Gomega) {
		var err error
		namespace, err = clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
		if err != nil {
			log.Printf("failed trying to create namespace (%s):%s\n", name, err.Error())
		}
		g.Expect(err).NotTo(HaveOccurred())
	}, namespaceOperationTimeout, namespaceOperationSleepBetweenRetries).Should(Succeed())
	return namespace, nil
}

// CreateNamespaceDeleteIfExist creates a namespace, deletes it first if it already exists
func CreateNamespaceDeleteIfExist(ctx context.Context, clientset *kubernetes.Clientset, name string, labels map[string]string) (*corev1.Namespace, error) {
	n, err := Get(ctx, clientset, name)
	if err == nil {
		// Delete existing namespace if exists to avoid dirty exit in last round of test
		log.Printf("namespace %s already exist", n)
		err := clientset.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
		if err != nil {
			log.Printf("Error trying to delete namespace (%s):%s\n", name, err.Error())
			return nil, err
		}
	}
	log.Printf("namespace %s does not exist, creating...", name)
	return Create(ctx, clientset, name, labels)
}

// Get returns a namespace for with a given name
func Get(ctx context.Context, clientset *kubernetes.Clientset, name string) (*corev1.Namespace, error) {
	opts := metav1.GetOptions{}
	namespace, err := clientset.CoreV1().Namespaces().Get(ctx, name, opts)
	if err != nil {
		log.Printf("failed trying to get namespace (%s):%s\n", name, err.Error())
		return nil, err
	}

	return namespace, nil
}
