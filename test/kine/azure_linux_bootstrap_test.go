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
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client/yamlprocessor"
)

type azureLinuxBootstrap struct {
	Files []struct {
		Path    string `yaml:"path"`
		Content string `yaml:"content"`
	} `yaml:"files"`
	PreKubeadmCommands []string `yaml:"preKubeadmCommands"`
}

func processAzureLinuxTemplate(t *testing.T, data []byte, nodeCIDRs string) []byte {
	t.Helper()
	processor := yamlprocessor.NewSimpleProcessor()
	variables, err := processor.GetVariableMap(data)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := processor.Process(data, func(name string) (string, error) {
		if name == "AZURE_LINUX_NODE_CIDRS" && nodeCIDRs != "" {
			return nodeCIDRs, nil
		}
		if variables[name] == nil {
			return "test", nil
		}
		return "", fmt.Errorf("unset variable %s", name)
	})
	if err != nil {
		t.Fatalf("process Azure Linux template: %v", err)
	}
	return rendered
}

func TestAzureLinuxBootstrapInheritance(t *testing.T) {
	root := repoRoot(t)
	var sharedScript string
	for _, flavor := range []string{"prow-azl3", "prow-ci-version-azl3", "prow-kine-postgres", "prow-kine-postgres-performance", "prow-kubeadm-etcd", "prow-kubeadm-etcd-performance"} {
		t.Run(flavor, func(t *testing.T) {
			// #nosec G204 -- Template arguments come from the fixed flavor list above.
			cmd := exec.CommandContext(t.Context(), "./hack/tools/bin/kustomize", "build",
				"--load-restrictor", "LoadRestrictionsNone", filepath.Join("templates", "test", "ci", flavor))
			cmd.Dir = root
			rendered, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("render Azure Linux flavor: %v\n%s", err, rendered)
			}
			if !bytes.Equal(rendered, readFile(t, filepath.Join(root, "templates", "test", "ci", "cluster-template-"+flavor+".yaml"))) {
				t.Fatal("Azure Linux flavor is stale; run hack/gen-flavors.sh")
			}
			nodeCIDRs := ""
			expectedCIDRs := "10.0.0.0/24 10.1.0.0/24"
			if strings.Contains(flavor, "kine-postgres") {
				nodeCIDRs, expectedCIDRs = "10.0.0.0/24 10.0.1.0/24", "10.0.0.0/24 10.0.1.0/24"
			}
			processed := processAzureLinuxTemplate(t, rendered, nodeCIDRs)
			decoder := yaml.NewDecoder(bytes.NewReader(processed))
			roles := map[string]bool{}
			cloudProviders := 0
			healthChecks := 0
			for {
				var object struct {
					Kind     string `yaml:"kind"`
					Metadata struct {
						Name string `yaml:"name"`
					} `yaml:"metadata"`
					Spec struct {
						KubeadmConfigSpec azureLinuxBootstrap `yaml:"kubeadmConfigSpec"`
						Template          struct {
							Spec azureLinuxBootstrap `yaml:"spec"`
						} `yaml:"template"`
						ValuesTemplate string `yaml:"valuesTemplate"`
					} `yaml:"spec"`
				}
				if err := decoder.Decode(&object); errors.Is(err, io.EOF) {
					break
				} else if err != nil {
					t.Fatal(err)
				}
				var bootstrap azureLinuxBootstrap
				var role string
				switch object.Kind {
				case "KubeadmControlPlane":
					bootstrap, role = object.Spec.KubeadmConfigSpec, "control-plane"
				case "KubeadmConfigTemplate":
					bootstrap, role = object.Spec.Template.Spec, "worker"
				case "HelmChartProxy":
					if strings.HasPrefix(object.Metadata.Name, "cloud-provider-azure-chart") {
						cloudProviders++
						if !strings.Contains(object.Spec.ValuesTemplate, `caCertDir: "/etc/pki/tls/certs"`) {
							t.Fatal("Azure Linux cloud controllers must inherit the native CA certificate directory")
						}
					}
					continue
				case "MachineHealthCheck":
					healthChecks++
					continue
				default:
					continue
				}
				roles[role] = true
				found := false
				for _, file := range bootstrap.Files {
					if file.Path != "/opt/azure-linux-setup.sh" {
						continue
					}
					found = true
					if file.Content == "" {
						t.Fatalf("%s bootstrap script was not populated", role)
					}
					if sharedScript == "" {
						sharedScript = file.Content
					} else if file.Content != sharedScript {
						t.Fatalf("%s must inherit the same shared Azure Linux bootstrap, not a separate copy", role)
					}
				}
				if !found {
					t.Fatalf("%s is missing the Azure Linux bootstrap script", role)
				}
				if len(bootstrap.PreKubeadmCommands) == 0 {
					t.Fatalf("%s has no pre-kubeadm setup", role)
				}
				command := bootstrap.PreKubeadmCommands[0]
				want := "bash /opt/azure-linux-setup.sh " + role + " " + expectedCIDRs + " || exit 1"
				if command != want {
					t.Fatalf("bootstrap must run first with matching node subnets: got %q, want %q", command, want)
				}
				failedSetup := filepath.Join(t.TempDir(), "failed-setup.sh")
				if err := os.WriteFile(failedSetup, []byte("echo SETUP_FAILED >&2\nexit 42\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				caller := strings.Replace(command, "/opt/azure-linux-setup.sh", `"$KINE_TEST_SETUP_SCRIPT"`, 1)
				// #nosec G204 -- The command is matched exactly above; the substituted fixture path is quoted.
				cmd = exec.CommandContext(t.Context(), "sh", "-c", caller+"\necho KUBEADM_STARTED\n")
				cmd.Env = append(cmd.Environ(), "KINE_TEST_SETUP_SCRIPT="+failedSetup)
				output, err := cmd.CombinedOutput()
				if err == nil || !bytes.Contains(output, []byte("SETUP_FAILED")) || bytes.Contains(output, []byte("KUBEADM_STARTED")) {
					t.Fatalf("%s must stop bootstrap on setup failure even without parent errexit: %v\n%s", role, err, output)
				}
			}
			if !roles["control-plane"] || !roles["worker"] || cloudProviders != 2 {
				t.Fatal("both node roles and both cloud-provider variants must inherit Azure Linux setup")
			}
			if strings.Contains(flavor, "kine-postgres") && healthChecks != 2 {
				t.Fatal("Kine must retain its control-plane and worker health checks")
			}
		})
	}
}

func TestAzureLinuxBootstrapFirewall(t *testing.T) {
	var patches []struct {
		Value yaml.Node `yaml:"value"`
	}
	if err := yaml.Unmarshal(readFile(t, filepath.Join(repoRoot(t), "templates", "test", "ci", "prow-azl3", "patches", "controller-manager.yaml")), &patches); err != nil {
		t.Fatal(err)
	}
	if len(patches) == 0 {
		t.Fatal("missing shared bootstrap patch")
	}
	var file struct {
		Content string `yaml:"content"`
	}
	if err := patches[0].Value.Decode(&file); err != nil {
		t.Fatal(err)
	}
	script := processAzureLinuxTemplate(t, []byte(file.Content), "")
	for _, tc := range []struct {
		name, version, backend, role, packages, fail string
	}{
		{"gen2-control-plane", "4.0", "firewalld", "control-plane", "missing", ""},
		{"gen2-worker", "4.0", "firewalld", "worker", "missing", ""},
		{"legacy-control-plane", "3.0", "iptables", "control-plane", "missing", ""},
		{"legacy-worker", "3.0", "iptables", "worker", "missing", ""},
		{"installed-prerequisites", "4.0", "firewalld", "worker", "installed", ""},
		{"package-recovery", "4.0", "firewalld", "worker", "retry", ""},
		{"legacy-package-recovery", "3.0", "iptables", "worker", "retry", ""},
		{"firewall-error", "4.0", "firewalld", "control-plane", "installed", "firewall"},
		{"package-error", "4.0", "firewalld", "control-plane", "missing", "packages"},
		{"metadata-error", "4.0", "firewalld", "worker", "missing", "metadata"},
		{"legacy-rule-error", "3.0", "iptables", "worker", "missing", "iptables"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			scriptPath := filepath.Join(dir, "setup.sh")
			safeScript := bytes.ReplaceAll(script, []byte("/etc/systemd/scripts/ip4save"), []byte(`"${KINE_TEST_DIR}/ip4save"`))
			if err := os.WriteFile(scriptPath, safeScript, 0o600); err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(t.Context(), "bash", "-c", `
set -euo pipefail
record() { printf '%s\n' "$*" >> "$KINE_TEST_DIR/commands"; }
source() {
  if [[ "$1" == /etc/os-release ]]; then
    VERSION_ID=$KINE_TEST_OS
  else
    builtin source "$@"
  fi
}
command() {
  if [[ "$1 $2" == "-v firewall-cmd" && "$KINE_TEST_BACKEND" == iptables ]]; then
    return 1
  fi
  builtin command "$@"
}
rpm() { record rpm "$@"; [[ "$KINE_TEST_PACKAGES" == installed ]]; }
install_attempts=0
package_operation() {
  if [[ "$1" == clean ]]; then
    [[ "$KINE_TEST_FAIL" != metadata ]]
    return
  fi
  install_attempts=$((install_attempts + 1))
  [[ "$KINE_TEST_FAIL" != packages && "$KINE_TEST_FAIL" != metadata ]] || return 1
  [[ "$KINE_TEST_PACKAGES" != retry || "$install_attempts" -ge 3 ]]
}
tdnf() { record tdnf "$@"; package_operation "$@"; }
dnf() { record dnf "$@"; package_operation "$@"; }
sleep() { record sleep "$@"; }
update-ca-trust() { record update-ca-trust "$@"; }
systemctl() { record systemctl "$@"; }
firewall-cmd() { record firewall-cmd "$@"; [[ "$KINE_TEST_FAIL" != firewall ]]; }
iptables() {
  record iptables "$@"
  [[ "$2" != -C && "$KINE_TEST_FAIL" != iptables ]]
}
iptables-save() { record iptables-save; printf 'saved firewall\n'; }
source "$KINE_TEST_SCRIPT" "$KINE_TEST_ROLE" 10.0.0.0/24 10.0.1.0/24
echo SETUP_COMPLETE
`)
			cmd.Dir = repoRoot(t)
			cmd.Env = append(cmd.Environ(), "KINE_TEST_DIR="+dir, "KINE_TEST_SCRIPT="+scriptPath,
				"KINE_TEST_OS="+tc.version, "KINE_TEST_BACKEND="+tc.backend,
				"KINE_TEST_ROLE="+tc.role, "KINE_TEST_PACKAGES="+tc.packages, "KINE_TEST_FAIL="+tc.fail)
			output, err := cmd.CombinedOutput()
			if (err == nil) != (tc.fail == "") {
				t.Fatalf("unexpected bootstrap result: %v\n%s", err, output)
			}
			commands := string(readFile(t, filepath.Join(dir, "commands")))
			manager, packages := "dnf", "ca-certificates firewalld"
			if tc.version == "3.0" {
				manager, packages = "tdnf", "ca-certificates ca-certificates-legacy"
			}
			if !strings.Contains(commands, "rpm -q "+packages+"\n") {
				t.Fatal("check installed prerequisites before accessing package repositories")
			}
			wantInstalls, wantCleans, wantSleeps := 1, 0, 0
			switch {
			case tc.packages == "installed":
				wantInstalls = 0
			case tc.packages == "retry" || tc.fail == "packages":
				wantInstalls, wantCleans, wantSleeps = 3, 2, 2
			case tc.fail == "metadata":
				wantInstalls, wantCleans = 1, 1
			}
			if strings.Count(commands, manager+" install -y "+packages+"\n") != wantInstalls ||
				strings.Count(commands, manager+" clean metadata\n") != wantCleans ||
				strings.Count(commands, "sleep 5\n") != wantSleeps {
				t.Fatalf("unexpected package retry sequence:\n%s", commands)
			}
			if tc.fail == "packages" || tc.fail == "metadata" {
				if strings.Contains(commands, "update-ca-trust") || strings.Contains(commands, "systemctl") {
					t.Fatal("package failure must stop setup before certificate and firewall configuration")
				}
			}
			if tc.fail != "" {
				if bytes.Contains(output, []byte("SETUP_COMPLETE")) {
					t.Fatal("bootstrap must stop when package or firewall configuration fails")
				}
				return
			}
			if !strings.Contains(commands, "update-ca-trust\n") {
				t.Fatal("bootstrap must update the native CA trust store")
			}
			if tc.backend == "firewalld" {
				for _, expected := range []string{
					"systemctl enable --now firewalld",
					`firewall-cmd --permanent --add-rich-rule=rule family="ipv4" source address="168.63.129.16" accept`,
					"firewall-cmd --reload",
				} {
					if !strings.Contains(commands, expected) {
						t.Errorf("missing native firewall setup: %s", expected)
					}
				}
				for _, cidr := range []string{"10.0.0.0/24", "10.0.1.0/24", "192.168.0.0/16", "10.96.0.0/12"} {
					if !strings.Contains(commands, "firewall-cmd --permanent --zone=trusted --add-source="+cidr) {
						t.Errorf("missing cluster traffic allowance for %s", cidr)
					}
				}
				if strings.Contains(commands, "iptables") || strings.Contains(commands, "tdnf") ||
					strings.Contains(commands, "ca-certificates-legacy") {
					t.Fatal("Azure Linux 4 must not use legacy package or firewall persistence paths")
				}
				if strings.Contains(commands, "--add-port=6443/tcp") != (tc.role == "control-plane") {
					t.Fatal("only control-plane nodes should expose the API port")
				}
			} else {
				if !strings.Contains(commands, "tdnf install -y ca-certificates ca-certificates-legacy") ||
					!strings.Contains(commands, "iptables -w -I INPUT -s 168.63.129.16 -j ACCEPT") ||
					!strings.Contains(commands, "iptables -w -I FORWARD -s 192.168.0.0/16 -j ACCEPT") ||
					!strings.Contains(commands, "iptables -w -I INPUT -s 10.0.1.0/24 -j ACCEPT") {
					t.Fatal("legacy firewall must allow platform, node, and forwarded Pod traffic")
				}
				if strings.Contains(commands, "INPUT -p tcp --dport 6443") != (tc.role == "control-plane") {
					t.Fatal("legacy API port allowance must be control-plane only")
				}
				if !bytes.Contains(readFile(t, filepath.Join(dir, "ip4save")), []byte("saved firewall")) {
					t.Fatal("legacy firewall rules must be persisted")
				}
			}
		})
	}
}
