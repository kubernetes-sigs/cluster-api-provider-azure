package wrappers

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	"github.com/Azure/go-autorest/autorest"
)

type DeploymentsCreateOrUpdateFutureWrapper struct {
	mock   bool
	future resources.DeploymentsCreateOrUpdateFuture
}

func (wrapper *DeploymentsCreateOrUpdateFutureWrapper) WaitForCompletion(ctx context.Context, client autorest.Client) error {
	if !wrapper.mock {
		return wrapper.future.Future.WaitForCompletionRef(ctx, client)
	}
	return nil
}

func (wrapper *DeploymentsCreateOrUpdateFutureWrapper) Result(clientWrapper *DeploymentsClientWrapper) (resources.DeploymentExtended, error) {
	if !wrapper.mock {
		return wrapper.future.Result(clientWrapper.Client)
	}
	mockDeploymentName := MockDeploymentName
	return resources.DeploymentExtended{Name: &mockDeploymentName}, nil
}
