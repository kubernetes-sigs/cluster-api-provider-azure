/*
Copyright 2020 The Kubernetes Authors.

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

// Run go generate to regenerate this mock.
//go:generate ../../../../hack/tools/bin/mockgen -destination client_mock.go -package mock_roleassignments -source ../client.go Client
//go:generate ../../../../hack/tools/bin/mockgen -destination roleassignments_mock.go -package mock_roleassignments -source ../roleassignments.go RoleAssignmentScope
//go:generate /usr/bin/env bash -c "cat ../../../../hack/boilerplate/boilerplate.generatego.txt client_mock.go > _client_mock.go && mv _client_mock.go client_mock.go"
//go:generate /usr/bin/env bash -c "cat ../../../../hack/boilerplate/boilerplate.generatego.txt roleassignments_mock.go > _roleassignments_mock.go && mv _roleassignments_mock.go roleassignments_mock.go"
package mock_roleassignments
