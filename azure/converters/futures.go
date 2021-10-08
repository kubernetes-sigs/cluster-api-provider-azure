/*
Copyright 2021 The Kubernetes Authors.

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

package converters

import (
	"encoding/base64"

	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

// SDKToFuture converts an SDK future to an infrav1.Future.
func SDKToFuture(future azureautorest.FutureAPI, futureType, service, resourceName, rgName string) (*infrav1.Future, error) {
	jsonData, err := future.MarshalJSON()
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal async future")
	}

	return &infrav1.Future{
		Type:          futureType,
		ResourceGroup: rgName,
		ServiceName:   service,
		Name:          resourceName,
		Data:          base64.URLEncoding.EncodeToString(jsonData),
	}, nil
}

// FutureToSDK converts an infrav1.Future to an SDK future.
func FutureToSDK(future infrav1.Future) (azureautorest.FutureAPI, error) {
	futureData, err := base64.URLEncoding.DecodeString(future.Data)
	if err != nil {
		return nil, errors.Wrap(err, "failed to base64 decode future data")
	}
	var genericFuture azureautorest.Future
	if err := genericFuture.UnmarshalJSON(futureData); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal future data")
	}
	return &genericFuture, nil
}
