/*
Copyright 2022 The Kubernetes Authors.

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

package privatedns

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/services/privatedns/mgmt/2018-09-01/privatedns"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/pkg/errors"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/util/tele"
)

// azureRecordsClient contains the Azure go-sdk Client for record sets.
type azureRecordsClient struct {
	recordsets privatedns.RecordSetsClient
}

// newRecordSetsClient creates a new record sets client from subscription ID.
func newRecordSetsClient(auth azure.Authorizer) *azureRecordsClient {
	recordsClient := privatedns.NewRecordSetsClientWithBaseURI(auth.BaseURI(), auth.SubscriptionID())
	azure.SetAutoRestClientDefaults(&recordsClient.Client, auth.Authorizer())
	return &azureRecordsClient{
		recordsets: recordsClient,
	}
}

// CreateOrUpdateAsync creates or updates a record asynchronously.
// Creating a record set is not a long running operation, so we don't ever return a future.
func (arc *azureRecordsClient) CreateOrUpdateAsync(ctx context.Context, spec azure.ResourceSpecGetter, parameters interface{}) (result interface{}, future azureautorest.FutureAPI, err error) {
	ctx, _, done := tele.StartSpanWithLogger(ctx, "privatedns.azureRecordsClient.CreateOrUpdateAsync")
	defer done()

	set, ok := parameters.(privatedns.RecordSet)
	if !ok {
		return nil, nil, errors.Errorf("%T is not a privatedns.RecordSet", parameters)
	}

	// Determine record type.
	var (
		recordType privatedns.RecordType
		aRecords   = set.RecordSetProperties.ARecords
		aaaRecords = set.RecordSetProperties.AaaaRecords
	)
	if aRecords != nil && len(*aRecords) > 0 && (*aRecords)[0].Ipv4Address != nil {
		recordType = privatedns.A
	} else if aaaRecords != nil && len(*aaaRecords) > 0 && (*aaaRecords)[0].Ipv6Address != nil {
		recordType = privatedns.AAAA
	}

	recordSet, err := arc.recordsets.CreateOrUpdate(ctx, spec.ResourceGroupName(), spec.OwnerResourceName(), recordType, spec.ResourceName(), set, "", "")
	if err != nil {
		return nil, nil, err
	}

	return recordSet, nil, err
}

// Get gets the specified record set. Noop for records.
func (arc *azureRecordsClient) Get(ctx context.Context, spec azure.ResourceSpecGetter) (result interface{}, err error) {
	return nil, nil
}

// DeleteAsync deletes a record asynchronously. Noop for records.
func (arc *azureRecordsClient) DeleteAsync(ctx context.Context, spec azure.ResourceSpecGetter) (future azureautorest.FutureAPI, err error) {
	return nil, nil
}

// IsDone returns true if the long-running operation has completed. Noop for records.
func (arc *azureRecordsClient) IsDone(ctx context.Context, future azureautorest.FutureAPI) (isDone bool, err error) {
	return true, nil
}

// Result fetches the result of a long-running operation future. Noop for records.
func (arc *azureRecordsClient) Result(ctx context.Context, future azureautorest.FutureAPI, futureType string) (result interface{}, err error) {
	return nil, nil
}
