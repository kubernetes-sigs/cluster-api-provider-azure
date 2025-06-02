/*
Copyright 2019 The Kubernetes Authors.

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
	"fmt"
	"time"

	"github.com/pkg/errors"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/groups"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/hcpopenshiftclusters"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/hcpopenshiftclustersexternalauth"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/keyvaults"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/networksecuritygroups"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/resourceskus"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/roleassignmentsaso"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/subnets"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/userassignedidentities"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/vaults"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/virtualnetworks"
	cplane "sigs.k8s.io/cluster-api-provider-azure/exp/api/controlplane/v1beta2"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// aroControlPlaneService is the reconciler called by the AROControlPlane controller.
type aroControlPlaneService struct {
	scope *scope.AROControlPlaneScope
	// services is the list of services that are reconciled by this controller.
	// The order of the services is important as it determines the order in which the services are reconciled.
	services   []azure.ServiceReconciler
	skuCache   *resourceskus.Cache
	Reconcile  func(context.Context) error
	Pause      func(context.Context) error
	Delete     func(context.Context) error
	kubeclient client.Client
}

// newAROControlPlaneService populates all the services based on input scope.
func newAROControlPlaneService(scope *scope.AROControlPlaneScope) (*aroControlPlaneService, error) {
	skuCache, err := resourceskus.GetCache(scope, scope.Location())
	if err != nil {
		return nil, errors.Wrap(err, "failed creating a NewCache")
	}
	keyVaultSvc, err := keyvaults.New(scope)
	if err != nil {
		return nil, err
	}
	hpcOpenshiftASOSvc, err := hcpopenshiftclusters.New(scope)
	if err != nil {
		return nil, err
	}
	hpcOpenshiftExternalAuthSvc, err := hcpopenshiftclustersexternalauth.New(scope)
	if err != nil {
		return nil, err
	}
	acs := &aroControlPlaneService{
		kubeclient: scope.Client,
		scope:      scope,
		services: []azure.ServiceReconciler{
			groups.New(scope),
			networksecuritygroups.New(scope),
			virtualnetworks.New(scope),
			subnets.New(scope),
			vaults.New(scope),
			keyVaultSvc,
			userassignedidentities.New(scope),
			roleassignmentsaso.New(scope),
			hpcOpenshiftASOSvc,          // ASO-based cluster provisioning
			hpcOpenshiftExternalAuthSvc, // ASO-based external auth configuration
			// hpcOpenshiftSecretsSvc removed - kubeconfig now comes from ASO secret
		},
		skuCache: skuCache,
	}
	acs.Reconcile = acs.reconcile
	acs.Pause = acs.pause
	acs.Delete = acs.delete

	return acs, nil
}

// Reconcile reconciles all the services in a predetermined order.
func (s *aroControlPlaneService) reconcile(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.aroControlPlaneService.Reconcile")
	defer done()

	for _, service := range s.services {
		serviceName := service.Name()
		log.V(4).Info(fmt.Sprintf("reconcile-service: %s", serviceName))
		if err := service.Reconcile(ctx); err != nil {
			// Special handling for external auth to set condition
			if serviceName == "hcpopenshiftclustersexternalauth" {
				conditions.MarkFalse(s.scope.ControlPlane, cplane.ExternalAuthReadyCondition,
					"ReconciliationFailed", clusterv1.ConditionSeverityWarning, "%s", err.Error())
			}
			return errors.Wrapf(err, "failed to reconcile AROControlPlane service %s", service.Name())
		}

		// Mark external auth as ready if reconciliation succeeded
		if serviceName == "hcpopenshiftclustersexternalauth" && s.scope.ControlPlane.Spec.EnableExternalAuthProviders {
			conditions.MarkTrue(s.scope.ControlPlane, cplane.ExternalAuthReadyCondition)
		}
	}

	// This ensures we always have fresh credentials and avoids cluster connectivity issues
	if err := s.reconcileKubeconfig(ctx); err != nil {
		return errors.Wrap(err, "failed to reconcile kubeconfig secret")
	}

	return nil
}

// Pause pauses all components making up the cluster.
func (s *aroControlPlaneService) pause(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "controllers.aroControlPlaneService.Pause")
	defer done()

	for _, service := range s.services {
		pauser, ok := service.(azure.Pauser)
		if !ok {
			continue
		}
		if err := pauser.Pause(ctx); err != nil {
			return errors.Wrapf(err, "failed to pause AROControlPlane service %s", service.Name())
		}
	}

	return nil
}

// Delete reconciles all the services in a predetermined order.
func (s *aroControlPlaneService) delete(ctx context.Context) error {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "controllers.aroControlPlaneService.Delete")
	defer done()

	// If the resource group is not managed we need to delete resources inside the group one by one.
	// services are deleted in reverse order from the order in which they are reconciled.
	for i := len(s.services) - 1; i >= 0; i-- {
		if err := s.services[i].Delete(ctx); err != nil {
			return errors.Wrapf(err, "failed to delete AROControlPlane service %s", s.services[i].Name())
		}
	}

	return nil
}

func (s *aroControlPlaneService) reconcileKubeconfig(ctx context.Context) error {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.aroControlPlaneService.reconcileKubeconfig")
	defer done()

	// Check if reconciliation is actually needed (metadata-based validation)
	if !s.scope.ShouldReconcileKubeconfig(ctx) {
		log.V(4).Info("kubeconfig is still valid, skipping reconciliation")
		return nil
	}

	log.V(4).Info("reconciling kubeconfig secret")

	// Get the admin kubeconfig data from ASO-created secret
	kubeconfigData, err := s.getKubeconfigFromASOSecret(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get kubeconfig from ASO secret")
	}
	if len(kubeconfigData) == 0 {
		return errors.New("no kubeconfig data available from ASO secret")
	}

	// Parse the kubeconfig to work with it
	kubeconfigFile, err := clientcmd.Load(kubeconfigData)
	if err != nil {
		return errors.Wrap(err, "failed to parse kubeconfig data")
	}

	// Token-based expiration handling
	var tokenExpiresIn *time.Duration

	// Check if this kubeconfig uses token-based authentication
	for _, authInfo := range kubeconfigFile.AuthInfos {
		if authInfo.Token != "" {
			// If we have token expiration info from scope, use it
			if s.scope.KubeonfigExpirationTimestamp != nil {
				tokenExpiresIn = ptr.To(time.Until(*s.scope.KubeonfigExpirationTimestamp))
				log.V(4).Info("kubeconfig token expiration", "expiresIn", tokenExpiresIn, "expiresAt", s.scope.KubeonfigExpirationTimestamp)
			}
			break
		}
	}

	// Handle certificate authority data
	var caData []byte
	if len(kubeconfigFile.Contexts) > 0 && len(kubeconfigFile.Clusters) > 0 {
		currentContext := kubeconfigFile.CurrentContext
		if currentContext == "" && len(kubeconfigFile.Contexts) == 1 {
			// Use the only context if current context is not set
			for contextName := range kubeconfigFile.Contexts {
				currentContext = contextName
				break
			}
		}

		if currentContext != "" {
			context := kubeconfigFile.Contexts[currentContext]
			if context != nil && context.Cluster != "" {
				cluster := kubeconfigFile.Clusters[context.Cluster]
				if cluster != nil {
					caData = cluster.CertificateAuthorityData
					if caData == nil && cluster.Server != "" {
						// ASO-created kubeconfig doesn't include CA certificate
						// Use insecure-skip-tls-verify as workaround
						log.V(4).Info("no CA data in kubeconfig from ASO, setting insecure skip TLS verify", "server", cluster.Server)
						cluster.InsecureSkipTLSVerify = true
					}

					if caData != nil {
						// Create/update CA secret
						caSecret := s.scope.MakeClusterCA()
						if _, err := controllerutil.CreateOrUpdate(ctx, s.kubeclient, caSecret, func() error {
							caSecret.Data = map[string][]byte{
								secret.TLSCrtDataName: caData,
								secret.TLSKeyDataName: []byte("placeholder"), // Required by CAPI
							}
							return nil
						}); err != nil {
							return errors.Wrap(err, "failed to reconcile certificate authority data secret")
						}
					}
				}
			}
		}
	}

	// Serialize the updated kubeconfig
	kubeconfigAdmin, err := clientcmd.Write(*kubeconfigFile)
	if err != nil {
		return errors.Wrap(err, "failed to serialize kubeconfig")
	}

	// Update the ASO-created kubeconfig secret directly
	// We need to preserve the secret's existing metadata (owner references from ASO)
	// and only update the data and add our tracking annotations
	secretName := secret.Name(s.scope.Cluster.Name, secret.Kubeconfig)
	kubeConfigSecret := s.scope.MakeEmptyKubeConfigSecret()

	// Get the existing secret to preserve its metadata
	if err := s.kubeclient.Get(ctx, client.ObjectKey{
		Namespace: s.scope.Namespace(),
		Name:      secretName,
	}, &kubeConfigSecret); err != nil {
		return errors.Wrap(err, "failed to get existing kubeconfig secret")
	}

	// Update the secret data with our modified kubeconfig
	if kubeConfigSecret.Data == nil {
		kubeConfigSecret.Data = make(map[string][]byte)
	}
	kubeConfigSecret.Data[secret.KubeconfigDataName] = kubeconfigAdmin

	// Ensure proper labels
	if kubeConfigSecret.Labels == nil {
		kubeConfigSecret.Labels = make(map[string]string)
	}
	kubeConfigSecret.Labels[clusterv1.ClusterNameLabel] = s.scope.ClusterName()

	// Add annotations for tracking
	if kubeConfigSecret.Annotations == nil {
		kubeConfigSecret.Annotations = make(map[string]string)
	}
	kubeConfigSecret.Annotations["aro.azure.com/kubeconfig-last-updated"] = time.Now().Format(time.RFC3339)

	// Remove refresh-needed annotation if it exists
	delete(kubeConfigSecret.Annotations, "aro.azure.com/kubeconfig-refresh-needed")

	// Add token expiration info if available
	if tokenExpiresIn != nil && *tokenExpiresIn > 0 {
		expirationTime := time.Now().Add(*tokenExpiresIn)
		kubeConfigSecret.Annotations["aro.azure.com/token-expires-at"] = expirationTime.Format(time.RFC3339)
	}

	// Update the secret (preserving existing owner references from ASO)
	if err := s.kubeclient.Update(ctx, &kubeConfigSecret); err != nil {
		return errors.Wrap(err, "failed to update kubeconfig secret")
	}

	// Store cluster-info if we have CA data
	if caData != nil {
		if err := s.scope.StoreClusterInfo(ctx, caData); err != nil {
			return errors.Wrap(err, "failed to construct cluster-info")
		}
	}

	// TODO: Add user kubeconfig support if needed
	// This would follow the same pattern as admin kubeconfig

	log.V(4).Info("successfully reconciled kubeconfig secret")
	return nil
}

func (s *aroControlPlaneService) getService(name string) (azure.ServiceReconciler, error) {
	for _, service := range s.services {
		if service.Name() == name {
			return service, nil
		}
	}
	return nil, errors.Errorf("service %s not found", name)
}

// getKubeconfigFromASOSecret reads the kubeconfig from the ASO-created secret.
func (s *aroControlPlaneService) getKubeconfigFromASOSecret(ctx context.Context) ([]byte, error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "controllers.aroControlPlaneService.getKubeconfigFromASOSecret")
	defer done()

	// The ASO secret name follows CAPI convention: {cluster-name}-kubeconfig
	secretName := secret.Name(s.scope.Cluster.Name, secret.Kubeconfig)
	asoSecret := s.scope.MakeEmptyKubeConfigSecret()

	log.V(4).Info("reading kubeconfig from ASO secret", "secretName", secretName, "namespace", s.scope.Namespace())

	err := s.kubeclient.Get(ctx, client.ObjectKey{
		Namespace: s.scope.Namespace(),
		Name:      secretName,
	}, &asoSecret)

	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			log.V(4).Info("ASO kubeconfig secret not yet created, will retry")
			return nil, errors.New("ASO kubeconfig secret not yet available")
		}
		return nil, errors.Wrap(err, "failed to get ASO kubeconfig secret")
	}

	// Extract kubeconfig data from secret
	kubeconfigData, ok := asoSecret.Data[secret.KubeconfigDataName]
	if !ok || len(kubeconfigData) == 0 {
		return nil, errors.Errorf("key %s not found in ASO kubeconfig secret", secret.KubeconfigDataName)
	}

	return kubeconfigData, nil
}
