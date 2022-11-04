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

package vmextensions

import (
	"context"
	"net/http"
	"testing"

	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/async/mock_async"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/vmextensions/mock_vmextensions"
	gomockinternal "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
)

var (
	extensionSpec1 = VMExtensionSpec{
		ExtensionSpec: azure.ExtensionSpec{
			Name:      "my-extension-1",
			VMName:    "my-vm",
			Publisher: "some-publisher",
			Version:   "1.0",
		},
		ResourceGroup: "my-rg",
		Location:      "test-location",
	}

	extensionSpec2 = VMExtensionSpec{
		ExtensionSpec: azure.ExtensionSpec{
			Name:      "my-extension-2",
			VMName:    "my-vm",
			Publisher: "other-publisher",
			Version:   "2.0",
		},
		ResourceGroup: "my-rg",
		Location:      "test-location",
	}

	internalError        = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusInternalServerError}, "Internal Server Error")
	extensionFailedError = errors.Wrapf(internalError, "extension state failed. This likely means the Kubernetes node bootstrapping process failed or timed out. Check VM boot diagnostics logs to learn more")

	notDoneError          = azure.NewOperationNotDoneError(&infrav1.Future{})
	extensionNotDoneError = errors.Wrapf(notDoneError, "extension is still in provisioning state. This likely means that bootstrapping has not yet completed on the VM")
)

func TestReconcileVMExtension(t *testing.T) {
	testcases := []struct {
		name          string
		expectedError string
		expect        func(s *mock_vmextensions.MockVMExtensionScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder)
	}{
		{
			name:          "extension is in succeeded state",
			expectedError: "",
			expect: func(s *mock_vmextensions.MockVMExtensionScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VMExtensionSpecs().Return([]azure.ResourceSpecGetter{&extensionSpec1})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &extensionSpec1, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.BootstrapSucceededCondition, serviceName, nil)
			},
		},
		{
			name:          "extension is in failed state",
			expectedError: extensionFailedError.Error(),
			expect: func(s *mock_vmextensions.MockVMExtensionScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VMExtensionSpecs().Return([]azure.ResourceSpecGetter{&extensionSpec1})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &extensionSpec1, serviceName).Return(nil, internalError)
				s.UpdatePutStatus(infrav1.BootstrapSucceededCondition, serviceName, gomockinternal.ErrStrEq(extensionFailedError.Error()))
			},
		},
		{
			name:          "extension is still creating",
			expectedError: extensionNotDoneError.Error(),
			expect: func(s *mock_vmextensions.MockVMExtensionScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VMExtensionSpecs().Return([]azure.ResourceSpecGetter{&extensionSpec1})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &extensionSpec1, serviceName).Return(nil, notDoneError)
				s.UpdatePutStatus(infrav1.BootstrapSucceededCondition, serviceName, gomockinternal.ErrStrEq(extensionNotDoneError.Error()))
			},
		},
		{
			name:          "reconcile multiple extensions",
			expectedError: "",
			expect: func(s *mock_vmextensions.MockVMExtensionScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VMExtensionSpecs().Return([]azure.ResourceSpecGetter{&extensionSpec1, &extensionSpec2})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &extensionSpec1, serviceName).Return(nil, nil)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &extensionSpec2, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.BootstrapSucceededCondition, serviceName, nil)
			},
		},
		{
			name:          "error creating the first extension",
			expectedError: extensionFailedError.Error(),
			expect: func(s *mock_vmextensions.MockVMExtensionScopeMockRecorder, r *mock_async.MockReconcilerMockRecorder) {
				s.VMExtensionSpecs().Return([]azure.ResourceSpecGetter{&extensionSpec1, &extensionSpec2})
				r.CreateOrUpdateResource(gomockinternal.AContext(), &extensionSpec1, serviceName).Return(nil, internalError)
				r.CreateOrUpdateResource(gomockinternal.AContext(), &extensionSpec2, serviceName).Return(nil, nil)
				s.UpdatePutStatus(infrav1.BootstrapSucceededCondition, serviceName, gomockinternal.ErrStrEq(extensionFailedError.Error()))
			},
		},
	}
	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)

			t.Parallel()
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()
			scopeMock := mock_vmextensions.NewMockVMExtensionScope(mockCtrl)
			asyncMock := mock_async.NewMockReconciler(mockCtrl)

			tc.expect(scopeMock.EXPECT(), asyncMock.EXPECT())

			s := &Service{
				Scope:      scopeMock,
				Reconciler: asyncMock,
			}

			err := s.Reconcile(context.TODO())
			if tc.expectedError != "" {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(tc.expectedError))
			} else {
				g.Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}
