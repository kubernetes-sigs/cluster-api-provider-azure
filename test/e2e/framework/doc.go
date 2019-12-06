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

// Package framework is mostly copied from the CAPI repository
// https://github.com/kubernetes-sigs/cluster-api/tree/master/test/framework
// Right now, it is hard to use CAPI's test/framework module for two reasons:
// - It has references to v1alpha3 types
// - The module has a redirect to a local file system directory
package framework
