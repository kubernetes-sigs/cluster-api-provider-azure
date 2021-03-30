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
// Source: ../subnets.go

// Package mock_subnets is a generated GoMock package.
package mock_subnets

import (
	reflect "reflect"

	autorest "github.com/Azure/go-autorest/autorest"
	logr "github.com/go-logr/logr"
	gomock "github.com/golang/mock/gomock"
	v1alpha4 "sigs.k8s.io/cluster-api-provider-azure/api/v1alpha4"
	azure "sigs.k8s.io/cluster-api-provider-azure/azure"
)

// MockSubnetScope is a mock of SubnetScope interface.
type MockSubnetScope struct {
	ctrl     *gomock.Controller
	recorder *MockSubnetScopeMockRecorder
}

// MockSubnetScopeMockRecorder is the mock recorder for MockSubnetScope.
type MockSubnetScopeMockRecorder struct {
	mock *MockSubnetScope
}

// NewMockSubnetScope creates a new mock instance.
func NewMockSubnetScope(ctrl *gomock.Controller) *MockSubnetScope {
	mock := &MockSubnetScope{ctrl: ctrl}
	mock.recorder = &MockSubnetScopeMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSubnetScope) EXPECT() *MockSubnetScopeMockRecorder {
	return m.recorder
}

// APIServerLBName mocks base method.
func (m *MockSubnetScope) APIServerLBName() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "APIServerLBName")
	ret0, _ := ret[0].(string)
	return ret0
}

// APIServerLBName indicates an expected call of APIServerLBName.
func (mr *MockSubnetScopeMockRecorder) APIServerLBName() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "APIServerLBName", reflect.TypeOf((*MockSubnetScope)(nil).APIServerLBName))
}

// APIServerLBPoolName mocks base method.
func (m *MockSubnetScope) APIServerLBPoolName(arg0 string) string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "APIServerLBPoolName", arg0)
	ret0, _ := ret[0].(string)
	return ret0
}

// APIServerLBPoolName indicates an expected call of APIServerLBPoolName.
func (mr *MockSubnetScopeMockRecorder) APIServerLBPoolName(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "APIServerLBPoolName", reflect.TypeOf((*MockSubnetScope)(nil).APIServerLBPoolName), arg0)
}

// AdditionalTags mocks base method.
func (m *MockSubnetScope) AdditionalTags() v1alpha4.Tags {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AdditionalTags")
	ret0, _ := ret[0].(v1alpha4.Tags)
	return ret0
}

// AdditionalTags indicates an expected call of AdditionalTags.
func (mr *MockSubnetScopeMockRecorder) AdditionalTags() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AdditionalTags", reflect.TypeOf((*MockSubnetScope)(nil).AdditionalTags))
}

// Authorizer mocks base method.
func (m *MockSubnetScope) Authorizer() autorest.Authorizer {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Authorizer")
	ret0, _ := ret[0].(autorest.Authorizer)
	return ret0
}

// Authorizer indicates an expected call of Authorizer.
func (mr *MockSubnetScopeMockRecorder) Authorizer() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Authorizer", reflect.TypeOf((*MockSubnetScope)(nil).Authorizer))
}

// AvailabilitySetEnabled mocks base method.
func (m *MockSubnetScope) AvailabilitySetEnabled() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AvailabilitySetEnabled")
	ret0, _ := ret[0].(bool)
	return ret0
}

// AvailabilitySetEnabled indicates an expected call of AvailabilitySetEnabled.
func (mr *MockSubnetScopeMockRecorder) AvailabilitySetEnabled() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AvailabilitySetEnabled", reflect.TypeOf((*MockSubnetScope)(nil).AvailabilitySetEnabled))
}

// BaseURI mocks base method.
func (m *MockSubnetScope) BaseURI() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "BaseURI")
	ret0, _ := ret[0].(string)
	return ret0
}

// BaseURI indicates an expected call of BaseURI.
func (mr *MockSubnetScopeMockRecorder) BaseURI() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "BaseURI", reflect.TypeOf((*MockSubnetScope)(nil).BaseURI))
}

// ClientID mocks base method.
func (m *MockSubnetScope) ClientID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ClientID")
	ret0, _ := ret[0].(string)
	return ret0
}

// ClientID indicates an expected call of ClientID.
func (mr *MockSubnetScopeMockRecorder) ClientID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClientID", reflect.TypeOf((*MockSubnetScope)(nil).ClientID))
}

// ClientSecret mocks base method.
func (m *MockSubnetScope) ClientSecret() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ClientSecret")
	ret0, _ := ret[0].(string)
	return ret0
}

// ClientSecret indicates an expected call of ClientSecret.
func (mr *MockSubnetScopeMockRecorder) ClientSecret() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClientSecret", reflect.TypeOf((*MockSubnetScope)(nil).ClientSecret))
}

// CloudEnvironment mocks base method.
func (m *MockSubnetScope) CloudEnvironment() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloudEnvironment")
	ret0, _ := ret[0].(string)
	return ret0
}

// CloudEnvironment indicates an expected call of CloudEnvironment.
func (mr *MockSubnetScopeMockRecorder) CloudEnvironment() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloudEnvironment", reflect.TypeOf((*MockSubnetScope)(nil).CloudEnvironment))
}

// ClusterName mocks base method.
func (m *MockSubnetScope) ClusterName() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ClusterName")
	ret0, _ := ret[0].(string)
	return ret0
}

// ClusterName indicates an expected call of ClusterName.
func (mr *MockSubnetScopeMockRecorder) ClusterName() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ClusterName", reflect.TypeOf((*MockSubnetScope)(nil).ClusterName))
}

// ControlPlaneRouteTable mocks base method.
func (m *MockSubnetScope) ControlPlaneRouteTable() v1alpha4.RouteTable {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ControlPlaneRouteTable")
	ret0, _ := ret[0].(v1alpha4.RouteTable)
	return ret0
}

// ControlPlaneRouteTable indicates an expected call of ControlPlaneRouteTable.
func (mr *MockSubnetScopeMockRecorder) ControlPlaneRouteTable() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ControlPlaneRouteTable", reflect.TypeOf((*MockSubnetScope)(nil).ControlPlaneRouteTable))
}

// ControlPlaneSubnet mocks base method.
func (m *MockSubnetScope) ControlPlaneSubnet() (string, v1alpha4.SubnetSpec) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ControlPlaneSubnet")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(v1alpha4.SubnetSpec)
	return ret0, ret1
}

// ControlPlaneSubnet indicates an expected call of ControlPlaneSubnet.
func (mr *MockSubnetScopeMockRecorder) ControlPlaneSubnet() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ControlPlaneSubnet", reflect.TypeOf((*MockSubnetScope)(nil).ControlPlaneSubnet))
}

// Enabled mocks base method.
func (m *MockSubnetScope) Enabled() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Enabled")
	ret0, _ := ret[0].(bool)
	return ret0
}

// Enabled indicates an expected call of Enabled.
func (mr *MockSubnetScopeMockRecorder) Enabled() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Enabled", reflect.TypeOf((*MockSubnetScope)(nil).Enabled))
}

// Error mocks base method.
func (m *MockSubnetScope) Error(err error, msg string, keysAndValues ...interface{}) {
	m.ctrl.T.Helper()
	varargs := []interface{}{err, msg}
	for _, a := range keysAndValues {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Error", varargs...)
}

// Error indicates an expected call of Error.
func (mr *MockSubnetScopeMockRecorder) Error(err, msg interface{}, keysAndValues ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{err, msg}, keysAndValues...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Error", reflect.TypeOf((*MockSubnetScope)(nil).Error), varargs...)
}

// HashKey mocks base method.
func (m *MockSubnetScope) HashKey() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HashKey")
	ret0, _ := ret[0].(string)
	return ret0
}

// HashKey indicates an expected call of HashKey.
func (mr *MockSubnetScopeMockRecorder) HashKey() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HashKey", reflect.TypeOf((*MockSubnetScope)(nil).HashKey))
}

// Info mocks base method.
func (m *MockSubnetScope) Info(msg string, keysAndValues ...interface{}) {
	m.ctrl.T.Helper()
	varargs := []interface{}{msg}
	for _, a := range keysAndValues {
		varargs = append(varargs, a)
	}
	m.ctrl.Call(m, "Info", varargs...)
}

// Info indicates an expected call of Info.
func (mr *MockSubnetScopeMockRecorder) Info(msg interface{}, keysAndValues ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	varargs := append([]interface{}{msg}, keysAndValues...)
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Info", reflect.TypeOf((*MockSubnetScope)(nil).Info), varargs...)
}

// IsAPIServerPrivate mocks base method.
func (m *MockSubnetScope) IsAPIServerPrivate() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsAPIServerPrivate")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsAPIServerPrivate indicates an expected call of IsAPIServerPrivate.
func (mr *MockSubnetScopeMockRecorder) IsAPIServerPrivate() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsAPIServerPrivate", reflect.TypeOf((*MockSubnetScope)(nil).IsAPIServerPrivate))
}

// IsIPv6Enabled mocks base method.
func (m *MockSubnetScope) IsIPv6Enabled() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsIPv6Enabled")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsIPv6Enabled indicates an expected call of IsIPv6Enabled.
func (mr *MockSubnetScopeMockRecorder) IsIPv6Enabled() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsIPv6Enabled", reflect.TypeOf((*MockSubnetScope)(nil).IsIPv6Enabled))
}

// IsVnetManaged mocks base method.
func (m *MockSubnetScope) IsVnetManaged() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsVnetManaged")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsVnetManaged indicates an expected call of IsVnetManaged.
func (mr *MockSubnetScopeMockRecorder) IsVnetManaged() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsVnetManaged", reflect.TypeOf((*MockSubnetScope)(nil).IsVnetManaged))
}

// Location mocks base method.
func (m *MockSubnetScope) Location() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Location")
	ret0, _ := ret[0].(string)
	return ret0
}

// Location indicates an expected call of Location.
func (mr *MockSubnetScopeMockRecorder) Location() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Location", reflect.TypeOf((*MockSubnetScope)(nil).Location))
}

// NodeRouteTable mocks base method.
func (m *MockSubnetScope) NodeRouteTable() v1alpha4.RouteTable {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NodeRouteTable")
	ret0, _ := ret[0].(v1alpha4.RouteTable)
	return ret0
}

// NodeRouteTable indicates an expected call of NodeRouteTable.
func (mr *MockSubnetScopeMockRecorder) NodeRouteTable() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NodeRouteTable", reflect.TypeOf((*MockSubnetScope)(nil).NodeRouteTable))
}

// NodeSubnet mocks base method.
func (m *MockSubnetScope) NodeSubnet() (string, v1alpha4.SubnetSpec) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NodeSubnet")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(v1alpha4.SubnetSpec)
	return ret0, ret1
}

// NodeSubnet indicates an expected call of NodeSubnet.
func (mr *MockSubnetScopeMockRecorder) NodeSubnet() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NodeSubnet", reflect.TypeOf((*MockSubnetScope)(nil).NodeSubnet))
}

// OutboundLBName mocks base method.
func (m *MockSubnetScope) OutboundLBName(arg0 string) string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "OutboundLBName", arg0)
	ret0, _ := ret[0].(string)
	return ret0
}

// OutboundLBName indicates an expected call of OutboundLBName.
func (mr *MockSubnetScopeMockRecorder) OutboundLBName(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OutboundLBName", reflect.TypeOf((*MockSubnetScope)(nil).OutboundLBName), arg0)
}

// OutboundPoolName mocks base method.
func (m *MockSubnetScope) OutboundPoolName(arg0 string) string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "OutboundPoolName", arg0)
	ret0, _ := ret[0].(string)
	return ret0
}

// OutboundPoolName indicates an expected call of OutboundPoolName.
func (mr *MockSubnetScopeMockRecorder) OutboundPoolName(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "OutboundPoolName", reflect.TypeOf((*MockSubnetScope)(nil).OutboundPoolName), arg0)
}

// ResourceGroup mocks base method.
func (m *MockSubnetScope) ResourceGroup() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ResourceGroup")
	ret0, _ := ret[0].(string)
	return ret0
}

// ResourceGroup indicates an expected call of ResourceGroup.
func (mr *MockSubnetScopeMockRecorder) ResourceGroup() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ResourceGroup", reflect.TypeOf((*MockSubnetScope)(nil).ResourceGroup))
}

// SetSubnet mocks base method.
func (m *MockSubnetScope) SetSubnet(arg0 string, arg1 v1alpha4.SubnetSpec) {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "SetSubnet", arg0, arg1)
}

// SetSubnet indicates an expected call of SetSubnet.
func (mr *MockSubnetScopeMockRecorder) SetSubnet(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SetSubnet", reflect.TypeOf((*MockSubnetScope)(nil).SetSubnet), arg0, arg1)
}

// Subnet mocks base method.
func (m *MockSubnetScope) Subnet(arg0 string) v1alpha4.SubnetSpec {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Subnet", arg0)
	ret0, _ := ret[0].(v1alpha4.SubnetSpec)
	return ret0
}

// Subnet indicates an expected call of Subnet.
func (mr *MockSubnetScopeMockRecorder) Subnet(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Subnet", reflect.TypeOf((*MockSubnetScope)(nil).Subnet), arg0)
}

// SubnetSpecs mocks base method.
func (m *MockSubnetScope) SubnetSpecs() []azure.SubnetSpec {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubnetSpecs")
	ret0, _ := ret[0].([]azure.SubnetSpec)
	return ret0
}

// SubnetSpecs indicates an expected call of SubnetSpecs.
func (mr *MockSubnetScopeMockRecorder) SubnetSpecs() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubnetSpecs", reflect.TypeOf((*MockSubnetScope)(nil).SubnetSpecs))
}

// SubscriptionID mocks base method.
func (m *MockSubnetScope) SubscriptionID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "SubscriptionID")
	ret0, _ := ret[0].(string)
	return ret0
}

// SubscriptionID indicates an expected call of SubscriptionID.
func (mr *MockSubnetScopeMockRecorder) SubscriptionID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "SubscriptionID", reflect.TypeOf((*MockSubnetScope)(nil).SubscriptionID))
}

// TenantID mocks base method.
func (m *MockSubnetScope) TenantID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TenantID")
	ret0, _ := ret[0].(string)
	return ret0
}

// TenantID indicates an expected call of TenantID.
func (mr *MockSubnetScopeMockRecorder) TenantID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TenantID", reflect.TypeOf((*MockSubnetScope)(nil).TenantID))
}

// V mocks base method.
func (m *MockSubnetScope) V(level int) logr.Logger {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "V", level)
	ret0, _ := ret[0].(logr.Logger)
	return ret0
}

// V indicates an expected call of V.
func (mr *MockSubnetScopeMockRecorder) V(level interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "V", reflect.TypeOf((*MockSubnetScope)(nil).V), level)
}

// Vnet mocks base method.
func (m *MockSubnetScope) Vnet() *v1alpha4.VnetSpec {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Vnet")
	ret0, _ := ret[0].(*v1alpha4.VnetSpec)
	return ret0
}

// Vnet indicates an expected call of Vnet.
func (mr *MockSubnetScopeMockRecorder) Vnet() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Vnet", reflect.TypeOf((*MockSubnetScope)(nil).Vnet))
}

// WithName mocks base method.
func (m *MockSubnetScope) WithName(name string) logr.Logger {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "WithName", name)
	ret0, _ := ret[0].(logr.Logger)
	return ret0
}

// WithName indicates an expected call of WithName.
func (mr *MockSubnetScopeMockRecorder) WithName(name interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WithName", reflect.TypeOf((*MockSubnetScope)(nil).WithName), name)
}

// WithValues mocks base method.
func (m *MockSubnetScope) WithValues(keysAndValues ...interface{}) logr.Logger {
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
func (mr *MockSubnetScopeMockRecorder) WithValues(keysAndValues ...interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "WithValues", reflect.TypeOf((*MockSubnetScope)(nil).WithValues), keysAndValues...)
}
