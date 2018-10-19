package resourcemanagement

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	"github.com/Azure/go-autorest/autorest"
)

type Service struct {
	GroupsClient resources.GroupsClient
	ctx          context.Context
}

func NewService(subscriptionId string) *Service {
	return &Service{
		GroupsClient: resources.NewGroupsClient(subscriptionId),
		ctx:          context.Background(),
	}
}

func (s *Service) SetAuthorizer(authorizer autorest.Authorizer) {
	s.GroupsClient.BaseClient.Client.Authorizer = authorizer
}
