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

package controllers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/groups"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/coalescing"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	capiv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha4"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

type (
	// Options are controller options extended.
	Options struct {
		controller.Options
		Cache *coalescing.ReconcileCache
	}
)

// AzureClusterToAzureMachinesMapper creates a mapping handler to transform AzureClusters into AzureMachines. The transform
// requires AzureCluster to map to the owning Cluster, then from the Cluster, collect the Machines belonging to the cluster,
// then finally projecting the infrastructure reference to the AzureMachine.
func AzureClusterToAzureMachinesMapper(ctx context.Context, c client.Client, ro runtime.Object, scheme *runtime.Scheme, log logr.Logger) (handler.MapFunc, error) {
	gvk, err := apiutil.GVKForObject(ro, scheme)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find GVK for AzureMachine")
	}

	return func(o client.Object) []ctrl.Request {
		ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultMappingTimeout)
		defer cancel()

		azCluster, ok := o.(*infrav1.AzureCluster)
		if !ok {
			log.Error(errors.Errorf("expected an AzureCluster, got %T instead", o), "failed to map AzureCluster")
			return nil
		}

		log = log.WithValues("AzureCluster", azCluster.Name, "Namespace", azCluster.Namespace)

		// Don't handle deleted AzureClusters
		if !azCluster.ObjectMeta.DeletionTimestamp.IsZero() {
			log.V(4).Info("AzureCluster has a deletion timestamp, skipping mapping.")
			return nil
		}

		clusterName, ok := GetOwnerClusterName(azCluster.ObjectMeta)
		if !ok {
			log.Info("unable to get the owner cluster")
			return nil
		}

		machineList := &clusterv1.MachineList{}
		machineList.SetGroupVersionKind(gvk)
		// list all of the requested objects within the cluster namespace with the cluster name label
		if err := c.List(ctx, machineList, client.InNamespace(azCluster.Namespace), client.MatchingLabels{clusterv1.ClusterLabelName: clusterName}); err != nil {
			return nil
		}

		mapFunc := util.MachineToInfrastructureMapFunc(gvk)
		var results []ctrl.Request
		for _, machine := range machineList.Items {
			m := machine
			azureMachines := mapFunc(&m)
			results = append(results, azureMachines...)
		}

		return results
	}, nil
}

// GetOwnerClusterName returns the name of the owning Cluster by finding a clusterv1.Cluster in the ownership references.
func GetOwnerClusterName(obj metav1.ObjectMeta) (string, bool) {
	for _, ref := range obj.OwnerReferences {
		if ref.Kind != "Cluster" {
			continue
		}
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return "", false
		}
		if gv.Group == clusterv1.GroupVersion.Group {
			return ref.Name, true
		}
	}
	return "", false
}

// GetObjectsToRequestsByNamespaceAndClusterName returns the slice of ctrl.Requests consisting the list items contained in the unstructured list.
func GetObjectsToRequestsByNamespaceAndClusterName(ctx context.Context, c client.Client, clusterKey client.ObjectKey, list *unstructured.UnstructuredList) []ctrl.Request {
	// list all of the requested objects within the cluster namespace with the cluster name label
	if err := c.List(ctx, list, client.InNamespace(clusterKey.Namespace), client.MatchingLabels{clusterv1.ClusterLabelName: clusterKey.Name}); err != nil {
		return nil
	}

	results := make([]ctrl.Request, len(list.Items))
	for i, obj := range list.Items {
		results[i] = ctrl.Request{
			NamespacedName: client.ObjectKey{Namespace: obj.GetNamespace(), Name: obj.GetName()},
		}
	}
	return results
}

// referSameObject returns true if a and b point to the same object.
func referSameObject(a, b metav1.OwnerReference) bool {
	aGV, err := schema.ParseGroupVersion(a.APIVersion)
	if err != nil {
		return false
	}

	bGV, err := schema.ParseGroupVersion(b.APIVersion)
	if err != nil {
		return false
	}

	return aGV.Group == bGV.Group && a.Kind == b.Kind && a.Name == b.Name
}

// GetCloudProviderSecret returns the required azure json secret for the provided parameters.
func GetCloudProviderSecret(d azure.ClusterScoper, namespace, name string, owner metav1.OwnerReference, identityType infrav1.VMIdentity, userIdentityID string) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      fmt.Sprintf("%s-azure-json", name),
			Labels: map[string]string{
				d.ClusterName(): string(infrav1.ResourceLifecycleOwned),
			},
			OwnerReferences: []metav1.OwnerReference{owner},
		},
	}

	var controlPlaneConfig, workerNodeConfig *CloudProviderConfig

	switch identityType {
	case infrav1.VMIdentitySystemAssigned:
		controlPlaneConfig, workerNodeConfig = systemAssignedIdentityCloudProviderConfig(d)
	case infrav1.VMIdentityUserAssigned:
		if len(userIdentityID) < 1 {
			return nil, errors.New("expected a non-empty userIdentityID")
		}
		controlPlaneConfig, workerNodeConfig = userAssignedIdentityCloudProviderConfig(d, userIdentityID)
	case infrav1.VMIdentityNone:
		controlPlaneConfig, workerNodeConfig = newCloudProviderConfig(d)
	}

	controlPlaneData, err := json.MarshalIndent(controlPlaneConfig, "", "    ")
	if err != nil {
		return nil, errors.Wrap(err, "failed control plane json marshal")
	}
	workerNodeData, err := json.MarshalIndent(workerNodeConfig, "", "    ")
	if err != nil {
		return nil, errors.Wrap(err, "failed worker node json marshal")
	}

	secret.Data = map[string][]byte{
		"control-plane-azure.json": controlPlaneData,
		"worker-node-azure.json":   workerNodeData,
		// added for backwards compatibility
		"azure.json": controlPlaneData,
	}

	return secret, nil
}

func systemAssignedIdentityCloudProviderConfig(d azure.ClusterScoper) (*CloudProviderConfig, *CloudProviderConfig) {
	controlPlaneConfig, workerConfig := newCloudProviderConfig(d)
	controlPlaneConfig.AadClientID = ""
	controlPlaneConfig.AadClientSecret = ""
	controlPlaneConfig.UseManagedIdentityExtension = true
	workerConfig.AadClientID = ""
	workerConfig.AadClientSecret = ""
	workerConfig.UseManagedIdentityExtension = true
	return controlPlaneConfig, workerConfig
}

func userAssignedIdentityCloudProviderConfig(d azure.ClusterScoper, identityID string) (*CloudProviderConfig, *CloudProviderConfig) {
	controlPlaneConfig, workerConfig := newCloudProviderConfig(d)
	controlPlaneConfig.AadClientID = ""
	controlPlaneConfig.AadClientSecret = ""
	controlPlaneConfig.UseManagedIdentityExtension = true
	controlPlaneConfig.UserAssignedIdentityID = identityID
	workerConfig.AadClientID = ""
	workerConfig.AadClientSecret = ""
	workerConfig.UseManagedIdentityExtension = true
	workerConfig.UserAssignedIdentityID = identityID
	return controlPlaneConfig, workerConfig
}

func newCloudProviderConfig(d azure.ClusterScoper) (controlPlaneConfig *CloudProviderConfig, workerConfig *CloudProviderConfig) {
	return (&CloudProviderConfig{
			Cloud:                        d.CloudEnvironment(),
			AadClientID:                  d.ClientID(),
			AadClientSecret:              d.ClientSecret(),
			TenantID:                     d.TenantID(),
			SubscriptionID:               d.SubscriptionID(),
			ResourceGroup:                d.ResourceGroup(),
			SecurityGroupName:            d.NodeSubnet().SecurityGroup.Name,
			SecurityGroupResourceGroup:   d.Vnet().ResourceGroup,
			Location:                     d.Location(),
			VMType:                       "vmss",
			VnetName:                     d.Vnet().Name,
			VnetResourceGroup:            d.Vnet().ResourceGroup,
			SubnetName:                   d.NodeSubnet().Name,
			RouteTableName:               d.NodeRouteTable().Name,
			LoadBalancerSku:              "Standard",
			MaximumLoadBalancerRuleCount: 250,
			UseManagedIdentityExtension:  false,
			UseInstanceMetadata:          true,
		}).overrideFromSpec(d),
		(&CloudProviderConfig{
			Cloud:                        d.CloudEnvironment(),
			AadClientID:                  d.ClientID(),
			AadClientSecret:              d.ClientSecret(),
			TenantID:                     d.TenantID(),
			SubscriptionID:               d.SubscriptionID(),
			ResourceGroup:                d.ResourceGroup(),
			SecurityGroupName:            d.NodeSubnet().SecurityGroup.Name,
			SecurityGroupResourceGroup:   d.Vnet().ResourceGroup,
			Location:                     d.Location(),
			VMType:                       "vmss",
			VnetName:                     d.Vnet().Name,
			VnetResourceGroup:            d.Vnet().ResourceGroup,
			SubnetName:                   d.NodeSubnet().Name,
			RouteTableName:               d.NodeRouteTable().Name,
			LoadBalancerSku:              "Standard",
			MaximumLoadBalancerRuleCount: 250,
			UseManagedIdentityExtension:  false,
			UseInstanceMetadata:          true,
		}).overrideFromSpec(d)
}

// overrideFromSpec overrides cloud provider config with the values provided in cluster spec.
func (cpc *CloudProviderConfig) overrideFromSpec(d azure.ClusterScoper) *CloudProviderConfig {
	if d.CloudProviderConfigOverrides() == nil {
		return cpc
	}

	for _, rateLimit := range d.CloudProviderConfigOverrides().RateLimits {
		switch rateLimit.Name {
		case infrav1.DefaultRateLimit:
			cpc.RateLimitConfig = *toCloudProviderRateLimitConfig(rateLimit.Config)
		case infrav1.RouteRateLimit:
			cpc.RouteRateLimit = toCloudProviderRateLimitConfig(rateLimit.Config)
		case infrav1.SubnetsRateLimit:
			cpc.SubnetsRateLimit = toCloudProviderRateLimitConfig(rateLimit.Config)
		case infrav1.InterfaceRateLimit:
			cpc.InterfaceRateLimit = toCloudProviderRateLimitConfig(rateLimit.Config)
		case infrav1.RouteTableRateLimit:
			cpc.RouteTableRateLimit = toCloudProviderRateLimitConfig(rateLimit.Config)
		case infrav1.LoadBalancerRateLimit:
			cpc.LoadBalancerRateLimit = toCloudProviderRateLimitConfig(rateLimit.Config)
		case infrav1.PublicIPAddressRateLimit:
			cpc.PublicIPAddressRateLimit = toCloudProviderRateLimitConfig(rateLimit.Config)
		case infrav1.SecurityGroupRateLimit:
			cpc.SecurityGroupRateLimit = toCloudProviderRateLimitConfig(rateLimit.Config)
		case infrav1.VirtualMachineRateLimit:
			cpc.VirtualMachineRateLimit = toCloudProviderRateLimitConfig(rateLimit.Config)
		case infrav1.StorageAccountRateLimit:
			cpc.StorageAccountRateLimit = toCloudProviderRateLimitConfig(rateLimit.Config)
		case infrav1.DiskRateLimit:
			cpc.DiskRateLimit = toCloudProviderRateLimitConfig(rateLimit.Config)
		case infrav1.SnapshotRateLimit:
			cpc.SnapshotRateLimit = toCloudProviderRateLimitConfig(rateLimit.Config)
		case infrav1.VirtualMachineScaleSetRateLimit:
			cpc.VirtualMachineScaleSetRateLimit = toCloudProviderRateLimitConfig(rateLimit.Config)
		case infrav1.VirtualMachineSizesRateLimit:
			cpc.VirtualMachineSizeRateLimit = toCloudProviderRateLimitConfig(rateLimit.Config)
		case infrav1.AvailabilitySetRateLimit:
			cpc.AvailabilitySetRateLimit = toCloudProviderRateLimitConfig(rateLimit.Config)
		}
	}

	cpc.BackOffConfig = toCloudProviderBackOffConfig(d.CloudProviderConfigOverrides().BackOffs)
	return cpc
}

// toCloudProviderRateLimitConfig returns converts infrav1.RateLimitConfig to RateLimitConfig that is required with the cloud provider.
func toCloudProviderRateLimitConfig(source infrav1.RateLimitConfig) *RateLimitConfig {
	rateLimitConfig := RateLimitConfig{}
	rateLimitConfig.CloudProviderRateLimit = source.CloudProviderRateLimit
	if source.CloudProviderRateLimitQPS != nil {
		rateLimitConfig.CloudProviderRateLimitQPS = float32(source.CloudProviderRateLimitQPS.AsApproximateFloat64())
	}
	rateLimitConfig.CloudProviderRateLimitBucket = source.CloudProviderRateLimitBucket
	if source.CloudProviderRateLimitQPSWrite != nil {
		rateLimitConfig.CloudProviderRateLimitQPSWrite = float32(source.CloudProviderRateLimitQPSWrite.AsApproximateFloat64())
	}
	rateLimitConfig.CloudProviderRateLimitBucketWrite = source.CloudProviderRateLimitBucketWrite
	return &rateLimitConfig
}

// CloudProviderConfig is an abbreviated version of the same struct in k/k.
type CloudProviderConfig struct {
	Cloud                        string `json:"cloud"`
	TenantID                     string `json:"tenantId"`
	SubscriptionID               string `json:"subscriptionId"`
	AadClientID                  string `json:"aadClientId,omitempty"`
	AadClientSecret              string `json:"aadClientSecret,omitempty"`
	ResourceGroup                string `json:"resourceGroup"`
	SecurityGroupName            string `json:"securityGroupName"`
	SecurityGroupResourceGroup   string `json:"securityGroupResourceGroup"`
	Location                     string `json:"location"`
	VMType                       string `json:"vmType"`
	VnetName                     string `json:"vnetName"`
	VnetResourceGroup            string `json:"vnetResourceGroup"`
	SubnetName                   string `json:"subnetName"`
	RouteTableName               string `json:"routeTableName"`
	LoadBalancerSku              string `json:"loadBalancerSku"`
	MaximumLoadBalancerRuleCount int    `json:"maximumLoadBalancerRuleCount"`
	UseManagedIdentityExtension  bool   `json:"useManagedIdentityExtension"`
	UseInstanceMetadata          bool   `json:"useInstanceMetadata"`
	UserAssignedIdentityID       string `json:"userAssignedIdentityId,omitempty"`
	CloudProviderRateLimitConfig
	BackOffConfig
}

// CloudProviderRateLimitConfig represents the rate limiting configurations in azure cloud provider config.
// See: https://kubernetes-sigs.github.io/cloud-provider-azure/install/configs/#per-client-rate-limiting.
// This is a copy of the struct used in cloud-provider-azure: https://github.com/kubernetes-sigs/cloud-provider-azure/blob/d585c2031925b39c925624302f22f8856e29e352/pkg/provider/azure_ratelimit.go#L25
type CloudProviderRateLimitConfig struct {
	RateLimitConfig

	RouteRateLimit                  *RateLimitConfig `json:"routeRateLimit,omitempty"`
	SubnetsRateLimit                *RateLimitConfig `json:"subnetsRateLimit,omitempty"`
	InterfaceRateLimit              *RateLimitConfig `json:"interfaceRateLimit,omitempty"`
	RouteTableRateLimit             *RateLimitConfig `json:"routeTableRateLimit,omitempty"`
	LoadBalancerRateLimit           *RateLimitConfig `json:"loadBalancerRateLimit,omitempty"`
	PublicIPAddressRateLimit        *RateLimitConfig `json:"publicIPAddressRateLimit,omitempty"`
	SecurityGroupRateLimit          *RateLimitConfig `json:"securityGroupRateLimit,omitempty"`
	VirtualMachineRateLimit         *RateLimitConfig `json:"virtualMachineRateLimit,omitempty"`
	StorageAccountRateLimit         *RateLimitConfig `json:"storageAccountRateLimit,omitempty"`
	DiskRateLimit                   *RateLimitConfig `json:"diskRateLimit,omitempty"`
	SnapshotRateLimit               *RateLimitConfig `json:"snapshotRateLimit,omitempty"`
	VirtualMachineScaleSetRateLimit *RateLimitConfig `json:"virtualMachineScaleSetRateLimit,omitempty"`
	VirtualMachineSizeRateLimit     *RateLimitConfig `json:"virtualMachineSizesRateLimit,omitempty"`
	AvailabilitySetRateLimit        *RateLimitConfig `json:"availabilitySetRateLimit,omitempty"`
}

// RateLimitConfig indicates the rate limit config options.
// This is a copy of the struct used in cloud-provider-azure: https://github.com/kubernetes-sigs/cloud-provider-azure/blob/d585c2031925b39c925624302f22f8856e29e352/pkg/azureclients/azure_client_config.go#L48
type RateLimitConfig struct {
	CloudProviderRateLimit            bool    `json:"cloudProviderRateLimit,omitempty"`
	CloudProviderRateLimitQPS         float32 `json:"cloudProviderRateLimitQPS,omitempty"`
	CloudProviderRateLimitBucket      int     `json:"cloudProviderRateLimitBucket,omitempty"`
	CloudProviderRateLimitQPSWrite    float32 `json:"cloudProviderRateLimitQPSWrite,omitempty"`
	CloudProviderRateLimitBucketWrite int     `json:"cloudProviderRateLimitBucketWrite,omitempty"`
}

// BackOffConfig indicates the back-off config options.
// This is a copy of the struct used in cloud-provider-azure: https://github.com/kubernetes-sigs/cloud-provider-azure/blob/d585c2031925b39c925624302f22f8856e29e352/pkg/azureclients/azure_client_config.go#L48
type BackOffConfig struct {
	CloudProviderBackoff         bool    `json:"cloudProviderBackoff,omitempty"`
	CloudProviderBackoffRetries  int     `json:"cloudProviderBackoffRetries,omitempty"`
	CloudProviderBackoffExponent float64 `json:"cloudProviderBackoffExponent,omitempty"`
	CloudProviderBackoffDuration int     `json:"cloudProviderBackoffDuration,omitempty"`
	CloudProviderBackoffJitter   float64 `json:"cloudProviderBackoffJitter,omitempty"`
}

// toCloudProviderBackOffConfig returns converts infrav1.BackOffConfig to BackOffConfig that is required with the cloud provider.
func toCloudProviderBackOffConfig(source infrav1.BackOffConfig) BackOffConfig {
	backOffConfig := BackOffConfig{}
	backOffConfig.CloudProviderBackoff = source.CloudProviderBackoff
	if source.CloudProviderBackoffExponent != nil {
		backOffConfig.CloudProviderBackoffExponent = source.CloudProviderBackoffExponent.AsApproximateFloat64()
	}
	backOffConfig.CloudProviderBackoffRetries = source.CloudProviderBackoffRetries
	if source.CloudProviderBackoffJitter != nil {
		backOffConfig.CloudProviderBackoffJitter = source.CloudProviderBackoffJitter.AsApproximateFloat64()
	}
	backOffConfig.CloudProviderBackoffDuration = source.CloudProviderBackoffDuration
	return backOffConfig
}

func reconcileAzureSecret(ctx context.Context, log logr.Logger, kubeclient client.Client, owner metav1.OwnerReference, new *corev1.Secret, clusterName string) error {
	ctx, span := tele.Tracer().Start(ctx, "controllers.reconcileAzureSecret")
	defer span.End()

	// Fetch previous secret, if it exists
	key := types.NamespacedName{
		Namespace: new.Namespace,
		Name:      new.Name,
	}
	old := &corev1.Secret{}
	err := kubeclient.Get(ctx, key, old)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrap(err, "failed to fetch existing azure json")
	}

	// Create if it wasn't found
	if apierrors.IsNotFound(err) {
		if err := kubeclient.Create(ctx, new); err != nil && !apierrors.IsAlreadyExists(err) {
			return errors.Wrap(err, "failed to create cluster azure json")
		}
		return nil
	}

	tag, exists := old.Labels[clusterName]

	if exists && tag != string(infrav1.ResourceLifecycleOwned) {
		log.V(2).Info("returning early from json reconcile, user provided secret already exists")
		return nil
	}

	// Otherwise, check ownership and data freshness. Update as necessary
	hasOwner := false
	for _, ownerRef := range old.OwnerReferences {
		if referSameObject(ownerRef, owner) {
			hasOwner = true
			break
		}
	}

	hasData := equality.Semantic.DeepEqual(old.Data, new.Data)
	if hasData && hasOwner {
		// no update required
		log.V(2).Info("returning early from json reconcile, no update needed")
		return nil
	}

	if !hasOwner {
		old.OwnerReferences = append(old.OwnerReferences, owner)
	}

	if !hasData {
		old.Data = new.Data
	}

	log.V(2).Info("updating azure json")
	if err := kubeclient.Update(ctx, old); err != nil {
		return errors.Wrap(err, "failed to update cluster azure json when diff was required")
	}

	log.V(2).Info("done updating azure json")

	return nil
}

// GetOwnerMachinePool returns the MachinePool object owning the current resource.
func GetOwnerMachinePool(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*capiv1exp.MachinePool, error) {
	ctx, span := tele.Tracer().Start(ctx, "controllers.GetOwnerMachinePool")
	defer span.End()

	for _, ref := range obj.OwnerReferences {
		if ref.Kind != "MachinePool" {
			continue
		}
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		if gv.Group == capiv1exp.GroupVersion.Group {
			return GetMachinePoolByName(ctx, c, obj.Namespace, ref.Name)
		}
	}
	return nil, nil
}

// GetOwnerAzureMachinePool returns the AzureMachinePool object owning the current resource.
func GetOwnerAzureMachinePool(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*infrav1exp.AzureMachinePool, error) {
	ctx, span := tele.Tracer().Start(ctx, "controllers.GetOwnerAzureMachinePool")
	defer span.End()

	for _, ref := range obj.OwnerReferences {
		if ref.Kind != "AzureMachinePool" {
			continue
		}

		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, errors.WithStack(err)
		}

		if gv.Group == infrav1exp.GroupVersion.Group {
			return GetAzureMachinePoolByName(ctx, c, obj.Namespace, ref.Name)
		}
	}
	return nil, nil
}

// GetMachinePoolByName finds and return a MachinePool object using the specified params.
func GetMachinePoolByName(ctx context.Context, c client.Client, namespace, name string) (*capiv1exp.MachinePool, error) {
	ctx, span := tele.Tracer().Start(ctx, "controllers.GetMachinePoolByName")
	defer span.End()

	m := &capiv1exp.MachinePool{}
	key := client.ObjectKey{Name: name, Namespace: namespace}
	if err := c.Get(ctx, key, m); err != nil {
		return nil, err
	}
	return m, nil
}

// GetAzureMachinePoolByName finds and return an AzureMachinePool object using the specified params.
func GetAzureMachinePoolByName(ctx context.Context, c client.Client, namespace, name string) (*infrav1exp.AzureMachinePool, error) {
	ctx, span := tele.Tracer().Start(ctx, "controllers.GetAzureMachinePoolByName")
	defer span.End()

	m := &infrav1exp.AzureMachinePool{}
	key := client.ObjectKey{Name: name, Namespace: namespace}
	if err := c.Get(ctx, key, m); err != nil {
		return nil, err
	}
	return m, nil
}

// ShouldDeleteIndividualResources returns false if the resource group is managed and the whole cluster is being deleted
// meaning that we can rely on a single resource group delete operation as opposed to deleting every individual VM resource.
func ShouldDeleteIndividualResources(ctx context.Context, clusterScope *scope.ClusterScope) bool {
	ctx, span := tele.Tracer().Start(ctx, "controllers.ShouldDeleteIndividualResources")
	defer span.End()

	if clusterScope.Cluster.DeletionTimestamp.IsZero() {
		return true
	}
	grpSvc := groups.New(clusterScope)
	managed, err := grpSvc.IsGroupManaged(ctx)
	// Since this is a best effort attempt to speed up delete, we don't fail the delete if we can't get the RG status.
	// Instead, take the long way and delete all resources one by one.
	return err != nil || !managed
}

// GetClusterIdentityFromRef returns the AzureClusterIdentity referenced by the AzureCluster.
func GetClusterIdentityFromRef(ctx context.Context, c client.Client, azureClusterNamespace string, ref *corev1.ObjectReference) (*infrav1.AzureClusterIdentity, error) {
	identity := &infrav1.AzureClusterIdentity{}
	if ref != nil {
		namespace := ref.Namespace
		if namespace == "" {
			namespace = azureClusterNamespace
		}
		key := client.ObjectKey{Name: ref.Name, Namespace: namespace}
		if err := c.Get(ctx, key, identity); err != nil {
			return nil, err
		}
		return identity, nil
	}
	return nil, nil
}
