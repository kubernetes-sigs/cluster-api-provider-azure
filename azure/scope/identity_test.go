/*
Copyright 2021 The Kubernetes Authors.

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

package scope

import (
	"context"
	"encoding/base64"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAllowedNamespaces(t *testing.T) {
	g := NewWithT(t)
	scheme := runtime.NewScheme()
	_ = infrav1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	tests := []struct {
		name             string
		identity         *infrav1.AzureClusterIdentity
		clusterNamespace string
		expected         bool
	}{
		{
			name: "allow any cluster namespace when empty",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					AllowedNamespaces: &infrav1.AllowedNamespaces{},
				},
			},
			clusterNamespace: "default",
			expected:         true,
		},
		{
			name: "no namespaces allowed when list is empty",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					AllowedNamespaces: &infrav1.AllowedNamespaces{
						NamespaceList: []string{},
					},
				},
			},
			clusterNamespace: "default",
			expected:         false,
		},
		{
			name: "allow cluster with namespace in list",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					AllowedNamespaces: &infrav1.AllowedNamespaces{
						NamespaceList: []string{"namespace24", "namespace32"},
					},
				},
			},
			clusterNamespace: "namespace24",
			expected:         true,
		},
		{
			name: "don't allow cluster with namespace not in list",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					AllowedNamespaces: &infrav1.AllowedNamespaces{
						NamespaceList: []string{"namespace24", "namespace32"},
					},
				},
			},
			clusterNamespace: "namespace8",
			expected:         false,
		},
		{
			name: "allow cluster when namespace has selector with matching label",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					AllowedNamespaces: &infrav1.AllowedNamespaces{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"c": "d"},
						},
					},
				},
			},
			clusterNamespace: "namespace8",
			expected:         true,
		},
		{
			name: "don't allow cluster when namespace has selector with different label",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					AllowedNamespaces: &infrav1.AllowedNamespaces{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"x": "y"},
						},
					},
				},
			},
			clusterNamespace: "namespace8",
			expected:         false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeNamespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "namespace8",
					Labels: map[string]string{"c": "d"},
				},
			}
			initObjects := []runtime.Object{tc.identity, fakeNamespace}
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()

			actual := IsClusterNamespaceAllowed(context.TODO(), fakeClient, tc.identity.Spec.AllowedNamespaces, tc.clusterNamespace)
			g.Expect(actual).To(Equal(tc.expected))
		})
	}
}

func TestHasClientSecret(t *testing.T) {
	tests := []struct {
		name     string
		identity *infrav1.AzureClusterIdentity
		want     bool
	}{
		{
			name: "user assigned identity",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type: infrav1.UserAssignedMSI,
				},
			},
			want: false,
		},
		{
			name: "service principal with secret",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:         infrav1.ServicePrincipal,
					ClientSecret: corev1.SecretReference{Name: "my-client-secret"},
				},
			},
			want: true,
		},
		{
			name: "service principal with certificate",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:         infrav1.ServicePrincipalCertificate,
					ClientSecret: corev1.SecretReference{Name: "my-client-secret"},
				},
			},
			want: true,
		},
		{
			name: "manual service principal",
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:         infrav1.ManualServicePrincipal,
					ClientSecret: corev1.SecretReference{Name: "my-client-secret"},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &AzureCredentialsProvider{
				Identity: tt.identity,
			}
			if got := p.hasClientSecret(); got != tt.want {
				t.Errorf("AzureCredentialsProvider.hasClientSecret() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetTokenCredential(t *testing.T) {
	g := NewWithT(t)

	// Test cert data was generated with this command:
	//    openssl req -x509 -noenc -days 3650 -newkey rsa:2048 --keyout - -subj /CN=localhost | base64
	encodedCertData := "LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk1JSUV2UUlCQURBTkJna3Foa2lHOXcwQkFRRUZBQVNDQktjd2dnU2pBZ0VBQW9JQkFRRGpyZEVyOVAwVGFVRVMKZHNwRTZjeW8yMk5VOHloUnJiWWxWOVZIMnZXdm5Qc1RoWGN4aG5kK2NVcWRORUJzd2h3Z0ZsVVFjZy9lU1Z4dwpyciszbmgrYkZUWldQY1krMUxRWXhmcEtHc3JDWFFmQjgyTERKSVpEWDRnSFlyV2YzWjI3MmpYTjFYZUZBS3RpCndES2dEWFh1UEg3cjVsSDd2QzNSWGVBZmZxTHdRSmhaZitOb0hOdHY5TUg5SWRVa1FmbURGWnRJL0NRekNyYjYKK3ZPUzZFbVVEL1EyRk5IQnpneENndUdxZ055QmNRYnhKOVFuZytaaklGdWhHWVhKbHN5UlV0ZXh5elRSNS92MApWTks4VXNaZ1JCRmhYcXJCdi9Sb0NDRyt4VkpZdG1kMFFzcnZOekRxRzZRbmpVQjIxelZYcXpLRWtXMmdSdGpYCmN3NHZZUWVoQWdNQkFBRUNnZ0VBUzZ4dGpnMG5Bb2trMGpTK1pPcEtsa01aQUZhemEzWnZ5SGlwa0hEejRQTXQKdGw3UmI1b1FaR3ZXVDJyYkVPcnhleTdCQmk3TEhHaEl1OEV4UXAvaFJHUG9CQUVUUDdYbHlDZ2hXUGtQdEV0RQpkVS9tWHhMb04wTnN6SHVmLzJzaTdwbUg4WXFHWjZRQjB0Z3IyMnV0NjBtYksrQUpGc0VFZjRhU3BCVXNwZXBKCjI4MDBzUUhzcVBFNkw2a1lrZloyR1JSWTFWOXZVcllFT0RLWnBXek1oTjNVQTluQUtIOVBCNnh2UDJPZHlNTmgKaEtnbVVVTU5JRnR3cjhwWmxKbjYwY2YwVXJXcmM1Q3ZxUUx1YUdZbHpEZ1VRR1Y0SkVWanFtOUY2bE1mRVBVdwplTjcwTVZlMXBjTGVMcTJyR0NWV1UzZ2FraC9IdkpxbFIvc2E1NDZIZ3dLQmdRRHlmMXZreVg0dzVzYm9pNkRKCmNsNWRNVUx0TU1ScEIxT2FNRlZPSmpJOWdaSjhtQ2RSanFYZFlvNWFTMktJcXhpZTh0R0c5K1NvaHhEQVdsNHQKbFNVdERzRTQ0ZlNtSUxxQzV6SWF3TlJRbm5rdjBYOEx3bVl1MFFkN1lBakpNbExUV3lEUnNqRDlYUnE0bnNSKwptSlZ3cnQ4NWlTcFM1VUZ5cnlFelBiRmowd0tCZ1FEd1d6cmFlTjBFY2NmMWlJWW1Rc1l5K3lNRUFsSE5SNXlpCmdQWHVBaFN5YnYySlJlUmhkVWIzOWhMci9Mdkt3MFplWGlMV1htWVVHcGJ5elB5WEltMHMrUEwzTFdsNjVHVEYKbCtjZlY1d2ZBZERrazZyQWRFUEVFMnB4Tjg1Q2h5YVBZUG9ZcjBvaG1WOTdWUWNZYzVGcVkrajF0TTZSMVJEdAovZldCU2E4aU93S0JnUUNwYTFkdFdXVERqNGdxVWRyc3d1MndtRWtVNDd4bFVJd1ZMbTE2NHU2NHovemk5WDZLCjJXbUNhV2ZoSjhmWWlnanlpOXpkT2ZYVDFFRmMwZ1g0UExvelo1cVJQalFwbUxZVjNLYkIwRFRGZW1KYWlUZ0UKcERXMXdhNURnUTNDVzFsSWR1TlAvZm1DR2ZrZ1FUUXc2ak9GL1hiUmdNWkVFZzJPclZJNXRZRm9wd0tCZ0VSOQppcWpFdGg1VkdlakNqWStMaVpUdmNVdnNLVWs0dGM2c3R1ZXFtaUU2ZFc3UGhzT3F1cDFmOW9aZWoxaTVDbTFMCm45dThMSlJmKzFHV3pnZDNIT3NxeVhsYjdHbkRlVi9BNkhCSzg4YjJLb05uL01rNG1ETGdZWDEvckh2U3JVOUEKRUNSR2x2WTZFVFpBeFhQWFFzR3hWS25uYXRHdGlGUjVBS05senMwUEFvR0FhNStYK0RVcUdoOWFFNUlEM3dydgpqa2p4UTJLTEZKQ05TcThmOUdTdXZwdmdYc3RIaDZ3S29NNnZNd0lTaGpnWHVVUkg4VWI0dWhSc1dueE1pbGRGCjdFRStRYVdVOWpuQ20ySFFZQXJmWHJBV3c2REJ1ZGlTa0JxZ0tjNkhqREh1bjVmWGxZVW84VWVzTk1RT3JnN2IKYnlkUVo1LzRWLzFvU1dQRVRrN2pTcjA9Ci0tLS0tRU5EIFBSSVZBVEUgS0VZLS0tLS0KLS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURDVENDQWZHZ0F3SUJBZ0lVRlNudEVuK1R2NkhNMnhKUmVFQ0pwSmNDN2lVd0RRWUpLb1pJaHZjTkFRRUwKQlFBd0ZERVNNQkFHQTFVRUF3d0piRzlqWVd4b2IzTjBNQjRYRFRJME1ERXdPREU1TlRReE5Gb1hEVE0wTURFdwpOVEU1TlRReE5Gb3dGREVTTUJBR0ExVUVBd3dKYkc5allXeG9iM04wTUlJQklqQU5CZ2txaGtpRzl3MEJBUUVGCkFBT0NBUThBTUlJQkNnS0NBUUVBNDYzUksvVDlFMmxCRW5iS1JPbk1xTnRqVlBNb1VhMjJKVmZWUjlyMXI1ejcKRTRWM01ZWjNmbkZLblRSQWJNSWNJQlpWRUhJUDNrbGNjSzYvdDU0Zm14VTJWajNHUHRTMEdNWDZTaHJLd2wwSAp3Zk5pd3lTR1ExK0lCMksxbjkyZHU5bzF6ZFYzaFFDcllzQXlvQTExN2p4KzYrWlIrN3d0MFYzZ0gzNmk4RUNZCldYL2phQnpiYi9UQi9TSFZKRUg1Z3hXYlNQd2tNd3EyK3Zyemt1aEpsQS8wTmhUUndjNE1Rb0xocW9EY2dYRUcKOFNmVUo0UG1ZeUJib1JtRnlaYk1rVkxYc2NzMDBlZjc5RlRTdkZMR1lFUVJZVjZxd2IvMGFBZ2h2c1ZTV0xabgpkRUxLN3pjdzZodWtKNDFBZHRjMVY2c3loSkZ0b0ViWTEzTU9MMkVIb1FJREFRQUJvMU13VVRBZEJnTlZIUTRFCkZnUVVmcnkvS0R0YW13TWxSUXNGUGJCaHpkdjJVNWN3SHdZRFZSMGpCQmd3Rm9BVWZyeS9LRHRhbXdNbFJRc0YKUGJCaHpkdjJVNWN3RHdZRFZSMFRBUUgvQkFVd0F3RUIvekFOQmdrcWhraUc5dzBCQVFzRkFBT0NBUUVBeVlzdApWdmV3S1JScHVZUldjNFhHNlduWXBoVWR5WkxNb0lscTBzeVoxYWo2WWJxb0s5Tk1IQVlFbkN2U292NnpJWk9hCnRyaHVVY2Y5R0Z6NWUwaUoyeklsRGMzMTJJd3N2NDF4aUMvYnMxNmtFbjhZZi9TdWpFWGFzajd2bUEzSHJGV2YKd1pUSC95Rkw1YXpvL2YrbEExUTI4WXdxRnBIbWxlMHkwTzUzVXRoNHAwdG13bG51K0NyTzlmSHAza1RsYjdmRAo2bXFmazlOcnQ4dE9DNGFIWURvcXRZVWdaaHg1OHhzSE1PVGV0S2VSbHA4SE1GOW9ST3RyaXo0blltNkloVHdvCjVrMUExM1MzQmpheGtaQ3lQWENnWHNzdVhhZ05MYXNycjVRcStWZ2RiL25EaFZlaFY4K1o0SjBZbnp5OU1ac0UKSDFOMU5mTXRzQStQRXF0UFhBPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo="
	certPEM, err := base64.StdEncoding.DecodeString(encodedCertData)
	g.Expect(err).NotTo(HaveOccurred())

	tests := []struct {
		name                         string
		cluster                      *infrav1.AzureCluster
		secret                       *corev1.Secret
		identity                     *infrav1.AzureClusterIdentity
		ActiveDirectoryAuthorityHost string
	}{
		{
			name: "workload identity",
			cluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
				},
			},
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.WorkloadIdentity,
					ClientID: fakeClientID,
					TenantID: fakeTenantID,
				},
			},
		},
		{
			name: "manual service principal",
			cluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
				},
			},
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.ManualServicePrincipal,
					TenantID: fakeTenantID,
					ClientSecret: corev1.SecretReference{
						Name: "test-identity-secret",
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-identity-secret",
				},
				Data: map[string][]byte{
					"clientSecret": []byte("fooSecret"),
				},
			},
			ActiveDirectoryAuthorityHost: "https://login.microsoftonline.com",
		},
		{
			name: "service principal",
			cluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
				},
			},
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.ServicePrincipal,
					TenantID: fakeTenantID,
					ClientSecret: corev1.SecretReference{
						Name: "test-identity-secret",
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-identity-secret",
				},
				Data: map[string][]byte{
					"clientSecret": []byte("fooSecret"),
				},
			},
			ActiveDirectoryAuthorityHost: "https://login.microsoftonline.com",
		},
		{
			name: "service principal certificate",
			cluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
				},
			},
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.ServicePrincipalCertificate,
					TenantID: fakeTenantID,
					ClientSecret: corev1.SecretReference{
						Name: "test-identity-secret",
					},
				},
			},
			secret: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-identity-secret",
				},
				Data: map[string][]byte{
					"clientSecret": certPEM,
				},
			},
		},
		{
			name: "user-assigned identity",
			cluster: &infrav1.AzureCluster{
				Spec: infrav1.AzureClusterSpec{
					AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
						IdentityRef: &corev1.ObjectReference{
							Kind: infrav1.AzureClusterIdentityKind,
						},
					},
				},
			},
			identity: &infrav1.AzureClusterIdentity{
				Spec: infrav1.AzureClusterIdentitySpec{
					Type:     infrav1.UserAssignedMSI,
					TenantID: fakeTenantID,
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	_ = infrav1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			initObjects := []runtime.Object{tt.cluster}
			if tt.identity != nil {
				initObjects = append(initObjects, tt.identity)
			}
			if tt.secret != nil {
				initObjects = append(initObjects, tt.secret)
			}
			fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(initObjects...).Build()
			provider, err := NewAzureClusterCredentialsProvider(context.Background(), fakeClient, tt.cluster)
			g.Expect(err).NotTo(HaveOccurred())
			cred, err := provider.GetTokenCredential(context.Background(), "", tt.ActiveDirectoryAuthorityHost, "")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cred).NotTo(BeNil())
		})
	}
}
