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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"strings"
	"os"
	"path/filepath"

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
		deployment.DeepCopyInto(deployObj)
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
		service.DeepCopyInto(serviceObj)
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
	service := &corev1.Service{}
	Eventually(func() bool {
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

	}, intervals...).Should(BeTrue(), func() string { return DescribeFailedService(input, service) })
}

// DescribeFailedService returns a string with information to help debug a failed service.
func DescribeFailedService(input WaitForServiceAvailableInput, service *corev1.Service) string {
	b := strings.Builder{}
	b.WriteString(fmt.Sprintf("Service %s/%s failed to get an IP for LoadBalancer.Ingress",
		input.Service.GetNamespace(), input.Service.GetName()))
	if service == nil {
		b.WriteString("\nService: nil\n")
	} else {
		b.WriteString(fmt.Sprintf("\nService:\n%s\n", prettyPrint(service)))
	}
	return b.String()
}

// jobsClientAdapter adapts a Job to work with WaitForJobAvailable.
type jobsClientAdapter struct {
	client typedbatchv1.JobInterface
}

// Get fetches the job named by the key and updates the status of the provided object.
func (c jobsClientAdapter) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	job, err := c.client.Get(key.Name, metav1.GetOptions{})
	if jobObj, ok := obj.(*batchv1.Job); ok {
		job.DeepCopyInto(jobObj)
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
	job := &batchv1.Job{}
	Eventually(func() bool {
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
	}, intervals...).Should(BeTrue(), func() string { return DescribeFailedJob(input, job) })
}

// DescribeFailedJob returns a string with information to help debug a failed job.
func DescribeFailedJob(input WaitForJobCompleteInput, job *batchv1.Job) string {
	b := strings.Builder{}
	b.WriteString(fmt.Sprintf("Job %s/%s failed to get status.Complete = True condition with a successful job",
		input.Job.GetNamespace(), input.Job.GetName()))
	if job == nil {
		b.WriteString("\nJob: nil\n")
	} else {
		b.WriteString(fmt.Sprintf("\nJob:\n%s\n", prettyPrint(job)))
	}
	return b.String()
}

// prettyPrint returns a formatted JSON version of the object given.
func prettyPrint(v interface{}) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(b)
}

// getAvailabilityZonesForRegion uses zone information in availableZonesPerLocation.json
// and returns the number of availability zones per region that would support the VM type used for e2e tests.
// will return an error if the region isn't recognized
// availableZonesPerLocation.json was generated by
// az vm list-skus -r "virtualMachines"  -z | jq 'map({(.locationInfo[0].location + "_" + .name): .locationInfo[0].zones}) | add' > availableZonesPerLocation.json
func getAvailabilityZonesForRegion(location string, size string) ([]string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	file, err := ioutil.ReadFile(filepath.Join(wd, "data/availableZonesPerLocation.json"))
	if err != nil {
		return nil, err
	}
	var data map[string][]string

	if err = json.Unmarshal(file, &data); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("%s_%s", location, size)

	return data[key], nil
}
