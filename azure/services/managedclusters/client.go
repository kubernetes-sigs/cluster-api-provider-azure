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

package managedclusters

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/containerservice/armcontainerservice/v4"
	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// CredentialGetter is a helper interface for getting managed cluster credentials.
type CredentialGetter interface {
	GetCredentials(context.Context, string, string) ([]byte, error)
}

// azureClient contains the Azure go-sdk Client.
type azureClient struct {
	managedclusters armcontainerservice.ManagedClustersClient
}

// newClient creates a new managed cluster client from an authorizer.
func newClient(auth azure.Authorizer) (*azureClient, error) {
	c, err := newManagedClustersClient(auth.SubscriptionID(), auth.CloudEnvironment())
	if err != nil {
		return nil, errors.Wrap(err, "failed to create managed clusters client")
	}
	return &azureClient{c}, nil
}

// newManagedClustersClient creates a new managed clusters client from subscription ID.
func newManagedClustersClient(subscriptionID string, azureEnvironment string) (armcontainerservice.ManagedClustersClient, error) {
	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return armcontainerservice.ManagedClustersClient{}, errors.Wrap(err, "failed to create default Azure credential")
	}
	opts, err := azure.ARMClientOptions(azureEnvironment)
	if err != nil {
		return armcontainerservice.ManagedClustersClient{}, errors.Wrap(err, "failed to create ARM client options")
	}
	factory, err := armcontainerservice.NewClientFactory(subscriptionID, credential, opts)
	if err != nil {
		return armcontainerservice.ManagedClustersClient{}, errors.Wrap(err, "failed to create client factory")
	}
	return *factory.NewManagedClustersClient(), nil
}

// Get gets a managed cluster.
func (ac *azureClient) Get(ctx context.Context, spec azure.ResourceSpecGetter) (result interface{}, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "managedclusters.azureClient.Get")
	defer done()

	resp, err := ac.managedclusters.Get(ctx, spec.ResourceGroupName(), spec.ResourceName(), nil)
	if err != nil {
		return nil, err
	}
	return resp.ManagedCluster, nil
}

// GetCredentials fetches the admin kubeconfig for a managed cluster.
func (ac *azureClient) GetCredentials(ctx context.Context, resourceGroupName, name string) ([]byte, error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "managedclusters.azureClient.GetCredentials")
	defer done()

	credentialList, err := ac.managedclusters.ListClusterAdminCredentials(ctx, resourceGroupName, name, nil)
	if err != nil {
		return nil, err
	}

	if len(credentialList.Kubeconfigs) == 0 {
		return nil, errors.New("no kubeconfigs available for the managed cluster")
	}

	return (credentialList.Kubeconfigs)[0].Value, nil
}

// CreateOrUpdateAsync creates or updates a managed cluster.
// It sends a PUT request to Azure and if accepted without error, the func will return a Poller which can be used to track the ongoing
// progress of the operation.
func (ac *azureClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, resumeToken string, parameters interface{}) (result interface{}, poller *runtime.Poller[armcontainerservice.ManagedClustersClientCreateOrUpdateResponse], err error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "managedclusters.azureClient.CreateOrUpdate")
	defer done()

	var managedCluster armcontainerservice.ManagedCluster
	if parameters != nil {
		mc, ok := parameters.(armcontainerservice.ManagedCluster)
		if !ok {
			return nil, nil, errors.Errorf("%T is not an armcontainerservice.ManagedCluster", parameters)
		}
		managedCluster = mc
	}

	// TODO: Add these custom headers
	// for key, value := range headerSpec.CustomHeaders() {
	// 	preparer.Header.Add(key, value)
	// }

	opts := &armcontainerservice.ManagedClustersClientBeginCreateOrUpdateOptions{ResumeToken: resumeToken}
	log.V(4).Info("sending request", "resumeToken", resumeToken)
	poller, err = ac.managedclusters.BeginCreateOrUpdate(ctx, spec.ResourceGroupName(), spec.ResourceName(), managedCluster, opts)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	result, err = poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{})
	if err != nil {
		// If an error occurs, return the poller.
		return nil, poller, err
	}

	// If the operation completed, return a nil poller.
	return result, nil, err
}

// DeleteAsync deletes a managed cluster asynchronously. DeleteAsync sends a DELETE
// request to Azure and if accepted without error, the func will return a Poller which can be used to track the ongoing
// progress of the operation.
func (ac *azureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter, resumeToken string) (poller *runtime.Poller[armcontainerservice.ManagedClustersClientDeleteResponse], err error) {
	ctx, log, done := tele.StartSpanWithLogger(ctx, "managedclusters.azureClient.DeleteAsync")
	defer done()

	opts := &armcontainerservice.ManagedClustersClientBeginDeleteOptions{ResumeToken: resumeToken}
	log.V(4).Info("sending request", "resumeToken", resumeToken)
	poller, err = ac.managedclusters.BeginDelete(ctx, spec.ResourceGroupName(), spec.ResourceName(), opts)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	_, err = poller.PollUntilDone(ctx, &runtime.PollUntilDoneOptions{})
	if err != nil {
		// If an error occurs, return the poller.
		// This means the long-running operation didn't finish in the specified timeout.
		return poller, err
	}

	// If the operation completed, return a nil poller.
	return nil, err
}

// IsDone returns true if the long-running operation has completed.
func (ac *azureClient) IsDone(ctx context.Context, poller interface{}) (isDone bool, err error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "managedclusters.azureClient.IsDone")
	defer done()

	switch t := poller.(type) {
	case *runtime.Poller[armcontainerservice.ManagedClustersClientCreateOrUpdateResponse]:
		c, _ := poller.(*runtime.Poller[armcontainerservice.ManagedClustersClientCreateOrUpdateResponse])
		return c.Done(), nil
	case *runtime.Poller[armcontainerservice.ManagedClustersClientDeleteResponse]:
		d, _ := poller.(*runtime.Poller[armcontainerservice.ManagedClustersClientDeleteResponse])
		return d.Done(), nil
	default:
		return false, errors.Errorf("unexpected poller type %T", t)
	}
}

// Result fetches the result of a long-running operation future.
func (ac *azureClient) Result(ctx context.Context, poller interface{}) (result interface{}, err error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "managedclusters.azureClient.Result")
	defer done()

	switch t := poller.(type) {
	case *runtime.Poller[armcontainerservice.ManagedClustersClientCreateOrUpdateResponse]:
		c, _ := poller.(*runtime.Poller[armcontainerservice.ManagedClustersClientCreateOrUpdateResponse])
		return c.Result(ctx)
	case *runtime.Poller[armcontainerservice.ManagedClustersClientDeleteResponse]:
		d, _ := poller.(*runtime.Poller[armcontainerservice.ManagedClustersClientDeleteResponse])
		return d.Result(ctx)
	default:
		return false, errors.Errorf("unexpected poller type %T", t)
	}
}
