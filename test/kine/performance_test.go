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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPerformancePreflight(t *testing.T) {
	for _, tc := range []string{
		"success", "conformance-success", "conformance-quota", "family-quota", "regional-quota", "missing-quota", "restricted-zone",
		"missing-zone", "no-ultra", "no-nvme", "wrong-architecture", "no-acceleration",
		"invalid-disk", "invalid-zone", "wrong-image", "azure-error",
	} {
		t.Run(tc, func(t *testing.T) {
			dir := t.TempDir()
			location := "test-region"
			if tc == "conformance-success" {
				location = "westus2"
			}
			var skus []map[string]any
			for _, machine := range []struct{ size, family, cpu string }{
				{"Standard_E112ibs_v5", "standardEIBSv5Family", "112"},
				{"Standard_D32s_v5", "standardDSv5Family", "32"},
				{"Standard_D16s_v5", "standardDSv5Family", "16"},
			} {
				capabilities := map[string]string{
					"vCPUs": machine.cpu, "CpuArchitectureType": "x64", "AcceleratedNetworkingEnabled": "True", "DiskControllerTypes": "SCSI,NVMe",
				}
				switch tc {
				case "no-nvme":
					capabilities["DiskControllerTypes"] = "SCSI"
				case "wrong-architecture":
					capabilities["CpuArchitectureType"] = "Arm64"
				case "no-acceleration":
					capabilities["AcceleratedNetworkingEnabled"] = "False"
				}
				var caps []map[string]string
				for name, value := range capabilities {
					caps = append(caps, map[string]string{"name": name, "value": value})
				}
				zone, ultra := "1", "True"
				if tc == "missing-zone" {
					zone = "2"
				}
				if tc == "no-ultra" {
					ultra = "False"
				}
				sku := map[string]any{
					"name": machine.size, "family": machine.family, "resourceType": "virtualMachines",
					"capabilities": caps,
					"locationInfo": []any{map[string]any{
						"location": location, "zones": []string{zone},
						"zoneDetails": []any{map[string]any{
							"name": []string{zone}, "capabilities": []any{map[string]string{"name": "UltraSSDAvailable", "value": ultra}},
						}},
					}},
				}
				if tc == "restricted-zone" {
					sku["restrictions"] = []any{map[string]any{"type": "Zone", "restrictionInfo": map[string]any{"zones": []string{"1"}}}}
				}
				skus = append(skus, sku)
			}
			data, err := json.Marshal(skus)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "skus-input.json"), data, 0o600); err != nil {
				t.Fatal(err)
			}
			quota := `[{"name":{"value":"standardEIBSv5Family"},"limit":112,"currentValue":0},
{"name":{"value":"standardDSv5Family"},"limit":64,"currentValue":0},
{"name":{"value":"cores"},"limit":176,"currentValue":0}]`
			switch tc {
			case "family-quota", "conformance-quota":
				quota = strings.Replace(quota, `"limit":64`, `"limit":63`, 1)
			case "regional-quota":
				quota = strings.Replace(quota, `"limit":176`, `"limit":175`, 1)
			case "missing-quota":
				quota = `[]`
			}
			if err := os.WriteFile(filepath.Join(dir, "quota-input.json"), []byte(quota), 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(t.Context(), "bash", "-c", `
source ./scripts/ci-kine-conformance.sh
export KINE_PROFILE=performance KINE_PREFLIGHT_ONLY=true
export AZURE_LOCATION=test-region AZURE_SUBSCRIPTION_ID=test-subscription KINE_ZONE=1
export CLUSTER_NAME=capz-perf-test ARTIFACTS="$KINE_TEST_DIR"
export WINDOWS=false IP_FAMILY=ipv4 TEST_K8S=false E2E_ARGS= KUBERNETES_VERSION=v1.35.4
unset CONFORMANCE_FLAVOR KINE_VM_SIZE KINE_VM_IMAGE KINE_DISK_SIZE_GB KINE_DISK_IOPS KINE_DISK_MBPS
unset AZURE_CONTROL_PLANE_MACHINE_TYPE AZURE_NODE_MACHINE_TYPE CONFORMANCE_CONTROL_PLANE_MACHINE_COUNT CONFORMANCE_WORKER_MACHINE_COUNT
case "$KINE_TEST_CASE" in
  invalid-disk) export KINE_DISK_IOPS=400001 ;;
  invalid-zone) export KINE_ZONE=4 ;;
  conformance-success)
    unset KINE_PROFILE AZURE_LOCATION KINE_ZONE
    export CONFORMANCE_FLAVOR=kine-postgres ;;
  conformance-quota)
    unset KINE_PROFILE
    export CONFORMANCE_FLAVOR=kine-postgres ;;
esac
az() {
  printf '%s\n' "$*" >> "$KINE_TEST_DIR/commands"
  [[ "$KINE_TEST_CASE" != azure-error ]] || return 17
  case "$1 $2 $3" in
    "vm list-skus --location") cat "$KINE_TEST_DIR/skus-input.json" ;;
    "vm list-usage --location") cat "$KINE_TEST_DIR/quota-input.json" ;;
    "vm image show")
      if [[ "$KINE_TEST_CASE" == wrong-image ]]; then
        echo '{"hyperVGeneration":"V1"}'
      else
        echo '{"name":"24.04.20261001","architecture":"x64","hyperVGeneration":"V2","features":[{"name":"DiskControllerTypes","value":"SCSI,NVMe"}]}'
      fi ;;
    *) echo "Unexpected Azure mutation in preflight" >&2; return 99 ;;
  esac
}
capz::kine::main
`)
			cmd.Dir = repoRoot(t)
			cmd.Env = append(cmd.Environ(), "KINE_TEST_DIR="+dir, "KINE_TEST_CASE="+tc)
			output, err := cmd.CombinedOutput()
			success := tc == "success" || tc == "conformance-success"
			if (err == nil) != success {
				t.Fatalf("unexpected result: %v\n%s", err, output)
			}
			if (tc == "family-quota" || tc == "conformance-quota") && !bytes.Contains(output, []byte("need 64, available 63")) {
				t.Fatalf("must aggregate control-plane and worker family quota: %s", output)
			}
			if success {
				settings := readFile(t, filepath.Join(dir, "kine-capz-perf-test", "performance-settings.txt"))
				if !bytes.Contains(settings, []byte("Canonical:ubuntu-24_04-lts:server:24.04.20261001")) ||
					!bytes.Contains(settings, []byte("disk-iops=400000")) {
					t.Fatalf("missing pinned image or disk settings: %s", settings)
				}
			}
			if bytes.Contains(output, []byte("Unexpected Azure mutation")) {
				t.Fatalf("preflight attempted to mutate Azure: %s", output)
			}
		})
	}
}

func TestPerformanceProvision(t *testing.T) {
	for _, mismatch := range []string{"none", "disk", "controller"} {
		t.Run(mismatch, func(t *testing.T) {
			dir := t.TempDir()
			cmd := exec.CommandContext(t.Context(), "bash", "-c", `
source ./scripts/ci-kine-conformance.sh
source ./scripts/kine/performance.sh
export AZURE_LOCATION=test-region KINE_ZONE=1 KINE_PROFILE=performance
unset KINE_VM_SIZE KINE_DISK_SIZE_GB KINE_DISK_IOPS KINE_DISK_MBPS
capz::kine::performance_defaults
artifact_dir=$KINE_TEST_DIR
state_dir=$KINE_TEST_DIR
KINE_RESOURCE_GROUP=test-kine
AZURE_SUBSCRIPTION_ID=test-subscription
TIMESTAMP=2026-01-01T00:00:00Z JOB_NAME=test BUILD_PROVENANCE=canary run_id=test
az() {
  printf '%s\n' "$*" >> "$KINE_TEST_DIR/commands"
  case "$1 $2" in
    "vm show")
      controller=NVMe
      [[ "$KINE_TEST_MISMATCH" != controller ]] || controller=SCSI
      printf '{"controller":"%s","dataDisks":[{"lun":0,"caching":"None"}]}\n' "$controller" ;;
    "disk show")
      iops=400000
      [[ "$KINE_TEST_MISMATCH" != disk ]] || iops=100
      printf '{"sku":"UltraSSD_LRS","iops":%s,"mbps":10000}\n' "$iops" ;;
    "vm run-command") echo '{"value":[{"message":"KINE_READY"}]}' ;;
  esac
}
capz::kine::provision
`)
			cmd.Dir = repoRoot(t)
			cmd.Env = append(cmd.Environ(), "KINE_TEST_DIR="+dir, "KINE_TEST_MISMATCH="+mismatch)
			output, err := cmd.CombinedOutput()
			if (err == nil) != (mismatch == "none") {
				t.Fatalf("unexpected provision result: %v\n%s", err, output)
			}
			commands := string(readFile(t, filepath.Join(dir, "commands")))
			for _, required := range []string{
				"--sku UltraSSD_LRS", "--disk-iops-read-write 400000", "--disk-mbps-read-write 10000",
				"--disk-controller-type NVMe", "--ultra-ssd-enabled true", "--attach-data-disks kine-data",
				"--data-disk-caching None", "--accelerated-networking true", "--zone 1",
				"--size Standard_E112ibs_v5", "--storage-sku Premium_LRS",
			} {
				if !strings.Contains(commands, required) {
					t.Errorf("missing %s", required)
				}
			}
		})
	}
}

func TestPerformanceFlavorAndStorage(t *testing.T) {
	root, state := repoRoot(t), t.TempDir()
	cmd := exec.CommandContext(t.Context(), "./hack/tools/bin/kustomize", "build",
		"--load-restrictor", "LoadRestrictionsNone", "./templates/test/ci/prow-kine-postgres-performance")
	cmd.Dir = root
	rendered, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("render performance flavor: %v\n%s", err, rendered)
	}
	if !bytes.Equal(rendered, readFile(t, filepath.Join(root, "templates", "test", "ci", "cluster-template-prow-kine-postgres-performance.yaml"))) {
		t.Fatal("performance flavor is stale; run hack/gen-flavors.sh")
	}
	if !bytes.Equal(rendered, readFile(t, filepath.Join(root, "templates", "test", "ci", "cluster-template-prow-kine-postgres.yaml"))) {
		t.Fatal("conformance and performance must use identical infrastructure templates")
	}
	for _, expected := range []string{
		"controlPlane: ${KINE_ZONE_1}", "controlPlane: ${KINE_ZONE_2}", "controlPlane: ${KINE_ZONE_3}",
		"failureDomain: ${KINE_ZONE}", "acceleratedNetworking: true", "storageAccountType: Premium_LRS",
		"https://10.250.0.4:2379",
	} {
		if !bytes.Contains(rendered, []byte(expected)) {
			t.Errorf("missing performance template invariant: %s", expected)
		}
	}
	cmd = exec.CommandContext(t.Context(), "bash", "-c", `
source ./scripts/ci-kine-conformance.sh
repo_root=$PWD
state_dir=$KINE_TEST_DIR
KINE_PROFILE=performance
umask 077
capz::kine::certificates
`)
	cmd.Dir = root
	cmd.Env = append(cmd.Environ(), "KINE_TEST_DIR="+state)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("render performance cloud-init: %v\n%s", err, output)
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
	found := false
	for _, file := range config.Files {
		if file.Path != "/opt/configure-kine-storage.sh" {
			continue
		}
		found = true
		decoded, err := base64.StdEncoding.DecodeString(file.Content)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(decoded, readFile(t, filepath.Join(root, "scripts", "kine", "configure-performance-storage.sh"))) {
			t.Fatal("performance cloud-init must install the exact storage initializer")
		}
		for _, required := range []string{"RequiresMountsFor=", "fsync = on", "synchronous_commit = on", "full_page_writes = on", "refusing OS-disk fallback"} {
			if !bytes.Contains(decoded, []byte(required)) {
				t.Errorf("missing durability invariant: %s", required)
			}
		}
		cmd = exec.CommandContext(t.Context(), "bash", "-n")
		cmd.Stdin = bytes.NewReader(decoded)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("invalid storage script: %v\n%s", err, output)
		}
	}
	if !found {
		t.Fatal("missing performance storage initializer")
	}
}
