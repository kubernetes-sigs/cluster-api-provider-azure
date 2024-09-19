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

// Package mock_privatedns can be regenerated using go generate.
//
//go:generate ../../../../hack/tools/bin/mockgen -destination privatedns_mock.go -package mock_privatedns -source ../privatedns.go Scope
//go:generate /usr/bin/env bash -c "cat ../../../../hack/boilerplate/boilerplate.generatego.txt privatedns_mock.go > _privatedns_mock.go && mv _privatedns_mock.go privatedns_mock.go"
package mock_privatedns
