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
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func readFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(name) //nolint:gosec // Paths are supplied by these tests, not user input.
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestFlavor(t *testing.T) {
	root := repoRoot(t)
	cmd := exec.CommandContext(t.Context(), "./hack/tools/bin/kustomize", "build",
		"--load-restrictor", "LoadRestrictionsNone", "./templates/test/ci/prow-kine-postgres")
	cmd.Dir = root
	rendered, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build flavor: %v\n%s", err, rendered)
	}
	generated := readFile(t, filepath.Join(root, "templates", "test", "ci", "cluster-template-prow-kine-postgres.yaml"))
	if !bytes.Equal(rendered, generated) {
		t.Fatal("generated Kine flavor is stale; run hack/gen-flavors.sh")
	}
	decoder := yaml.NewDecoder(bytes.NewReader(rendered))
	found := map[string]bool{}
	for {
		var object struct {
			Kind     string `yaml:"kind"`
			Metadata struct {
				Name string `yaml:"name"`
			} `yaml:"metadata"`
			Spec struct {
				NetworkSpec struct {
					Vnet struct {
						CIDRs    []string `yaml:"cidrBlocks"`
						Peerings []struct {
							ResourceGroup string `yaml:"resourceGroup"`
							Name          string `yaml:"remoteVnetName"`
						} `yaml:"peerings"`
					} `yaml:"vnet"`
				} `yaml:"networkSpec"`
				KubeadmConfigSpec struct {
					ClusterConfiguration struct {
						Etcd struct {
							Local    any `yaml:"local"`
							External struct {
								Endpoints []string `yaml:"endpoints"`
								CA        string   `yaml:"caFile"`
								Cert      string   `yaml:"certFile"`
								Key       string   `yaml:"keyFile"`
							} `yaml:"external"`
						} `yaml:"etcd"`
					} `yaml:"clusterConfiguration"`
					Init struct {
						NodeRegistration struct {
							Ignore []string          `yaml:"ignorePreflightErrors"`
							Args   map[string]string `yaml:"kubeletExtraArgs"`
						} `yaml:"nodeRegistration"`
					} `yaml:"initConfiguration"`
					DiskSetup struct {
						Partitions  []any `yaml:"partitions"`
						Filesystems []struct {
							Label string `yaml:"label"`
						} `yaml:"filesystems"`
					} `yaml:"diskSetup"`
					Mounts []any `yaml:"mounts"`
				} `yaml:"kubeadmConfigSpec"`
				Template struct {
					Spec struct {
						DataDisks []any `yaml:"dataDisks"`
					} `yaml:"spec"`
				} `yaml:"template"`
			} `yaml:"spec"`
			Data map[string]string `yaml:"data"`
		}
		if err := decoder.Decode(&object); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
		switch object.Kind {
		case "KubeadmControlPlane":
			found["control-plane"] = true
			config := object.Spec.KubeadmConfigSpec
			etcd := config.ClusterConfiguration.Etcd
			if etcd.Local != nil || len(etcd.External.Endpoints) != 1 || etcd.External.Endpoints[0] != "https://10.250.0.4:2379" {
				t.Fatalf("expected only the external Kine endpoint: %+v", etcd)
			}
			if etcd.External.CA != "/etc/kubernetes/pki/etcd/ca.crt" ||
				etcd.External.Cert != "/etc/kubernetes/pki/apiserver-etcd-client.crt" ||
				etcd.External.Key != "/etc/kubernetes/pki/apiserver-etcd-client.key" {
				t.Fatal("external etcd must use the CABPK certificate paths")
			}
			node := config.Init.NodeRegistration
			if len(node.Ignore) != 1 || node.Ignore[0] != "ExternalEtcdVersion" || node.Args["cloud-provider"] != "external" {
				t.Fatalf("unexpected preflight exception or lost Azure kubelet configuration: %+v", node)
			}
			if len(config.Mounts) != 0 || len(config.DiskSetup.Partitions) != 0 ||
				len(config.DiskSetup.Filesystems) != 1 || config.DiskSetup.Filesystems[0].Label != "ephemeral0" {
				t.Fatal("remove only the unused etcd disk setup, preserving the ephemeral filesystem")
			}
		case "AzureCluster":
			found["network"] = true
			vnet := object.Spec.NetworkSpec.Vnet
			if len(vnet.CIDRs) != 1 || vnet.CIDRs[0] != "10.0.0.0/16" ||
				len(vnet.Peerings) != 1 || vnet.Peerings[0].ResourceGroup != "${KINE_RESOURCE_GROUP}" ||
				vnet.Peerings[0].Name != "kine-vnet" {
				t.Fatalf("unexpected datastore peering: %+v", vnet)
			}
		case "AzureMachineTemplate":
			if object.Metadata.Name == "${CLUSTER_NAME}-control-plane" {
				found["machine"] = true
				if len(object.Spec.Template.Spec.DataDisks) != 0 {
					t.Fatal("control plane must not allocate an unused etcd data disk")
				}
			}
		case "Secret":
			switch object.Metadata.Name {
			case "${CLUSTER_NAME}-etcd":
				found["ca"] = true
				if object.Data["tls.crt"] != "${KINE_CA_CERT_B64}" || object.Data["tls.key"] != "" {
					t.Fatal("distribute the datastore CA certificate, never its signing key")
				}
			case "${CLUSTER_NAME}-apiserver-etcd-client":
				found["client"] = true
				if object.Data["tls.crt"] != "${KINE_CLIENT_CERT_B64}" || object.Data["tls.key"] != "${KINE_CLIENT_KEY_B64}" {
					t.Fatal("missing apiserver mTLS credentials")
				}
			}
		}
	}
	for _, name := range []string{"control-plane", "network", "machine", "ca", "client"} {
		if !found[name] {
			t.Errorf("missing %s resource", name)
		}
	}
}

func TestCertificatesAndCloudInit(t *testing.T) {
	root, state := repoRoot(t), t.TempDir()
	cmd := exec.CommandContext(t.Context(), "bash", "-c", `
source ./scripts/ci-kine-conformance.sh
repo_root=$PWD
state_dir=$KINE_TEST_DIR
umask 077
capz::kine::certificates
`)
	cmd.Dir = root
	cmd.Env = append(cmd.Environ(), "KINE_TEST_DIR="+state)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate credentials: %v\n%s", err, output)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(readFile(t, filepath.Join(state, "ca.crt"))) {
		t.Fatal("invalid CA")
	}
	for _, tc := range []struct {
		name  string
		dns   string
		usage x509.ExtKeyUsage
	}{
		{"server", "10.250.0.4", x509.ExtKeyUsageServerAuth},
		{"server", "127.0.0.1", x509.ExtKeyUsageServerAuth},
		{"client", "", x509.ExtKeyUsageClientAuth},
	} {
		block, _ := pem.Decode(readFile(t, filepath.Join(state, tc.name+".crt")))
		if block == nil {
			t.Fatalf("invalid %s PEM", tc.name)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := cert.Verify(x509.VerifyOptions{
			Roots: roots, DNSName: tc.dns, KeyUsages: []x509.ExtKeyUsage{tc.usage},
		}); err != nil {
			t.Fatalf("verify %s certificate: %v", tc.name, err)
		}
	}
	var config struct {
		Packages []string `yaml:"packages"`
		Files    []struct {
			Path        string `yaml:"path"`
			Permissions string `yaml:"permissions"`
			Encoding    string `yaml:"encoding"`
			Content     string `yaml:"content"`
		} `yaml:"write_files"`
	}
	cloudInit := readFile(t, filepath.Join(state, "cloud-init.yaml"))
	if err := yaml.Unmarshal(cloudInit, &config); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(cloudInit, []byte("__CA_CERT__")) {
		t.Fatal("unexpanded cloud-init credentials")
	}
	certificates := 0
	for _, file := range config.Files {
		if file.Encoding == "b64" {
			certificates++
			decoded, err := base64.StdEncoding.DecodeString(file.Content)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(decoded, readFile(t, filepath.Join(state, filepath.Base(file.Path)))) {
				t.Fatalf("incorrect rendered credential %s", file.Path)
			}
			if strings.HasSuffix(file.Path, ".key") && file.Permissions != "0600" {
				t.Fatalf("private key has unsafe permissions: %s", file.Path)
			}
		}
		if strings.HasPrefix(file.Content, "#!/usr/bin/env bash") {
			cmd := exec.CommandContext(t.Context(), "bash", "-n")
			cmd.Stdin = strings.NewReader(file.Content)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("invalid embedded script %s: %v\n%s", file.Path, err, output)
			}
		}
	}
	if certificates != 5 {
		t.Fatalf("expected five credential files, got %d", certificates)
	}
}

func TestReadinessDiagnostics(t *testing.T) {
	var config struct {
		Files []struct {
			Path    string `yaml:"path"`
			Content string `yaml:"content"`
		} `yaml:"write_files"`
	}
	if err := yaml.Unmarshal(readFile(t, filepath.Join(repoRoot(t), "scripts", "kine", "cloud-init.yaml")), &config); err != nil {
		t.Fatal(err)
	}
	var check string
	for _, file := range config.Files {
		if file.Path == "/usr/local/bin/check-kine" {
			check = file.Content
		}
	}
	if check == "" {
		t.Fatal("missing Kine readiness script")
	}
	for _, tc := range []struct {
		name     string
		wantExit int
		want     []string
	}{
		{"success", 0, []string{"etcdctl version: test", "active", "Kine transaction exit status: 0\nSUCCESS\n\nOK", "Kine read exit status: 0\nValue: <ready>"}},
		{"inactive", 3, []string{"inactive", "Kine service check exit status: 3"}},
		{"transaction-error", 7, []string{"transaction RPC failed", "Kine transaction exit status: 7"}},
		{"transaction-rejected", 1, []string{"Kine transaction exit status: 0\nFAILURE", "Kine transaction did not report SUCCESS."}},
		{"read-error", 8, []string{"read RPC failed", "Kine read exit status: 8"}},
		{"read-mismatch", 1, []string{"Kine read exit status: 0\nValue: <wrong>", "does not match expected value <ready>"}},
		{"read-warning", 0, []string{"read warning", "Kine read exit status: 0\nValue: <ready>"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), "bash", "-s")
			cmd.Env = append(cmd.Environ(), "KINE_TEST_CHECK_CASE="+tc.name)
			cmd.Stdin = strings.NewReader(`
systemctl() {
  if [[ "$KINE_TEST_CHECK_CASE" == inactive ]]; then echo inactive; return 3; fi
  echo active
}
cat() {
  [[ "$*" == /proc/sys/kernel/random/uuid ]] || return 99
  echo offline-smoke-key
}
etcdctl() {
  if [[ "$*" == version ]]; then echo "etcdctl version: test"; return; fi
  case "$2" in
    txn)
      command cat > /dev/null
      if [[ "$KINE_TEST_CHECK_CASE" == transaction-error ]]; then
        echo "transaction RPC failed" >&2
        return 7
      fi
      if [[ "$KINE_TEST_CHECK_CASE" == transaction-rejected ]]; then
        echo FAILURE
      else
        printf 'SUCCESS\n\nOK\n'
      fi ;;
    get)
      [[ "$3" == /capz-kine-smoke/offline-smoke-key ]] || return 99
      if [[ "$KINE_TEST_CHECK_CASE" == read-error ]]; then
        echo "read RPC failed" >&2
        return 8
      fi
      if [[ "$KINE_TEST_CHECK_CASE" == read-warning ]]; then echo "read warning" >&2; fi
      if [[ "$KINE_TEST_CHECK_CASE" == read-mismatch ]]; then echo wrong; else echo ready; fi ;;
    *) return 99 ;;
  esac
}
` + check)
			output, err := cmd.CombinedOutput()
			exitCode := 0
			if err != nil {
				var exitError *exec.ExitError
				if !errors.As(err, &exitError) {
					t.Fatal(err)
				}
				exitCode = exitError.ExitCode()
			}
			if exitCode != tc.wantExit {
				t.Fatalf("exit=%d, want %d\n%s", exitCode, tc.wantExit, output)
			}
			for _, want := range tc.want {
				if !bytes.Contains(output, []byte(want)) {
					t.Errorf("missing diagnostic %q\n%s", want, output)
				}
			}
			if ready := bytes.Contains(output, []byte("\nKINE_READY\n")); ready != (tc.wantExit == 0) {
				t.Fatalf("unexpected readiness marker: %s", output)
			}
		})
	}
}

func TestProvisionReadiness(t *testing.T) {
	for _, ready := range []bool{true, false} {
		t.Run(map[bool]string{true: "ready", false: "guest-command-failed"}[ready], func(t *testing.T) {
			dir := t.TempDir()
			message := "KINE_READY"
			if !ready {
				message = "cloud-init failed"
			}
			cmd := exec.CommandContext(t.Context(), "bash", "-c", `
source ./scripts/ci-kine-conformance.sh
state_dir=$KINE_TEST_DIR
artifact_dir=$KINE_TEST_DIR
KINE_RESOURCE_GROUP=test-kine
AZURE_SUBSCRIPTION_ID=test-subscription
AZURE_LOCATION=test-region
run_id=test-run
message=$KINE_TEST_MESSAGE
az() {
  printf '%s\n' "$*" >> "$state_dir/commands"
  if [[ "$*" == "vm run-command invoke"* ]]; then
    printf '{"value":[{"message":"%s"}]}\n' "$message"
  fi
}
capz::kine::provision
`)
			cmd.Dir = repoRoot(t)
			cmd.Env = append(cmd.Environ(), "KINE_TEST_DIR="+dir, "KINE_TEST_MESSAGE="+message)
			output, err := cmd.CombinedOutput()
			if (err == nil) != ready {
				t.Fatalf("readiness=%t: %v\n%s", ready, err, output)
			}
			commands := string(readFile(t, filepath.Join(dir, "commands")))
			for _, expected := range []string{
				"--source-address-prefixes 10.0.0.0/24",
				"--destination-port-ranges 2379",
				"network nat gateway create",
				"--private-ip-address 10.250.0.4",
				"--nics kine-nic",
				"--subscription test-subscription --only-show-errors",
			} {
				if !strings.Contains(commands, expected) {
					t.Errorf("missing provisioning invariant: %s", expected)
				}
			}
		})
	}
}

func TestCleanupOwnership(t *testing.T) {
	for _, tc := range []struct {
		name     string
		exists   string
		owner    string
		skip     string
		initial  string
		wantExit int
		deleted  bool
	}{
		{"successful-run", "false", "test-run", "false", "0", 0, true},
		{"collect-diagnostics", "false", "test-run", "false", "7", 7, true},
		{"failed-run", "false", "test-run", "false", "7", 7, true},
		{"live-workload", "true", "test-run", "false", "0", 1, false},
		{"unknown-workload", "error", "test-run", "false", "0", 1, false},
		{"wrong-owner", "false", "someone-else", "false", "0", 1, false},
		{"unknown-owner", "false", "error", "false", "0", 1, false},
		{"delete-failed", "false", "test-run", "false", "0", 1, true},
		{"skip-cleanup", "true", "test-run", "true", "0", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			state := filepath.Join(dir, "private")
			if err := os.Mkdir(state, 0o700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(t.Context(), "bash", "-c", `
source ./scripts/ci-kine-conformance.sh
state_dir="${KINE_TEST_DIR}/private"
artifact_dir=$KINE_TEST_DIR
fixture_started=true
vm_created=false
CLUSTER_NAME=test-workload
KINE_RESOURCE_GROUP=test-kine
run_id=test-run
workload_exists=$KINE_TEST_WORKLOAD_EXISTS
fixture_owner=$KINE_TEST_FIXTURE_OWNER
SKIP_CLEANUP=$KINE_TEST_SKIP_CLEANUP
test_name=$KINE_TEST_NAME
if [[ "$test_name" == collect-diagnostics ]]; then vm_created=true; fi
capz::kine::az() {
  printf '%s\n' "$*" >> "$artifact_dir/commands"
  case "$1 $2" in
    "vm run-command")
      echo '{"value":[{"message":"cloud-init status: error"}]}' ;;
    "group exists")
      [[ "$workload_exists" != error ]] || return 1
      echo "$workload_exists" ;;
    "group show")
      [[ "$fixture_owner" != error ]] || return 1
      echo "$fixture_owner" ;;
    "group delete")
      [[ "$test_name" != delete-failed ]] || return 1
      [[ "$*" == "group delete --name test-kine --yes" ]] ;;
    *) return 1 ;;
  esac
}
capz::kine::cleanup "$KINE_TEST_INITIAL_EXIT"
`)
			cmd.Dir = repoRoot(t)
			cmd.Env = append(cmd.Environ(),
				"KINE_TEST_DIR="+dir,
				"KINE_TEST_WORKLOAD_EXISTS="+tc.exists,
				"KINE_TEST_FIXTURE_OWNER="+tc.owner,
				"KINE_TEST_SKIP_CLEANUP="+tc.skip,
				"KINE_TEST_INITIAL_EXIT="+tc.initial,
				"KINE_TEST_NAME="+tc.name,
			)
			output, err := cmd.CombinedOutput()
			exitCode := 0
			if err != nil {
				var exitError *exec.ExitError
				if !errors.As(err, &exitError) {
					t.Fatal(err)
				}
				exitCode = exitError.ExitCode()
			}
			if exitCode != tc.wantExit {
				t.Fatalf("exit=%d, want %d\n%s", exitCode, tc.wantExit, output)
			}
			commands, err := os.ReadFile(filepath.Join(dir, "commands")) //nolint:gosec // Read only from this test's temporary directory.
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				t.Fatal(err)
			}
			if deleted := bytes.Contains(commands, []byte("group delete")); deleted != tc.deleted {
				t.Fatalf("fixture deleted=%t, want %t\n%s", deleted, tc.deleted, output)
			}
			if tc.name == "collect-diagnostics" {
				for _, expected := range []string{"journalctl --no-pager -u kine", "cloud-init status --long", "cloud-init status exit code:"} {
					if !bytes.Contains(commands, []byte(expected)) {
						t.Errorf("missing VM diagnostic command %q", expected)
					}
				}
				if !bytes.Contains(readFile(t, filepath.Join(dir, "datastore-logs.json")), []byte("cloud-init status: error")) {
					t.Error("missing cloud-init diagnostic artifact")
				}
			}
			if _, err := os.Stat(state); !errors.Is(err, os.ErrNotExist) {
				t.Fatal("temporary private credentials were not removed")
			}
		})
	}
}

func TestRegionSelection(t *testing.T) {
	for _, state := range []string{"unset", "empty", "explicit"} {
		t.Run(state, func(t *testing.T) {
			dir := t.TempDir()
			cmd := exec.CommandContext(t.Context(), "bash", "-c", `
source ./scripts/ci-kine-conformance.sh
export AZURE_SUBSCRIPTION_ID=test-subscription
export ARTIFACTS="$KINE_TEST_DIR" TMPDIR="$KINE_TEST_DIR" CLUSTER_NAME=capz-kine-unit-test
export WINDOWS=false IP_FAMILY=ipv4 TEST_K8S=false E2E_ARGS=
export CONFORMANCE_FLAVOR=kine-postgres KUBERNETES_VERSION=v1.35.4
case "$KINE_TEST_LOCATION_STATE" in
  unset) unset AZURE_LOCATION ;;
  empty) export AZURE_LOCATION= ;;
  explicit) export AZURE_LOCATION=test-region ;;
esac
az() {
  if [[ "$*" != "group exists"* ]]; then
    echo "Unexpected Azure mutation in an offline test" >&2
    return 99
  fi
  printf 'azure=%s\n' "${AZURE_LOCATION:?}" >> "$KINE_TEST_DIR/regions"
  echo false
}
ssh-keygen() { :; }
capz::kine::certificates() { :; }
capz::kine::provision() {
  printf 'fixture=%s\n' "${AZURE_LOCATION:?}" >> "$KINE_TEST_DIR/regions"
}
bash() {
  if [[ "$1" != "$repo_root/scripts/ci-conformance.sh" ]]; then
    echo "Unexpected subprocess in an offline test" >&2
    return 99
  fi
  command bash -c 'printf "workload=%s\n" "${AZURE_LOCATION:?}"' >> "$KINE_TEST_DIR/regions"
}
capz::kine::main
`)
			cmd.Dir = repoRoot(t)
			cmd.Env = append(cmd.Environ(), "KINE_TEST_DIR="+dir, "KINE_TEST_LOCATION_STATE="+state)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("run wrapper: %v\n%s", err, output)
			}
			lines := strings.Split(strings.TrimSpace(string(readFile(t, filepath.Join(dir, "regions")))), "\n")
			if len(lines) != 4 {
				t.Fatalf("expected two Azure checks, fixture provisioning, and workload invocation: %v", lines)
			}
			region := strings.TrimPrefix(lines[0], "azure=")
			if region == "" || (state == "explicit" && region != "test-region") {
				t.Fatalf("unexpected region for %s input: %q", state, region)
			}
			for i, prefix := range []string{"azure=", "azure=", "fixture=", "workload="} {
				if lines[i] != prefix+region {
					t.Fatalf("region must be selected before Azure calls and shared with the workload: %v", lines)
				}
			}
		})
	}
}

func TestRejectUnsupportedRuns(t *testing.T) {
	for _, tc := range []struct {
		name    string
		setting string
		message string
	}{
		{"windows", "WINDOWS=true", "supports Linux/IPv4"},
		{"ipv6", "IP_FAMILY=ipv6", "supports Linux/IPv4"},
		{"ci-artifacts", "E2E_ARGS=-kubetest.use-ci-artifacts", "supports Linux/IPv4"},
		{"unpinned", "KUBERNETES_VERSION=stable-1.35", "Pin KUBERNETES_VERSION"},
		{"wrong-flavor", "CONFORMANCE_FLAVOR=other", "requires CONFORMANCE_FLAVOR"},
		{"existing-group", "KUBERNETES_VERSION=v1.35.4", "Refusing to reuse resource group"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), "bash", "-c", `
source ./scripts/ci-kine-conformance.sh
export AZURE_SUBSCRIPTION_ID=test-subscription AZURE_LOCATION=test-region
export ARTIFACTS="$KINE_TEST_DIR" CLUSTER_NAME=capz-kine-unit-test
export WINDOWS=false IP_FAMILY=ipv4 TEST_K8S=false E2E_ARGS=
export CONFORMANCE_FLAVOR=kine-postgres KUBERNETES_VERSION=v1.35.4
export "$KINE_TEST_SETTING"
az() {
  if [[ "$*" == "group exists"* ]]; then echo true; else
    echo "Unexpected Azure mutation in an offline test" >&2
    return 99
  fi
}
capz::kine::main
`)
			cmd.Dir = repoRoot(t)
			cmd.Env = append(cmd.Environ(), "KINE_TEST_DIR="+t.TempDir(), "KINE_TEST_SETTING="+tc.setting)
			output, err := cmd.CombinedOutput()
			if err == nil || !bytes.Contains(output, []byte(tc.message)) {
				t.Fatalf("expected rejection %q: %v\n%s", tc.message, err, output)
			}
		})
	}
}
