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

package gomega

import (
	"testing"

	"github.com/onsi/gomega"

	"sigs.k8s.io/cluster-api-provider-azure/internal/test/record"
)

var (
	defaultLogEntry = record.LogEntry{
		Values: []interface{}{
			"foo",
			"bin",
			"bax",
		},
		LogFunc: "Info",
		Level:   2,
	}
)

func TestLogContains(t *testing.T) {
	cases := []struct {
		Name        string
		LogEntry    record.LogEntry
		Matcher     LogMatcher
		ShouldMatch bool
	}{
		{
			Name:     "MatchesCompletely",
			LogEntry: defaultLogEntry,
			Matcher: LogContains(
				"foo",
				"bin",
				"bax").WithLevel(2).WithLogFunc("Info"),
			ShouldMatch: true,
		},
		{
			Name:     "MatchesWithoutSpecifyingLevel",
			LogEntry: defaultLogEntry,
			Matcher: LogContains(
				"foo",
				"bin",
				"bax").WithLogFunc("Info"),
			ShouldMatch: true,
		},
		{
			Name:     "MatchesWithoutSpecifyingLogFunc",
			LogEntry: defaultLogEntry,
			Matcher: LogContains(
				"foo",
				"bin",
				"bax"),
			ShouldMatch: true,
		},
		{
			Name:        "MatchesWithoutSpecifyingAllValues",
			LogEntry:    defaultLogEntry,
			Matcher:     LogContains("foo", "bax"),
			ShouldMatch: true,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			success, err := c.Matcher.Match(c.LogEntry)
			g.Expect(err).NotTo(gomega.HaveOccurred())
			g.Expect(success).To(gomega.Equal(c.ShouldMatch))
		})
	}
}

func TestLogContainsEntries(t *testing.T) {
	entries := []record.LogEntry{
		defaultLogEntry,
		{
			Values: []interface{}{
				"controller",
				"AzureCluster",
				"predicate",
				"ClusterUnpaused",
				"predicate",
				"ClusterCreateNotPaused",
				"eventType",
				"create",
				"namespace",
				"default",
				"cluster",
				"foo-52824hhgdv",
				"eventType",
				"create",
				"namespace",
				"default",
				"cluster",
				"cluster-xxnmwzz2wz",
				"eventType",
				"create",
				"namespace",
				"default",
				"cluster",
				"foo-zljvddw5c2",
				"msg",
				"Cluster is not paused, allowing further processing",
			},
			LogFunc: "Error",
			Level:   6,
		},
	}

	g := gomega.NewWithT(t)
	g.Expect(entries).To(gomega.ContainElements([]LogMatcher{
		LogContains("bin"),
		LogContains("controller",
			"AzureCluster",
			"predicate",
			"ClusterUnpaused",
			"predicate",
			"ClusterCreateNotPaused",
			"eventType",
			"create",
			"namespace",
			"default",
			"cluster",
			"foo-zljvddw5c2",
			"msg",
			"Cluster is not paused, allowing further processing"),
	}))
}
