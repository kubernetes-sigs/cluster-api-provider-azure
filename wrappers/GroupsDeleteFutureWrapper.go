package wrappers

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2018-02-01/resources"
	"github.com/Azure/go-autorest/autorest"
)

type GroupsDeleteFutureWrapper struct {
	mock   bool
	future resources.GroupsDeleteFuture
}

func (wrapper *GroupsDeleteFutureWrapper) WaitForCompletion(ctx context.Context, client autorest.Client) error {
	if !wrapper.mock {
		return wrapper.future.Future.WaitForCompletionRef(ctx, client)
	}
	return nil
}
