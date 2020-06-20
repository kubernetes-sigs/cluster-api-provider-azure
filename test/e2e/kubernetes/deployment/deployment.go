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
	"log"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
)

// CreateLinuxDeployment will create a deployment for a given image with a name in a namespace
func CreateLinuxDeployment(clientset *kubernetes.Clientset, image, name, namespace string, labels map[string]string) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
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
				},
			},
		},
	}

	deployment, err := clientset.AppsV1().Deployments(namespace).Create(deployment)
	if err != nil {
		log.Printf("Error trying to deploy %s [%s] in namespace %s:%s\n", name, image, namespace, err.Error())
		return nil, err
	}

	return deployment, nil
}

func GetPodsFromDeployment(clientset *kubernetes.Clientset, deployment *appsv1.Deployment) ([]corev1.Pod, error) {
	opts := metav1.ListOptions{
		LabelSelector: labels.Set(deployment.Labels).String(),
		Limit:         100,
	}
	pods, err := clientset.CoreV1().Pods(deployment.GetNamespace()).List(opts)
	if err != nil {
		log.Printf("Error trying to get the pods from deployment %s:%s\n", deployment.GetName(), err.Error())
		return nil, err
	}
	return pods.Items, nil
}
