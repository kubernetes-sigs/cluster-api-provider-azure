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

// Package deployment provides a helper utilities for building Kubernetes deployments within this test suite
package deployment

import (
	"context"
	"fmt"
	"log"
	"time"

	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	typedappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	ExternalLoadbalancer                   = LoadbalancerType("external")
	InternalLoadbalancer                   = LoadbalancerType("internal")
	deploymentOperationTimeout             = 30 * time.Second
	deploymentOperationSleepBetweenRetries = 3 * time.Second
)

type (
	// Builder provides a helper interface for building Kubernetes deployments within this test suite
	Builder struct {
		deployment *appsv1.Deployment
	}

	LoadbalancerType string
)

// Create will create a deployment for a given image with a name in a namespace
func Create(image, name, namespace string) *Builder {
	e2eDeployment := &Builder{
		deployment: &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    map[string]string{},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To[int32](1),
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": name,
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": name,
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  name,
								Image: image,
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("10m"),
										corev1.ResourceMemory: resource.MustParse("10M"),
									},
								},
							},
						},
						NodeSelector: map[string]string{
							"kubernetes.io/os": "linux",
						},
					},
				},
			},
		},
	}

	return e2eDeployment
}

func (d *Builder) AddLabels(labels map[string]string) {
	for k, v := range labels {
		d.deployment.ObjectMeta.Labels[k] = v
		d.deployment.Spec.Selector.MatchLabels[k] = v
		d.deployment.Spec.Template.ObjectMeta.Labels[k] = v
	}
}

func (d *Builder) GetName() string {
	return d.deployment.Name
}

func (d *Builder) SetReplicas(replicas int32) *Builder {
	d.deployment.Spec.Replicas = &replicas
	return d
}

func (d *Builder) AddContainerPort(name, portName string, portNumber int32, protocol corev1.Protocol) {
	for _, c := range d.deployment.Spec.Template.Spec.Containers {
		if c.Name == name {
			c.Ports = []corev1.ContainerPort{
				{
					Name:          portName,
					ContainerPort: portNumber,
					Protocol:      protocol,
				},
			}
		}
	}
}

func (d *Builder) AddPVC(pvcName string) *Builder {
	volumes := []corev1.Volume{
		{
			Name: "managed",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		},
	}
	d.deployment.Spec.Template.Spec.Volumes = volumes
	return d
}

func (d *Builder) Deploy(ctx context.Context, clientset *kubernetes.Clientset) (*appsv1.Deployment, error) {
	var deployment *appsv1.Deployment
	Eventually(func(g Gomega) {
		var err error
		deployment, err = d.Client(clientset).Create(ctx, d.deployment, metav1.CreateOptions{})
		if err != nil {
			log.Printf("Error trying to deploy %s in namespace %s:%s\n", d.deployment.Name, d.deployment.ObjectMeta.Namespace, err.Error())
		}
		g.Expect(err).NotTo(HaveOccurred())
	}, deploymentOperationTimeout, deploymentOperationSleepBetweenRetries).Should(Succeed())

	return deployment, nil
}

func (d *Builder) Client(clientset *kubernetes.Clientset) typedappsv1.DeploymentInterface {
	return clientset.AppsV1().Deployments(d.deployment.ObjectMeta.Namespace)
}

func (d *Builder) GetPodsFromDeployment(ctx context.Context, clientset *kubernetes.Clientset) ([]corev1.Pod, error) {
	opts := metav1.ListOptions{
		LabelSelector: labels.Set(d.deployment.Labels).String(),
		Limit:         100,
	}
	var pods *corev1.PodList
	Eventually(func(g Gomega) {
		var err error
		pods, err = clientset.CoreV1().Pods(d.deployment.GetNamespace()).List(ctx, opts)
		if err != nil {
			log.Printf("Error trying to get the pods from deployment %s:%s\n", d.deployment.GetName(), err.Error())
		}
		g.Expect(err).NotTo(HaveOccurred())
	}, deploymentOperationTimeout, deploymentOperationSleepBetweenRetries).Should(Succeed())
	return pods.Items, nil
}

func (d *Builder) CreateServiceResourceSpec(ports []corev1.ServicePort, lbtype LoadbalancerType, ipFamilies []corev1.IPFamily) *corev1.Service {
	suffix := "elb"
	annotations := map[string]string{}
	if lbtype == InternalLoadbalancer {
		suffix = "ilb"
		annotations["service.beta.kubernetes.io/azure-load-balancer-internal"] = "true"
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        fmt.Sprintf("%s-%s", d.deployment.Name, suffix),
			Namespace:   d.deployment.Namespace,
			Annotations: annotations,
		},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeLoadBalancer,
			Ports: ports,
			Selector: map[string]string{
				"app": d.deployment.Name,
			},
			IPFamilies: ipFamilies,
		},
	}
}

func (d *Builder) SetImage(name, image string) {
	for i, c := range d.deployment.Spec.Template.Spec.Containers {
		if c.Name == name {
			c.Image = image
			d.deployment.Spec.Template.Spec.Containers[i] = c
		}
	}
}

func (d *Builder) AddWindowsSelectors() *Builder {
	if d.deployment.Spec.Template.Spec.NodeSelector == nil {
		d.deployment.Spec.Template.Spec.NodeSelector = map[string]string{}
	}

	d.deployment.Spec.Template.Spec.NodeSelector["kubernetes.io/os"] = "windows"
	return d
}

// AddMachinePoolSelectors will add node selectors which will ensure the workload runs on a specific MachinePool
func (d *Builder) AddMachinePoolSelectors(machinePoolName string) *Builder {
	if d.deployment.Spec.Template.Spec.NodeSelector == nil {
		d.deployment.Spec.Template.Spec.NodeSelector = map[string]string{}
	}

	d.deployment.Spec.Template.Spec.NodeSelector[clusterv1.OwnerKindAnnotation] = "MachinePool"
	d.deployment.Spec.Template.Spec.NodeSelector[clusterv1.OwnerNameAnnotation] = machinePoolName
	return d
}

func (d *Builder) AddPodAntiAffinity(affinity corev1.PodAntiAffinity) *Builder {
	if d.deployment.Spec.Template.Spec.Affinity == nil {
		d.deployment.Spec.Template.Spec.Affinity = &corev1.Affinity{}
	}

	d.deployment.Spec.Template.Spec.Affinity.PodAntiAffinity = &affinity
	return d
}
