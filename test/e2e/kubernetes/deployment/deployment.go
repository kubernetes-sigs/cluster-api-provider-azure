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

package deployment

import (
	"fmt"
	typedappsv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"log"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
)

type deploymentBuilder struct {
	deployment *appsv1.Deployment
}

type LoadbalancerType string

const (
	ExternalLoadbalancer = LoadbalancerType("external")
	InternalLoadbalancer = LoadbalancerType("internal")
)

// CreateDeployment will create a deployment for a given image with a name in a namespace
func CreateDeployment(image, name, namespace string) *deploymentBuilder {
	e2eDeployment := &deploymentBuilder{
		deployment: &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
				Labels:    map[string]string{},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: pointer.Int32Ptr(1),
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

func (d *deploymentBuilder) AddLabels(labels map[string]string) {
	for k, v := range labels {
		d.deployment.ObjectMeta.Labels[k] = v
		d.deployment.Spec.Selector.MatchLabels[k] = v
		d.deployment.Spec.Template.ObjectMeta.Labels[k] = v
	}
}

func (d *deploymentBuilder) AddContainerPort(name, portName string, portNumber int32, protocol corev1.Protocol) {
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

func (d *deploymentBuilder) Deploy(clientset *kubernetes.Clientset) (*appsv1.Deployment, error) {
	deployment, err := d.Client(clientset).Create(d.deployment)
	if err != nil {
		log.Printf("Error trying to deploy %s in namespace %s:%s\n", d.deployment.Name, d.deployment.ObjectMeta.Namespace, err.Error())
		return nil, err
	}

	return deployment, nil
}

func (d *deploymentBuilder) Client(clientset *kubernetes.Clientset) typedappsv1.DeploymentInterface {
	return clientset.AppsV1().Deployments(d.deployment.ObjectMeta.Namespace)
}

func (d *deploymentBuilder) GetPodsFromDeployment(clientset *kubernetes.Clientset) ([]corev1.Pod, error) {
	opts := metav1.ListOptions{
		LabelSelector: labels.Set(d.deployment.Labels).String(),
		Limit:         100,
	}
	pods, err := clientset.CoreV1().Pods(d.deployment.GetNamespace()).List(opts)
	if err != nil {
		log.Printf("Error trying to get the pods from deployment %s:%s\n", d.deployment.GetName(), err.Error())
		return nil, err
	}
	return pods.Items, nil
}

func (d *deploymentBuilder) GetService(ports []corev1.ServicePort, lbtype LoadbalancerType) *corev1.Service {
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
		},
	}
}

func (d *deploymentBuilder) SetImage(name, image string) {
	for i, c := range d.deployment.Spec.Template.Spec.Containers {
		if c.Name == name {
			c.Image = image
			d.deployment.Spec.Template.Spec.Containers[i] = c
		}
	}
}

func (d *deploymentBuilder) AddWindowsSelectors() {
	if d.deployment.Spec.Template.Spec.NodeSelector == nil {
		d.deployment.Spec.Template.Spec.NodeSelector = map[string]string{}
	}

	d.deployment.Spec.Template.Spec.NodeSelector["kubernetes.io/os"] = "windows"
}
