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

// Package cloudtest provides utilities for testing cloud providers.
package cloudtest

import (
	"encoding/json"
	"testing"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
)

// RuntimeRawExtension takes anything and turns it into a *runtime.RawExtension.
// This is helpful for creating clusterv1.Cluster/Machine objects that need
// a specific AzureClusterProviderSpec or Status.
func RuntimeRawExtension(t *testing.T, p interface{}) *runtime.RawExtension {
	t.Helper()
	out, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	return &runtime.RawExtension{
		Raw: out,
	}
}

// Log implements logr.Logger for testing. Do not use if you actually want to
// test log messages.
type Log struct{}

// Init initializes the logger from runtime information.
func (l *Log) Init(_ logr.RuntimeInfo) {}

// Error logs an error, with the given message and key/value pairs as context.
func (l *Log) Error(_ error, _ string, _ ...interface{}) {}

// V returns a new Logger instance for a specific verbosity level, relative to this Logger.
func (l *Log) V(_ int) logr.LogSink { return l }

// WithValues returns a new Logger instance with additional key/value pairs.
func (l *Log) WithValues(_ ...interface{}) logr.LogSink { return l }

// WithName returns a new Logger instance with the specified name element added to the Logger's name.
func (l *Log) WithName(_ string) logr.LogSink { return l }

// Info logs a non-error message with the given key/value pairs as context.
func (l *Log) Info(_ int, _ string, _ ...interface{}) {}

// Enabled tests whether this Logger is enabled.
func (l *Log) Enabled(_ int) bool { return false }
