package privatelinks

import (
	"context"
	"encoding/json"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2022-07-01/network"
	"github.com/Azure/go-autorest/autorest"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/pkg/errors"

	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/reconciler"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

type azureClient struct {
	privateLinks network.PrivateLinkServicesClient
}

// newClient creates a new azureClient.
func newClient(auth azure.Authorizer) *azureClient {
	c := newPrivateLinksClient(auth.SubscriptionID(), auth.BaseURI(), auth.Authorizer())
	return &azureClient{c}
}

// newPrivateLinksClient creates a new private links client from a subscription ID, base URI and authorizer.
func newPrivateLinksClient(subscriptionID string, baseURI string, authorizer autorest.Authorizer) network.PrivateLinkServicesClient {
	privateLinksClient := network.NewPrivateLinkServicesClientWithBaseURI(baseURI, subscriptionID)
	azure.SetAutoRestClientDefaults(&privateLinksClient.Client, authorizer)
	return privateLinksClient
}

// Get returns the specified private link.
func (ac *azureClient) Get(ctx context.Context, spec azure.ResourceSpecGetter) (result interface{}, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "privatelinks.azureClient.Get")
	defer done()

	return ac.privateLinks.Get(ctx, spec.ResourceGroupName(), spec.ResourceName(), "")
}

func (ac *azureClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, parameters interface{}) (result interface{}, future azureautorest.FutureAPI, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "privatelinks.azureClient.CreateOrUpdateAsync")
	defer done()

	privateLink, ok := parameters.(network.PrivateLinkService)
	if !ok {
		return nil, nil, errors.Errorf("%T is not a network.PrivateLinkService", parameters)
	}

	createFuture, err := ac.privateLinks.CreateOrUpdate(ctx, spec.ResourceGroupName(), spec.ResourceName(), privateLink)
	if err != nil {
		return nil, nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = createFuture.WaitForCompletionRef(ctx, ac.privateLinks.Client)
	if err != nil {
		// If an error occurs, return the future. This means the long-running
		// operation didn't finish in the specified timeout.
		return nil, &createFuture, err
	}

	result, err = createFuture.Result(ac.privateLinks)
	return result, nil, err
}

func (ac *azureClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter) (future azureautorest.FutureAPI, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "privatelinks.azureClient.DeleteAsync")
	defer done()

	deleteFuture, err := ac.privateLinks.Delete(ctx, spec.ResourceGroupName(), spec.ResourceName())
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, reconciler.DefaultAzureCallTimeout)
	defer cancel()

	err = deleteFuture.WaitForCompletionRef(ctx, ac.privateLinks.Client)
	if err != nil {
		// If an error occurs, return the future. This means the long-running
		// operation didn't finish in the specified timeout.
		return &deleteFuture, err
	}

	_, err = deleteFuture.Result(ac.privateLinks)
	return nil, err
}

func (ac *azureClient) IsDone(ctx context.Context, future azureautorest.FutureAPI) (isDOne bool, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "privatelinks.azureClient.IsDone")
	defer done()

	return future.DoneWithContext(ctx, ac.privateLinks)
}

func (ac *azureClient) Result(ctx context.Context, future azureautorest.FutureAPI, futureType string) (result interface{}, err error) {
	_, _, done := tele.StartSpanWithLogger(ctx, "privatelinks.azureClient.Result")
	defer done()

	if future == nil {
		return nil, errors.Errorf("cannot get result from nil future")
	}

	switch futureType {
	case infrav1.PutFuture:
		var createFeature *network.PrivateLinkServicesCreateOrUpdateFuture
		jsonData, err := future.MarshalJSON()
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal future")
		}
		if err := json.Unmarshal(jsonData, &createFeature); err != nil {
			return nil, errors.Wrap(err, "failed to unmarshal future data")
		}
		return createFeature.Result(ac.privateLinks)
	case infrav1.DeleteFuture:
		// Delete does not return a result private link
		return nil, nil
	default:
		return nil, errors.Errorf("unknown future type %q", futureType)
	}
}
