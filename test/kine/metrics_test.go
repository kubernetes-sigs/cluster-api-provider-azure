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
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"text/template"

	corev1 "k8s.io/api/core/v1"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
)

func TestKineMetricsMonitor(t *testing.T) {
	for _, job := range []string{"kine", "etcd"} {
		t.Run(job, func(t *testing.T) {
			testDatastoreMetricsMonitor(t, job)
		})
	}
}

func testDatastoreMetricsMonitor(t *testing.T, job string) {
	t.Helper()
	data := readFile(t, filepath.Join(repoRoot(t), "scripts", "kine", "clusterloader", "prometheus", "datastore.yaml"))
	tmpl, err := template.New("metrics").Option("missingkey=error").Parse(string(data))
	if err != nil {
		t.Fatal(err)
	}
	ca := []byte("public CA certificate")
	var rendered bytes.Buffer
	parameters := map[string]string{"CL2_DATASTORE_JOB": job}
	if job == "kine" {
		parameters["CL2_KINE_CA_CERT_B64"] = base64.StdEncoding.EncodeToString(ca)
	}
	if err := tmpl.Execute(&rendered, parameters); err != nil {
		t.Fatal(err)
	}
	decoder := yamlutil.NewYAMLOrJSONDecoder(&rendered, 4096)
	found := map[string]bool{}
	for {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		var object struct {
			Kind     string `json:"kind"`
			Metadata struct {
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"metadata"`
		}
		if err := json.Unmarshal(raw, &object); err != nil {
			t.Fatal(err)
		}
		namespace := "monitoring"
		if job == "etcd" && object.Kind == "Service" {
			namespace = "kube-system"
		}
		if found[object.Kind] || object.Metadata.Namespace != namespace {
			t.Fatalf("duplicate or incorrectly scoped metrics resource: %+v", object)
		}
		found[object.Kind] = true
		switch object.Kind {
		case "Secret":
			if job != "kine" {
				t.Fatal("local etcd monitoring must not distribute datastore credentials")
			}
			var secret corev1.Secret
			if err := json.Unmarshal(raw, &secret); err != nil {
				t.Fatal(err)
			}
			if secret.Name != job+"-metrics-ca" || len(secret.Data) != 1 || !bytes.Equal(secret.Data["ca.crt"], ca) {
				t.Fatal("metrics scraping must distribute only the public CA certificate")
			}
		case "Service":
			var service corev1.Service
			if err := json.Unmarshal(raw, &service); err != nil {
				t.Fatal(err)
			}
			if len(service.Spec.Ports) != 1 || len(service.Spec.ExternalIPs) != 0 || service.Spec.Type == corev1.ServiceTypeLoadBalancer {
				t.Fatal("metrics service must remain private with one diagnostic port")
			}
			if job == "etcd" {
				if service.Name != "capz-local-etcd-metrics" || service.Labels["datastore-job"] != "etcd" ||
					len(service.Spec.Selector) != 2 || service.Spec.Selector["component"] != "etcd" ||
					service.Spec.Selector["tier"] != "control-plane" || service.Spec.Ports[0].Port != 2381 ||
					service.Spec.Ports[0].Name != "http-metrics" {
					t.Fatal("local monitoring must select kubeadm's static Pods on their metrics-only port")
				}
			} else if service.Name != "kine-metrics" || service.Labels["k8s-app"] != "kine" ||
				len(service.Spec.Selector) != 0 || service.Spec.Ports[0].Name != "https-metrics" || service.Spec.Ports[0].Port != 8080 {
				t.Fatal("Kine monitoring must select its single private TLS endpoint")
			}
		case "Endpoints":
			if job != "kine" {
				t.Fatal("local etcd endpoints must follow the Pod selector, not a hardcoded VM")
			}
			var endpoints struct {
				Subsets []struct {
					Addresses []struct {
						IP string `json:"ip"`
					} `json:"addresses"`
					Ports []struct {
						Name string `json:"name"`
						Port int32  `json:"port"`
					} `json:"ports"`
				} `json:"subsets"`
			}
			if err := json.Unmarshal(raw, &endpoints); err != nil {
				t.Fatal(err)
			}
			if object.Metadata.Name != job+"-metrics" || len(endpoints.Subsets) != 1 {
				t.Fatal("expected one private datastore endpoint")
			}
			subset := endpoints.Subsets[0]
			if len(subset.Addresses) != 1 || subset.Addresses[0].IP != "10.250.0.4" ||
				len(subset.Ports) != 1 || subset.Ports[0].Name != "https-metrics" || subset.Ports[0].Port != 8080 {
				t.Fatal("metrics must target the datastore's private TLS listener")
			}
		case "ServiceMonitor":
			var monitor struct {
				Spec struct {
					JobLabel  string `json:"jobLabel"`
					Endpoints []struct {
						Port      string `json:"port"`
						Scheme    string `json:"scheme"`
						Path      string `json:"path"`
						TLSConfig struct {
							ServerName         string `json:"serverName"`
							InsecureSkipVerify bool   `json:"insecureSkipVerify"`
							CA                 struct {
								Secret struct {
									Name string `json:"name"`
									Key  string `json:"key"`
								} `json:"secret"`
							} `json:"ca"`
						} `json:"tlsConfig"`
					} `json:"endpoints"`
					Selector struct {
						MatchLabels map[string]string `json:"matchLabels"`
					} `json:"selector"`
					NamespaceSelector struct {
						MatchNames []string `json:"matchNames"`
					} `json:"namespaceSelector"`
				} `json:"spec"`
			}
			if err := json.Unmarshal(raw, &monitor); err != nil {
				t.Fatal(err)
			}
			if len(monitor.Spec.Endpoints) != 1 {
				t.Fatal("monitor must have exactly one diagnostic endpoint")
			}
			endpoint := monitor.Spec.Endpoints[0]
			if job == "etcd" {
				if monitor.Spec.JobLabel != "datastore-job" || monitor.Spec.Selector.MatchLabels["k8s-app"] != "capz-local-etcd" ||
					strings.Join(monitor.Spec.NamespaceSelector.MatchNames, ",") != "kube-system" ||
					endpoint.Port != "http-metrics" || endpoint.Scheme != "http" || endpoint.Path != "/metrics" {
					t.Fatal("local etcd monitoring must use the private kube-system metrics Service")
				}
			} else if monitor.Spec.JobLabel != "k8s-app" || monitor.Spec.Selector.MatchLabels["k8s-app"] != "kine" ||
				endpoint.Port != "https-metrics" || endpoint.Scheme != "https" || endpoint.Path != "/metrics" ||
				endpoint.TLSConfig.ServerName != "10.250.0.4" || endpoint.TLSConfig.InsecureSkipVerify ||
				endpoint.TLSConfig.CA.Secret.Name != job+"-metrics-ca" || endpoint.TLSConfig.CA.Secret.Key != "ca.crt" {
				t.Fatal("datastore scraping must verify both the certificate authority and server identity")
			}
		default:
			t.Fatalf("unexpected metrics resource: %s", object.Kind)
		}
	}
	expected := 4
	if job == "etcd" {
		expected = 2
	}
	if len(found) != expected {
		t.Fatalf("missing metrics resources: %v", found)
	}
}

func TestKineMetricsRequireData(t *testing.T) {
	for _, action := range []string{"start", "gather"} {
		t.Run(action, func(t *testing.T) {
			foundAvailability, foundStatus := false, false
			for _, m := range clusterLoaderMeasurements(t, action) {
				if !strings.HasPrefix(m.Identifier, "Kine") && m.Identifier != "PodPatchResponses" {
					continue
				}
				p := m.Params
				if p.Action != action || !p.EnableViolations || len(p.Queries) == 0 {
					t.Fatalf("diagnostic measurements must expose missing-data failures: %+v", m)
				}
				for _, q := range p.Queries {
					if !q.RequireSamples || strings.Contains(q.Query, "or vector(0)") {
						t.Fatalf("diagnostics must not substitute zeros for a missing scrape: %+v", q)
					}
					if m.Identifier == "PodPatchResponses" {
						foundStatus = true
						if strings.Join(p.Dimensions, ",") != "code" || !strings.Contains(q.Query, `subresource=""`) ||
							!strings.Contains(q.Query, "sum by (code)") {
							t.Fatal("PATCH diagnostics must distinguish HTTP response codes and exclude status updates")
						}
					} else if !strings.Contains(q.Query, `job="kine"`) {
						t.Fatalf("Kine diagnostics must not mix other processes into the result: %s", q.Query)
					}
					if m.Identifier == "KineScrapeAvailability" {
						foundAvailability = true
						if q.Threshold == nil || *q.Threshold != 1 || !q.LowerBound || len(q.Aggregations) != 0 ||
							!strings.Contains(q.Query, `min_over_time(up{job="kine"}[%v])`) {
							t.Fatal("scrape availability must enforce a full-window lower bound on an instant query")
						}
					}
					if m.Identifier == "KineCompactions" || m.Identifier == "KineInsertConflicts" {
						if !strings.Contains(q.Query, `or (0 * max(up{job="kine"} == 1))`) {
							t.Fatal("unused event counters may only report zero with a healthy scrape")
						}
					}
				}
			}
			if !foundAvailability || !foundStatus {
				t.Fatal("missing scrape availability gate or HTTP status breakdown")
			}
		})
	}
}

func TestKineProfilingStartMarker(t *testing.T) {
	for _, action := range []string{"start", "gather"} {
		measurements := clusterLoaderMeasurements(t, action)
		count := 0
		for i, m := range measurements {
			if m.Identifier != "ProfilingStart" {
				continue
			}
			count++
			if action != "start" || i != 0 || m.Method != "Exec" || m.Params.Timeout != "10s" ||
				strings.Join(m.Params.Command, " ") != "touch ${CL2_REPORT_DIR}/profiling-start" {
				t.Fatal("profiling must be anchored once, before measurements start, not during setup or gather")
			}
		}
		if (count == 1) != (action == "start") {
			t.Fatalf("unexpected profiling marker count for %s: %d", action, count)
		}
	}
}

func TestClusterLoaderDiagnosticRunner(t *testing.T) {
	root := repoRoot(t)
	script := readFile(t, filepath.Join(root, "scripts", "kine", "run-clusterloader.sh"))
	revision := regexp.MustCompile(`(?m)^revision=([0-9a-f]+)$`).FindSubmatch(script)
	if len(revision) != 2 {
		t.Fatal("missing pinned CL2 revision")
	}
	for _, tc := range []struct {
		name, mode, ca, exitCode, backend string
	}{
		{"live", "", "", "0", ""},
		{"kine-postgres", "", "public-ca", "0", "kine-postgres"},
		{"empty-argument", "empty", "public-ca", "0", ""},
		{"load-failure", "", "public-ca", "17", ""},
		{"missing-ca", "", "", "0", "kine-postgres"},
		{"dry-run", "--dry-run", "public-ca", "0", ""},
		{"prepare-only", "--prepare-only", "", "0", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			report := filepath.Join(dir, "report")
			cache := filepath.Join(dir, "_artifacts", "perf-tests", string(revision[1]))
			for _, path := range []string{report, filepath.Join(cache, "source", "clusterloader2", "testing", "write-throughput", "modules", "writer")} {
				if err := os.MkdirAll(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			files := map[string][]byte{
				filepath.Join("scripts", "kine", "run-clusterloader.sh"):                             script,
				filepath.Join("scripts", "kine", "backend.sh"):                                       readFile(t, filepath.Join(root, "scripts", "kine", "backend.sh")),
				filepath.Join("scripts", "kine", "clusterloader", "measurements.yaml"):               []byte("__DATASTORE_PREFIX__ runtime job=__DATASTORE_JOB__\n"),
				filepath.Join("scripts", "kine", "clusterloader", "measurements-kubeadm-etcd.yaml"):  []byte("etcd measurements\n"),
				filepath.Join("scripts", "kine", "clusterloader", "measurements-kine-postgres.yaml"): []byte("kine measurements\n"),
				filepath.Join("scripts", "kine", "clusterloader", "writer.yaml"):                     []byte("writer"),
				filepath.Join("_artifacts", "perf-tests", string(revision[1]), "source.tar.gz"):      []byte("mock archive"),
			}
			for name, content := range files {
				path := filepath.Join(dir, name)
				if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, content, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			binary := `#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$@" > "$KINE_TEST_DIR/args"
printf '%s' "${CL2_KINE_CA_CERT_B64:-}" > "$KINE_TEST_DIR/ca"
printf '%s' "$CL2_DATASTORE_JOB" > "$KINE_TEST_DIR/backend-job"
exit "$KINE_TEST_EXIT"
`
			binaryPath := filepath.Join(cache, "clusterloader2")
			if err := os.WriteFile(binaryPath, []byte(binary), 0o600); err != nil {
				t.Fatal(err)
			}
			snapshot := filepath.Join(report, "prometheus_snapshot.tar.gz")
			if err := os.WriteFile(snapshot, []byte("stale snapshot"), 0o600); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(report, "profiling-start")
			if err := os.WriteFile(marker, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			kubeconfig := filepath.Join(dir, "kubeconfig")
			if err := os.WriteFile(kubeconfig, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(t.Context(), "bash", "-c", `
chmod 0700 "$KINE_TEST_BINARY"
shasum() { cat >/dev/null; }
tar() { :; }
export -f shasum tar
args=()
case "$KINE_TEST_MODE" in
  empty) args+=("") ;;
  "") ;;
  *) args+=("$KINE_TEST_MODE") ;;
esac
exec bash ./scripts/kine/run-clusterloader.sh "${args[@]}"
`)
			cmd.Dir = dir
			cmd.Env = append(cmd.Environ(), "KINE_TEST_DIR="+dir, "KINE_TEST_MODE="+tc.mode,
				"KINE_TEST_EXIT="+tc.exitCode, "KINE_TEST_BINARY="+binaryPath,
				"KINE_CA_CERT_B64="+tc.ca, "KUBECONFIG="+kubeconfig, "CL2_REPORT_DIR="+report,
				"KINE_PROFILE=performance", "KINE_BACKEND="+tc.backend, "KINE_KUBEADM_ETCD_IMAGE=")
			output, err := cmd.CombinedOutput()
			if tc.name == "missing-ca" {
				if err == nil || !bytes.Contains(output, []byte("KINE_CA_CERT_B64")) {
					t.Fatalf("missing CA must fail explicitly: %v\n%s", err, output)
				}
			} else if tc.exitCode != "0" {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) || exitErr.ExitCode() != 17 {
					t.Fatalf("must preserve CL2's failing exit code: %v\n%s", err, output)
				}
			} else if err != nil {
				t.Fatalf("run CL2 wrapper: %v\n%s", err, output)
			}
			if tc.name != "missing-ca" && tc.mode != "--prepare-only" {
				args := string(readFile(t, filepath.Join(dir, "args")))
				if !strings.Contains(args, "--experimental-prometheus-snapshot-to-report-dir=true") ||
					!strings.Contains(args, "--prometheus-additional-monitors-path="+filepath.Join(dir, "scripts", "kine", "clusterloader", "prometheus")) {
					t.Fatal("runner must install metrics monitors and archive Prometheus before teardown")
				}
				wantCA := ""
				if tc.backend == "kine-postgres" {
					wantCA = tc.ca
				}
				if string(readFile(t, filepath.Join(dir, "ca"))) != wantCA {
					t.Fatal("runner must forward the public CA into CL2 template variables")
				}
				job, prefix, backend, version := "etcd", "Etcd", "kubeadm-etcd", "kubeadm-managed"
				if tc.backend == "kine-postgres" {
					job, prefix, backend, version = "kine", "Kine", "kine-postgres", "v0.17.2"
				}
				if string(readFile(t, filepath.Join(dir, "backend-job"))) != job ||
					string(readFile(t, filepath.Join(cache, "workload", "modules", "measurements.yaml"))) != prefix+" runtime job="+job+"\n"+job+" measurements\n" {
					t.Fatal("runner must select only the chosen backend's measurement set")
				}
				settings := string(readFile(t, filepath.Join(report, "settings.txt")))
				if !strings.Contains(settings, "datastore-backend="+backend+"\ndatastore-version="+version+"\n") {
					t.Fatal("runner must archive the actual backend and pinned version")
				}
			}
			_, statErr := os.Stat(snapshot)
			live := tc.mode == "" || tc.mode == "empty"
			if live && !errors.Is(statErr, os.ErrNotExist) {
				t.Fatal("live runs must remove stale snapshots before execution")
			}
			if !live && statErr != nil {
				t.Fatal("prepare-only and dry-run must not delete earlier snapshots")
			}
			_, markerErr := os.Stat(marker)
			if live && !errors.Is(markerErr, os.ErrNotExist) {
				t.Fatal("live runs must remove the old profiling clock")
			}
			if !live && markerErr != nil {
				t.Fatal("prepare-only and dry-run must not change the profiling clock")
			}
		})
	}
}
