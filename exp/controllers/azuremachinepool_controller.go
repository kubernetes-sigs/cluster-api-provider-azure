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
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/tools/record"
	capiv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	capierrors "sigs.k8s.io/cluster-api/errors"
	capiv1exp "sigs.k8s.io/cluster-api/exp/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	capzcntr "sigs.k8s.io/cluster-api-provider-azure/controllers"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-azure/cloud/services/scalesets"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1alpha3"
)

type (
	// AzureMachinePoolReconciler reconciles a AzureMachinePool object
	AzureMachinePoolReconciler struct {
		client.Client
		Log      logr.Logger
		Scheme   *runtime.Scheme
		Recorder record.EventRecorder
	}

	// azureMachinePoolService provides structure and behavior around the operations needed to reconcile Azure Machine Pools
	azureMachinePoolService struct {
		machinePoolScope           *scope.MachinePoolScope
		clusterScope               *scope.ClusterScope
		virtualMachinesScaleSetSvc azure.GetterService
	}

	// annotationReaderWriter provides an interface to read and write annotations
	annotationReaderWriter interface {
		GetAnnotations() map[string]string
		SetAnnotations(annotations map[string]string)
	}
)

func (r *AzureMachinePoolReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&infrav1exp.AzureMachinePool{}).
		Watches(
			&source.Kind{Type: &capiv1exp.MachinePool{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: machinePoolToInfrastructureMapFunc(infrav1exp.GroupVersion.WithKind("AzureMachinePool"), r.Log),
			},
		).
		Watches(
			&source.Kind{Type: &infrav1.AzureCluster{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: azureClusterToAzureMachinePoolsFunc(r.Client, r.Log),
			}).
		Complete(r)
}

// +kubebuilder:rbac:groups=exp.infrastructure.cluster.x-k8s.io,resources=azuremachinepools,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=exp.infrastructure.cluster.x-k8s.io,resources=azuremachinepools/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=exp.cluster.x-k8s.io,resources=machinepools;machinepools/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch

func (r *AzureMachinePoolReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx := context.TODO()
	logger := r.Log.WithValues("namespace", req.Namespace, "azureMachinePool", req.Name)

	azMachinePool := &infrav1exp.AzureMachinePool{}
	err := r.Get(ctx, req.NamespacedName, azMachinePool)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the CAPI MachinePool.
	machinePool, err := getOwnerMachinePool(ctx, r.Client, azMachinePool.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if machinePool == nil {
		logger.Info("MachinePool Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	logger = logger.WithValues("machinePool", machinePool.Name)

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machinePool.ObjectMeta)
	if err != nil {
		logger.Info("MachinePool is missing cluster label or cluster does not exist")
		return reconcile.Result{}, nil
	}

	logger = logger.WithValues("cluster", cluster.Name)

	azureCluster := &infrav1.AzureCluster{}

	azureClusterName := client.ObjectKey{
		Namespace: azMachinePool.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, azureClusterName, azureCluster); err != nil {
		logger.Info("AzureCluster is not available yet")
		return reconcile.Result{}, nil
	}

	logger = logger.WithValues("AzureCluster", azureCluster.Name)

	// Create the cluster scope
	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Client:       r.Client,
		Logger:       logger,
		Cluster:      cluster,
		AzureCluster: azureCluster,
	})
	if err != nil {
		return reconcile.Result{}, err
	}

	// Create the machine pool scope
	machinePoolScope, err := scope.NewMachinePoolScope(scope.MachinePoolScopeParams{
		Logger:           logger,
		Client:           r.Client,
		Cluster:          cluster,
		MachinePool:      machinePool,
		AzureCluster:     azureCluster,
		AzureMachinePool: azMachinePool,
	})
	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}

	// Always close the scope when exiting this function so we can persist any AzureMachine changes.
	defer func() {
		if err := machinePoolScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted machine pools
	if !azMachinePool.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(machinePoolScope, clusterScope)
	}

	// Handle non-deleted machine pools
	return r.reconcileNormal(ctx, machinePoolScope, clusterScope)
}

func (r *AzureMachinePoolReconciler) reconcileNormal(ctx context.Context, machinePoolScope *scope.MachinePoolScope, clusterScope *scope.ClusterScope) (_ reconcile.Result, reterr error) {
	machinePoolScope.Info("Reconciling AzureMachinePool")
	// If the AzureMachine is in an error state, return early.
	if machinePoolScope.AzureMachinePool.Status.FailureReason != nil || machinePoolScope.AzureMachinePool.Status.FailureMessage != nil {
		machinePoolScope.Info("Error state detected, skipping reconciliation")
		return reconcile.Result{}, nil
	}

	// If the AzureMachine doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(machinePoolScope.AzureMachinePool, capiv1exp.MachinePoolFinalizer)
	// Register the finalizer immediately to avoid orphaning Azure resources on delete
	if err := machinePoolScope.PatchObject(); err != nil {
		return reconcile.Result{}, err
	}

	if !machinePoolScope.Cluster.Status.InfrastructureReady {
		machinePoolScope.Info("Cluster infrastructure is not ready yet")
		return reconcile.Result{}, nil
	}

	// Make sure bootstrap data is available and populated.
	if machinePoolScope.MachinePool.Spec.Template.Spec.Bootstrap.DataSecretName == nil {
		machinePoolScope.Info("Bootstrap data secret reference is not yet available")
		return reconcile.Result{}, nil
	}

	// Check that the image is valid
	// NOTE: this validation logic is also in the validating webhook
	if machinePoolScope.AzureMachinePool.Spec.Template.Image != nil {
		image := machinePoolScope.AzureMachinePool.Spec.Template.Image
		if errs := infrav1.ValidateImage(image, field.NewPath("image")); len(errs) > 0 {
			agg := kerrors.NewAggregate(errs.ToAggregate().Errors())
			machinePoolScope.Info("Invalid image: %s", agg.Error())
			r.Recorder.Eventf(machinePoolScope.AzureMachinePool, corev1.EventTypeWarning, "InvalidImage", "Invalid image: %s", agg.Error())
			return reconcile.Result{}, nil
		}
	}

	ams := newAzureMachinePoolService(machinePoolScope, clusterScope)

	// Get or create the virtual machine.
	vmss, err := ams.CreateOrUpdate()
	if err != nil {
		return reconcile.Result{}, err
	}

	// Make sure Spec.ProviderID is always set.
	machinePoolScope.AzureMachinePool.Spec.ProviderID = fmt.Sprintf("azure:///%s", vmss.ID)
	providerIDList := make([]string, len(vmss.Instances))
	var readyCount int32
	for i, vm := range vmss.Instances {
		providerIDList[i] = fmt.Sprintf("azure:///%s", vm.ID)
		if vm.State == infrav1.VMStateSucceeded {
			readyCount++
		}
	}
	machinePoolScope.AzureMachinePool.Spec.ProviderIDList = providerIDList
	machinePoolScope.AzureMachinePool.Status.ProvisioningState = &vmss.State
	machinePoolScope.AzureMachinePool.Status.Replicas = int32(len(providerIDList))
	machinePoolScope.SetAnnotation("cluster-api-provider-azure", "true")

	switch vmss.State {
	case infrav1.VMStateSucceeded:
		machinePoolScope.Info("Machine Pool is running", "id", *machinePoolScope.GetID())
		machinePoolScope.SetReady()
	case infrav1.VMStateUpdating:
		machinePoolScope.Info("Machine Pool is updating", "id", *machinePoolScope.GetID())
	default:
		machinePoolScope.SetFailureReason(capierrors.UpdateMachineError)
		machinePoolScope.SetFailureMessage(errors.Errorf("Azure VMSS state %q is unexpected", vmss.State))
	}

	// Ensure that the tags are correct.
	err = r.reconcileTags(machinePoolScope, clusterScope, machinePoolScope.AdditionalTags())
	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to ensure tags: %+v", err)
	}

	return reconcile.Result{}, nil
}

func (r *AzureMachinePoolReconciler) reconcileDelete(machinePoolScope *scope.MachinePoolScope, clusterScope *scope.ClusterScope) (_ reconcile.Result, reterr error) {
	machinePoolScope.Info("Handling deleted AzureMachinePool")

	if err := newAzureMachinePoolService(machinePoolScope, clusterScope).Delete(); err != nil {
		return reconcile.Result{}, errors.Wrapf(err, "error deleting AzureCluster %s/%s", clusterScope.Namespace(), clusterScope.Name())
	}

	defer func() {
		if reterr == nil {
			// VM is deleted so remove the finalizer.
			controllerutil.RemoveFinalizer(machinePoolScope.AzureMachinePool, capiv1exp.MachinePoolFinalizer)
		}
	}()

	return reconcile.Result{}, nil
}

// machinePoolToInfrastructureMapFunc returns a handler.ToRequestsFunc that watches for
// MachinePool events and returns reconciliation requests for an infrastructure provider object.
func machinePoolToInfrastructureMapFunc(gvk schema.GroupVersionKind, log logr.Logger) handler.ToRequestsFunc {
	return func(o handler.MapObject) []reconcile.Request {
		m, ok := o.Object.(*capiv1exp.MachinePool)
		if !ok {
			log.Info("attempt to map incorrect type", "type", fmt.Sprintf("%T", o.Object))
			return nil
		}

		gk := gvk.GroupKind()
		// Return early if the GroupVersionKind doesn't match what we expect.
		infraGK := m.Spec.Template.Spec.InfrastructureRef.GroupVersionKind().GroupKind()
		if gk != infraGK {
			log.Info("gk does not match", "gk", gk, "infraGK", infraGK)
			return nil
		}

		return []reconcile.Request{
			{
				NamespacedName: client.ObjectKey{
					Namespace: m.Namespace,
					Name:      m.Spec.Template.Spec.InfrastructureRef.Name,
				},
			},
		}
	}
}

// azureClusterToAzureMachinePoolsFunc is a handler.ToRequestsFunc to be used to enqueue
// requests for reconciliation of AzureMachinePools.
func azureClusterToAzureMachinePoolsFunc(kClient client.Client, log logr.Logger) handler.ToRequestsFunc {
	return func(o handler.MapObject) []reconcile.Request {
		c, ok := o.Object.(*infrav1.AzureCluster)
		if !ok {
			log.Error(errors.Errorf("expected a AzureCluster but got a %T", o.Object), "failed to get AzureCluster")
			return nil
		}
		logWithValues := log.WithValues("AzureCluster", c.Name, "Namespace", c.Namespace)

		cluster, err := util.GetOwnerCluster(context.TODO(), kClient, c.ObjectMeta)
		switch {
		case apierrors.IsNotFound(err) || cluster == nil:
			logWithValues.Info("owning cluster not found")
			return nil
		case err != nil:
			logWithValues.Error(err, "failed to get owning cluster")
			return nil
		}

		labels := map[string]string{capiv1.ClusterLabelName: cluster.Name}
		mpl := &capiv1exp.MachinePoolList{}
		if err := kClient.List(context.TODO(), mpl, client.InNamespace(c.Namespace), client.MatchingLabels(labels)); err != nil {
			logWithValues.Error(err, "failed to list Machines")
			return nil
		}

		var result []reconcile.Request
		for _, m := range mpl.Items {
			if m.Spec.Template.Spec.InfrastructureRef.Name == "" {
				continue
			}
			result = append(result, reconcile.Request{
				NamespacedName: client.ObjectKey{
					Namespace: m.Namespace,
					Name:      m.Spec.Template.Spec.InfrastructureRef.Name,
				},
			})
		}

		return result
	}
}

// Ensure that the tags of the machine are correct
func (r *AzureMachinePoolReconciler) reconcileTags(machinePoolScope *scope.MachinePoolScope, clusterScope *scope.ClusterScope, additionalTags map[string]string) error {
	machinePoolScope.Info("Updating tags on AzureMachinePool")
	annotation, err := r.AnnotationJSON(machinePoolScope.AzureMachinePool, capzcntr.TagsLastAppliedAnnotation)
	if err != nil {
		return err
	}
	changed, created, deleted, newAnnotation := capzcntr.TagsChanged(annotation, additionalTags)
	if changed {
		vmssSpec := &scalesets.Spec{
			Name: machinePoolScope.Name(),
		}
		svc := scalesets.NewService(machinePoolScope.AzureClients.Authorizer, machinePoolScope.AzureClients.SubscriptionID)
		vm, err := svc.Client.Get(clusterScope.Context, clusterScope.ResourceGroup(), machinePoolScope.Name())
		if err != nil {
			return errors.Wrapf(err, "failed to query AzureMachine VMSS")
		}
		tags := vm.Tags
		for k, v := range created {
			tags[k] = to.StringPtr(v)
		}

		for k := range deleted {
			delete(tags, k)
		}

		vm.Tags = tags

		err = svc.Client.CreateOrUpdate(
			clusterScope.Context,
			clusterScope.ResourceGroup(),
			vmssSpec.Name,
			vm)
		if err != nil {
			return errors.Wrapf(err, "cannot update VMSS tags")
		}

		// We also need to update the annotation if anything changed.
		err = r.updateAnnotationJSON(machinePoolScope.AzureMachinePool, capzcntr.TagsLastAppliedAnnotation, newAnnotation)
		if err != nil {
			return err
		}
	}

	return nil
}

// updateAnnotationJSON updates the `annotation` on an `annotationReaderWriter` with
// `content`. `content` in this case should be a `map[string]interface{}`
// suitable for turning into JSON. This `content` map will be marshalled into a
// JSON string before being set as the given `annotation`.
func (r *AzureMachinePoolReconciler) updateAnnotationJSON(rw annotationReaderWriter, annotation string, content map[string]interface{}) error {
	b, err := json.Marshal(content)
	if err != nil {
		return err
	}

	r.updateAnnotation(rw, annotation, string(b))
	return nil
}

// updateMachinePoolAnnotation updates the `annotation` on an `annotationReaderWriter` with
// `content`.
func (r *AzureMachinePoolReconciler) updateAnnotation(rw annotationReaderWriter, annotation string, content string) {
	// Get the annotations
	annotations := rw.GetAnnotations()

	// Set our annotation to the given content.
	annotations[annotation] = content

	// Update the machine pool object with these annotations
	rw.SetAnnotations(annotations)
}

// Returns a map[string]interface from a JSON annotation.
// This method gets the given `annotation` from an `annotationReaderWriter` and unmarshalls it
// from a JSON string into a `map[string]interface{}`.
func (r *AzureMachinePoolReconciler) AnnotationJSON(rw annotationReaderWriter, annotation string) (map[string]interface{}, error) {
	out := map[string]interface{}{}

	jsonAnnotation := r.Annotation(rw, annotation)
	if len(jsonAnnotation) == 0 {
		return out, nil
	}

	err := json.Unmarshal([]byte(jsonAnnotation), &out)
	if err != nil {
		return out, err
	}

	return out, nil
}

// Fetches the specific machine annotation.
func (r *AzureMachinePoolReconciler) Annotation(rw annotationReaderWriter, annotation string) string {
	return rw.GetAnnotations()[annotation]
}

// newAzureMachinePoolService populates all the services based on input scope
func newAzureMachinePoolService(machinePoolScope *scope.MachinePoolScope, clusterScope *scope.ClusterScope) *azureMachinePoolService {
	return &azureMachinePoolService{
		machinePoolScope:           machinePoolScope,
		clusterScope:               clusterScope,
		virtualMachinesScaleSetSvc: scalesets.NewService(machinePoolScope.AzureClients.Authorizer, machinePoolScope.AzureClients.SubscriptionID),
	}
}

func (s *azureMachinePoolService) CreateOrUpdate() (*infrav1exp.VMSS, error) {
	ampSpec := s.machinePoolScope.AzureMachinePool.Spec
	var replicas int64
	if s.machinePoolScope.MachinePool.Spec.Replicas != nil {
		replicas = int64(to.Int32(s.machinePoolScope.MachinePool.Spec.Replicas))
	}

	decoded, err := base64.StdEncoding.DecodeString(ampSpec.Template.SSHPublicKey)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to base64 decode ssh public key")
	}

	image, err := getVMImage(s.machinePoolScope)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get VMSS image")
	}

	bootstrapData, err := s.machinePoolScope.GetBootstrapData()
	if err != nil {
		return nil, errors.Wrap(err, "failed to retrieve bootstrap data")
	}

	vmssSpec := &scalesets.Spec{
		Name:                  s.machinePoolScope.Name(),
		ResourceGroup:         s.clusterScope.ResourceGroup(),
		Location:              s.clusterScope.Location(),
		ClusterName:           s.clusterScope.Name(),
		MachinePoolName:       s.machinePoolScope.Name(),
		Sku:                   ampSpec.Template.VMSize,
		Capacity:              replicas,
		SSHKeyData:            string(decoded),
		Image:                 image,
		OSDisk:                ampSpec.Template.OSDisk,
		CustomData:            bootstrapData,
		AdditionalTags:        s.machinePoolScope.AdditionalTags(),
		SubnetID:              s.clusterScope.AzureCluster.Spec.NetworkSpec.Subnets[0].ID,
		AcceleratedNetworking: ampSpec.Template.AcceleratedNetworking,
	}

	err = s.virtualMachinesScaleSetSvc.Reconcile(context.TODO(), vmssSpec)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create or get machine")
	}

	newVMSS, err := s.virtualMachinesScaleSetSvc.Get(s.clusterScope.Context, vmssSpec)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get VMSS")
	}

	vmss, ok := newVMSS.(*infrav1exp.VMSS)
	if !ok {
		return nil, errors.New("returned incorrect VMSS interface")
	}
	if vmss.State == "" {
		return nil, errors.Errorf("VMSS %s is nil provisioning state, reconcile", s.machinePoolScope.Name())
	}

	if vmss.State == infrav1.VMStateFailed {
		// If VM failed provisioning, delete it so it can be recreated
		err = s.virtualMachinesScaleSetSvc.Delete(s.clusterScope.Context, vmssSpec)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to delete machine pool")
		}
		return nil, errors.Errorf("VMSS %s is deleted, retry creating in next reconcile", s.machinePoolScope.Name())
	} else if vmss.State != infrav1.VMStateSucceeded {
		return nil, errors.Errorf("VMSS %s is still in provisioningState %s, reconcile", s.machinePoolScope.Name(), vmss.State)
	}

	return vmss, nil
}

// Delete reconciles all the services in pre determined order
func (s *azureMachinePoolService) Delete() error {
	vmssSpec := &scalesets.Spec{
		Name: s.machinePoolScope.Name(),
	}

	err := s.virtualMachinesScaleSetSvc.Delete(s.clusterScope.Context, vmssSpec)
	if err != nil {
		return errors.Wrapf(err, "failed to delete machine pool")
	}

	return nil
}

// Get fetches a VMSS if it exists
func (s *azureMachinePoolService) Get() (*infrav1exp.VMSS, error) {
	vmssSpec := &scalesets.Spec{
		Name: s.machinePoolScope.Name(),
	}

	vmss, err := s.virtualMachinesScaleSetSvc.Get(s.clusterScope.Context, vmssSpec)
	if err != nil && !azure.ResourceNotFound(err) {
		return nil, errors.Wrapf(err, "failed to fetch machine pool")
	}

	if err != nil && azure.ResourceNotFound(err) {
		return nil, nil
	}

	return vmss.(*infrav1exp.VMSS), err
}

// getOwnerMachinePool returns the MachinePool object owning the current resource.
func getOwnerMachinePool(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*capiv1exp.MachinePool, error) {
	for _, ref := range obj.OwnerReferences {
		if ref.Kind == "MachinePool" && ref.APIVersion == capiv1exp.GroupVersion.String() {
			return getMachinePoolByName(ctx, c, obj.Namespace, ref.Name)
		}
	}
	return nil, nil
}

// getMachinePoolByName finds and return a Machine object using the specified params.
func getMachinePoolByName(ctx context.Context, c client.Client, namespace, name string) (*capiv1exp.MachinePool, error) {
	m := &capiv1exp.MachinePool{}
	key := client.ObjectKey{Name: name, Namespace: namespace}
	if err := c.Get(ctx, key, m); err != nil {
		return nil, err
	}
	return m, nil
}

// Pick image from the machine configuration, or use a default one.
func getVMImage(scope *scope.MachinePoolScope) (*infrav1.Image, error) {
	// Use custom Marketplace image, Image ID or a Shared Image Gallery image if provided
	if scope.AzureMachinePool.Spec.Template.Image != nil {
		return scope.AzureMachinePool.Spec.Template.Image, nil
	}
	scope.Info("No image specified for machine pool, using default", "machinePool", scope.AzureMachinePool.GetName())
	return azure.GetDefaultUbuntuImage(to.String(scope.MachinePool.Spec.Template.Spec.Version))
}
