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
// Source: ../service.go

// Package mock_publicips is a generated GoMock package.
package mock_publicips

import (
	autorest "github.com/Azure/go-autorest/autorest"
	logr "github.com/go-logr/logr"
	gomock "github.com/golang/mock/gomock"
	reflect "reflect"
	v1alpha3 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha3"
	azure "sigs.k8s.io/cluster-api-provider-azure/cloud"
)

// MockPublicIPScope is a mock of PublicIPScope interface.
type MockPublicIPScope struct {
	ctrl     *gomock.Controller
	recorder *MockPublicIPScopeMockRecorder
}

// MockPublicIPScopeMockRecorder is the mock recorder for MockPublicIPScope.
type MockPublicIPScopeMockRecorder struct {
	mock *MockPublicIPScope
}

// NewMockPublicIPScope creates a new mock instance.
func NewMockPublicIPScope(ctrl *gomock.Controller) *MockPublicIPScope {
	mock := &MockPublicIPScope{ctrl: ctrl}
	mock.recorder = &MockPublicIPScopeMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockPublicIPScope) EXPECT() *MockPublicIPScopeMockRecorder {
	return m.recorder
}

// Info mocks base method.
func (m *MockPublicIPScope) Info(msg string, keysAndValues ...interface{}) {
	m.ctrl.T.Helper()
	varargs := []interface{}{msg}
	for _, a := range keysAndValues {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Info", varargs...)
}

// Info indicates an expected call of Info.
func (mr *MockPublicIPScopeMockRecorder) Info(msg interface{}, keysAndValues ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{msg}, keysAndValues...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Info", reflect.TypeOf((*MockPublicIPScope)(nil).Info), varargs...)
}

// Enabled mocks base method.
func (m *MockPublicIPScope) Enabled() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Enabled")
	ret0, _ := ret[0].(bool)
	return ret0
}

// Enabled indicates an expected call of Enabled.
func (mr *MockPublicIPScopeMockRecorder) Enabled() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Enabled", reflect.TypeOf((*MockPublicIPScope)(nil).Enabled))
}

// Error mocks base method.
func (m *MockPublicIPScope) Error(err error, msg string, keysAndValues ...interface{}) {
	m.ctrl.T.Helper()
	varargs := []interface{}{err, msg}
	for _, a := range keysAndValues {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Error", varargs...)
}

// Error indicates an expected call of Error.
func (mr *MockPublicIPScopeMockRecorder) Error(err, msg interface{}, keysAndValues ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{err, msg}, keysAndValues...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Error", reflect.TypeOf((*MockPublicIPScope)(nil).Error), varargs...)
}

// V mocks base method.
func (m *MockPublicIPScope) V(level int) logr.InfoLogger {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "V", level)
	ret0, _ := ret[0].(logr.InfoLogger)
	return ret0
}

// V indicates an expected call of V.
func (mr *MockPublicIPScopeMockRecorder) V(level interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "V", reflect.TypeOf((*MockPublicIPScope)(nil).V), level)
}

// WithValues mocks base method.
func (m *MockPublicIPScope) WithValues(keysAndValues ...interface{}) logr.Logger {
	m.ctrl.T.Helper()
	varargs := []interface{}{}
	for _, a := range keysAndValues {
		varargs = append(varargs, a)
	}
	ret := m.ctrl.Call(m, "WithValues", varargs...)
	ret0, _ := ret[0].(logr.Logger)
	return ret0
}

// WithValues indicates an expected call of WithValues.
func (mr *MockPublicIPScopeMockRecorder) WithValues(keysAndValues ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WithValues", reflect.TypeOf((*MockPublicIPScope)(nil).WithValues), keysAndValues...)
}

// WithName mocks base method.
func (m *MockPublicIPScope) WithName(name string) logr.Logger {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WithName", name)
	ret0, _ := ret[0].(logr.Logger)
	return ret0
}

// WithName indicates an expected call of WithName.
func (mr *MockPublicIPScopeMockRecorder) WithName(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WithName", reflect.TypeOf((*MockPublicIPScope)(nil).WithName), name)
}

// SubscriptionID mocks base method.
func (m *MockPublicIPScope) SubscriptionID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubscriptionID")
	ret0, _ := ret[0].(string)
	return ret0
}

// SubscriptionID indicates an expected call of SubscriptionID.
func (mr *MockPublicIPScopeMockRecorder) SubscriptionID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscriptionID", reflect.TypeOf((*MockPublicIPScope)(nil).SubscriptionID))
}

// ClientID mocks base method.
func (m *MockPublicIPScope) ClientID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ClientID")
	ret0, _ := ret[0].(string)
	return ret0
}

// ClientID indicates an expected call of ClientID.
func (mr *MockPublicIPScopeMockRecorder) ClientID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClientID", reflect.TypeOf((*MockPublicIPScope)(nil).ClientID))
}

// ClientSecret mocks base method.
func (m *MockPublicIPScope) ClientSecret() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ClientSecret")
	ret0, _ := ret[0].(string)
	return ret0
}

// ClientSecret indicates an expected call of ClientSecret.
func (mr *MockPublicIPScopeMockRecorder) ClientSecret() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClientSecret", reflect.TypeOf((*MockPublicIPScope)(nil).ClientSecret))
}

// CloudEnvironment mocks base method.
func (m *MockPublicIPScope) CloudEnvironment() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloudEnvironment")
	ret0, _ := ret[0].(string)
	return ret0
}

// CloudEnvironment indicates an expected call of CloudEnvironment.
func (mr *MockPublicIPScopeMockRecorder) CloudEnvironment() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloudEnvironment", reflect.TypeOf((*MockPublicIPScope)(nil).CloudEnvironment))
}

// TenantID mocks base method.
func (m *MockPublicIPScope) TenantID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TenantID")
	ret0, _ := ret[0].(string)
	return ret0
}

// TenantID indicates an expected call of TenantID.
func (mr *MockPublicIPScopeMockRecorder) TenantID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TenantID", reflect.TypeOf((*MockPublicIPScope)(nil).TenantID))
}

// BaseURI mocks base method.
func (m *MockPublicIPScope) BaseURI() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BaseURI")
	ret0, _ := ret[0].(string)
	return ret0
}

// BaseURI indicates an expected call of BaseURI.
func (mr *MockPublicIPScopeMockRecorder) BaseURI() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BaseURI", reflect.TypeOf((*MockPublicIPScope)(nil).BaseURI))
}

// Authorizer mocks base method.
func (m *MockPublicIPScope) Authorizer() autorest.Authorizer {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Authorizer")
	ret0, _ := ret[0].(autorest.Authorizer)
	return ret0
}

// Authorizer indicates an expected call of Authorizer.
func (mr *MockPublicIPScopeMockRecorder) Authorizer() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Authorizer", reflect.TypeOf((*MockPublicIPScope)(nil).Authorizer))
}

// ResourceGroup mocks base method.
func (m *MockPublicIPScope) ResourceGroup() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ResourceGroup")
	ret0, _ := ret[0].(string)
	return ret0
}

// ResourceGroup indicates an expected call of ResourceGroup.
func (mr *MockPublicIPScopeMockRecorder) ResourceGroup() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ResourceGroup", reflect.TypeOf((*MockPublicIPScope)(nil).ResourceGroup))
}

// ClusterName mocks base method.
func (m *MockPublicIPScope) ClusterName() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ClusterName")
	ret0, _ := ret[0].(string)
	return ret0
}

// ClusterName indicates an expected call of ClusterName.
func (mr *MockPublicIPScopeMockRecorder) ClusterName() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClusterName", reflect.TypeOf((*MockPublicIPScope)(nil).ClusterName))
}

// Location mocks base method.
func (m *MockPublicIPScope) Location() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Location")
	ret0, _ := ret[0].(string)
	return ret0
}

// Location indicates an expected call of Location.
func (mr *MockPublicIPScopeMockRecorder) Location() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Location", reflect.TypeOf((*MockPublicIPScope)(nil).Location))
}

// AdditionalTags mocks base method.
func (m *MockPublicIPScope) AdditionalTags() v1alpha3.Tags {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AdditionalTags")
	ret0, _ := ret[0].(v1alpha3.Tags)
	return ret0
}

// AdditionalTags indicates an expected call of AdditionalTags.
func (mr *MockPublicIPScopeMockRecorder) AdditionalTags() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AdditionalTags", reflect.TypeOf((*MockPublicIPScope)(nil).AdditionalTags))
}

// Vnet mocks base method.
func (m *MockPublicIPScope) Vnet() *v1alpha3.VnetSpec {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Vnet")
	ret0, _ := ret[0].(*v1alpha3.VnetSpec)
	return ret0
}

// Vnet indicates an expected call of Vnet.
func (mr *MockPublicIPScopeMockRecorder) Vnet() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Vnet", reflect.TypeOf((*MockPublicIPScope)(nil).Vnet))
}

// IsVnetManaged mocks base method.
func (m *MockPublicIPScope) IsVnetManaged() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsVnetManaged")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsVnetManaged indicates an expected call of IsVnetManaged.
func (mr *MockPublicIPScopeMockRecorder) IsVnetManaged() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsVnetManaged", reflect.TypeOf((*MockPublicIPScope)(nil).IsVnetManaged))
}

// NodeSubnet mocks base method.
func (m *MockPublicIPScope) NodeSubnet() *v1alpha3.SubnetSpec {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NodeSubnet")
	ret0, _ := ret[0].(*v1alpha3.SubnetSpec)
	return ret0
}

// NodeSubnet indicates an expected call of NodeSubnet.
func (mr *MockPublicIPScopeMockRecorder) NodeSubnet() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NodeSubnet", reflect.TypeOf((*MockPublicIPScope)(nil).NodeSubnet))
}

// ControlPlaneSubnet mocks base method.
func (m *MockPublicIPScope) ControlPlaneSubnet() *v1alpha3.SubnetSpec {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ControlPlaneSubnet")
	ret0, _ := ret[0].(*v1alpha3.SubnetSpec)
	return ret0
}

// ControlPlaneSubnet indicates an expected call of ControlPlaneSubnet.
func (mr *MockPublicIPScopeMockRecorder) ControlPlaneSubnet() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ControlPlaneSubnet", reflect.TypeOf((*MockPublicIPScope)(nil).ControlPlaneSubnet))
}

// RouteTable mocks base method.
func (m *MockPublicIPScope) RouteTable() *v1alpha3.RouteTable {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RouteTable")
	ret0, _ := ret[0].(*v1alpha3.RouteTable)
	return ret0
}

// RouteTable indicates an expected call of RouteTable.
func (mr *MockPublicIPScopeMockRecorder) RouteTable() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RouteTable", reflect.TypeOf((*MockPublicIPScope)(nil).RouteTable))
}

// NatGateway mocks base method.
func (m *MockPublicIPScope) NatGateway() *v1alpha3.NatGateway {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NatGateway")
	ret0, _ := ret[0].(*v1alpha3.NatGateway)
	return ret0
}

// NatGateway indicates an expected call of NatGateway.
func (mr *MockPublicIPScopeMockRecorder) NatGateway() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NatGateway", reflect.TypeOf((*MockPublicIPScope)(nil).NatGateway))
}

// PublicIPSpecs mocks base method.
func (m *MockPublicIPScope) PublicIPSpecs() []azure.PublicIPSpec {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "PublicIPSpecs")
	ret0, _ := ret[0].([]azure.PublicIPSpec)
	return ret0
}

// PublicIPSpecs indicates an expected call of PublicIPSpecs.
func (mr *MockPublicIPScopeMockRecorder) PublicIPSpecs() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "PublicIPSpecs", reflect.TypeOf((*MockPublicIPScope)(nil).PublicIPSpecs))
}
