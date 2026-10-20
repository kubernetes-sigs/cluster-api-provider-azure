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
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCompactionHistory(t *testing.T) {
	var config struct {
		Files []struct {
			Path    string `yaml:"path"`
			Content string `yaml:"content"`
		} `yaml:"write_files"`
	}
	if err := yaml.Unmarshal(readFile(t, filepath.Join(repoRoot(t), "scripts", "kine", "cloud-init.yaml")), &config); err != nil {
		t.Fatal(err)
	}
	var script string
	for _, file := range config.Files {
		if file.Path == "/usr/local/bin/collect-kine-compactions" {
			script = file.Content
		}
	}
	if script == "" {
		t.Fatal("missing compaction-history collector")
	}
	for _, tc := range []struct {
		name, log, expected string
		failJournal         bool
	}{
		{"history", "2026-10-20T10:00:00+0000 kine kine[1]: \033[36mINFO\033[0m[2026-10-20T10:00:00Z] COMPACT compacted from 1 to 100 in 1 transactions over 2ms\n" +
			strings.Repeat("2026-10-20T10:01:00+0000 kine kine[1]: INFO[2026-10-20T10:01:00Z] Slow SQL: payload includes COMPACT compacted from and Compact failed: do not export\n", 100) +
			"2026-10-20T10:02:00+0000 kine kine[1]: ERRO[2026-10-20T10:02:00Z] Compact failed: context deadline exceeded\n",
			"2026-10-20T10:00:00+0000 kine kine[1]: INFO[2026-10-20T10:00:00Z] COMPACT compacted from 1 to 100 in 1 transactions over 2ms\n2026-10-20T10:02:00+0000 kine kine[1]: ERRO[2026-10-20T10:02:00Z] Compact failed: context deadline exceeded\n", false},
		{"no-compactions", "ordinary log message\n", "", false},
		{"journal-error", "", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "collect.sh")
			if err := os.WriteFile(path, []byte(script), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "journal"), []byte(tc.log), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(t.Context(), "bash", "-c", `
journalctl() {
  [[ "$*" == "--quiet --no-pager --boot --unit kine --output short-iso-precise" ]] || return 99
  [[ "$KINE_TEST_JOURNAL_FAIL" != true ]] || { echo "journal unavailable" >&2; return 1; }
  cat "$KINE_TEST_DIR/journal"
}
export -f journalctl
bash "$KINE_TEST_DIR/collect.sh"
`)
			cmd.Env = append(cmd.Environ(), "KINE_TEST_DIR="+dir, "KINE_TEST_JOURNAL_FAIL="+map[bool]string{true: "true", false: "false"}[tc.failJournal])
			output, err := cmd.CombinedOutput()
			if tc.failJournal {
				if err == nil || bytes.Contains(output, []byte("KINE_COMPACTIONS_END")) {
					t.Fatal("journal errors must not produce a successful history frame")
				}
				return
			}
			if err != nil {
				t.Fatalf("collect journal: %v\n%s", err, output)
			}
			result := decodeCompactionHistory(t, string(output), dir)
			if result != tc.expected {
				t.Fatalf("compaction history must include old summaries and exclude SQL payloads: %q", result)
			}
		})
	}
}

func decodeCompactionHistory(t *testing.T, message, dir string) string {
	t.Helper()
	data, err := json.Marshal(map[string]any{"value": []any{map[string]string{"message": "Enable succeeded:\n[stdout]\n" + message + "\n[stderr]\n"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "response.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), "bash", "-c", `
source ./scripts/ci-kine-conformance.sh
artifact_dir=$KINE_TEST_DIR
KINE_RESOURCE_GROUP=test-kine
capz::kine::az() { cat "$KINE_TEST_DIR/response.json"; }
capz::kine::collect_compactions
`)
	cmd.Dir = repoRoot(t)
	cmd.Env = append(cmd.Environ(), "KINE_TEST_DIR="+dir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("decode compaction history: %v\n%s", err, output)
	}
	return string(readFile(t, filepath.Join(dir, "compaction.log")))
}

func TestCompactionHistoryRejectsTruncation(t *testing.T) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write([]byte("COMPACT compacted from 1 to 100\n")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	valid := base64.StdEncoding.EncodeToString(compressed.Bytes())
	truncated := base64.StdEncoding.EncodeToString(compressed.Bytes()[:compressed.Len()-8])
	for _, message := range []string{
		valid + "\nKINE_COMPACTIONS_END\n",
		"KINE_COMPACTIONS_BEGIN\n" + valid + "\n",
		"KINE_COMPACTIONS_BEGIN\n" + truncated + "\nKINE_COMPACTIONS_END\n",
		"KINE_COMPACTIONS_BEGIN\nAAAA\nKINE_COMPACTIONS_END\n",
	} {
		dir := t.TempDir()
		data, err := json.Marshal(map[string]any{"value": []any{map[string]string{"message": message}}})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "response.json"), data, 0o600); err != nil {
			t.Fatal(err)
		}
		cmd := exec.CommandContext(t.Context(), "bash", "-c", `
source ./scripts/ci-kine-conformance.sh
artifact_dir=$KINE_TEST_DIR
KINE_RESOURCE_GROUP=test-kine
capz::kine::az() { cat "$KINE_TEST_DIR/response.json"; }
capz::kine::collect_compactions
`)
		cmd.Dir = repoRoot(t)
		cmd.Env = append(cmd.Environ(), "KINE_TEST_DIR="+dir)
		if output, err := cmd.CombinedOutput(); err == nil || len(output) == 0 {
			t.Fatalf("incomplete history must fail with a diagnostic: %v\n%s", err, output)
		}
		if _, err := os.Stat(filepath.Join(dir, "compaction.log")); err == nil {
			t.Fatal("partial compaction history must not be published as complete")
		}
	}
}

func TestCompactionHistoryCleanup(t *testing.T) {
	for _, tc := range []struct {
		name, profile string
		fail          bool
	}{
		{"conformance", "conformance", false},
		{"performance", "performance", false},
		{"collection-failure", "performance", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.Mkdir(filepath.Join(dir, "private"), 0o700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(t.Context(), "bash", "-c", `
source ./scripts/ci-kine-conformance.sh
artifact_dir=$KINE_TEST_DIR
state_dir="$KINE_TEST_DIR/private"
KINE_RESOURCE_GROUP=test-kine
fixture_started=false
vm_created=true
capz::kine::az() { echo '{"value":[]}'; }
capz::kine::collect_compactions() {
  echo collected > "$KINE_TEST_DIR/compaction-called"
  [[ "$KINE_TEST_FAIL" != true ]]
}
capz::kine::cleanup 0
`)
			cmd.Dir = repoRoot(t)
			cmd.Env = append(cmd.Environ(), "KINE_TEST_DIR="+dir, "KINE_PROFILE="+tc.profile,
				"KINE_TEST_FAIL="+map[bool]string{true: "true", false: "false"}[tc.fail])
			output, err := cmd.CombinedOutput()
			if (err != nil) != tc.fail {
				t.Fatalf("cleanup must report collection failures: %v\n%s", err, output)
			}
			_, statErr := os.Stat(filepath.Join(dir, "compaction-called"))
			if (statErr == nil) != (tc.profile == "performance") {
				t.Fatal("compaction collection must run only for the performance fixture")
			}
		})
	}
}
