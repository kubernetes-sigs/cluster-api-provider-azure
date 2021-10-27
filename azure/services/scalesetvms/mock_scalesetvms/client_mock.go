/*
Copyright The Kubernetes Authors.

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

// Code generated by MockGen. DO NOT EDIT.
// Source: ../client.go

// Package mock_scalesetvms is a generated GoMock package.
package mock_scalesetvms

import (
	context "context"
	reflect "reflect"

	compute "github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-07-01/compute"
	autorest "github.com/Azure/go-autorest/autorest"
	gomock "github.com/golang/mock/gomock"
	v1beta1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
)

// Mockclient is a mock of client interface.
type Mockclient struct {
	ctrl     *gomock.Controller
	recorder *MockclientMockRecorder
}

// MockclientMockRecorder is the mock recorder for Mockclient.
type MockclientMockRecorder struct {
	mock *Mockclient
}

// NewMockclient creates a new mock instance.
func NewMockclient(ctrl *gomock.Controller) *Mockclient {
	mock := &Mockclient{ctrl: ctrl}
	mock.recorder = &MockclientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *Mockclient) EXPECT() *MockclientMockRecorder {
	return m.recorder
}

// DeleteAsync mocks base method.
func (m *Mockclient) DeleteAsync(arg0 context.Context, arg1, arg2, arg3 string) (*v1beta1.Future, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteAsync", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(*v1beta1.Future)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteAsync indicates an expected call of DeleteAsync.
func (mr *MockclientMockRecorder) DeleteAsync(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteAsync", reflect.TypeOf((*Mockclient)(nil).DeleteAsync), arg0, arg1, arg2, arg3)
}

// Get mocks base method.
func (m *Mockclient) Get(arg0 context.Context, arg1, arg2, arg3 string) (compute.VirtualMachineScaleSetVM, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(compute.VirtualMachineScaleSetVM)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockclientMockRecorder) Get(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*Mockclient)(nil).Get), arg0, arg1, arg2, arg3)
}

// GetResultIfDone mocks base method.
func (m *Mockclient) GetResultIfDone(ctx context.Context, future *v1beta1.Future) (compute.VirtualMachineScaleSetVM, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetResultIfDone", ctx, future)
	ret0, _ := ret[0].(compute.VirtualMachineScaleSetVM)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetResultIfDone indicates an expected call of GetResultIfDone.
func (mr *MockclientMockRecorder) GetResultIfDone(ctx, future interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetResultIfDone", reflect.TypeOf((*Mockclient)(nil).GetResultIfDone), ctx, future)
}

// MockgenericScaleSetVMFuture is a mock of genericScaleSetVMFuture interface.
type MockgenericScaleSetVMFuture struct {
	ctrl     *gomock.Controller
	recorder *MockgenericScaleSetVMFutureMockRecorder
}

// MockgenericScaleSetVMFutureMockRecorder is the mock recorder for MockgenericScaleSetVMFuture.
type MockgenericScaleSetVMFutureMockRecorder struct {
	mock *MockgenericScaleSetVMFuture
}

// NewMockgenericScaleSetVMFuture creates a new mock instance.
func NewMockgenericScaleSetVMFuture(ctrl *gomock.Controller) *MockgenericScaleSetVMFuture {
	mock := &MockgenericScaleSetVMFuture{ctrl: ctrl}
	mock.recorder = &MockgenericScaleSetVMFutureMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockgenericScaleSetVMFuture) EXPECT() *MockgenericScaleSetVMFutureMockRecorder {
	return m.recorder
}

// DoneWithContext mocks base method.
func (m *MockgenericScaleSetVMFuture) DoneWithContext(ctx context.Context, sender autorest.Sender) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DoneWithContext", ctx, sender)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DoneWithContext indicates an expected call of DoneWithContext.
func (mr *MockgenericScaleSetVMFutureMockRecorder) DoneWithContext(ctx, sender interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DoneWithContext", reflect.TypeOf((*MockgenericScaleSetVMFuture)(nil).DoneWithContext), ctx, sender)
}

// Result mocks base method.
func (m *MockgenericScaleSetVMFuture) Result(client compute.VirtualMachineScaleSetVMsClient) (compute.VirtualMachineScaleSetVM, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Result", client)
	ret0, _ := ret[0].(compute.VirtualMachineScaleSetVM)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Result indicates an expected call of Result.
func (mr *MockgenericScaleSetVMFutureMockRecorder) Result(client interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Result", reflect.TypeOf((*MockgenericScaleSetVMFuture)(nil).Result), client)
}
