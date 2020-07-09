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
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	typedappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	typedbatchv1 "k8s.io/client-go/kubernetes/typed/batch/v1"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// deploymentsClientAdapter adapts a Deployment to work with WaitForDeploymentsAvailable in the
// CAPI e2e test framework.
type deploymentsClientAdapter struct {
	client typedappsv1.DeploymentInterface
}

// Get fetches the deployment named by the key and updates the status of the provided object.
func (c deploymentsClientAdapter) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	deployment, err := c.client.Get(key.Name, metav1.GetOptions{})
	if deployObj, ok := obj.(*appsv1.Deployment); ok {
		deployObj.Status = deployment.Status
	}
	return err
}

// servicesClientAdapter adapts a Service to work with WaitForServicesAvailable.
type servicesClientAdapter struct {
	client typedcorev1.ServiceInterface
}

// Get fetches the service named by the key and updates the status of the provided object.
func (c servicesClientAdapter) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	service, err := c.client.Get(key.Name, metav1.GetOptions{})
	if serviceObj, ok := obj.(*corev1.Service); ok {
		serviceObj.Status = service.Status
	}
	return err
}

// WaitForServiceAvailableInput is the input for WaitForServiceAvailable.
type WaitForServiceAvailableInput struct {
	Getter  framework.Getter
	Service *corev1.Service
}

// WaitForServiceAvailable waits until the Service has an IP address available on each Ingress.
func WaitForServiceAvailable(ctx context.Context, input WaitForServiceAvailableInput, intervals ...interface{}) {
	By(fmt.Sprintf("waiting for service %s/%s to be available", input.Service.GetNamespace(), input.Service.GetName()))
	Eventually(func() bool {
		service := &corev1.Service{}
		key := client.ObjectKey{
			Namespace: input.Service.GetNamespace(),
			Name:      input.Service.GetName(),
		}
		if err := input.Getter.Get(ctx, key, service); err != nil {
			return false
		}
		if service.Status.LoadBalancer.Ingress != nil {
			for _, i := range service.Status.LoadBalancer.Ingress {
				if net.ParseIP(i.IP) == nil {
					return false
				}
			}
			return true
		}
		return false

	}, intervals...).Should(BeTrue(), "Service %s/%s failed to get an IP for LoadBalancer.Ingress", input.Service.GetNamespace(), input.Service.GetName())
}

// jobsClientAdapter adapts a Job to work with WaitForJobAvailable.
type jobsClientAdapter struct {
	client typedbatchv1.JobInterface
}

// Get fetches the job named by the key and updates the status of the provided object.
func (c jobsClientAdapter) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	job, err := c.client.Get(key.Name, metav1.GetOptions{})
	if jobObj, ok := obj.(*batchv1.Job); ok {
		jobObj.Status = job.Status
	}
	return err
}

// WaitForJobCompleteInput is the input for WaitForJobComplete.
type WaitForJobCompleteInput struct {
	Getter framework.Getter
	Job    *batchv1.Job
}

// WaitForJobComplete waits until the Job completes with at least one success.
func WaitForJobComplete(ctx context.Context, input WaitForJobCompleteInput, intervals ...interface{}) {
	By(fmt.Sprintf("waiting for job %s/%s to be complete", input.Job.GetNamespace(), input.Job.GetName()))
	Eventually(func() bool {
		job := &batchv1.Job{}
		key := client.ObjectKey{
			Namespace: input.Job.GetNamespace(),
			Name:      input.Job.GetName(),
		}
		if err := input.Getter.Get(ctx, key, job); err != nil {
			return false
		}
		for _, c := range job.Status.Conditions {
			if c.Type == batchv1.JobComplete && c.Status == corev1.ConditionTrue {
				return job.Status.Succeeded > 0
			}
		}
		return false
	}, intervals...).Should(BeTrue(), "Job %s/%s failed to get status.Complete = True condition with a successful job", input.Job.GetNamespace(), input.Job.GetName())
}
