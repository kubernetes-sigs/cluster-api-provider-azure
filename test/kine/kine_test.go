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
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
					Subnets []struct {
						Name  string   `yaml:"name"`
						Role  string   `yaml:"role"`
						CIDRs []string `yaml:"cidrBlocks"`
					} `yaml:"subnets"`
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
						Image     *struct {
							ComputeGallery struct {
								Gallery string `yaml:"gallery"`
								Name    string `yaml:"name"`
								Version string `yaml:"version"`
							} `yaml:"computeGallery"`
						} `yaml:"image"`
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
			vnetPrefix := netip.MustParsePrefix(vnet.CIDRs[0])
			fixturePrefix := netip.MustParsePrefix("10.250.0.0/24")
			if vnetPrefix.Overlaps(fixturePrefix) {
				t.Fatal("workload and datastore VNets must not overlap")
			}
			expectedSubnets := map[string]string{"control-plane": "10.0.0.0/24", "node": "10.0.1.0/24"}
			subnets := object.Spec.NetworkSpec.Subnets
			if len(subnets) != len(expectedSubnets) {
				t.Fatalf("expected control-plane and node subnets: %+v", subnets)
			}
			var subnetPrefixes []netip.Prefix
			for _, subnet := range subnets {
				want, ok := expectedSubnets[subnet.Role]
				if !ok || subnet.Name != subnet.Role+"-subnet" || len(subnet.CIDRs) != 1 || subnet.CIDRs[0] != want {
					t.Fatalf("unexpected subnet configuration: %+v", subnet)
				}
				delete(expectedSubnets, subnet.Role)
				prefix := netip.MustParsePrefix(subnet.CIDRs[0])
				if prefix.Bits() < vnetPrefix.Bits() || !vnetPrefix.Contains(prefix.Addr()) {
					t.Fatalf("subnet %s is outside workload VNet %s", prefix, vnetPrefix)
				}
				for _, other := range subnetPrefixes {
					if prefix.Overlaps(other) {
						t.Fatalf("subnets %s and %s overlap", prefix, other)
					}
				}
				subnetPrefixes = append(subnetPrefixes, prefix)
			}
		case "AzureMachineTemplate":
			if object.Metadata.Name == "${CLUSTER_NAME}-control-plane" {
				found["machine"] = true
				if len(object.Spec.Template.Spec.DataDisks) != 0 {
					t.Fatal("control plane must not allocate an unused etcd data disk")
				}
				image := object.Spec.Template.Spec.Image
				if image == nil || image.ComputeGallery.Gallery != "ClusterAPI-f72ceb4f-5159-4c26-a0fe-2ea738f0d019" ||
					image.ComputeGallery.Name != "capi-azurelinux-4-gen2" ||
					image.ComputeGallery.Version != "${KINE_CONTROL_PLANE_IMAGE_VERSION}" {
					t.Fatalf("control plane must use the Kubernetes-matched Azure Linux Gen2 community image: %+v", image)
				}
			} else if object.Metadata.Name == "${CLUSTER_NAME}-md-0" && object.Spec.Template.Spec.Image != nil {
				t.Fatal("workers must retain the default Ubuntu reference image")
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
unset KINE_POSTGRES_MAX_CONNECTIONS KINE_DATASTORE_MAX_OPEN_CONNECTIONS KINE_DATASTORE_MAX_IDLE_CONNECTIONS
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
	scripts := map[string]string{
		"/opt/configure-kine-storage.sh": "configure-performance-storage.sh",
		"/opt/kine-connections.sh":       "connections.sh",
	}
	for _, file := range config.Files {
		if script, ok := scripts[file.Path]; ok {
			decoded, err := base64.StdEncoding.DecodeString(file.Content)
			if err != nil || !bytes.Equal(decoded, readFile(t, filepath.Join(root, "scripts", "kine", script))) {
				t.Fatalf("conformance must use the shared storage and connection scripts: %s", file.Path)
			}
			delete(scripts, file.Path)
			continue
		}
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
	if len(scripts) != 0 {
		t.Fatalf("missing shared scripts: %v", scripts)
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
TIMESTAMP=2026-01-01T00:00:00Z
JOB_NAME=test-kine-job
BUILD_PROVENANCE=https://github.com/test/repo/pull/123/commits/test-head
run_id=test-run
message=$KINE_TEST_MESSAGE
unset KINE_VM_SIZE KINE_ZONE KINE_DISK_SIZE_GB KINE_DISK_IOPS KINE_DISK_MBPS
unset KINE_POSTGRES_MAX_CONNECTIONS KINE_DATASTORE_MAX_OPEN_CONNECTIONS KINE_DATASTORE_MAX_IDLE_CONNECTIONS
capz::kine::performance_defaults
az() {
  printf '%s\n' "$*" >> "$state_dir/commands"
  case "$1 $2" in
    "vm show") echo '{"controller":"NVMe","dataDisks":[{"lun":0,"caching":"None"}]}' ;;
    "disk show") echo '{"sku":{"name":"UltraSSD_LRS"},"zones":["2"],"diskSizeGB":512,"diskIOPSReadWrite":400000,"diskMBpsReadWrite":10000}' ;;
    "vm run-command") printf '{"value":[{"message":"%s"}]}\n' "$message" ;;
  esac
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
			groupCreate := strings.SplitN(commands, "\n", 2)[0]
			for _, tag := range []string{
				"capz-kine-run=test-run",
				"creationTimestamp=2026-01-01T00:00:00Z",
				"jobName=test-kine-job",
				"buildProvenance=https://github.com/test/repo/pull/123/commits/test-head",
			} {
				if !strings.HasPrefix(groupCreate, "group create ") || !strings.Contains(groupCreate, tag) {
					t.Errorf("missing resource-group tag %q: %s", tag, groupCreate)
				}
			}
			for _, expected := range []string{
				"--source-address-prefixes 10.0.0.0/24",
				"--destination-port-ranges 2379",
				"network nat gateway create",
				"--private-ip-address 10.250.0.4",
				"--nics kine-nic",
				"--subscription test-subscription --only-show-errors",
				"--size Standard_E112ibs_v5",
				"--sku UltraSSD_LRS",
				"--disk-controller-type NVMe",
				"--disk-iops-read-write 400000",
				"--disk-mbps-read-write 10000",
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

func TestCITagInitialization(t *testing.T) {
	for _, tc := range []struct {
		name       string
		job        string
		provenance string
	}{
		{"local", "cluster-api-provider-azure-e2e", "canary"},
		{"prow", "test-kine-job", "https://github.com/test/repo/pull/123/commits/test-head"},
		{"overrides", "custom-job", "canary"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			cmd := exec.CommandContext(t.Context(), "bash", "-c", `
source ./scripts/ci-kine-conformance.sh
export AZURE_SUBSCRIPTION_ID=test-subscription AZURE_LOCATION=test-region
export ARTIFACTS="$KINE_TEST_DIR" CLUSTER_NAME=capz-kine-unit-test
export WINDOWS=false IP_FAMILY=ipv4 TEST_K8S=false E2E_ARGS=
export CONFORMANCE_FLAVOR=kine-postgres KUBERNETES_VERSION=v1.35.4
unset KINE_KUBERNETES_VERSION
unset TIMESTAMP JOB_NAME BUILD_PROVENANCE REPO_OWNER REPO_NAME PULL_BASE_SHA PULL_NUMBER PULL_PULL_SHA
capz::kine::performance_preflight() { :; }
case "$KINE_TEST_TAG_CASE" in
  prow)
    export JOB_NAME=test-kine-job REPO_OWNER=test REPO_NAME=repo
    export PULL_BASE_SHA=test-base PULL_NUMBER=123 PULL_PULL_SHA=test-head ;;
  overrides)
    export TIMESTAMP=2026-01-01T00:00:00Z JOB_NAME=custom-job ;;
esac
az() {
  if [[ "$*" != "group exists"* ]]; then
    echo "Unexpected Azure mutation in an offline test" >&2
    return 99
  fi
  command bash -c 'printf "%s\n" "${TIMESTAMP:?}" "${JOB_NAME:?}" "${BUILD_PROVENANCE:?}"' > "$KINE_TEST_DIR/tags"
  echo true
}
capz::kine::main
`)
			cmd.Dir = repoRoot(t)
			cmd.Env = append(cmd.Environ(), "KINE_TEST_DIR="+dir, "KINE_TEST_TAG_CASE="+tc.name)
			output, err := cmd.CombinedOutput()
			if err == nil || !bytes.Contains(output, []byte("Refusing to reuse resource group")) {
				t.Fatalf("expected to stop before provisioning: %v\n%s", err, output)
			}
			tags := strings.Split(strings.TrimSpace(string(readFile(t, filepath.Join(dir, "tags")))), "\n")
			if len(tags) != 3 {
				t.Fatalf("expected exported timestamp, job name, and provenance: %v", tags)
			}
			timestamp, err := time.Parse(time.RFC3339, tags[0])
			if err != nil {
				t.Fatalf("invalid creation timestamp: %v", err)
			}
			if tc.name == "overrides" {
				if tags[0] != "2026-01-01T00:00:00Z" {
					t.Fatalf("timestamp override was not preserved: %s", tags[0])
				}
			} else if age := time.Since(timestamp); age < -time.Minute || age > time.Minute {
				t.Fatalf("default timestamp must reflect this run: %s", tags[0])
			}
			if tags[1] != tc.job || tags[2] != tc.provenance {
				t.Fatalf("unexpected CI tags: %v", tags)
			}
		})
	}
}

func TestRegionSelection(t *testing.T) {
	for _, state := range []string{"unset", "empty", "explicit", "performance", "machine-overrides", "kubernetes-override"} {
		t.Run(state, func(t *testing.T) {
			dir := t.TempDir()
			cmd := exec.CommandContext(t.Context(), "bash", "-c", `
source ./scripts/ci-kine-conformance.sh
export AZURE_SUBSCRIPTION_ID=test-subscription
export ARTIFACTS="$KINE_TEST_DIR" TMPDIR="$KINE_TEST_DIR" CLUSTER_NAME=capz-kine-unit-test
export WINDOWS=false IP_FAMILY=ipv4 TEST_K8S=false E2E_ARGS=
export CONFORMANCE_FLAVOR=kine-postgres KUBERNETES_VERSION=v1.35.4
unset KINE_PROFILE KINE_PREFLIGHT_ONLY KINE_ZONE KINE_VM_SIZE KINE_DISK_SIZE_GB KINE_DISK_IOPS KINE_DISK_MBPS
unset KINE_KUBERNETES_VERSION KINE_CONTROL_PLANE_IMAGE_VERSION
unset KINE_POSTGRES_MAX_CONNECTIONS KINE_DATASTORE_MAX_OPEN_CONNECTIONS KINE_DATASTORE_MAX_IDLE_CONNECTIONS
unset AZURE_CONTROL_PLANE_MACHINE_TYPE AZURE_NODE_MACHINE_TYPE
case "$KINE_TEST_LOCATION_STATE" in
  unset) unset AZURE_LOCATION ;;
  empty) export AZURE_LOCATION= KINE_ZONE= ;;
  explicit) export AZURE_LOCATION=test-region KINE_ZONE=3 ;;
  machine-overrides)
    unset AZURE_LOCATION
    export AZURE_CONTROL_PLANE_MACHINE_TYPE=Standard_D16s_v5 AZURE_NODE_MACHINE_TYPE=Standard_D8s_v5 ;;
  kubernetes-override)
    unset AZURE_LOCATION
    export KINE_KUBERNETES_VERSION=v1.35.3 ;;
  performance)
    unset AZURE_LOCATION
    export KINE_PROFILE=performance CONFORMANCE_FLAVOR=kine-postgres-performance ;;
esac
capz::kine::performance_preflight() {
  printf 'preflight=%s\n' "${AZURE_LOCATION:?}" >> "$KINE_TEST_DIR/regions"
}
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
  if [[ "$1" == "$repo_root/scripts/kine/run-clusterloader.sh" ]]; then
    [[ "$KINE_TEST_LOCATION_STATE" == performance && "$2" == --prepare-only ]] || return 99
    printf 'prepare=%s\n' "${AZURE_LOCATION:?}" >> "$KINE_TEST_DIR/regions"
    return 0
  fi
  if [[ "$1" != "$repo_root/scripts/ci-conformance.sh" ]]; then
    echo "Unexpected subprocess in an offline test" >&2
    return 99
  fi
  command bash -c 'printf "workload=%s\n" "${AZURE_LOCATION:?}"' >> "$KINE_TEST_DIR/regions"
  command bash -c 'printf "%s\n" "$KINE_VM_SIZE" "$KINE_DISK_IOPS" "$KINE_DISK_MBPS" "$KINE_ZONE" "$AZURE_CONTROL_PLANE_MACHINE_TYPE" "$AZURE_NODE_MACHINE_TYPE" "$CONFORMANCE_FLAVOR" "$KINE_ZONE_1" "$KINE_ZONE_2" "$KINE_ZONE_3" "$KINE_ZONE_YAML" "$KUBERNETES_VERSION" "$KINE_CONTROL_PLANE_IMAGE_VERSION"' > "$KINE_TEST_DIR/settings"
}
capz::kine::main
`)
			cmd.Dir = repoRoot(t)
			cmd.Env = append(cmd.Environ(), "KINE_TEST_DIR="+dir, "KINE_TEST_LOCATION_STATE="+state)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("run wrapper: %v\n%s", err, output)
			}
			lines := strings.Split(strings.TrimSpace(string(readFile(t, filepath.Join(dir, "regions")))), "\n")
			prefixes := []string{"preflight="}
			if state == "performance" {
				prefixes = append(prefixes, "prepare=")
			}
			prefixes = append(prefixes, "azure=", "azure=", "fixture=", "workload=")
			if len(lines) != len(prefixes) {
				t.Fatalf("unexpected preflight/provision/workload sequence: %v", lines)
			}
			region, zone, flavor := "westus2", "2", "kine-postgres"
			zones := []string{"false", "true", "false"}
			if state == "explicit" {
				region, zone = "test-region", "3"
				zones = []string{"false", "false", "true"}
			}
			if state == "performance" {
				flavor = "kine-postgres-performance"
			}
			controlPlane, worker := "Standard_D64s_v6", "Standard_D16s_v5"
			if state == "machine-overrides" {
				controlPlane, worker = "Standard_D16s_v5", "Standard_D8s_v5"
			}
			version := "v1.35.5"
			if state == "kubernetes-override" {
				version = "v1.35.3"
			}
			if !bytes.Contains(output, []byte("Kine overrides KUBERNETES_VERSION=v1.35.4 with KINE_KUBERNETES_VERSION="+version)) {
				t.Fatalf("must explicitly log the effective Kubernetes version instead of the Prow pin: %s", output)
			}
			for i, prefix := range prefixes {
				if lines[i] != prefix+region {
					t.Fatalf("region must be selected before Azure calls and shared with the workload: %v", lines)
				}
			}
			settings := string(readFile(t, filepath.Join(dir, "settings")))
			want := "Standard_E112ibs_v5\n400000\n10000\n" + zone + "\n" + controlPlane + "\n" + worker + "\n" + flavor + "\n" + strings.Join(zones, "\n") + "\n\"" + zone + "\"\n" + version + "\n" + strings.TrimPrefix(version, "v") + "\n"
			if settings != want {
				t.Fatalf("both workloads must inherit the same high-I/O defaults and preserve placement overrides:\n%s", settings)
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
		{"unpinned", "KINE_KUBERNETES_VERSION=stable-1.35", "Pin KINE_KUBERNETES_VERSION"},
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
unset KINE_KUBERNETES_VERSION
export "$KINE_TEST_SETTING"
capz::kine::performance_preflight() { :; }
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
