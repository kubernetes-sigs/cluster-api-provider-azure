/*
Copyright 2023 The Kubernetes Authors.

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

package v1beta1

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
)

func TestAzureManagedControlPlane_SetDefaultSSHPublicKey(t *testing.T) {
	g := NewWithT(t)

	type test struct {
		m *AzureManagedControlPlane
	}

	existingPublicKey := "testpublickey"
	publicKeyExistTest := test{m: createAzureManagedControlPlaneWithSSHPublicKey(existingPublicKey)}
	publicKeyNotExistTest := test{m: createAzureManagedControlPlaneWithSSHPublicKey("")}

	err := publicKeyExistTest.m.setDefaultSSHPublicKey()
	g.Expect(err).To(BeNil())
	g.Expect(publicKeyExistTest.m.Spec.SSHPublicKey).To(Equal(existingPublicKey))

	err = publicKeyNotExistTest.m.setDefaultSSHPublicKey()
	g.Expect(err).To(BeNil())
	g.Expect(publicKeyNotExistTest.m.Spec.SSHPublicKey).NotTo(BeEmpty())
}

func createAzureManagedControlPlaneWithSSHPublicKey(sshPublicKey string) *AzureManagedControlPlane {
	return hardcodedAzureManagedControlPlaneWithSSHKey(sshPublicKey)
}

func hardcodedAzureManagedControlPlaneWithSSHKey(sshPublicKey string) *AzureManagedControlPlane {
	return &AzureManagedControlPlane{
		Spec: AzureManagedControlPlaneSpec{
			SSHPublicKey: sshPublicKey,
		},
	}
}

func TestSetDefaultAutoScalerProfile(t *testing.T) {
	m := &AzureManagedControlPlane{
		Spec: AzureManagedControlPlaneSpec{
			AutoScalerProfile: &AutoScalerProfile{},
		},
	}

	// Test when all fields are nil
	m.setDefaultAutoScalerProfile()

	g := NewWithT(t)

	// Verify that the default values are set
	g.Expect(m.Spec.AutoScalerProfile).NotTo(BeNil())
	g.Expect(m.Spec.AutoScalerProfile.BalanceSimilarNodeGroups).To(Equal((*BalanceSimilarNodeGroups)(pointer.String(string(BalanceSimilarNodeGroupsFalse)))))
	g.Expect(m.Spec.AutoScalerProfile.Expander).To(Equal((*Expander)(pointer.String(string(ExpanderRandom)))))
	g.Expect(m.Spec.AutoScalerProfile.MaxEmptyBulkDelete).To(Equal(pointer.String("10")))
	g.Expect(m.Spec.AutoScalerProfile.MaxGracefulTerminationSec).To(Equal(pointer.String("600")))
	g.Expect(m.Spec.AutoScalerProfile.MaxNodeProvisionTime).To(Equal(pointer.String("15m")))
	g.Expect(m.Spec.AutoScalerProfile.MaxTotalUnreadyPercentage).To(Equal(pointer.String("45")))
	g.Expect(m.Spec.AutoScalerProfile.NewPodScaleUpDelay).To(Equal(pointer.String("0s")))
	g.Expect(m.Spec.AutoScalerProfile.OkTotalUnreadyCount).To(Equal(pointer.String("3")))
	g.Expect(m.Spec.AutoScalerProfile.ScanInterval).To(Equal(pointer.String("10s")))
	g.Expect(m.Spec.AutoScalerProfile.ScaleDownDelayAfterAdd).To(Equal(pointer.String("10m")))
	g.Expect(m.Spec.AutoScalerProfile.ScaleDownDelayAfterDelete).To(Equal(m.Spec.AutoScalerProfile.ScanInterval))
	g.Expect(m.Spec.AutoScalerProfile.ScaleDownDelayAfterFailure).To(Equal(pointer.String("3m")))
	g.Expect(m.Spec.AutoScalerProfile.ScaleDownUnneededTime).To(Equal(pointer.String("10m")))
	g.Expect(m.Spec.AutoScalerProfile.ScaleDownUnreadyTime).To(Equal(pointer.String("20m")))
	g.Expect(m.Spec.AutoScalerProfile.ScaleDownUtilizationThreshold).To(Equal(pointer.String("0.5")))
	g.Expect(m.Spec.AutoScalerProfile.SkipNodesWithLocalStorage).To(Equal((*SkipNodesWithLocalStorage)(pointer.String(string(SkipNodesWithLocalStorageFalse)))))
	g.Expect(m.Spec.AutoScalerProfile.SkipNodesWithSystemPods).To(Equal((*SkipNodesWithSystemPods)(pointer.String(string(SkipNodesWithSystemPodsTrue)))))
}
