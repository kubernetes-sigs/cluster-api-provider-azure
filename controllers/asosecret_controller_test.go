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

package controllers

import (
	"context"
	"os"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestASOSecretReconcile(t *testing.T) {
	os.Setenv("AZURE_CLIENT_ID", "fooClient")             //nolint:tenv // we want to use os.Setenv here instead of t.Setenv
	os.Setenv("AZURE_CLIENT_SECRET", "fooSecret")         //nolint:tenv // we want to use os.Setenv here instead of t.Setenv
	os.Setenv("AZURE_TENANT_ID", "fooTenant")             //nolint:tenv // we want to use os.Setenv here instead of t.Setenv
	os.Setenv("AZURE_SUBSCRIPTION_ID", "fooSubscription") //nolint:tenv // we want to use os.Setenv here instead of t.Setenv

	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	_ = clientgoscheme.AddToScheme(scheme)

	defaultCluster := getASOCluster()
	defaultAzureCluster := getASOAzureCluster()
	defaultAzureManagedControlPlane := getASOAzureManagedControlPlane()
	defaultASOSecret := getASOSecret(defaultAzureCluster)
	defaultClusterIdentityType := infrav1.ServicePrincipal

	cases := map[string]struct {
		clusterName string
		objects     []runtime.Object
		err         string
		event       string
		asoSecret   *corev1.Secret
	}{
		"should not fail if the azure cluster is not found": {
			clusterName: defaultAzureCluster.Name,
			objects: []runtime.Object{
				getASOCluster(func(c *clusterv1.Cluster) {
					c.Spec.InfrastructureRef.Name = defaultAzureCluster.Name
					c.Spec.InfrastructureRef.Kind = defaultAzureCluster.Kind
				}),
			},
		},
		"should not fail for AzureCluster without ownerRef set yet": {
			clusterName: defaultAzureCluster.Name,
			objects: []runtime.Object{
				getASOAzureCluster(func(c *infrav1.AzureCluster) {
					c.ObjectMeta.OwnerReferences = nil
				}),
				defaultCluster,
			},
		},
		"should reconcile normally for AzureCluster with IdentityRef configured": {
			clusterName: defaultAzureCluster.Name,
			objects: []runtime.Object{
				getASOAzureCluster(func(c *infrav1.AzureCluster) {
					c.Spec.IdentityRef = &corev1.ObjectReference{
						Name:      "my-azure-cluster-identity",
						Namespace: "default",
					}
				}),
				getASOAzureClusterIdentity(func(identity *infrav1.AzureClusterIdentity) {
					identity.Spec.Type = defaultClusterIdentityType
					identity.Spec.ClientSecret = corev1.SecretReference{
						Name:      "fooSecret",
						Namespace: "default",
					}
				}),
				getASOAzureClusterIdentitySecret(),
				defaultCluster,
			},
			asoSecret: getASOSecret(defaultAzureCluster, func(s *corev1.Secret) {
				s.Data = map[string][]byte{
					"AZURE_SUBSCRIPTION_ID": []byte("123"),
					"AZURE_TENANT_ID":       []byte("fooTenant"),
					"AZURE_CLIENT_ID":       []byte("fooClient"),
					"AZURE_CLIENT_SECRET":   []byte("fooSecret"),
				}
			}),
		},
		"should reconcile normally for AzureManagedControlPlane with IdentityRef configured": {
			clusterName: defaultAzureManagedControlPlane.Name,
			objects: []runtime.Object{
				getASOAzureManagedControlPlane(func(c *infrav1.AzureManagedControlPlane) {
					c.Spec.IdentityRef = &corev1.ObjectReference{
						Name:      "my-azure-cluster-identity",
						Namespace: "default",
					}
				}),
				getASOAzureClusterIdentity(func(identity *infrav1.AzureClusterIdentity) {
					identity.Spec.Type = defaultClusterIdentityType
					identity.Spec.ClientSecret = corev1.SecretReference{
						Name:      "fooSecret",
						Namespace: "default",
					}
				}),
				getASOAzureClusterIdentitySecret(),
				defaultCluster,
			},
			asoSecret: getASOSecret(defaultAzureManagedControlPlane, func(s *corev1.Secret) {
				s.Data = map[string][]byte{
					"AZURE_SUBSCRIPTION_ID": []byte("fooSubscription"),
					"AZURE_TENANT_ID":       []byte("fooTenant"),
					"AZURE_CLIENT_ID":       []byte("fooClient"),
					"AZURE_CLIENT_SECRET":   []byte("fooSecret"),
				}
			}),
		},
		"should reconcile normally for AzureManagedControlPlane with IdentityRef configured of type Service Principal with Certificate": {
			clusterName: defaultAzureManagedControlPlane.Name,
			objects: []runtime.Object{
				getASOAzureManagedControlPlane(func(c *infrav1.AzureManagedControlPlane) {
					c.Spec.IdentityRef = &corev1.ObjectReference{
						Name:      "my-azure-cluster-identity",
						Namespace: "default",
					}
				}),
				getASOAzureClusterIdentity(func(identity *infrav1.AzureClusterIdentity) {
					identity.Spec.Type = infrav1.ServicePrincipalCertificate
					identity.Spec.CertPath = "../test/setup/certificate"
				}),
				defaultCluster,
			},
			asoSecret: getASOSecret(defaultAzureManagedControlPlane, func(s *corev1.Secret) {
				s.Data = map[string][]byte{
					"AZURE_SUBSCRIPTION_ID":             []byte("fooSubscription"),
					"AZURE_TENANT_ID":                   []byte("fooTenant"),
					"AZURE_CLIENT_ID":                   []byte("fooClient"),
					"AZURE_CLIENT_CERTIFICATE_PASSWORD": []byte(""),
					"AZURE_CLIENT_CERTIFICATE":          []byte("-----BEGIN PRIVATE KEY-----\nMIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDjrdEr9P0TaUES\ndspE6cyo22NU8yhRrbYlV9VH2vWvnPsThXcxhnd+cUqdNEBswhwgFlUQcg/eSVxw\nrr+3nh+bFTZWPcY+1LQYxfpKGsrCXQfB82LDJIZDX4gHYrWf3Z272jXN1XeFAKti\nwDKgDXXuPH7r5lH7vC3RXeAffqLwQJhZf+NoHNtv9MH9IdUkQfmDFZtI/CQzCrb6\n+vOS6EmUD/Q2FNHBzgxCguGqgNyBcQbxJ9Qng+ZjIFuhGYXJlsyRUtexyzTR5/v0\nVNK8UsZgRBFhXqrBv/RoCCG+xVJYtmd0QsrvNzDqG6QnjUB21zVXqzKEkW2gRtjX\ncw4vYQehAgMBAAECggEAS6xtjg0nAokk0jS+ZOpKlkMZAFaza3ZvyHipkHDz4PMt\ntl7Rb5oQZGvWT2rbEOrxey7BBi7LHGhIu8ExQp/hRGPoBAETP7XlyCghWPkPtEtE\ndU/mXxLoN0NszHuf/2si7pmH8YqGZ6QB0tgr22ut60mbK+AJFsEEf4aSpBUspepJ\n2800sQHsqPE6L6kYkfZ2GRRY1V9vUrYEODKZpWzMhN3UA9nAKH9PB6xvP2OdyMNh\nhKgmUUMNIFtwr8pZlJn60cf0UrWrc5CvqQLuaGYlzDgUQGV4JEVjqm9F6lMfEPUw\neN70MVe1pcLeLq2rGCVWU3gakh/HvJqlR/sa546HgwKBgQDyf1vkyX4w5sboi6DJ\ncl5dMULtMMRpB1OaMFVOJjI9gZJ8mCdRjqXdYo5aS2KIqxie8tGG9+SohxDAWl4t\nlSUtDsE44fSmILqC5zIawNRQnnkv0X8LwmYu0Qd7YAjJMlLTWyDRsjD9XRq4nsR+\nmJVwrt85iSpS5UFyryEzPbFj0wKBgQDwWzraeN0Eccf1iIYmQsYy+yMEAlHNR5yi\ngPXuAhSybv2JReRhdUb39hLr/LvKw0ZeXiLWXmYUGpbyzPyXIm0s+PL3LWl65GTF\nl+cfV5wfAdDkk6rAdEPEE2pxN85ChyaPYPoYr0ohmV97VQcYc5FqY+j1tM6R1RDt\n/fWBSa8iOwKBgQCpa1dtWWTDj4gqUdrswu2wmEkU47xlUIwVLm164u64z/zi9X6K\n2WmCaWfhJ8fYigjyi9zdOfXT1EFc0gX4PLozZ5qRPjQpmLYV3KbB0DTFemJaiTgE\npDW1wa5DgQ3CW1lIduNP/fmCGfkgQTQw6jOF/XbRgMZEEg2OrVI5tYFopwKBgER9\niqjEth5VGejCjY+LiZTvcUvsKUk4tc6stueqmiE6dW7PhsOqup1f9oZej1i5Cm1L\nn9u8LJRf+1GWzgd3HOsqyXlb7GnDeV/A6HBK88b2KoNn/Mk4mDLgYX1/rHvSrU9A\nECRGlvY6ETZAxXPXQsGxVKnnatGtiFR5AKNlzs0PAoGAa5+X+DUqGh9aE5ID3wrv\njkjxQ2KLFJCNSq8f9GSuvpvgXstHh6wKoM6vMwIShjgXuURH8Ub4uhRsWnxMildF\n7EE+QaWU9jnCm2HQYArfXrAWw6DBudiSkBqgKc6HjDHun5fXlYUo8UesNMQOrg7b\nbydQZ5/4V/1oSWPETk7jSr0=\n-----END PRIVATE KEY-----\n-----BEGIN CERTIFICATE-----\nMIIDCTCCAfGgAwIBAgIUFSntEn+Tv6HM2xJReECJpJcC7iUwDQYJKoZIhvcNAQEL\nBQAwFDESMBAGA1UEAwwJbG9jYWxob3N0MB4XDTI0MDEwODE5NTQxNFoXDTM0MDEw\nNTE5NTQxNFowFDESMBAGA1UEAwwJbG9jYWxob3N0MIIBIjANBgkqhkiG9w0BAQEF\nAAOCAQ8AMIIBCgKCAQEA463RK/T9E2lBEnbKROnMqNtjVPMoUa22JVfVR9r1r5z7\nE4V3MYZ3fnFKnTRAbMIcIBZVEHIP3klccK6/t54fmxU2Vj3GPtS0GMX6ShrKwl0H\nwfNiwySGQ1+IB2K1n92du9o1zdV3hQCrYsAyoA117jx+6+ZR+7wt0V3gH36i8ECY\nWX/jaBzbb/TB/SHVJEH5gxWbSPwkMwq2+vrzkuhJlA/0NhTRwc4MQoLhqoDcgXEG\n8SfUJ4PmYyBboRmFyZbMkVLXscs00ef79FTSvFLGYEQRYV6qwb/0aAghvsVSWLZn\ndELK7zcw6hukJ41Adtc1V6syhJFtoEbY13MOL2EHoQIDAQABo1MwUTAdBgNVHQ4E\nFgQUfry/KDtamwMlRQsFPbBhzdv2U5cwHwYDVR0jBBgwFoAUfry/KDtamwMlRQsF\nPbBhzdv2U5cwDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0BAQsFAAOCAQEAyYst\nVvewKRRpuYRWc4XG6WnYphUdyZLMoIlq0syZ1aj6YbqoK9NMHAYEnCvSov6zIZOa\ntrhuUcf9GFz5e0iJ2zIlDc312Iwsv41xiC/bs16kEn8Yf/SujEXasj7vmA3HrFWf\nwZTH/yFL5azo/f+lA1Q28YwqFpHmle0y0O53Uth4p0tmwlnu+CrO9fHp3kTlb7fD\n6mqfk9Nrt8tOC4aHYDoqtYUgZhx58xsHMOTetKeRlp8HMF9oROtriz4nYm6IhTwo\n5k1A13S3BjaxkZCyPXCgXssuXagNLasrr5Qq+Vgdb/nDhVehV8+Z4J0Ynzy9MZsE\nH1N1NfMtsA+PEqtPXA==\n-----END CERTIFICATE-----\n"),
				}
			}),
		},
		"should reconcile normally for AzureCluster with an IdentityRef of type WorkloadIdentity": {
			clusterName: defaultAzureCluster.Name,
			objects: []runtime.Object{
				getASOAzureCluster(func(c *infrav1.AzureCluster) {
					c.Spec.IdentityRef = &corev1.ObjectReference{
						Name:      "my-azure-cluster-identity",
						Namespace: "default",
					}
				}),
				getASOAzureClusterIdentity(func(identity *infrav1.AzureClusterIdentity) {
					identity.Spec.Type = "WorkloadIdentity"
				}),
				defaultCluster,
			},
			asoSecret: getASOSecret(defaultAzureCluster, func(s *corev1.Secret) {
				s.Data = map[string][]byte{
					"AZURE_SUBSCRIPTION_ID": []byte("123"),
					"AZURE_TENANT_ID":       []byte("fooTenant"),
					"AZURE_CLIENT_ID":       []byte("fooClient"),
					"AUTH_MODE":             []byte("workloadidentity"),
				}
			}),
		},
		"should reconcile normally for AzureManagedControlPlane with an IdentityRef of type WorkloadIdentity": {
			clusterName: defaultAzureManagedControlPlane.Name,
			objects: []runtime.Object{
				getASOAzureManagedControlPlane(func(c *infrav1.AzureManagedControlPlane) {
					c.Spec.IdentityRef = &corev1.ObjectReference{
						Name:      "my-azure-cluster-identity",
						Namespace: "default",
					}
				}),
				getASOAzureClusterIdentity(func(identity *infrav1.AzureClusterIdentity) {
					identity.Spec.Type = infrav1.WorkloadIdentity
				}),
				defaultCluster,
			},
			asoSecret: getASOSecret(defaultAzureManagedControlPlane, func(s *corev1.Secret) {
				s.Data = map[string][]byte{
					"AZURE_SUBSCRIPTION_ID": []byte("fooSubscription"),
					"AZURE_TENANT_ID":       []byte("fooTenant"),
					"AZURE_CLIENT_ID":       []byte("fooClient"),
					"AUTH_MODE":             []byte("workloadidentity"),
				}
			}),
		},
		"should reconcile normally for AzureCluster with an IdentityRef of type UserAssignedMSI": {
			clusterName: defaultAzureCluster.Name,
			objects: []runtime.Object{
				getASOAzureCluster(func(c *infrav1.AzureCluster) {
					c.Spec.IdentityRef = &corev1.ObjectReference{
						Name:      "my-azure-cluster-identity",
						Namespace: "default",
					}
				}),
				getASOAzureClusterIdentity(func(identity *infrav1.AzureClusterIdentity) {
					identity.Spec.Type = infrav1.UserAssignedMSI
				}),
				defaultCluster,
			},
			asoSecret: getASOSecret(defaultAzureCluster, func(s *corev1.Secret) {
				s.Data = map[string][]byte{
					"AZURE_SUBSCRIPTION_ID": []byte("123"),
					"AZURE_TENANT_ID":       []byte("fooTenant"),
					"AZURE_CLIENT_ID":       []byte("fooClient"),
					"AUTH_MODE":             []byte("podidentity"),
				}
			}),
		},
		"should reconcile normally for AzureManagedControlPlane with an IdentityRef of type UserAssignedMSI": {
			clusterName: defaultAzureManagedControlPlane.Name,
			objects: []runtime.Object{
				getASOAzureManagedControlPlane(func(c *infrav1.AzureManagedControlPlane) {
					c.Spec.IdentityRef = &corev1.ObjectReference{
						Name:      "my-azure-cluster-identity",
						Namespace: "default",
					}
				}),
				getASOAzureClusterIdentity(func(identity *infrav1.AzureClusterIdentity) {
					identity.Spec.Type = infrav1.UserAssignedMSI
				}),
				defaultCluster,
			},
			asoSecret: getASOSecret(defaultAzureManagedControlPlane, func(s *corev1.Secret) {
				s.Data = map[string][]byte{
					"AZURE_SUBSCRIPTION_ID": []byte("fooSubscription"),
					"AZURE_TENANT_ID":       []byte("fooTenant"),
					"AZURE_CLIENT_ID":       []byte("fooClient"),
					"AUTH_MODE":             []byte("podidentity"),
				}
			}),
		},
		"should fail if IdentityRef secret doesn't exist": {
			clusterName: defaultAzureManagedControlPlane.Name,
			objects: []runtime.Object{
				getASOAzureManagedControlPlane(func(c *infrav1.AzureManagedControlPlane) {
					c.Spec.IdentityRef = &corev1.ObjectReference{
						Name:      "my-azure-cluster-identity",
						Namespace: "default",
					}
				}),
				getASOAzureClusterIdentity(func(identity *infrav1.AzureClusterIdentity) {
					identity.Spec.Type = defaultClusterIdentityType
					identity.Spec.ClientSecret = corev1.SecretReference{
						Name:      "fooSecret",
						Namespace: "default",
					}
				}),
				defaultCluster,
			},
			err: "secrets \"fooSecret\" not found",
		},
		"should return if cluster does not exist": {
			clusterName: defaultAzureCluster.Name,
			objects: []runtime.Object{
				defaultAzureCluster,
			},
			err: "failed to get Cluster/my-cluster: clusters.cluster.x-k8s.io \"my-cluster\" not found",
		},
		"should return if cluster is paused": {
			clusterName: defaultAzureCluster.Name,
			objects: []runtime.Object{
				getASOCluster(func(c *clusterv1.Cluster) {
					c.Spec.Paused = true
				}),
				getASOAzureCluster(func(c *infrav1.AzureCluster) {
					c.Spec.IdentityRef = &corev1.ObjectReference{
						Name:      "my-azure-cluster-identity",
						Namespace: "default",
					}
				}),
				getASOAzureClusterIdentity(func(identity *infrav1.AzureClusterIdentity) {
					identity.Spec.Type = defaultClusterIdentityType
					identity.Spec.ClientSecret = corev1.SecretReference{
						Name:      "fooSecret",
						Namespace: "default",
					}
				}),
				getASOAzureClusterIdentitySecret(),
			},
			event: "AzureCluster or linked Cluster is marked as paused. Won't reconcile",
		},
		"should return if azureCluster is not yet available": {
			clusterName: defaultAzureCluster.Name,
			objects: []runtime.Object{
				defaultCluster,
			},
			event: "AzureClusterObjectNotFound AzureCluster object default/my-azure-cluster not found",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			g := NewWithT(t)
			clientBuilder := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tc.objects...).Build()

			reconciler := &ASOSecretReconciler{
				Client:   clientBuilder,
				Recorder: record.NewFakeRecorder(128),
			}

			_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: "default",
					Name:      tc.clusterName,
				},
			})

			existingASOSecret := &corev1.Secret{}
			asoSecretErr := clientBuilder.Get(context.Background(), types.NamespacedName{
				Namespace: defaultASOSecret.Namespace,
				Name:      defaultASOSecret.Name,
			}, existingASOSecret)

			if tc.asoSecret != nil {
				g.Expect(asoSecretErr).NotTo(HaveOccurred())
				g.Expect(tc.asoSecret.Data).To(BeEquivalentTo(existingASOSecret.Data))
			} else {
				g.Expect(asoSecretErr).To(HaveOccurred())
			}

			if tc.err != "" {
				g.Expect(err).To(MatchError(ContainSubstring(tc.err)))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
			if tc.event != "" {
				g.Expect(reconciler.Recorder.(*record.FakeRecorder).Events).To(Receive(ContainSubstring(tc.event)))
			}
		})
	}
}

func getASOCluster(changes ...func(*clusterv1.Cluster)) *clusterv1.Cluster {
	input := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster",
			Namespace: "default",
		},
		Spec: clusterv1.ClusterSpec{
			InfrastructureRef: &corev1.ObjectReference{
				APIVersion: infrav1.GroupVersion.String(),
			},
		},
		Status: clusterv1.ClusterStatus{
			InfrastructureReady: true,
		},
	}

	for _, change := range changes {
		change(input)
	}

	return input
}

func getASOAzureCluster(changes ...func(*infrav1.AzureCluster)) *infrav1.AzureCluster {
	input := &infrav1.AzureCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-azure-cluster",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       "Cluster",
					Name:       "my-cluster",
				},
			},
		},
		Spec: infrav1.AzureClusterSpec{
			AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
				SubscriptionID: "123",
			},
		},
	}
	for _, change := range changes {
		change(input)
	}

	return input
}

func getASOAzureManagedControlPlane(changes ...func(*infrav1.AzureManagedControlPlane)) *infrav1.AzureManagedControlPlane {
	input := &infrav1.AzureManagedControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-azure-managed-control-plane",
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       "my-cluster",
					Kind:       "Cluster",
					APIVersion: clusterv1.GroupVersion.String(),
				},
			},
		},
		Spec: infrav1.AzureManagedControlPlaneSpec{},
		Status: infrav1.AzureManagedControlPlaneStatus{
			Ready:       true,
			Initialized: true,
		},
	}
	for _, change := range changes {
		change(input)
	}

	return input
}

func getASOAzureClusterIdentity(changes ...func(identity *infrav1.AzureClusterIdentity)) *infrav1.AzureClusterIdentity {
	input := &infrav1.AzureClusterIdentity{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-azure-cluster-identity",
			Namespace: "default",
		},
		Spec: infrav1.AzureClusterIdentitySpec{
			ClientID: "fooClient",
			TenantID: "fooTenant",
		},
	}

	for _, change := range changes {
		change(input)
	}

	return input
}

func getASOAzureClusterIdentitySecret(changes ...func(secret *corev1.Secret)) *corev1.Secret {
	input := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fooSecret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"clientSecret": []byte("fooSecret"),
		},
	}

	for _, change := range changes {
		change(input)
	}

	return input
}

func getASOSecret(cluster client.Object, changes ...func(secret *corev1.Secret)) *corev1.Secret {
	input := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster-aso-secret",
			Namespace: "default",
			Labels: map[string]string{
				"my-cluster": "owned",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: cluster.GetObjectKind().GroupVersionKind().GroupVersion().String(),
					Kind:       cluster.GetObjectKind().GroupVersionKind().Kind,
					Name:       cluster.GetName(),
					UID:        cluster.GetUID(),
					Controller: ptr.To(true),
				},
			},
		},
		Data: map[string][]byte{
			"AZURE_SUBSCRIPTION_ID": []byte("fooSubscription"),
		},
	}

	for _, change := range changes {
		change(input)
	}

	return input
}
