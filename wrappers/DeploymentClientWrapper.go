package wrappers

import (
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	"net/http"
	"context"
)

const (
	MockDeploymentName = "mock"
	MockSubscriptionID = "mock"
)

type DeploymentsClientWrapper struct {
	Client resources.DeploymentsClient
	mock   *DeploymentsClientMock
}

type DeploymentsClientMock struct{}

func GetDeploymentsClient(SubscriptionID string) *DeploymentsClientWrapper {
	if SubscriptionID == MockSubscriptionID {
		return &DeploymentsClientWrapper{
			mock: &DeploymentsClientMock{},
		}
	}
	return &DeploymentsClientWrapper{
		Client: resources.NewDeploymentsClient(SubscriptionID),
	}
}

func (wrapper *DeploymentsClientWrapper) SetAuthorizer(Authorizer autorest.Authorizer) {
	if wrapper.mock == nil {
		wrapper.Client.BaseClient.Client.Authorizer = Authorizer
	}
}

func (wrapper *DeploymentsClientWrapper) Validate(ctx context.Context, rg string, deploymentName string, deployment resources.Deployment)  (resources.DeploymentValidateResult, error) {
	if wrapper.mock == nil {
		return wrapper.Client.Validate(ctx, rg, deploymentName, deployment)
	}
	return resources.DeploymentValidateResult{}, nil
}

func (wrapper *DeploymentsClientWrapper) CreateOrUpdate(ctx context.Context, rgName string, deploymentName string, deployment resources.Deployment) (*DeploymentsCreateOrUpdateFutureWrapper, error) {
	if wrapper.mock == nil {
		future, err := wrapper.Client.CreateOrUpdate(ctx, rgName, deploymentName, deployment)
		return &DeploymentsCreateOrUpdateFutureWrapper{ mock: false, future: future, }, err
	}
	return &DeploymentsCreateOrUpdateFutureWrapper{ mock: true }, nil
}

func (wrapper *DeploymentsClientWrapper) Get(ctx context.Context, rgName string, deploymentName string) (resources.DeploymentExtended, error) {
	if wrapper.mock == nil {
		return wrapper.Client.Get(ctx, rgName, deploymentName)
	}
	mockDeploymentName := MockDeploymentName
	return resources.DeploymentExtended{
		Response: autorest.Response{
			Response: &http.Response{
				StatusCode: 200,
			},
		},
		Name: &mockDeploymentName }, nil
}

func (wrapper *DeploymentsClientWrapper) CheckExistence(ctx context.Context, resourceGroupName string, deploymentName string) (autorest.Response, error) {
	if wrapper.mock == nil {
		return wrapper.Client.CheckExistence(ctx, resourceGroupName, deploymentName)
	}
	return autorest.Response{Response: &http.Response{StatusCode: 200}}, nil
}

func (wrapper *DeploymentsClientWrapper) Delete(ctx context.Context, resourceGroupName string, deploymentName string) (*DeploymentsDeleteFutureWrapper, error) {
	if wrapper.mock == nil {
		future, err := wrapper.Client.Delete(ctx, resourceGroupName, deploymentName)
		return &DeploymentsDeleteFutureWrapper { mock: false, future: future }, err
	}
	return &DeploymentsDeleteFutureWrapper { mock: true }, nil
}

