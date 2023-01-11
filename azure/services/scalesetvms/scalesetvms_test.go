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

package scalesetvms

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	"github.com/Azure/go-autorest/autorest"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	infrav1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-azure/azure"
	"sigs.k8s.io/cluster-api-provider-azure/azure/converters"
	"sigs.k8s.io/cluster-api-provider-azure/azure/scope"
	"sigs.k8s.io/cluster-api-provider-azure/azure/services/scalesetvms/mock_scalesetvms"
	infrav1exp "sigs.k8s.io/cluster-api-provider-azure/exp/api/v1beta1"
	gomock2 "sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomock"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	expv1 "sigs.k8s.io/cluster-api/exp/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	autorest404 = autorest.NewErrorWithResponse("", "", &http.Response{StatusCode: http.StatusNotFound}, "Not Found")
)

func TestNewService(t *testing.T) {
	g := NewGomegaWithT(t)
	scheme := runtime.NewScheme()
	_ = clusterv1.AddToScheme(scheme)
	_ = expv1.AddToScheme(scheme)
	_ = infrav1.AddToScheme(scheme)
	_ = infrav1exp.AddToScheme(scheme)

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).Build()
	s, err := scope.NewClusterScope(context.Background(), scope.ClusterScopeParams{
		AzureClients: scope.AzureClients{
			Authorizer: autorest.NullAuthorizer{},
		},
		Client:  client,
		Cluster: cluster,
		AzureCluster: &infrav1.AzureCluster{
			Spec: infrav1.AzureClusterSpec{
				AzureClusterClassSpec: infrav1.AzureClusterClassSpec{
					Location:       "test-location",
					SubscriptionID: "123",
				},
				ResourceGroup: "my-rg",
				NetworkSpec: infrav1.NetworkSpec{
					Vnet: infrav1.VnetSpec{Name: "my-vnet", ResourceGroup: "my-rg"},
				},
			},
		},
	})
	g.Expect(err).NotTo(HaveOccurred())

	mpms, err := scope.NewMachinePoolMachineScope(scope.MachinePoolMachineScopeParams{
		Client:                  client,
		MachinePool:             new(expv1.MachinePool),
		AzureMachinePool:        new(infrav1exp.AzureMachinePool),
		AzureMachinePoolMachine: new(infrav1exp.AzureMachinePoolMachine),
		ClusterScope:            s,
	})
	g.Expect(err).NotTo(HaveOccurred())
	actual := NewService(mpms)
	g.Expect(actual).NotTo(BeNil())
}

func TestService_Reconcile(t *testing.T) {
	cases := []struct {
		Name       string
		Setup      func(s *mock_scalesetvms.MockScaleSetVMScopeMockRecorder, m *mock_scalesetvms.MockclientMockRecorder)
		Err        error
		CheckIsErr bool
	}{
		{
			Name: "should reconcile successfully",
			Setup: func(s *mock_scalesetvms.MockScaleSetVMScopeMockRecorder, m *mock_scalesetvms.MockclientMockRecorder) {
				s.ResourceGroup().Return("rg")
				s.InstanceID().Return("0")
				s.ProviderID().Return("foo")
				s.ScaleSetName().Return("scaleset")
				vm := compute.VirtualMachineScaleSetVM{
					InstanceID: pointer.String("0"),
				}
				m.Get(gomock2.AContext(), "rg", "scaleset", "0").Return(vm, nil)
				s.SetVMSSVM(converters.SDKToVMSSVM(vm))
			},
		},
		{
			Name: "if 404, then should respond with transient error",
			Setup: func(s *mock_scalesetvms.MockScaleSetVMScopeMockRecorder, m *mock_scalesetvms.MockclientMockRecorder) {
				s.ResourceGroup().Return("rg")
				s.InstanceID().Return("0")
				s.ProviderID().Return("foo")
				s.ScaleSetName().Return("scaleset")
				m.Get(gomock2.AContext(), "rg", "scaleset", "0").Return(compute.VirtualMachineScaleSetVM{}, autorest404)
			},
			Err:        azure.WithTransientError(errors.New("instance does not exist yet"), 30*time.Second),
			CheckIsErr: true,
		},
		{
			Name: "if other error, then should respond with error",
			Setup: func(s *mock_scalesetvms.MockScaleSetVMScopeMockRecorder, m *mock_scalesetvms.MockclientMockRecorder) {
				s.ResourceGroup().Return("rg")
				s.InstanceID().Return("0")
				s.ProviderID().Return("foo")
				s.ScaleSetName().Return("scaleset")
				m.Get(gomock2.AContext(), "rg", "scaleset", "0").Return(compute.VirtualMachineScaleSetVM{}, errors.New("boom"))
			},
			Err: errors.Wrap(errors.New("boom"), "failed getting instance"),
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			var (
				g          = NewWithT(t)
				mockCtrl   = gomock.NewController(t)
				scopeMock  = mock_scalesetvms.NewMockScaleSetVMScope(mockCtrl)
				clientMock = mock_scalesetvms.NewMockclient(mockCtrl)
			)
			defer mockCtrl.Finish()

			scopeMock.EXPECT().SubscriptionID().Return("subID").AnyTimes()
			scopeMock.EXPECT().BaseURI().Return("https://localhost/").AnyTimes()
			scopeMock.EXPECT().Authorizer().Return(nil).AnyTimes()

			service := NewService(scopeMock)
			service.Client = clientMock
			c.Setup(scopeMock.EXPECT(), clientMock.EXPECT())

			if err := service.Reconcile(context.TODO()); c.Err == nil {
				g.Expect(err).To(Succeed())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(c.Err.Error()))
				if c.CheckIsErr {
					g.Expect(errors.Is(err, c.Err)).To(BeTrue())
				}
			}
		})
	}
}

func TestService_Delete(t *testing.T) {
	cases := []struct {
		Name       string
		Setup      func(s *mock_scalesetvms.MockScaleSetVMScopeMockRecorder, m *mock_scalesetvms.MockclientMockRecorder)
		Err        error
		CheckIsErr bool
	}{
		{
			Name: "should start deleting successfully if no long running operation is active",
			Setup: func(s *mock_scalesetvms.MockScaleSetVMScopeMockRecorder, m *mock_scalesetvms.MockclientMockRecorder) {
				s.ResourceGroup().Return("rg")
				s.InstanceID().Return("0")
				s.ProviderID().Return("foo")
				s.ScaleSetName().Return("scaleset")
				s.GetLongRunningOperationState("0", serviceName, infrav1.DeleteFuture).Return(nil)
				future := &infrav1.Future{
					Type: infrav1.DeleteFuture,
				}
				m.DeleteAsync(gomock2.AContext(), "rg", "scaleset", "0").Return(future, nil)
				s.SetLongRunningOperationState(future)
				m.GetResultIfDone(gomock2.AContext(), future).Return(compute.VirtualMachineScaleSetVM{}, azure.WithTransientError(azure.NewOperationNotDoneError(future), 15*time.Second))
				m.Get(gomock2.AContext(), "rg", "scaleset", "0").Return(compute.VirtualMachineScaleSetVM{}, nil)
			},
			CheckIsErr: true,
			Err: errors.Wrap(azure.WithTransientError(azure.NewOperationNotDoneError(&infrav1.Future{
				Type: infrav1.DeleteFuture,
			}), 15*time.Second), "failed to get result of long running operation"),
		},
		{
			Name: "should finish deleting successfully when there's a long running operation that has completed",
			Setup: func(s *mock_scalesetvms.MockScaleSetVMScopeMockRecorder, m *mock_scalesetvms.MockclientMockRecorder) {
				s.ResourceGroup().Return("rg")
				s.InstanceID().Return("0")
				s.ProviderID().Return("foo")
				s.ScaleSetName().Return("scaleset")
				future := &infrav1.Future{
					Type: infrav1.DeleteFuture,
				}
				s.GetLongRunningOperationState("0", serviceName, infrav1.DeleteFuture).Return(future)
				m.GetResultIfDone(gomock2.AContext(), future).Return(compute.VirtualMachineScaleSetVM{}, nil)
				s.DeleteLongRunningOperationState("0", serviceName, infrav1.DeleteFuture)
				m.Get(gomock2.AContext(), "rg", "scaleset", "0").Return(compute.VirtualMachineScaleSetVM{}, nil)
			},
		},
		{
			Name: "should not error when deleting, but resource is 404",
			Setup: func(s *mock_scalesetvms.MockScaleSetVMScopeMockRecorder, m *mock_scalesetvms.MockclientMockRecorder) {
				s.ResourceGroup().Return("rg")
				s.InstanceID().Return("0")
				s.ProviderID().Return("foo")
				s.ScaleSetName().Return("scaleset")
				s.GetLongRunningOperationState("0", serviceName, infrav1.DeleteFuture).Return(nil)
				m.DeleteAsync(gomock2.AContext(), "rg", "scaleset", "0").Return(nil, autorest404)
				m.Get(gomock2.AContext(), "rg", "scaleset", "0").Return(compute.VirtualMachineScaleSetVM{}, nil)
			},
		},
		{
			Name: "should error when deleting, but a non-404 error is returned from DELETE call",
			Setup: func(s *mock_scalesetvms.MockScaleSetVMScopeMockRecorder, m *mock_scalesetvms.MockclientMockRecorder) {
				s.ResourceGroup().Return("rg")
				s.InstanceID().Return("0")
				s.ProviderID().Return("foo")
				s.ScaleSetName().Return("scaleset")
				s.GetLongRunningOperationState("0", serviceName, infrav1.DeleteFuture).Return(nil)
				m.DeleteAsync(gomock2.AContext(), "rg", "scaleset", "0").Return(nil, errors.New("boom"))
				m.Get(gomock2.AContext(), "rg", "scaleset", "0").Return(compute.VirtualMachineScaleSetVM{}, nil)
			},
			Err: errors.Wrap(errors.New("boom"), "failed to delete instance scaleset/0"),
		},
		{
			Name: "should return error when a long running operation is active and getting the result returns an error",
			Setup: func(s *mock_scalesetvms.MockScaleSetVMScopeMockRecorder, m *mock_scalesetvms.MockclientMockRecorder) {
				s.ResourceGroup().Return("rg")
				s.InstanceID().Return("0")
				s.ProviderID().Return("foo")
				s.ScaleSetName().Return("scaleset")
				future := &infrav1.Future{
					Type: infrav1.DeleteFuture,
				}
				s.GetLongRunningOperationState("0", serviceName, infrav1.DeleteFuture).Return(future)
				m.GetResultIfDone(gomock2.AContext(), future).Return(compute.VirtualMachineScaleSetVM{}, errors.New("boom"))
				m.Get(gomock2.AContext(), "rg", "scaleset", "0").Return(compute.VirtualMachineScaleSetVM{}, nil)
			},
			Err: errors.Wrap(errors.New("boom"), "failed to get result of long running operation"),
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			var (
				g          = NewWithT(t)
				mockCtrl   = gomock.NewController(t)
				scopeMock  = mock_scalesetvms.NewMockScaleSetVMScope(mockCtrl)
				clientMock = mock_scalesetvms.NewMockclient(mockCtrl)
			)
			defer mockCtrl.Finish()

			scopeMock.EXPECT().SubscriptionID().Return("subID").AnyTimes()
			scopeMock.EXPECT().BaseURI().Return("https://localhost/").AnyTimes()
			scopeMock.EXPECT().Authorizer().Return(nil).AnyTimes()

			service := NewService(scopeMock)
			service.Client = clientMock
			c.Setup(scopeMock.EXPECT(), clientMock.EXPECT())

			if err := service.Delete(context.TODO()); c.Err == nil {
				g.Expect(err).To(Succeed())
			} else {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err).To(MatchError(c.Err.Error()))
				if c.CheckIsErr {
					g.Expect(errors.Is(err, c.Err)).To(BeTrue())
				}
			}
		})
	}
}
