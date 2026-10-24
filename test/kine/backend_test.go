/*
Copyright 2026 The Kubernetes Authors.

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

package kine_test

import (
	"bytes"
	"errors"
	"io"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDatastoreBackendSelection(t *testing.T) {
	for _, tc := range []struct {
		name, profile, backend, want, errorText string
	}{
		{"conformance-default", "", "", "kine-postgres\nv0.17.2\nkine\n", ""},
		{"performance-default", "performance", "", "kubeadm-etcd\nkubeadm-managed\netcd\n", ""},
		{"performance-kine", "performance", "kine-postgres", "kine-postgres\nv0.17.2\nkine\n", ""},
		{"explicit-local", "performance", "kubeadm-etcd", "kubeadm-etcd\nkubeadm-managed\netcd\n", ""},
		{"local-conformance", "conformance", "kubeadm-etcd", "kubeadm-etcd\nkubeadm-managed\netcd\n", ""},
		{"invalid-backend", "performance", "other", "", "KINE_BACKEND must be"},
		{"retired-standalone-etcd", "performance", "etcd", "", "standalone etcd has been removed"},
		{"invalid-profile", "other", "", "", "Unknown KINE_PROFILE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), "bash", "-c", `
set -euo pipefail
source ./scripts/kine/backend.sh
unset KINE_KUBEADM_ETCD_IMAGE
capz::kine::backend_defaults
printf '%s\n' "$KINE_BACKEND" "$KINE_DATASTORE_VERSION" "$KINE_METRICS_JOB"
`)
			cmd.Dir = repoRoot(t)
			cmd.Env = append(cmd.Environ(), "KINE_PROFILE="+tc.profile, "KINE_BACKEND="+tc.backend)
			output, err := cmd.CombinedOutput()
			if tc.errorText != "" {
				if err == nil || !bytes.Contains(output, []byte(tc.errorText)) {
					t.Fatalf("expected explicit backend rejection: %v\n%s", err, output)
				}
			} else if err != nil || string(output) != tc.want {
				t.Fatalf("incorrect backend selection: %v\n%s", err, output)
			}
		})
	}
}

func TestLocalEtcdRejectsExternalResources(t *testing.T) {
	for _, operation := range []string{"certificates", "provision"} {
		t.Run(operation, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), "bash", "-c", `
source ./scripts/ci-kine-conformance.sh
KINE_PROFILE=performance KINE_BACKEND=kubeadm-etcd
az() { echo "unexpected Azure call"; return 99; }
case "$KINE_TEST_OPERATION" in
  certificates) capz::kine::certificates ;;
  provision) capz::kine::provision ;;
esac
`)
			cmd.Dir = repoRoot(t)
			cmd.Env = append(cmd.Environ(), "KINE_TEST_OPERATION="+operation)
			output, err := cmd.CombinedOutput()
			if err == nil || len(output) == 0 || bytes.Contains(output, []byte("unexpected Azure call")) {
				t.Fatalf("local mode must reject external setup before any side effect: %v\n%s", err, output)
			}
		})
	}
}

func TestKubeadmLocalFlavor(t *testing.T) {
	var localMachine map[string]any
	for _, flavor := range []string{"kubeadm-etcd", "kubeadm-etcd-performance", "kine-postgres"} {
		t.Run(flavor, func(t *testing.T) {
			// #nosec G204 -- Flavor names come only from the fixed cases above.
			cmd := exec.CommandContext(t.Context(), "./hack/tools/bin/kustomize", "build", "--load-restrictor", "LoadRestrictionsNone",
				"./templates/test/ci/prow-"+flavor)
			cmd.Dir = repoRoot(t)
			rendered, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("render comparison flavor: %v\n%s", err, rendered)
			}
			if !bytes.Equal(rendered, readFile(t, filepath.Join(repoRoot(t), "templates/test/ci/cluster-template-prow-"+flavor+".yaml"))) {
				t.Fatal("generated flavor is stale")
			}
			local := strings.HasPrefix(flavor, "kubeadm-etcd")
			if local && (bytes.Contains(rendered, []byte("${KINE_RESOURCE_GROUP}")) || bytes.Contains(rendered, []byte("${KINE_CA_CERT_B64}"))) {
				t.Fatal("local mode must not reference an external datastore network or credentials")
			}
			decoder := yaml.NewDecoder(bytes.NewReader(rendered))
			foundMachine, foundControlPlane := false, false
			for {
				var obj struct {
					Kind     string                `yaml:"kind"`
					Metadata struct{ Name string } `yaml:"metadata"`
					Spec     struct {
						Template struct{ Spec map[string]any } `yaml:"template"`
						Kubeadm  struct {
							Cluster struct {
								APIServer struct {
									Args map[string]string `yaml:"extraArgs"`
								} `yaml:"apiServer"`
								Etcd struct {
									Local *struct {
										DataDir string            `yaml:"dataDir"`
										Args    map[string]string `yaml:"extraArgs"`
									} `yaml:"local"`
									External any `yaml:"external"`
								} `yaml:"etcd"`
							} `yaml:"clusterConfiguration"`
						} `yaml:"kubeadmConfigSpec"`
					} `yaml:"spec"`
				}
				if err := decoder.Decode(&obj); errors.Is(err, io.EOF) {
					break
				} else if err != nil {
					t.Fatal(err)
				}
				if obj.Kind == "AzureMachineTemplate" && obj.Metadata.Name == "${CLUSTER_NAME}-control-plane" {
					foundMachine = true
					if localMachine == nil {
						localMachine = obj.Spec.Template.Spec
					} else if !reflect.DeepEqual(localMachine, obj.Spec.Template.Spec) {
						t.Fatal("control-plane hardware must be identical in both datastore modes")
					}
				}
				if obj.Kind == "KubeadmControlPlane" {
					foundControlPlane = true
					cluster := obj.Spec.Kubeadm.Cluster
					if cluster.APIServer.Args["feature-gates"] != "ConcurrentWatchObjectDecode=true,ListFromCacheSnapshot=false" {
						t.Fatal("both modes must retain the same API-server feature gates")
					}
					if local && (cluster.Etcd.External != nil || cluster.Etcd.Local == nil ||
						cluster.Etcd.Local.DataDir != "/var/lib/etcddisk/etcd" ||
						cluster.Etcd.Local.Args["quota-backend-bytes"] != "34359738368" ||
						cluster.Etcd.Local.Args["listen-metrics-urls"] != "http://0.0.0.0:2381") {
						t.Fatal("local mode must use kubeadm etcd on the dedicated disk with private metrics")
					}
					if !local && (cluster.Etcd.Local != nil || cluster.Etcd.External == nil) {
						t.Fatal("Kine mode must not also run a local etcd member")
					}
				}
			}
			if !foundMachine || !foundControlPlane {
				t.Fatal("missing comparison control-plane resources")
			}
		})
	}
}

func TestEtcdMeasurements(t *testing.T) {
	for _, action := range []string{"start", "gather"} {
		t.Run(action, func(t *testing.T) {
			common := map[string]clusterLoaderMeasurement{}
			for _, m := range clusterLoaderMeasurements(t, action) {
				if !strings.HasPrefix(m.Identifier, "Kine") {
					common[m.Identifier] = m
				}
			}
			found := map[string]bool{}
			for _, m := range clusterLoaderMeasurementsForBackend(t, action, "kubeadm-etcd") {
				if strings.HasPrefix(m.Identifier, "Kine") {
					t.Fatalf("local etcd must not require Kine/SQL metrics: %s", m.Identifier)
				}
				if !strings.HasPrefix(m.Identifier, "Etcd") {
					if !reflect.DeepEqual(common[m.Identifier], m) {
						t.Fatalf("common workload measurement changed between backends: %s", m.Identifier)
					}
					delete(common, m.Identifier)
					continue
				}
				found[m.Identifier] = true
				if !m.Params.EnableViolations || len(m.Params.Queries) == 0 {
					t.Fatalf("etcd diagnostics must report missing data: %s", m.Identifier)
				}
				for _, query := range m.Params.Queries {
					if !query.RequireSamples || !strings.Contains(query.Query, `job="etcd"`) ||
						strings.Contains(query.Query, "or vector(0)") {
						t.Fatalf("invalid local etcd query: %s", query.Query)
					}
				}
			}
			for _, name := range []string{"EtcdScrapeAvailability", "EtcdCPU", "EtcdMemory", "EtcdLeaderAvailability", "EtcdDatabaseSize", "EtcdWALFsyncP99", "EtcdBackendCommitP99"} {
				if !found[name] {
					t.Errorf("missing etcd measurement: %s", name)
				}
			}
			if len(common) != 0 {
				t.Fatalf("local etcd omitted common measurements: %v", common)
			}
		})
	}
}
