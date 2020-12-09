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

package logentries

import (
	"sigs.k8s.io/cluster-api-provider-azure/internal/test/matchers/gomega"
)

type (
	EntryCriteria struct {
		ClusterNamespace   string
		ClusterName        string
		InfraControllers   []string
		ClusterControllers []string
	}
)

func GenerateCreateNotPausedLogEntries(ec EntryCriteria) []gomega.LogMatcher {
	infraEntries := make([]gomega.LogMatcher, len(ec.InfraControllers))
	for i, c := range ec.InfraControllers {
		c := c
		infraEntries[i] = gomega.LogContains(
			"controller",
			c,
			"predicate",
			"ClusterUnpausedAndInfrastructureReady",
			"predicate",
			"ClusterCreateNotPaused",
			"eventType",
			"create",
			"namespace",
			ec.ClusterNamespace,
			"cluster",
			ec.ClusterName,
			"msg",
			"Cluster is not paused, allowing further processing",
		).WithLevel(4).WithLogFunc("Info")
	}

	clusterEntries := make([]gomega.LogMatcher, len(ec.ClusterControllers))
	for i, c := range ec.ClusterControllers {
		c := c
		clusterEntries[i] = gomega.LogContains(
			"controller",
			c,
			"predicate",
			"ClusterUnpaused",
			"predicate",
			"ClusterCreateNotPaused",
			"eventType",
			"create",
			"namespace",
			ec.ClusterNamespace,
			"cluster",
			ec.ClusterName,
			"msg",
			"Cluster is not paused, allowing further processing",
		).WithLevel(4).WithLogFunc("Info")
	}
	return append(clusterEntries, infraEntries...)
}

func GenerateUpdatePausedClusterLogEntries(ec EntryCriteria) []gomega.LogMatcher {
	infraEntries := make([]gomega.LogMatcher, len(ec.InfraControllers))
	for i, c := range ec.InfraControllers {
		c := c
		infraEntries[i] = gomega.LogContains(
			"controller",
			c,
			"predicate",
			"ClusterUnpausedAndInfrastructureReady",
			"predicate",
			"ClusterUpdateUnpaused",
			"eventType",
			"update",
			"namespace",
			ec.ClusterNamespace,
			"cluster",
			ec.ClusterName,
			"msg",
			"Cluster was not unpaused, blocking further processing",
		).WithLevel(4).WithLogFunc("Info")
	}

	clusterEntries := make([]gomega.LogMatcher, len(ec.ClusterControllers))
	for i, c := range ec.ClusterControllers {
		c := c
		clusterEntries[i] = gomega.LogContains(
			"controller",
			c,
			"predicate",
			"ClusterUnpaused",
			"predicate",
			"ClusterUpdateUnpaused",
			"eventType",
			"update",
			"namespace",
			ec.ClusterNamespace,
			"cluster",
			ec.ClusterName,
			"msg",
			"Cluster was not unpaused, blocking further processing",
		).WithLevel(4).WithLogFunc("Info")
	}
	return append(clusterEntries, infraEntries...)
}

func GenerateUpdateUnpausedClusterLogEntries(ec EntryCriteria) []gomega.LogMatcher {
	infraEntries := make([]gomega.LogMatcher, len(ec.InfraControllers))
	for i, c := range ec.InfraControllers {
		c := c
		infraEntries[i] = gomega.LogContains(
			"controller",
			c,
			"predicate",
			"ClusterUnpausedAndInfrastructureReady",
			"predicate",
			"ClusterUpdateUnpaused",
			"eventType",
			"update",
			"namespace",
			ec.ClusterNamespace,
			"cluster",
			ec.ClusterName,
			"msg",
			"Cluster was unpaused, allowing further processing",
		).WithLevel(4).WithLogFunc("Info")
	}

	clusterEntries := make([]gomega.LogMatcher, len(ec.ClusterControllers))
	for i, c := range ec.ClusterControllers {
		c := c
		clusterEntries[i] = gomega.LogContains(
			"controller",
			c,
			"predicate",
			"ClusterUnpaused",
			"predicate",
			"ClusterUpdateUnpaused",
			"eventType",
			"update",
			"namespace",
			ec.ClusterNamespace,
			"cluster",
			ec.ClusterName,
			"msg",
			"Cluster was unpaused, allowing further processing",
		).WithLevel(4).WithLogFunc("Info")
	}
	return append(clusterEntries, infraEntries...)
}
