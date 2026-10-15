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

	"gopkg.in/yaml.v3"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
)

func TestPerformancePreflight(t *testing.T) {
	for _, tc := range []string{
		"success", "conformance-success", "conformance-quota", "family-quota", "regional-quota", "missing-quota", "restricted-zone",
		"missing-zone", "no-ultra", "no-nvme", "wrong-architecture", "no-acceleration",
		"invalid-disk", "invalid-zone", "wrong-image", "azure-error",
		"string-quota", "mixed-quota", "string-family-quota", "string-regional-quota", "invalid-quota",
	} {
		t.Run(tc, func(t *testing.T) {
			dir := t.TempDir()
			location, advertisedZone := "test-region", "1"
			if tc == "conformance-success" {
				location, advertisedZone = "westus2", "2"
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
				zone, ultra := advertisedZone, "True"
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
			case "string-quota", "mixed-quota", "string-family-quota", "string-regional-quota", "invalid-quota":
				quota = `[{"name":{"value":"standardEIBSv5Family"},"limit":"120","currentValue":"8"},
{"name":{"value":"standardDSv5Family"},"limit":"72","currentValue":"8"},
{"name":{"value":"cores"},"limit":"184","currentValue":"8"}]`
				switch tc {
				case "mixed-quota":
					quota = strings.NewReplacer(`"limit":"120"`, `"limit":120`, `"currentValue":"8"`, `"currentValue":8`).Replace(quota)
				case "string-family-quota":
					quota = strings.Replace(quota, `"limit":"72"`, `"limit":"71"`, 1)
				case "string-regional-quota":
					quota = strings.Replace(quota, `"limit":"184"`, `"limit":"183"`, 1)
				case "invalid-quota":
					quota = strings.Replace(quota, `"limit":"120"`, `"limit":"invalid"`, 1)
				}
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
			success := tc == "success" || tc == "conformance-success" || tc == "string-quota" || tc == "mixed-quota"
			if (err == nil) != success {
				t.Fatalf("unexpected result: %v\n%s", err, output)
			}
			if (tc == "family-quota" || tc == "conformance-quota" || tc == "string-family-quota") && !bytes.Contains(output, []byte("need 64, available 63")) {
				t.Fatalf("must aggregate control-plane and worker family quota: %s", output)
			}
			if (tc == "regional-quota" || tc == "string-regional-quota") && !bytes.Contains(output, []byte("need 176, available 175")) {
				t.Fatalf("must aggregate regional quota: %s", output)
			}
			if success {
				var plan struct {
					Quota []struct {
						Required  int `json:"required"`
						Available int `json:"available"`
					} `json:"quota"`
				}
				if err := json.Unmarshal(readFile(t, filepath.Join(dir, "kine-capz-perf-test", "capacity-plan.json")), &plan); err != nil {
					t.Fatalf("capacity plan must contain numeric quota values: %v", err)
				}
				if len(plan.Quota) != 3 {
					t.Fatalf("expected two family quotas and regional quota: %+v", plan.Quota)
				}
				for _, quota := range plan.Quota {
					if quota.Available != quota.Required {
						t.Fatalf("expected exactly enough available quota: %+v", quota)
					}
				}
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
	for _, mismatch := range []string{"none", "disk", "disk-throughput", "disk-sku", "null-iops", "disk-api-error", "controller"} {
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
      sku=UltraSSD_LRS iops=400000 mbps=10000
      case "$KINE_TEST_MISMATCH" in
        disk) iops=100 ;;
        disk-throughput) mbps=100 ;;
        disk-sku) sku=Premium_LRS ;;
        null-iops) iops=null ;;
        disk-api-error) echo "Azure disk lookup failed" >&2; return 17 ;;
      esac
      printf '{"sku":{"name":"%s"},"zones":["1"],"diskSizeGB":512,"diskIOPSReadWrite":%s,"diskMBpsReadWrite":%s}\n' "$sku" "$iops" "$mbps" ;;
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
			observed, diskMismatch := map[string]string{
				"disk":            `{"sku":"UltraSSD_LRS","iops":100,"mbps":10000}`,
				"disk-throughput": `{"sku":"UltraSSD_LRS","iops":400000,"mbps":100}`,
				"disk-sku":        `{"sku":"Premium_LRS","iops":400000,"mbps":10000}`,
				"null-iops":       `{"sku":"UltraSSD_LRS","iops":null,"mbps":10000}`,
			}[mismatch]
			if diskMismatch && (!bytes.Contains(output, []byte("Expected: sku=UltraSSD_LRS, iops=400000, mbps=10000")) ||
				!bytes.Contains(output, []byte("Observed: "+observed))) {
				t.Fatalf("disk mismatch must report expected and observed settings: %s", output)
			}
			if mismatch == "none" {
				var disk struct {
					Size int `json:"size"`
				}
				if err := json.Unmarshal(readFile(t, filepath.Join(dir, "disk-configuration.json")), &disk); err != nil {
					t.Fatal(err)
				}
				if disk.Size != 512 {
					t.Fatalf("must project diskSizeGB from the Azure response: %+v", disk)
				}
			}
			commands := string(readFile(t, filepath.Join(dir, "commands")))
			if mismatch != "none" && strings.Contains(commands, "vm run-command") {
				t.Fatalf("must stop before guest readiness when provisioning validation fails: %s", commands)
			}
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

func TestPerformanceFailureDomain(t *testing.T) {
	root := repoRoot(t)
	for _, zone := range []string{"1", "2", "3"} {
		t.Run(zone, func(t *testing.T) {
			cmd := exec.CommandContext(t.Context(), "bash", "-c", `
source ./scripts/kine/performance.sh
unset KINE_DISK_SIZE_GB KINE_DISK_IOPS KINE_DISK_MBPS
unset CONFORMANCE_CONTROL_PLANE_MACHINE_COUNT CONFORMANCE_WORKER_MACHINE_COUNT
capz::kine::performance_defaults
printf '%s' "$KINE_ZONE_YAML"
`)
			cmd.Dir = root
			cmd.Env = append(cmd.Environ(), "KINE_ZONE="+zone)
			zoneYAML, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("derive YAML zone: %v\n%s", err, zoneYAML)
			}
			for _, flavor := range []string{"kine-postgres", "kine-postgres-performance"} {
				t.Run(flavor, func(t *testing.T) {
					template := readFile(t, filepath.Join(root, "templates", "test", "ci", "cluster-template-prow-"+flavor+".yaml"))
					rendered := strings.NewReplacer("${KINE_ZONE}", zone, "${KINE_ZONE_YAML}", string(zoneYAML)).Replace(string(template))
					decoder := yamlutil.NewYAMLOrJSONDecoder(strings.NewReader(rendered), 4096)
					found := 0
					for {
						var raw json.RawMessage
						var object struct {
							Kind string `json:"kind"`
							Spec struct {
								Template struct {
									Spec struct {
										FailureDomain string `json:"failureDomain"`
									} `json:"spec"`
								} `json:"template"`
							} `json:"spec"`
						}
						if err := decoder.Decode(&raw); errors.Is(err, io.EOF) {
							break
						} else if err != nil {
							t.Fatalf("decode substituted template through Kubernetes YAML/JSON conversion: %v", err)
						}
						if err := json.Unmarshal(raw, &object); err != nil {
							t.Fatalf("decode Kubernetes JSON into typed fields: %v", err)
						}
						if object.Kind == "MachineDeployment" {
							found++
							if object.Spec.Template.Spec.FailureDomain != zone {
								t.Fatalf("expected string failureDomain %q, got %q", zone, object.Spec.Template.Spec.FailureDomain)
							}
						}
					}
					if found != 1 {
						t.Fatalf("expected one MachineDeployment, got %d", found)
					}
				})
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
		"failureDomain: ${KINE_ZONE_YAML}", "acceleratedNetworking: true", "storageAccountType: Premium_LRS",
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

func TestClusterLoaderGeneratorMonitor(t *testing.T) {
	root := repoRoot(t)
	var monitor struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
		Metadata   struct {
			Name      string `yaml:"name"`
			Namespace string `yaml:"namespace"`
		} `yaml:"metadata"`
		Spec struct {
			Selector struct {
				MatchLabels map[string]string `yaml:"matchLabels"`
			} `yaml:"selector"`
			NamespaceSelector struct {
				MatchNames []string `yaml:"matchNames"`
			} `yaml:"namespaceSelector"`
			Endpoints []struct {
				Port              string `yaml:"port"`
				Scheme            string `yaml:"scheme"`
				Path              string `yaml:"path"`
				Interval          string `yaml:"interval"`
				HonorLabels       bool   `yaml:"honorLabels"`
				BearerTokenSecret struct {
					Name string `yaml:"name"`
					Key  string `yaml:"key"`
				} `yaml:"bearerTokenSecret"`
				Relabelings []struct {
					TargetLabel string `yaml:"targetLabel"`
					Replacement string `yaml:"replacement"`
				} `yaml:"relabelings"`
				MetricRelabelings []struct {
					SourceLabels []string `yaml:"sourceLabels"`
					Regex        string   `yaml:"regex"`
					Action       string   `yaml:"action"`
				} `yaml:"metricRelabelings"`
			} `yaml:"endpoints"`
		} `yaml:"spec"`
	}
	data := readFile(t, filepath.Join(root, "scripts", "kine", "clusterloader", "prometheus", "generators.yaml"))
	if err := yaml.Unmarshal(data, &monitor); err != nil {
		t.Fatal(err)
	}
	if monitor.APIVersion != "monitoring.coreos.com/v1" || monitor.Kind != "ServiceMonitor" ||
		monitor.Metadata.Name != "kine-load-generators" || monitor.Metadata.Namespace != "monitoring" {
		t.Fatalf("unexpected generator monitor identity: %+v", monitor.Metadata)
	}
	if monitor.Spec.Selector.MatchLabels["k8s-app"] != "kubelet" ||
		strings.Join(monitor.Spec.NamespaceSelector.MatchNames, ",") != "kube-system" || len(monitor.Spec.Endpoints) != 1 {
		t.Fatal("generator monitor must select only the existing kubelet service")
	}
	endpoint := monitor.Spec.Endpoints[0]
	if endpoint.Port != "https-metrics" || endpoint.Scheme != "https" || endpoint.Path != "/metrics/cadvisor" ||
		endpoint.Interval != "30s" || !endpoint.HonorLabels ||
		endpoint.BearerTokenSecret.Name != "prometheus-token" || endpoint.BearerTokenSecret.Key != "token" {
		t.Fatal("generator monitor must use authenticated cAdvisor scraping and preserve workload labels")
	}
	if len(endpoint.MetricRelabelings) != 2 {
		t.Fatal("expected namespace/container and metric allowlists")
	}
	if len(endpoint.Relabelings) != 1 || endpoint.Relabelings[0].TargetLabel != "job" ||
		endpoint.Relabelings[0].Replacement != "kine-load-generators" {
		t.Fatal("generator scrape targets must be distinguishable from the upstream kubelet targets")
	}
	var filters []*regexp.Regexp
	for _, rule := range endpoint.MetricRelabelings {
		if rule.Action != "keep" {
			t.Fatalf("expected keep allowlist, got %q", rule.Action)
		}
		filter, err := regexp.Compile("^(" + rule.Regex + ")$")
		if err != nil {
			t.Fatal(err)
		}
		filters = append(filters, filter)
	}
	for _, tc := range []struct {
		namespace, container, metric string
		keep                         bool
	}{
		{"test-run-1", "write-generator", "container_cpu_usage_seconds_total", true},
		{"test-run-1", "watch-generator", "container_cpu_usage_seconds_total", true},
		{"test-run-1", "write-generator", "container_cpu_cfs_periods_total", true},
		{"test-run-1", "watch-generator", "container_cpu_cfs_throttled_periods_total", true},
		{"test-run-1", "write-generator", "container_memory_working_set_bytes", true},
		{"test-run-1", "watch-generator", "container_memory_working_set_bytes", true},
		{"test-run-1", "nginx", "container_cpu_usage_seconds_total", false},
		{"test-run-1", "POD", "container_cpu_usage_seconds_total", false},
		{"test-run-1", "", "container_cpu_usage_seconds_total", false},
		{"kube-system", "kube-apiserver", "container_cpu_usage_seconds_total", false},
		{"monitoring", "prometheus", "container_memory_working_set_bytes", false},
		{"other-test-run-1", "write-generator", "container_cpu_usage_seconds_total", false},
		{"test-run-1", "write-generator", "container_network_receive_bytes_total", false},
	} {
		labels := map[string]string{"namespace": tc.namespace, "container": tc.container, "__name__": tc.metric}
		keep := true
		for i, rule := range endpoint.MetricRelabelings {
			var values []string
			for _, label := range rule.SourceLabels {
				value, ok := labels[label]
				if !ok {
					t.Fatalf("unexpected relabel source %q", label)
				}
				values = append(values, value)
			}
			keep = keep && filters[i].MatchString(strings.Join(values, ";"))
		}
		if keep != tc.keep {
			t.Errorf("unexpected scrape decision for %+v: keep=%t", tc, keep)
		}
	}
	runner := readFile(t, filepath.Join(root, "scripts", "kine", "run-clusterloader.sh"))
	if !bytes.Contains(runner, []byte(`--prometheus-additional-monitors-path="${repo_root}/scripts/kine/clusterloader/prometheus"`)) {
		t.Fatal("CL2 runner must install the generator monitor")
	}
}

func TestClusterLoaderGeneratorMeasurements(t *testing.T) {
	data := readFile(t, filepath.Join(repoRoot(t), "scripts", "kine", "clusterloader", "measurements.yaml"))
	for _, action := range []string{"start", "gather"} {
		t.Run(action, func(t *testing.T) {
			var config struct {
				Steps []struct {
					Measurements []struct {
						Identifier string `yaml:"Identifier"`
						Method     string `yaml:"Method"`
						Params     struct {
							Action           string   `yaml:"action"`
							EnableViolations bool     `yaml:"enableViolations"`
							MetricName       string   `yaml:"metricName"`
							MetricVersion    string   `yaml:"metricVersion"`
							Unit             string   `yaml:"unit"`
							Dimensions       []string `yaml:"dimensions"`
							Queries          []struct {
								Query          string   `yaml:"query"`
								Aggregations   []string `yaml:"aggregations"`
								RequireSamples bool     `yaml:"requireSamples"`
							} `yaml:"queries"`
						} `yaml:"Params"`
					} `yaml:"measurements"`
				} `yaml:"steps"`
			}
			if err := yaml.Unmarshal([]byte(strings.ReplaceAll(string(data), "{{.action}}", action)), &config); err != nil {
				t.Fatal(err)
			}
			expected := map[string]string{
				"LoadGeneratorCPU":           "cores",
				"LoadGeneratorCPUThrottling": "percent",
				"LoadGeneratorMemory":        "MiB",
			}
			for _, step := range config.Steps {
				for _, m := range step.Measurements {
					if !strings.HasPrefix(m.Identifier, "LoadGenerator") {
						continue
					}
					unit, ok := expected[m.Identifier]
					if !ok {
						t.Fatalf("unexpected or duplicate generator measurement %q", m.Identifier)
					}
					p := m.Params
					if m.Method != "GenericPrometheusQuery" || p.Action != action || p.MetricName != m.Identifier ||
						p.MetricVersion != "v1" || p.Unit != unit || !p.EnableViolations ||
						strings.Join(p.Dimensions, ",") != "namespace,pod,container" || len(p.Queries) != 1 {
						t.Fatalf("invalid generator measurement %q: %+v", m.Identifier, p)
					}
					q := p.Queries[0]
					if !q.RequireSamples || strings.Join(q.Aggregations, ",") != "Avg,Max" ||
						!strings.Contains(q.Query, `namespace=~"test-.*",container=~"(write|watch)-generator"`) ||
						strings.Contains(q.Query, "or vector(0)") {
						t.Fatalf("generator query must be scoped and reject missing data: %+v", q)
					}
					delete(expected, m.Identifier)
				}
			}
			if len(expected) != 0 {
				t.Fatalf("missing generator measurements: %v", expected)
			}
		})
	}
}
