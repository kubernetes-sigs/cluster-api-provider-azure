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

package pvc

import (
	"context"
	"log"
	"time"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
)

const (
	pvcOperationTimeout             = 30 * time.Second
	pvcOperationSleepBetweenRetries = 3 * time.Second
)

/*
Example PVC spec with sc name in annotation
---
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: dd-managed-hdd-5g
  annotations:
    volume.beta.kubernetes.io/storage-class: managedhdd
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 5Gi
*/

/*
Example PVC spec with sc name in storageClassName field
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: pvc-azuredisk
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 5Gi
  storageClassName: managed-csi
*/

type Builder struct {
	pvc *corev1.PersistentVolumeClaim
}

func Create(pvcName string, storageRequest string) (*Builder, error) {
	qunatity, err := resource.ParseQuantity(storageRequest)
	if err != nil {
		return nil, err
	}
	pvcBuilder := &Builder{
		pvc: &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name: pvcName,
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{
					corev1.ReadWriteOnce,
				},
				Resources: corev1.VolumeResourceRequirements{
					Requests: map[corev1.ResourceName]resource.Quantity{
						corev1.ResourceStorage: qunatity,
					},
				},
			},
		},
	}
	return pvcBuilder, nil
}

func (b *Builder) WithAnnotations(annotations map[string]string) *Builder {
	b.pvc.Annotations = annotations
	return b
}

func (b *Builder) WithStorageClass(scName string) *Builder {
	b.pvc.Spec.StorageClassName = ptr.To(scName)
	return b
}

func (b *Builder) DeployPVC(clientset *kubernetes.Clientset) error {
	Eventually(func(g Gomega) {
		_, err := clientset.CoreV1().PersistentVolumeClaims("default").Create(context.TODO(), b.pvc, metav1.CreateOptions{})
		if err != nil {
			log.Printf("Error trying to deploy storage class %s in namespace %s:%s\n", b.pvc.Name, b.pvc.ObjectMeta.Namespace, err.Error())
		}
		g.Expect(err).To(HaveOccurred())
	}, pvcOperationTimeout, pvcOperationSleepBetweenRetries).Should(Succeed())

	return nil
}
