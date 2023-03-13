/*
Copyright 2023 The Kubernetes Authors.

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
	"testing"
	"time"

	"github.com/Azure/go-autorest/autorest"
	azureautorest "github.com/Azure/go-autorest/autorest/azure"
	. "github.com/onsi/gomega"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

func TestResourceGroupNotFound(t *testing.T) {
	g := NewWithT(t)
	cases := []struct {
		Name     string
		Err      error
		Expected bool
	}{
		{
			Name:     "nil error",
			Err:      nil,
			Expected: false,
		},
		{
			Name:     "non detailed error",
			Err:      errors.New("foo"),
			Expected: false,
		},
		{
			Name:     "non service error",
			Err:      autorest.DetailedError{},
			Expected: false,
		},
		{
			Name: "service error with non resource group not found code",
			Err: autorest.DetailedError{
				Original: &azureautorest.ServiceError{
					Code: "foo",
				},
			},
			Expected: false,
		},
		{
			Name: "service error with resource group not found code",
			Err: autorest.DetailedError{
				Original: &azureautorest.ServiceError{
					Code: codeResourceGroupNotFound,
				},
			},
			Expected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			actual := ResourceGroupNotFound(tc.Err)
			g.Expect(actual).To(Equal(tc.Expected))
		})
	}
}

func TestResourceNotFound(t *testing.T) {
	g := NewWithT(t)
	cases := []struct {
		Name     string
		Err      error
		Expected bool
	}{
		{
			Name:     "nil error",
			Err:      nil,
			Expected: false,
		},
		{
			Name:     "non detailed error",
			Err:      errors.New("foo"),
			Expected: false,
		},
		{
			Name:     "detailed error with non 404 status code",
			Err:      autorest.DetailedError{},
			Expected: false,
		},
		{
			Name: "detailed error with 404 status code",
			Err: autorest.DetailedError{
				StatusCode: 404,
			},
			Expected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			actual := ResourceNotFound(tc.Err)
			g.Expect(actual).To(Equal(tc.Expected))
		})
	}
}

func TestResourceConflict(t *testing.T) {
	g := NewWithT(t)
	cases := []struct {
		Name     string
		Err      error
		Expected bool
	}{
		{
			Name:     "nil error",
			Err:      nil,
			Expected: false,
		},
		{
			Name:     "non detailed error",
			Err:      errors.New("foo"),
			Expected: false,
		},
		{
			Name:     "detailed error with non 409 status code",
			Err:      autorest.DetailedError{},
			Expected: false,
		},
		{
			Name: "detailed error with 409 status code",
			Err: autorest.DetailedError{
				StatusCode: 409,
			},
			Expected: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			actual := ResourceConflict(tc.Err)
			g.Expect(actual).To(Equal(tc.Expected))
		})
	}
}

func TestVMDeletedError(t *testing.T) {
	g := NewWithT(t)
	err := VMDeletedError{ProviderID: "test-provider-id"}
	g.Expect(err.Error()).To(Equal("VM with provider id \"test-provider-id\" has been deleted"))
}

func TestReconcileError_Error(t *testing.T) {
	g := NewWithT(t)
	cases := []struct {
		Name     string
		Err      ReconcileError
		Expected string
	}{
		{
			Name:     "empty error",
			Err:      ReconcileError{},
			Expected: "reconcile error occurred with unknown recovery type. The actual error is: ",
		},
		{
			Name: "transient error",
			Err: ReconcileError{
				errorType:    TransientErrorType,
				error:        errors.New("Transient error"),
				requestAfter: time.Second * 10,
			},
			Expected: "Transient error. Object will be requeued after 10s",
		},
		{
			Name: "terminal error",
			Err: ReconcileError{
				errorType: TerminalErrorType,
				error:     errors.New("Terminal error"),
			},
			Expected: "reconcile error that cannot be recovered occurred: Terminal error. Object will not be requeued",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			actual := tc.Err.Error()
			g.Expect(actual).To(Equal(tc.Expected))
		})
	}
}

func TestReconcileError_IsTransient(t *testing.T) {
	g := NewWithT(t)
	g.Expect(getTransientError().IsTransient()).To(BeTrue())
	g.Expect(getTerminalError().IsTransient()).To(BeFalse())
}

func TestReconcileError_IsTerminal(t *testing.T) {
	g := NewWithT(t)
	g.Expect(getTransientError().IsTerminal()).To(BeFalse())
	g.Expect(getTerminalError().IsTerminal()).To(BeTrue())
}

func TestReconcileError_Is(t *testing.T) {
	g := NewWithT(t)
	g.Expect(getTransientError().Is(getTransientError())).To(BeTrue())
	g.Expect(getTransientError().Is(errors.New("test-error"))).To(BeFalse())
}

func TestReconcileError_RequeueAfter(t *testing.T) {
	g := NewWithT(t)
	g.Expect(getTransientError().RequeueAfter()).To(Equal(time.Second * 10))
	g.Expect(getTerminalError().RequeueAfter()).To(Equal(time.Duration(0)))
}

func TestWithTransientError(t *testing.T) {
	g := NewWithT(t)
	g.Expect(WithTransientError(errors.New("foo"), time.Second*10)).To(Equal(ReconcileError{
		errorType:    TransientErrorType,
		error:        errors.New("foo"),
		requestAfter: time.Second * 10,
	}))
}

func TestWithTerminalError(t *testing.T) {
	g := NewWithT(t)
	g.Expect(WithTerminalError(errors.New("foo"))).To(Equal(ReconcileError{
		errorType: TerminalErrorType,
		error:     errors.New("foo"),
	}))
}

func TestNewOperationNotDoneError(t *testing.T) {
	g := NewWithT(t)
	testFuture := &infrav1.Future{Name: "test-future"}
	g.Expect(NewOperationNotDoneError(testFuture)).To(Equal(OperationNotDoneError{Future: testFuture}))
}

func TestOperationNotDone_Error(t *testing.T) {
	g := NewWithT(t)
	err := OperationNotDoneError{
		Future: &infrav1.Future{
			Name:          "test-future",
			Type:          "create",
			ResourceGroup: "test-rg",
		},
	}
	g.Expect(err.Error()).To(Equal("operation type create on Azure resource test-rg/test-future is not done"))
}

func TestOperationNotDone_Is(t *testing.T) {
	g := NewWithT(t)
	err := OperationNotDoneError{&infrav1.Future{Name: "test-future"}}
	g.Expect(err.Is(OperationNotDoneError{})).To(BeTrue())
	g.Expect(err.Is(ReconcileError{})).To(BeFalse())
}

func TestIsOperationNotDoneError(t *testing.T) {
	g := NewWithT(t)
	g.Expect(IsOperationNotDoneError(OperationNotDoneError{})).To(BeTrue())
	g.Expect(IsOperationNotDoneError(ReconcileError{})).To(BeFalse())
}

func getTransientError() ReconcileError {
	return ReconcileError{
		errorType:    TransientErrorType,
		error:        errors.New("Transient error"),
		requestAfter: time.Second * 10,
	}
}

func getTerminalError() ReconcileError {
	return ReconcileError{
		errorType: TerminalErrorType,
		error:     errors.New("Terminal error"),
	}
}
