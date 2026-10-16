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
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestConnectionBudgetDefaults(t *testing.T) {
	for _, tc := range []struct {
		name, postgres, open, idle, want, errorText string
	}{
		{"default", "", "", "", "1024\n960\n128\n", ""},
		{"derived", "512", "", "", "512\n448\n128\n", ""},
		{"smaller-pool", "128", "", "", "128\n64\n64\n", ""},
		{"explicit", "512", "256", "64", "512\n256\n64\n", ""},
		{"idle-at-cap", "512", "256", "256", "512\n256\n256\n", ""},
		{"no-headroom", "64", "", "", "", "must exceed"},
		{"zero-postgres", "0", "", "", "", "positive integer"},
		{"invalid-postgres", "1024x", "", "", "", "positive integer"},
		{"leading-zero", "01024", "", "", "", "positive integer"},
		{"unlimited-pool", "", "0", "", "", "unlimited pool"},
		{"negative-pool", "", "-1", "", "", "unlimited pool"},
		{"invalid-pool", "", "bad", "", "", "positive integer"},
		{"no-reserve", "", "1024", "", "", "at least 64"},
		{"insufficient-reserve", "", "961", "", "", "at least 64"},
		{"idle-too-large", "", "128", "129", "", "max-idle <= max-open"},
		{"zero-idle", "", "", "0", "", "positive integer"},
		{"invalid-idle", "", "", "1.5", "", "positive integer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), "bash", "-c", `
set -euo pipefail
source ./scripts/kine/connections.sh
capz::kine::connection_defaults
printf '%s\n' "$KINE_POSTGRES_MAX_CONNECTIONS" "$KINE_DATASTORE_MAX_OPEN_CONNECTIONS" "$KINE_DATASTORE_MAX_IDLE_CONNECTIONS"
`)
			cmd.Dir = repoRoot(t)
			cmd.Env = append(cmd.Environ(),
				"KINE_POSTGRES_MAX_CONNECTIONS="+tc.postgres,
				"KINE_DATASTORE_MAX_OPEN_CONNECTIONS="+tc.open,
				"KINE_DATASTORE_MAX_IDLE_CONNECTIONS="+tc.idle,
			)
			output, err := cmd.CombinedOutput()
			if tc.errorText != "" {
				if err == nil || !bytes.Contains(output, []byte(tc.errorText)) {
					t.Fatalf("expected %q error, got %v: %s", tc.errorText, err, output)
				}
				return
			}
			if err != nil || string(output) != tc.want {
				t.Fatalf("unexpected budget: %v\n%s", err, output)
			}
		})
	}
}

func TestConnectionBudgetCloudInit(t *testing.T) {
	for _, tc := range []struct{ name, postgres, open, idle, want string }{
		{"default", "", "", "", "KINE_POSTGRES_MAX_CONNECTIONS=1024\nKINE_DATASTORE_MAX_OPEN_CONNECTIONS=960\nKINE_DATASTORE_MAX_IDLE_CONNECTIONS=128\n"},
		{"derived", "512", "", "", "KINE_POSTGRES_MAX_CONNECTIONS=512\nKINE_DATASTORE_MAX_OPEN_CONNECTIONS=448\nKINE_DATASTORE_MAX_IDLE_CONNECTIONS=128\n"},
		{"explicit", "512", "256", "64", "KINE_POSTGRES_MAX_CONNECTIONS=512\nKINE_DATASTORE_MAX_OPEN_CONNECTIONS=256\nKINE_DATASTORE_MAX_IDLE_CONNECTIONS=64\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state := t.TempDir()
			cmd := exec.CommandContext(t.Context(), "bash", "-c", `
source ./scripts/ci-kine-conformance.sh
repo_root=$PWD
state_dir=$KINE_TEST_DIR
umask 077
capz::kine::certificates
`)
			cmd.Dir = repoRoot(t)
			cmd.Env = append(cmd.Environ(), "KINE_TEST_DIR="+state,
				"KINE_POSTGRES_MAX_CONNECTIONS="+tc.postgres,
				"KINE_DATASTORE_MAX_OPEN_CONNECTIONS="+tc.open,
				"KINE_DATASTORE_MAX_IDLE_CONNECTIONS="+tc.idle,
			)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("render connection configuration: %v\n%s", err, output)
			}
			var config struct {
				Files []struct {
					Path    string `yaml:"path"`
					Content string `yaml:"content"`
				} `yaml:"write_files"`
			}
			if err := yaml.Unmarshal(readFile(t, filepath.Join(state, "cloud-init.yaml")), &config); err != nil {
				t.Fatal(err)
			}
			foundSettings, foundService := false, false
			for _, file := range config.Files {
				switch file.Path {
				case "/etc/kine/connections.env":
					foundSettings = true
					if file.Content != tc.want {
						t.Fatalf("incorrect shared connection settings: %s", file.Content)
					}
				case "/etc/systemd/system/kine.service":
					foundService = strings.Contains(file.Content, "EnvironmentFile=/etc/kine/connections.env")
				}
			}
			if !foundSettings || !foundService {
				t.Fatal("Kine must consume the shared connection settings")
			}
		})
	}
	storage := string(readFile(t, filepath.Join(repoRoot(t), "scripts", "kine", "configure-performance-storage.sh")))
	for _, required := range []string{
		"source /etc/kine/connections.env",
		"capz::kine::connection_defaults",
		"max_connections = ${KINE_POSTGRES_MAX_CONNECTIONS}",
		"capz::kine::verify_postgres_connections",
	} {
		if !strings.Contains(storage, required) {
			t.Errorf("PostgreSQL initialization must consume and verify the shared budget: missing %s", required)
		}
	}
}

func TestPostgresConnectionBudget(t *testing.T) {
	for _, tc := range []struct {
		name, settings string
		queryFails     bool
		wantOK         bool
	}{
		{"default", "1024|1021", false, true},
		{"wrong-server-limit", "512|509", false, false},
		{"no-spare-normal-slot", "1024|960", false, false},
		{"too-many-reserved", "1024|959", false, false},
		{"invalid-response", "1024", false, false},
		{"invalid-usable", "1024|bad", false, false},
		{"extra-fields", "1024|1021|extra", false, false},
		{"impossible-capacity", "1024|1025", false, false},
		{"query-failed-with-output", "1024|1021", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), "bash", "-c", `
set -euo pipefail
source ./scripts/kine/connections.sh
unset KINE_POSTGRES_MAX_CONNECTIONS KINE_DATASTORE_MAX_OPEN_CONNECTIONS KINE_DATASTORE_MAX_IDLE_CONNECTIONS
capz::kine::connection_defaults
runuser() {
  [[ "$*" == *"superuser_reserved_connections"* && "$*" == *"'reserved_connections'"* ]] || return 99
  printf '%s\n' "$KINE_TEST_POSTGRES_SETTINGS"
  [[ "$KINE_TEST_QUERY_FAILS" != yes ]]
}
if capz::kine::verify_postgres_connections; then
  exit 0
fi
exit 1
`)
			cmd.Dir = repoRoot(t)
			fail := "no"
			if tc.queryFails {
				fail = "yes"
			}
			cmd.Env = append(cmd.Environ(), "KINE_TEST_POSTGRES_SETTINGS="+tc.settings, "KINE_TEST_QUERY_FAILS="+fail)
			output, err := cmd.CombinedOutput()
			if (err == nil) != tc.wantOK {
				t.Fatalf("unexpected PostgreSQL budget validation: %v\n%s", err, output)
			}
			if tc.wantOK && !bytes.Contains(output, []byte("Kine max-open=960, max-idle=128")) {
				t.Fatalf("missing verified connection settings: %s", output)
			}
			if !tc.wantOK && len(output) == 0 {
				t.Fatal("invalid PostgreSQL settings must produce an explicit diagnostic")
			}
		})
	}
}
