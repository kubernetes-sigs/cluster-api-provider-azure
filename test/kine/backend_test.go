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
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDatastoreBackendSelection(t *testing.T) {
	for _, tc := range []struct {
		name, profile, backend, want, errorText string
	}{
		{"conformance-default", "", "", "kine-postgres\nv0.17.2\nkine\n", ""},
		{"performance-default", "performance", "", "etcd\nv3.6.6\netcd\n", ""},
		{"performance-kine", "performance", "kine-postgres", "kine-postgres\nv0.17.2\nkine\n", ""},
		{"explicit-etcd", "performance", "etcd", "etcd\nv3.6.6\netcd\n", ""},
		{"invalid-backend", "performance", "other", "", "KINE_BACKEND must be"},
		{"etcd-conformance", "conformance", "etcd", "", "supported only with KINE_PROFILE=performance"},
		{"invalid-profile", "other", "", "", "Unknown KINE_PROFILE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), "bash", "-c", `
set -euo pipefail
source ./scripts/kine/backend.sh
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

func TestEtcdBaselineCloudInit(t *testing.T) {
	for _, profiling := range []string{"false", "true"} {
		t.Run(profiling, func(t *testing.T) {
			dir := t.TempDir()
			cmd := exec.CommandContext(t.Context(), "bash", "-c", `
source ./scripts/ci-kine-conformance.sh
repo_root=$PWD
state_dir=$KINE_TEST_DIR
KINE_PROFILE=performance
unset KINE_BACKEND
KINE_POSTGRES_MAX_CONNECTIONS='unused|invalid'
KINE_DATASTORE_MAX_OPEN_CONNECTIONS=invalid
umask 077
capz::kine::certificates
`)
			cmd.Dir = repoRoot(t)
			cmd.Env = append(cmd.Environ(), "KINE_TEST_DIR="+dir, "KINE_COLLECT_PROFILES="+profiling)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("render default etcd control: %v\n%s", err, output)
			}
			rendered := readFile(t, filepath.Join(dir, "cloud-init.yaml"))
			if placeholders := regexp.MustCompile(`__[A-Z_]+__`).FindAll(rendered, -1); len(placeholders) != 0 {
				t.Fatalf("unexpanded etcd configuration placeholders: %q", placeholders)
			}
			var config struct {
				Packages []string `yaml:"packages"`
				Files    []struct {
					Path, Content, Encoding, Permissions string
				} `yaml:"write_files"`
			}
			if err := yaml.Unmarshal(rendered, &config); err != nil {
				t.Fatal(err)
			}
			for _, pkg := range config.Packages {
				if strings.Contains(pkg, "postgres") || pkg == "etcd-client" {
					t.Fatal("the native etcd control must not install PostgreSQL or an unpinned etcd client")
				}
			}
			files := map[string]string{}
			for _, file := range config.Files {
				content := []byte(file.Content)
				if file.Encoding == "b64" {
					var err error
					content, err = base64.StdEncoding.DecodeString(file.Content)
					if err != nil {
						t.Fatal(err)
					}
				}
				if strings.HasSuffix(file.Path, ".key") && file.Permissions != "0600" {
					t.Fatalf("unsafe private-key permissions: %s", file.Path)
				}
				if bytes.HasPrefix(content, []byte("#!/usr/bin/env bash")) {
					check := exec.CommandContext(t.Context(), "bash", "-n")
					check.Stdin = bytes.NewReader(content)
					if output, err := check.CombinedOutput(); err != nil {
						t.Fatalf("invalid guest script %s: %v\n%s", file.Path, err, output)
					}
				}
				files[file.Path] = string(content)
			}
			if _, exists := files["/etc/systemd/system/kine.service"]; exists {
				t.Fatal("etcd must replace, not accompany, Kine/PostgreSQL")
			}
			for path, source := range map[string]string{
				"/opt/mount-datastore-disk.sh":   "mount-datastore-disk.sh",
				"/usr/local/bin/check-datastore": "check-datastore.sh",
			} {
				if files[path] != string(readFile(t, filepath.Join(repoRoot(t), "scripts", "kine", source))) {
					t.Fatalf("both backends must use the shared disk/readiness implementation: %s", path)
				}
			}
			service := files["/etc/systemd/system/etcd.service"]
			for _, required := range []string{
				"User=etcd", "RequiresMountsFor=/var/lib/kine-postgresql",
				"ExecStartPre=/usr/bin/mountpoint -q /var/lib/kine-postgresql",
				"--data-dir=/var/lib/kine-postgresql/etcd",
				"--listen-client-urls=https://0.0.0.0:2379", "--advertise-client-urls=https://10.250.0.4:2379",
				"--client-cert-auth=true", "--trusted-ca-file=/etc/kine/ca.crt",
				"--listen-peer-urls=http://127.0.0.1:2380", "--initial-cluster=capz-etcd=http://127.0.0.1:2380",
				"--listen-metrics-urls=http://127.0.0.1:2381", "--enable-grpc-gateway=false",
				"--unsafe-no-fsync=false", "--auto-compaction-retention=0",
				"--quota-backend-bytes=34359738368", "--enable-pprof=" + profiling,
			} {
				if !strings.Contains(service, required) {
					t.Errorf("missing matched etcd invariant: %s", required)
				}
			}
			proxy := files["/etc/kine/etcd-metrics-nginx.conf"]
			for _, required := range []string{
				"listen 10.250.0.4:8080 ssl;", "proxy_ssl_verify on;", "proxy_ssl_name localhost;",
				"proxy_ssl_trusted_certificate /etc/kine/ca.crt;",
				"proxy_ssl_certificate /etc/kine/client.crt;", "proxy_ssl_certificate_key /etc/kine/client.key;",
				"location = /metrics", "location = /debug/pprof/profile", "location = /debug/pprof/heap",
				"location = /debug/pprof/goroutine", "location / { return 404; }",
			} {
				if !strings.Contains(proxy, required) {
					t.Errorf("missing private metrics/profiling restriction: %s", required)
				}
			}
			if strings.Count(proxy, "location ") != 5 || strings.Count(proxy, "limit_except GET { deny all; }") != 4 {
				t.Fatal("worker access must expose only the four allowlisted diagnostic paths")
			}
			installer := files["/opt/install-etcd.sh"]
			for _, required := range []string{
				"etcd-v3.6.6-linux-amd64.tar.gz", "887afaa4a99f22d802ccdfbe65730a5e79aa5c9ce2c8799c67e9d804c50ecedb",
				"sha256sum --check", "source /opt/mount-datastore-disk.sh", "systemctl disable --now nginx",
				"/usr/local/bin/check-datastore etcd", "/usr/local/bin/check-etcd-metrics",
			} {
				if !strings.Contains(installer, required) {
					t.Errorf("missing native etcd installation invariant: %s", required)
				}
			}
			if strings.Index(installer, "systemctl enable --now etcd-metrics") >= strings.Index(installer, "/usr/local/bin/check-etcd-metrics") {
				t.Fatal("metrics readiness must be checked after starting the proxy")
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
			for _, m := range clusterLoaderMeasurementsForBackend(t, action, "etcd") {
				if strings.HasPrefix(m.Identifier, "Kine") {
					t.Fatalf("native etcd must not require Kine/SQL metrics: %s", m.Identifier)
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
						t.Fatalf("invalid native etcd query: %s", query.Query)
					}
				}
			}
			for _, name := range []string{"EtcdScrapeAvailability", "EtcdCPU", "EtcdMemory", "EtcdLeaderAvailability", "EtcdDatabaseSize", "EtcdWALFsyncP99", "EtcdBackendCommitP99"} {
				if !found[name] {
					t.Errorf("missing etcd measurement: %s", name)
				}
			}
			if len(common) != 0 {
				t.Fatalf("etcd omitted common measurements: %v", common)
			}
		})
	}
}

func TestEtcdCleanupStatus(t *testing.T) {
	for _, healthy := range []bool{true, false} {
		t.Run(map[bool]string{true: "healthy", false: "guest-failure"}[healthy], func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "private"), 0o700); err != nil {
				t.Fatal(err)
			}

			cmd := exec.CommandContext(t.Context(), "bash", "-c", `
source ./scripts/ci-kine-conformance.sh
KINE_PROFILE=performance KINE_BACKEND=etcd
artifact_dir=$KINE_TEST_DIR
state_dir="$KINE_TEST_DIR/private"
KINE_RESOURCE_GROUP=test-etcd
fixture_started=false
vm_created=true
capz::kine::az() {
  printf '%s\n' "$*" >> "$KINE_TEST_DIR/commands"
  if [[ "$*" == *collect-etcd-status* && "$KINE_TEST_HEALTHY" == true ]]; then
    echo '{"value":[{"message":"ETCD_STATUS_OK"}]}'
  else
    echo '{"value":[{"message":"diagnostic output"}]}'
  fi
}
capz::kine::collect_compactions() { echo collected > "$KINE_TEST_DIR/compaction-called"; }
capz::kine::cleanup 0
`)
			cmd.Dir = repoRoot(t)
			cmd.Env = append(cmd.Environ(), "KINE_TEST_DIR="+dir, "KINE_TEST_HEALTHY="+map[bool]string{true: "true", false: "false"}[healthy])
			output, err := cmd.CombinedOutput()
			if (err == nil) != healthy {
				t.Fatalf("guest status failures must not look successful: %v\n%s", err, output)
			}
			commands := string(readFile(t, filepath.Join(dir, "commands")))
			if !strings.Contains(commands, "collect-etcd-status") || !strings.Contains(commands, "-u etcd -u etcd-metrics") {
				t.Fatal("etcd cleanup must collect native status and the selected services")
			}
			if _, err := os.Stat(filepath.Join(dir, "compaction-called")); err != nil {
				t.Fatal("compaction diagnostics must still be attempted after a status failure")
			}
		})
	}
}

func etcdGuestScript(t *testing.T, path string) string {
	t.Helper()
	var config struct {
		Files []struct {
			Path, Content string
		} `yaml:"write_files"`
	}
	if err := yaml.Unmarshal(readFile(t, filepath.Join(repoRoot(t), "scripts", "kine", "cloud-init-etcd.yaml")), &config); err != nil {
		t.Fatal(err)
	}
	for _, file := range config.Files {
		if file.Path == path {
			return strings.ReplaceAll(file.Content, "__ETCD_VERSION__", "v3.6.6")
		}
	}
	t.Fatalf("missing etcd guest script %s", path)
	return ""
}

func TestEtcdMetricsReadiness(t *testing.T) {
	script := strings.ReplaceAll(etcdGuestScript(t, "/usr/local/bin/check-etcd-metrics"),
		"__METRICS_BIND_ADDRESS__", "10.250.0.4")
	for _, tc := range []struct {
		name                  string
		readyAt, wantAttempts int
	}{
		{"already-ready", 1, 1},
		{"delayed-listener", 3, 3},
		{"never-ready", 0, 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cmd := exec.CommandContext(t.Context(), "bash", "-s")
			cmd.Stdin = strings.NewReader(`
calls=0
curl() {
  calls=$((calls + 1))
  printf '%s\n' "$*" >> "$KINE_TEST_DIR/attempts"
  if (( KINE_TEST_READY_AT == 0 || calls < KINE_TEST_READY_AT )); then
    echo "simulated connection refused" >&2
    return 7
  fi
  echo 'etcd_server_has_leader 1'
}
sleep() { [[ "$1" == 1 ]]; }
` + script)
			cmd.Env = append(cmd.Environ(), "KINE_TEST_DIR="+dir, "KINE_TEST_READY_AT="+strconv.Itoa(tc.readyAt))
			output, err := cmd.CombinedOutput()
			wantOK := tc.readyAt > 0
			if (err == nil) != wantOK || bytes.Contains(output, []byte("ETCD_METRICS_READY")) != wantOK {
				t.Fatalf("readiness must wait for success and fail on exhaustion: %v\n%s", err, output)
			}
			attempts := strings.Split(strings.TrimSpace(string(readFile(t, filepath.Join(dir, "attempts")))), "\n")
			if len(attempts) != tc.wantAttempts {
				t.Fatalf("got %d attempts, want %d", len(attempts), tc.wantAttempts)
			}
			for _, args := range attempts {
				for _, required := range []string{"--fail", "--connect-timeout 2", "--max-time 5", "--cacert /etc/kine/ca.crt", "https://10.250.0.4:8080/metrics"} {
					if !strings.Contains(args, required) {
						t.Fatalf("readiness must retain bounded, verified TLS requests: %s", args)
					}
				}
				if strings.Contains(args, "--insecure") || strings.Contains(args, " -k") {
					t.Fatal("readiness must not bypass TLS verification")
				}
			}
			if !wantOK && !bytes.Contains(output, []byte("did not become ready")) {
				t.Fatal("retry exhaustion must produce an explicit diagnostic")
			}
		})
	}
}

func TestEtcdStatusDiagnostics(t *testing.T) {
	script := etcdGuestScript(t, "/usr/local/bin/collect-etcd-status")
	for _, tc := range []struct {
		name, status, alarms string
		ok                   bool
	}{
		{"healthy", `[{"Status":{"version":"3.6.6","leader":1}}]`, `{}`, true},
		{"wrong-version", `[{"Status":{"version":"3.5.24","leader":1}}]`, `{}`, false},
		{"no-leader", `[{"Status":{"version":"3.6.6","leader":0}}]`, `{}`, false},
		{"server-error", `[{"Status":{"version":"3.6.6","leader":1,"errors":["unhealthy"]}}]`, `{}`, false},
		{"no-space", `[{"Status":{"version":"3.6.6","leader":1}}]`, `{"alarms":[{"alarm":1}]}`, false},
		{"missing-response", `[]`, `{}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), "bash", "-s")
			cmd.Stdin = strings.NewReader(`
etcdctl() {
  case "$*" in
    *"endpoint status"*) printf '%s\n' "$KINE_TEST_STATUS" ;;
    *"alarm list"*) printf '%s\n' "$KINE_TEST_ALARMS" ;;
    *) return 99 ;;
  esac
}
` + script)
			cmd.Env = append(cmd.Environ(), "KINE_TEST_STATUS="+tc.status, "KINE_TEST_ALARMS="+tc.alarms)
			output, err := cmd.CombinedOutput()
			if (err == nil) != tc.ok || bytes.Contains(output, []byte("\nETCD_STATUS_OK\n")) != tc.ok {
				t.Fatalf("status and alarms must be checked before declaring success: %v\n%s", err, output)
			}
		})
	}
}

func TestEtcdCompactionDiagnostics(t *testing.T) {
	script := etcdGuestScript(t, "/usr/local/bin/collect-etcd-compactions")
	for _, tc := range []struct {
		name, journal string
		ok            bool
	}{
		{"projected-messages", `{"ts":"2026-10-22T00:00:00Z","level":"info","msg":"finished scheduled compaction","compact-revision":42,"took":"1ms","key":"do-not-export"}
{"ts":"2026-10-22T00:00:01Z","level":"info","msg":"request completed","request":"compact payload must not be exported"}`, true},
		{"malformed-journal", "not JSON", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "journal"), []byte(tc.journal), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(t.Context(), "bash", "-s")
			cmd.Stdin = strings.NewReader(`
journalctl() {
  [[ "$*" == "--quiet --no-pager --boot --unit etcd _COMM=etcd --output cat" ]] || return 99
  cat "$KINE_TEST_DIR/journal"
}
` + script)
			cmd.Env = append(cmd.Environ(), "KINE_TEST_DIR="+dir)
			output, err := cmd.CombinedOutput()
			if (err == nil) != tc.ok {
				t.Fatalf("unexpected compaction collection result: %v\n%s", err, output)
			}
			if !tc.ok {
				if bytes.Contains(output, []byte("KINE_COMPACTIONS_END")) {
					t.Fatal("malformed journal data must not produce a successful history frame")
				}
				return
			}
			history := decodeCompactionHistory(t, string(output), dir)
			if !strings.Contains(history, `"revision":42`) || strings.Contains(history, "do-not-export") ||
				strings.Contains(history, "request completed") || strings.Contains(history, "payload") {
				t.Fatalf("only compaction summary fields should be retained: %s", history)
			}
		})
	}
}
