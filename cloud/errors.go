/*
Copyright 2019 The Kubernetes Authors.

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

package azure

import (
	"errors"
	"fmt"

	"github.com/Azure/go-autorest/autorest"
)

// ErrNotOwned is returned when a resource can't be deleted because it isn't owned.
var ErrNotOwned = errors.New("resource is not managed and cannot be deleted")

// ResourceNotFound parses the error to check if it's a resource not found error.
func ResourceNotFound(err error) bool {
	derr := autorest.DetailedError{}
	return errors.As(err, &derr) && derr.StatusCode == 404
}

// VMDeletedError is returned when a virtual machine is deleted outside of capz
type VMDeletedError struct {
	ProviderID string
}

// Error returns the error string
func (vde VMDeletedError) Error() string {
	return fmt.Sprintf("VM with provider id %q has been deleted", vde.ProviderID)
}
