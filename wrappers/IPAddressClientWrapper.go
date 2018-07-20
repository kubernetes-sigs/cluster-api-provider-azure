package wrappers

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2018-01-01/network"
	"github.com/Azure/go-autorest/autorest"
)

type IPAddressClientWrapper struct {
	client network.PublicIPAddressesClient
	mock   *IPAddressClientMock
}

type IPAddressClientMock struct{}

func GetPublicIPAddressesClient(SubscriptionID string) *IPAddressClientWrapper {
	if SubscriptionID == MockDeploymentName {
		return &IPAddressClientWrapper{
			mock: &IPAddressClientMock{},
		}
	} else {
		return &IPAddressClientWrapper{
			client: network.NewPublicIPAddressesClient(SubscriptionID),
		}
	}
}

func (wrapper *IPAddressClientWrapper) Get(ctx context.Context, rg string, IPName string, expand string) (network.PublicIPAddress, error) {
	if wrapper.mock == nil {
		return wrapper.client.Get(ctx, rg, IPName, expand)
	} else {
		ip := "1.1.1.1"
		return network.PublicIPAddress{PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{IPAddress: &ip}}, nil
	}
}

func (wrapper *IPAddressClientWrapper) SetAuthorizer(Authorizer autorest.Authorizer) {
	if wrapper.mock == nil {
		wrapper.client.Authorizer = Authorizer
	}
}
