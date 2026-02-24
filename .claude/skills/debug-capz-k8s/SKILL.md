---
name: debug-capz-k8s
description: Debug CAPZ (Cluster API Provider Azure) Kubernetes cluster failures. Covers live cluster inspection via kubectl, VM-level debugging via az CLI, Prow/GCS artifact analysis, and build log triage. Knows CAPZ template flavors, E2E test structure, addon deployment (Calico, cloud-provider-azure, CSI), common failure patterns, and transient errors to ignore.
---

# Debug CAPZ Kubernetes Clusters

You are debugging failures in CAPZ (Cluster API Provider Azure) clusters. CAPZ
uses Cluster API to provision Kubernetes clusters on Azure, with various flavors
for different configurations (standard, Windows, GPU, AKS, IPv6, etc.).

## Safety Rules

This skill is for **diagnosis and analysis only**. Follow these rules strictly:

### Never do without explicit user permission

- **Git:** Do not run `git commit`, `git push`, `git merge`, `git rebase`,
  `git reset`, or `git tag`. Read-only git commands (`git status`, `git diff`,
  `git log`, `git branch`) are fine.
- **Azure CLI:** Do not run any `az` commands that create, modify, or delete
  resources. Only use read-only commands: `az vm run-command invoke` (to inspect
  VMs), `az account show`, `az resource list`, `az vm list`. Never run
  `az vm delete`, `az group delete`, `az vm update`, or similar.
- **kubectl:** Do not modify or delete existing cluster resources (Machines,
  MachineDeployments, AzureMachines, HelmChartProxies, Nodes, etc.) without
  asking first. This includes `kubectl delete`, `kubectl patch`, or
  `kubectl edit` on existing resources.
- **File changes:** Do not modify test specs, template files, kustomization
  files, test scripts, or any other repo files without asking the user first.

### Okay to do without asking

- **Debug pods and ephemeral resources:** Creating temporary debug pods
  (`kubectl run`), ephemeral containers (`kubectl debug`), port-forwards, and
  watchers (`kubectl get -w`) is fine — these are non-destructive and help
  diagnosis.
- **kubectl apply for new debug resources:** You can `kubectl apply` new
  resources for debugging purposes (e.g. a test pod to check DNS or network
  connectivity) as long as you are not modifying existing cluster components.
- **kubectl exec and logs:** Running commands inside existing pods for
  inspection is fine.

### Cluster rescue mode

If the user explicitly asks you to fix, rescue, or recover their cluster (e.g.
"fix the cluster", "get the nodes running", "make it work"), you may use
mutating kubectl commands (`kubectl apply`, `kubectl edit`, `kubectl patch`,
`kubectl delete`) and mutating `az` commands as needed to resolve the issue.
Explain what you are doing and why as you go.

### Always do

- **Diagnose first, propose fixes second.** Identify the root cause, explain it,
  then suggest what to change and where — including the specific file paths and
  the nature of the fix.
- **Offer to make fixes.** After diagnosing an issue, tell the user which files
  need changes (e.g. the kustomize patch, the rendered cluster template, the
  test script, the `preKubeadmCommands` in a `KubeadmConfigTemplate`) and what
  the fix would be. Ask if they want you to apply it.
- **Show your reasoning.** When proposing a fix, explain why it addresses the
  root cause and what the expected behavior change is.

## Prerequisites

The following tools and credentials are needed depending on the debugging
approach. If a command fails due to missing auth or tools, let the user know
what's needed and how to set it up rather than retrying.

### For PR-based triage (gh CLI)

- **`gh` CLI** must be installed and authenticated with repo read access.
  If `gh pr checks` fails with an auth error, tell the user to run
  `gh auth login` or set `GITHUB_TOKEN` / `GH_TOKEN`.

### For live cluster debugging (kubectl)

- **`kubectl`** must be installed with kubeconfig pointing to the **management
  cluster** (the cluster running CAPI controllers).
- If `kubectl get clusters -A` fails with connection refused or unauthorized,
  the user likely needs to set their kubeconfig:
  - `export KUBECONFIG=/path/to/mgmt-kubeconfig.yaml`
  - Or for AKS: `az aks get-credentials --resource-group <rg> --name <aks-name>`
- **Workload cluster kubeconfig** is obtained from the management cluster (see
  Step 2g) and typically saved to `/tmp/workload-kubeconfig.yaml`. Commands
  against the workload cluster use `KUBECONFIG=/tmp/workload-kubeconfig.yaml`.

### For VM-level debugging (az CLI)

- **`az` CLI** must be installed and logged in to the Azure subscription that
  owns the cluster VMs. If `az vm run-command` fails with auth errors, the user
  needs to run `az login` and possibly `az account set --subscription <sub-id>`.

### For Prow artifact analysis (web fetch)

- No credentials needed — Prow artifacts for kubernetes-sigs repos are publicly
  accessible on GCS via `gcsweb.k8s.io`.

### Optional tools

- **`clusterctl`** — For `clusterctl get kubeconfig` and `clusterctl describe`.
  Not strictly required (kubectl equivalents exist) but convenient.
- **`crictl`** — Useful when running commands on a node via `az vm run-command`,
  for inspecting container runtime state.

## Architecture Overview

### Cluster API Resource Hierarchy

```
Cluster (top-level desired state)
├── AzureCluster (Azure infra: VNet, subnets, LB, NSGs, resource group)
├── KubeadmControlPlane (control plane machines + kubeadm config)
│   ├── Machine → AzureMachine (individual CP VM)
│   └── KubeadmConfig (per-node bootstrap config)
├── MachineDeployment (worker node group, one per OS/config)
│   ├── MachineSet
│   │   ├── Machine → AzureMachine (individual worker VM)
│   │   └── KubeadmConfig
├── MachinePool (optional, VMSS-based workers)
│   └── AzureMachinePool
├── HelmChartProxy (addon definitions, matched by cluster labels)
│   └── HelmReleaseProxy (per-cluster Helm release)
└── ClusterResourceSet (optional, for non-Helm addons)
```

### Addon Deployment (via CAAPH - Cluster API Addon Provider Helm)

Addons are deployed by HelmChartProxy resources that match cluster labels:

| Addon | Cluster Label | Purpose |
|---|---|---|
| Calico CNI | `cni: calico` | Pod networking |
| cloud-provider-azure | `cloud-provider: azure` | CCM + cloud-node-manager (auto-detect version) |
| cloud-provider-azure (CI) | `cloud-provider: azure-ci` | CCM + CNM with explicit image tags |
| azuredisk-csi-driver | `azuredisk-csi: true` | Azure Disk CSI |
| GPU operator | `gpu-operator: true` | NVIDIA GPU support |

### Control Plane Initialization Flow

1. First control plane Machine is created → AzureMachine provisions Azure VM
2. Cloud-init runs `kubeadm init` on the first CP node
3. `EnsureControlPlaneInitialized` waits for the API server to be reachable
4. CAAPH installs CNI (Calico) and cloud-provider-azure via Helm
5. Remaining CP nodes join via `kubeadm join`
6. Worker MachineDeployments scale up, workers join the cluster
7. cloud-controller-manager sets `.spec.providerID` on each Node
8. CAPI Machine transitions to `Running` once providerID is set

### Node Image Provisioning

- Base images come from the CAPZ community gallery:
  `ClusterAPI-f72ceb4f-5159-4c26-a0fe-2ea738f0d019`
- Common image definitions:
  - `capi-ubun2-2404` — Ubuntu 24.04
  - `capi-azurelinux-3` — Azure Linux 3
  - `capi-win-2019-containerd` / `capi-windows` — Windows
- Gallery image version determines the pre-installed kubelet/kubeadm version
- CI-version flavors download k8s binaries via `preKubeadmCommands` from `dl.k8s.io/ci/`

### CAPZ E2E Test Structure

**Three types of E2E tests:**

1. **CAPZ-specific tests** (`azure_test.go`) — Tests Azure-specific features:
   VM extensions, security groups, failure domains, load balancers, network
   policies, machine pool scaling, autoscaling, spot VMs, etc. Each test:
   - Calls `clusterctl.ApplyClusterTemplateAndWait()` with a specific flavor
   - Waits for cluster to be ready
   - Runs Azure-specific verification specs
   - Cleans up (deletes cluster, verifies Azure RG deletion)

2. **Upstream CAPI tests** (`capi_test.go`) — Runs standard CAPI E2E specs:
   QuickStart, MachineDeployment rollout, self-hosted, MachineHealthCheck
   remediation, scale, clusterctl upgrade.

3. **Conformance tests** (`conformance_test.go`) — Kubernetes conformance via
   kubetest. Selects flavor based on CI artifacts/IP family.

**CI entry points:**
- `scripts/ci-e2e.sh` → `make test-e2e` (CAPZ + CAPI tests, `GINKGO_NODES=10`)
- `scripts/ci-conformance.sh` → `make test-conformance` (conformance, `GINKGO_NODES=1`)
- `scripts/ci-entrypoint.sh` → General-purpose: creates cluster, runs arbitrary commands

### Template Flavors

35+ flavors under `templates/test/ci/prow-*/`, including:

| Category | Flavors |
|---|---|
| Standard | `prow` (base HA), `prow-custom-vnet`, `prow-spot`, `prow-private` |
| CI version | `prow-ci-version`, `prow-ci-version-dual-stack`, `prow-ci-version-ipv6` |
| OS variants | `prow-azl3` (Azure Linux 3), `prow-flatcar-sysext` (Flatcar) |
| Windows | `prow-windows`, `prow-ci-version-windows`, `prow-machine-pool-windows` |
| MachinePool | `prow-machine-pool`, `prow-machine-pool-flex`, `prow-machine-pool-ci-version` |
| AKS | `prow-aks`, `prow-aks-aso`, `prow-aks-clusterclass`, `prow-aks-topology` |
| ClusterClass | `prow-topology`, `prow-topology-rke2`, `prow-clusterclass-ci-default` |
| Networking | `prow-dual-stack`, `prow-ipv6`, `prow-azure-cni-v1` |
| Special | `prow-nvidia-gpu`, `prow-edgezone`, `prow-apiserver-ilb` |
| Custom builds | `prow-dalec-custom-builds` (dalec k8s images) |

Each flavor is built via kustomize from a base template + patches. The rendered
template is at `templates/test/ci/cluster-template-prow-<flavor>.yaml`.

## Step 1: Determine Debugging Approach

Based on what is available, choose one or more approaches:

| Available | Approach |
|---|---|
| CAPZ GitHub PR URL | [PR-Based Triage](#pr-based-triage) — start here if the user has a PR link |
| Live management cluster kubeconfig | [Live Cluster Debugging](#live-cluster-debugging) |
| Live workload cluster kubeconfig | [Workload Cluster Debugging](#workload-cluster-debugging) |
| Azure subscription access (`az` CLI) | [VM-Level Debugging](#vm-level-debugging) |
| Prow job URL or GCS artifacts link | [Prow Artifact Analysis](#prow-artifact-analysis) |
| Build log file | [Build Log Analysis](#build-log-analysis) |

Ask the user which of these are available before proceeding.

## Step 1a: PR-Based Triage

If the user provides a CAPZ PR URL (e.g.
`https://github.com/kubernetes-sigs/cluster-api-provider-azure/pull/6106`),
start by listing the failing checks and working backwards from there.

### List failing checks

```bash
# Extract owner/repo and PR number from the URL, then list checks
gh pr checks <PR_NUMBER> --repo kubernetes-sigs/cluster-api-provider-azure
```

This outputs one line per check with: name, status (pass/fail/pending), duration,
and URL. Focus on lines with `fail` status. Example output:
```
pull-cluster-api-provider-azure-conformance-azl3-with-ci-artifacts  fail  0  https://prow.k8s.io/view/gs/kubernetes-ci-logs/pr-logs/pull/kubernetes-sigs_cluster-api-provider-azure/6106/pull-cluster-api-provider-azure-conformance-azl3-with-ci-artifacts/2025980454284300288
```

### Navigate from Prow URL to artifacts

Each Prow check URL follows this pattern:
```
https://prow.k8s.io/view/gs/<GCS_PATH>
```

To get the GCS artifacts URL, replace the `prow.k8s.io/view/gs/` prefix with
`gcsweb.k8s.io/gcs/`:
```
Prow:      https://prow.k8s.io/view/gs/kubernetes-ci-logs/pr-logs/pull/.../<build-id>
Build log: https://gcsweb.k8s.io/gcs/kubernetes-ci-logs/pr-logs/pull/.../<build-id>/build-log.txt
Artifacts: https://gcsweb.k8s.io/gcs/kubernetes-ci-logs/pr-logs/pull/.../<build-id>/artifacts/
```

### Triage order for PR failures

1. **Identify which checks failed** — use `gh pr checks` as above
2. **Categorize the failures** by check name:
   - `conformance*` → Kubernetes conformance test failure
   - `e2e*` → CAPZ-specific or CAPI E2E test failure
   - `*custom-builds*` → Custom-built k8s binaries (dalec or other)
   - `*azl3*` → Azure Linux 3 specific
   - `*windows*` → Windows specific
   - `*ci-version*` or `*ci-artifacts*` → Using CI/pre-release k8s binaries
3. **Fetch the build log** for each failing check — look for the first fatal
   error or timeout (see [Build Log Analysis](#build-log-analysis))
4. **Fetch specific artifacts** if the build log points to a node-level failure
   (see [Prow Artifact Analysis](#prow-artifact-analysis))
5. **Check if failures share a root cause** — multiple checks failing from the
   same base SHA often share a common issue (e.g. a template change that broke
   multiple flavors)

## Step 2: Live Cluster Debugging

Use kubectl against the **management cluster** (the cluster running CAPI controllers —
either a kind cluster or an AKS cluster depending on `MGMT_CLUSTER_TYPE`).

### 2a. Identify the workload cluster

```bash
# Find all workload clusters
kubectl get clusters -A

# Get the cluster name and namespace
CLUSTER_NAME=$(kubectl get clusters -A -o jsonpath='{.items[0].metadata.name}')
CLUSTER_NS=$(kubectl get clusters -A -o jsonpath='{.items[0].metadata.namespace}')
```

### 2b. Check control plane status

```bash
# KubeadmControlPlane status — check Ready, replicas, version
kubectl get kcp -n "$CLUSTER_NS"
kubectl describe kcp -n "$CLUSTER_NS"

# Check individual control plane machines
kubectl get machines -n "$CLUSTER_NS" -l cluster.x-k8s.io/control-plane-name
kubectl get machines -n "$CLUSTER_NS" -o wide

# Key conditions to examine on each Machine:
#   Ready, InfrastructureReady, BootstrapReady, NodeHealthy
```

**What to look for:**
- `Provisioned` but not `Running` = VM exists but kubelet never registered with API server
- `BootstrapReady=False` = cloud-init or kubeadm failed
- Version mismatch between KCP `.spec.version` and machine reported version
- `FailureReason` / `FailureMessage` on Machine or AzureMachine status

### 2c. Check MachineDeployments and MachinePools (workers)

```bash
kubectl get machinedeployments -n "$CLUSTER_NS"
kubectl get machinepools -n "$CLUSTER_NS"

# Check for stuck rollouts — compare desired vs ready replicas
kubectl describe machinedeployment -n "$CLUSTER_NS" <name>

# Check individual worker machines
kubectl get machines -n "$CLUSTER_NS" -l cluster.x-k8s.io/deployment-name=<md-name>
```

### 2d. Check Azure infrastructure

```bash
kubectl get azurecluster -n "$CLUSTER_NS" -o yaml
kubectl get azuremachines -n "$CLUSTER_NS" -o wide
kubectl get azuremachinetemplates -n "$CLUSTER_NS"
kubectl get azuremachinepools -n "$CLUSTER_NS"  # if using MachinePools
```

**Look for:**
- `azurecluster` `Ready` condition and networking status (subnets, LB, NSGs)
- `azuremachine` provisioning state and VM ID
- Failed VM provisioning (quota, SKU availability, etc.)

### 2e. Check addons

```bash
# HelmChartProxy — are addons matching the cluster?
kubectl get helmchartproxies -A
kubectl get helmreleaseproxies -n "$CLUSTER_NS"

# Check specific addon status
kubectl describe helmchartproxy cloud-provider-azure-chart
kubectl describe helmchartproxy calico-chart
kubectl describe helmchartproxy azuredisk-csi-driver-chart

# Check if cluster has the right labels for addon matching
kubectl get cluster -n "$CLUSTER_NS" "$CLUSTER_NAME" -o jsonpath='{.metadata.labels}'
```

**Key check:** The cluster labels determine which HelmChartProxies match.
Missing labels = addons never installed. Common required labels:
`cni: calico`, `cloud-provider: azure`, `azuredisk-csi: true`.

### 2f. Check controller logs on management cluster

```bash
# CAPZ controller — reconciles AzureCluster, AzureMachine, etc.
kubectl logs -n capz-system deployment/capz-controller-manager --tail=200

# CAPI controller — reconciles Cluster, Machine, MachineDeployment, etc.
kubectl logs -n capi-system deployment/capi-controller-manager --tail=200

# CAAPH controller — reconciles HelmChartProxy/HelmReleaseProxy (addon installation)
kubectl logs -n caaph-system deployment/caaph-controller-manager --tail=200

# Kubeadm bootstrap controller — generates cloud-init configs
kubectl logs -n capi-kubeadm-bootstrap-system deployment/capi-kubeadm-bootstrap-controller-manager --tail=200

# Kubeadm control plane controller
kubectl logs -n capi-kubeadm-control-plane-system deployment/capi-kubeadm-control-plane-controller-manager --tail=200
```

### 2g. Get workload cluster kubeconfig

```bash
clusterctl get kubeconfig -n "$CLUSTER_NS" "$CLUSTER_NAME" > /tmp/workload-kubeconfig.yaml

# Or via kubectl secret
kubectl get secret -n "$CLUSTER_NS" "${CLUSTER_NAME}-kubeconfig" \
    -o jsonpath='{.data.value}' | base64 -d > /tmp/workload-kubeconfig.yaml
```

## Step 3: Workload Cluster Debugging

Use `KUBECONFIG=/tmp/workload-kubeconfig.yaml` for all commands in this section.

```bash
# Check node status
KUBECONFIG=/tmp/workload-kubeconfig.yaml kubectl get nodes -o wide

# Check all system pods
KUBECONFIG=/tmp/workload-kubeconfig.yaml kubectl get pods -n kube-system -o wide

# Check critical components:
#   kube-proxy       — DaemonSet, must run on every node for ClusterIP routing
#   coredns          — Deployment, DNS resolution
#   calico-node      — DaemonSet, pod networking
#   cloud-controller-manager  — sets providerID, manages routes
#   cloud-node-manager        — manages node addresses, labels
KUBECONFIG=/tmp/workload-kubeconfig.yaml kubectl get pods -n kube-system \
    --field-selector=status.phase!=Running

# Describe failing pods for events
KUBECONFIG=/tmp/workload-kubeconfig.yaml kubectl describe pod -n kube-system <pod-name>

# Check DaemonSet status (should match number of nodes)
KUBECONFIG=/tmp/workload-kubeconfig.yaml kubectl get daemonsets -n kube-system
```

**Critical dependency chain (if any link breaks, downstream components fail):**

1. **kube-proxy** must be running → provides ClusterIP service routing
2. **Calico** (or other CNI) must be running → provides pod-to-pod networking
3. **CoreDNS** needs CNI → provides DNS resolution
4. **cloud-node-manager** needs ClusterIP routing → manages node cloud metadata
5. **cloud-controller-manager** needs networking → sets `providerID` on Nodes
6. Without `providerID`, CAPI Machine never transitions to `Running`
7. Without Running machines, MachineDeployment stays stuck at 0 ready

## Step 4: VM-Level Debugging

When nodes are stuck in `Provisioned` (VM exists but kubelet never registered
with the API server), debug at the VM level.

### 4a. Find the VM

```bash
# Get Azure VM names from AzureMachine resources
kubectl get azuremachines -n "$CLUSTER_NS" \
    -o custom-columns=NAME:.metadata.name,VMID:.spec.providerID,READY:.status.ready

# The VM name is usually the AzureMachine name
# The resource group is usually the cluster name
VM_NAME="<azuremachine-name>"
RG="$CLUSTER_NAME"
```

### 4b. Check kubelet status and logs

```bash
az vm run-command invoke \
    --resource-group "$RG" --name "$VM_NAME" \
    --command-id RunShellScript \
    --scripts "systemctl status kubelet; echo '---'; journalctl -u kubelet --no-pager -n 100"
```

### 4c. Check kubelet configuration

```bash
# Ubuntu: kubelet args from systemd drop-in
az vm run-command invoke \
    --resource-group "$RG" --name "$VM_NAME" \
    --command-id RunShellScript \
    --scripts "cat /etc/default/kubelet 2>/dev/null; cat /etc/sysconfig/kubelet 2>/dev/null; kubelet --version"

# Azure Linux 3 (azl3): kubelet reads extra args from /etc/sysconfig/kubelet
# Check for stale flags from the gallery image (e.g. --pod-infra-container-image)
```

### 4d. Check cloud-init logs

```bash
az vm run-command invoke \
    --resource-group "$RG" --name "$VM_NAME" \
    --command-id RunShellScript \
    --scripts "tail -300 /var/log/cloud-init-output.log"
```

### 4e. Check binary versions and container images

```bash
az vm run-command invoke \
    --resource-group "$RG" --name "$VM_NAME" \
    --command-id RunShellScript \
    --scripts "kubeadm version; kubelet --version; kubectl version --client; crictl images"
```

### 4f. Check kubeadm status

```bash
az vm run-command invoke \
    --resource-group "$RG" --name "$VM_NAME" \
    --command-id RunShellScript \
    --scripts "ls -la /etc/kubernetes/manifests/; cat /etc/kubernetes/kubelet.conf 2>/dev/null | head -5; cat /var/log/kubeadm-init.log 2>/dev/null || echo 'no kubeadm-init log'"
```

### 4g. Check networking from VM

```bash
# Can the VM reach the API server?
az vm run-command invoke \
    --resource-group "$RG" --name "$VM_NAME" \
    --command-id RunShellScript \
    --scripts "curl -sk https://localhost:6443/healthz 2>&1 || echo 'API server not reachable locally'; nslookup dl.k8s.io 2>&1 || echo 'DNS not working'"
```

## Step 5: Prow Artifact Analysis

CAPZ Prow jobs store artifacts on GCS. The base URL pattern is:
```
https://gcsweb.k8s.io/gcs/kubernetes-ci-logs/pr-logs/pull/<org>_<repo>/<pr>/<job>/<build-id>/
```

For periodic jobs:
```
https://gcsweb.k8s.io/gcs/kubernetes-ci-logs/logs/<job>/<build-id>/
```

### 5a. Artifact directory structure

```
artifacts/
  clusters/<cluster-name>/
    machines/<machine-name>/
      kubelet.log              # Full kubelet log from the VM
      kubelet-version.txt      # Kubelet binary version string
      cloud-init-output.log    # Cloud-init bootstrap log
    resources/                 # YAML dumps of all CAPI resources
      Cluster.yaml
      AzureCluster.yaml
      MachineDeployment.yaml
      KubeadmControlPlane.yaml
      Machine.yaml
      AzureMachine.yaml
      ...
  management-cluster/
    controllers/
      capz-controller-manager.log
      capi-controller-manager.log
      caaph-controller-manager.log
build-log.txt                  # Top-level test runner output
finished.json                  # Pass/fail status and timing
```

### 5b. Triage order

1. **build-log.txt** — Start here. Look for the first fatal error or timeout.
2. **kubelet.log** on failing machines — Startup crashes, flag errors, certs, API connection.
3. **cloud-init-output.log** — Did bootstrap scripts run successfully? Binary downloads?
4. **Controller logs** — CAPZ/CAPI reconciliation errors, VM provisioning failures.
5. **Resource YAMLs** — Verify template expansion was correct (versions, labels, images).

## Step 6: Build Log Analysis

Search for these patterns in order of priority:

| Pattern | Indicates |
|---|---|
| `FAIL` / `FAILED` | Test failures |
| `timed out` / `Timed out` / `timeout` | Operations that exceeded time limits |
| `ImagePullBackOff` / `ErrImagePull` | Container image issues |
| `CrashLoopBackOff` | Pods repeatedly crashing |
| `preflight` | kubeadm preflight check failures |
| `unknown flag` | Deprecated/removed CLI flags |
| `version` near `error` | Version mismatch |
| `quota` / `OperationNotAllowed` | Azure resource quota exceeded |
| `SkuNotAvailable` | VM SKU not available in region |
| `connection refused` (persistent) | Service not running (transient during startup — see below) |

## Common Failure Patterns

### Azure Infrastructure Failures

**VM provisioning failure (quota/SKU):**
- AzureMachine stuck in `Creating` with `FailureMessage`
- Check: `kubectl describe azuremachine -n "$CLUSTER_NS" <name>`
- Common: quota exceeded, SKU not available in region, spot eviction

**Resource group deletion stuck:**
- After test cleanup, Azure RG not deleted
- Check for leaked resources: `az resource list --resource-group <rg>`
- Common: dangling NICs, public IPs, or disks

### Control Plane Failures

**Control plane never initializes (0/N ready):**
1. Check the **first** CP machine — `kubeadm init` only runs on the first node
2. If first CP machine is `Provisioned` but not `Running`, debug at VM level
3. Common causes: binary download failure, etcd timeout, wrong kubeadm config,
   cloud-init script error

**Control plane partially initialized (1/3 or 2/3 ready):**
1. First CP node initialized but others can't join
2. Check `kubeadm join` logs on failing CP nodes
3. Common: certificate distribution failure, API server unreachable from joining nodes,
   etcd member add failure

### Worker Node Failures

**MachineDeployment stuck at 0 ready:**
1. Check Machine conditions — is `NodeHealthy` false?
2. If machines are `Provisioned` but not `Running` → kubelet never registered →
   use VM-level debugging
3. If machines cycle through delete/recreate → MachineHealthCheck is killing them

**MachinePool (VMSS) not scaling:**
1. Check AzureMachinePool status and conditions
2. VMSS-specific: check Azure activity log for scale-out failures
3. Common: VMSS image reference mismatch, extension failures

### Networking Failures

**Pods stuck in ContainerCreating (CNI not ready):**
- Calico not installed → check cluster label `cni: calico`
- Calico pods crashing → check calico-node DaemonSet logs
- Azure CNI v1 misconfigured → check azure-vnet-ipam logs

**Services unreachable (kube-proxy not running):**
- kube-proxy DaemonSet not scheduled or crashing
- Check if kube-proxy image exists for the cluster's k8s version
- For custom builds or LTS: the image tag may not exist at `registry.k8s.io`

**VNet peering / private DNS issues (AKS management cluster):**
- Workload cluster can't reach management cluster or registries
- Check VNet peering status and private DNS zone links

### CNI Failures

CNI (Container Network Interface) is responsible for pod-to-pod networking. CAPZ
clusters typically use Calico, installed via HelmChartProxy matched by the
`cni: calico` cluster label. Without a functioning CNI, pods stay in
`ContainerCreating` and the cluster is effectively unusable.

**Calico not installed at all:**
- Missing `cni: calico` label on the Cluster resource
- HelmChartProxy `calico-chart` not matching → `kubectl get helmchartproxies`
- HelmReleaseProxy not created or stuck → check CAAPH controller logs

**Calico pods in CrashLoopBackOff or Init:Error:**
- Check calico-node DaemonSet logs:
  ```bash
  KUBECONFIG=/tmp/workload-kubeconfig.yaml kubectl logs -n kube-system daemonset/calico-node -c calico-node --tail=100
  ```
- Common causes:
  - IP pool CIDR conflict with the VNet or service CIDR
  - VXLAN/IPIP encapsulation issues (Azure blocks raw IP-in-IP; VXLAN is default)
  - Felix readiness probe failures due to missing iptables/nftables
  - `calico-node` can't reach the API server (kube-proxy dependency — fix
    kube-proxy first)

**calico-kube-controllers failing:**
- Usually a cascading failure — it needs working pod networking and API access
- If calico-node is healthy but controllers are not, check RBAC and service
  account token issues

**Partial CNI — some nodes have networking, others don't:**
- calico-node pod not scheduled on affected nodes (taints, resource constraints)
- BGP peering failures between nodes (if using BGP mode instead of VXLAN)
- Node-to-node connectivity blocked by NSG rules (check Azure NSG on subnet)

**Debug checklist for CNI issues:**
```bash
# 1. Is the CNI addon installed?
kubectl get helmchartproxies -A | grep -i calico
kubectl get helmreleaseproxies -n "$CLUSTER_NS" | grep -i calico

# 2. Is calico-node running on all nodes?
KUBECONFIG=/tmp/workload-kubeconfig.yaml kubectl get pods -n kube-system -l k8s-app=calico-node -o wide

# 3. Check calico-node logs on a failing node
KUBECONFIG=/tmp/workload-kubeconfig.yaml kubectl logs -n kube-system <calico-node-pod> -c calico-node --tail=100

# 4. Check IPPool configuration
KUBECONFIG=/tmp/workload-kubeconfig.yaml kubectl get ippools -o yaml

# 5. Check node NetworkUnavailable condition (should be False when CNI is working)
KUBECONFIG=/tmp/workload-kubeconfig.yaml kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{range .status.conditions[?(@.type=="NetworkUnavailable")]}{.status}{end}{"\n"}{end}'
```

### CAPZ Bootstrapping VM Extension (Cloud-Init Gotcha)

**IMPORTANT:** When an AzureMachine fails with a VM extension error like
`VMExtensionProvisioningError` or `VMExtensionHandlerNonTransientError` for the
`CAPZ.Linux.Bootstrapping` extension, **do NOT debug the extension itself**. The
extension is a thin wrapper that waits for cloud-init to finish by polling for a
sentinel file (typically `/run/cluster-api/bootstrap-success.complete`).

The extension's only job is:
1. Wait for cloud-init to write the sentinel file
2. Report success if the file appears, or fail/timeout if it doesn't

**If the extension fails, the actual root cause is almost always cloud-init
failing.** The extension error is a symptom, not the cause.

**Debug approach when you see a VM extension error:**
1. **Skip the extension error** — it's not informative beyond "cloud-init didn't finish"
2. Go straight to cloud-init logs on the VM:
   ```bash
   az vm run-command invoke \
       --resource-group "$RG" --name "$VM_NAME" \
       --command-id RunShellScript \
       --scripts "tail -300 /var/log/cloud-init-output.log; echo '===CLOUD-INIT-STATUS==='; cloud-init status --long"
   ```
3. Check if `preKubeadmCommands` failed (binary downloads, package installs,
   script errors):
   ```bash
   az vm run-command invoke \
       --resource-group "$RG" --name "$VM_NAME" \
       --command-id RunShellScript \
       --scripts "cat /var/log/cloud-init-output.log | grep -A5 -i 'error\|fail\|fatal'"
   ```
4. Check if the sentinel file was created:
   ```bash
   az vm run-command invoke \
       --resource-group "$RG" --name "$VM_NAME" \
       --command-id RunShellScript \
       --scripts "ls -la /run/cluster-api/ 2>/dev/null || echo 'sentinel dir does not exist'"
   ```

**Common cloud-init failures that surface as VM extension errors:**
- `preKubeadmCommands` script syntax error or failed command (e.g. `curl` to
  download binaries returned 404/500)
- `kubeadm init` or `kubeadm join` failed (invalid flags, version skew, cert issues)
- Disk full (especially `/var` partition on small VMs — etcd and container images)
- Network not ready when cloud-init ran (DHCP timeout, DNS resolution failure)
- Package install failure (`dpkg -i` or `rpm -i` returned nonzero)

### Image Pull Failures (ImagePullBackOff)

**Common causes:**
- Image tag doesn't exist (LTS version tags, unreleased versions, typos)
- Registry unreachable (network isolation, missing VNet peering)
- Authentication failure (private registry without imagePullSecret)
- Rate limiting (Docker Hub, registry.k8s.io)

**Debug:** Check pod events for the exact image reference being pulled:
```bash
KUBECONFIG=/tmp/workload-kubeconfig.yaml kubectl describe pod -n kube-system <pod>
```

### Version Skew (kubeadm preflight failure)

**Symptom:** `kubeadm init` or `kubeadm join` fails: "kubelet version is higher
than the control plane version"

**Root cause:** Gallery image ships kubelet at version X.Y.Z, but target
`KUBERNETES_VERSION` is lower. kubeadm requires `kubelet <= control plane` within
the same minor. Binary replacement script may have failed silently, leaving the
gallery kubelet in place.

**Debug:** Check `kubelet --version` on the VM vs `KUBERNETES_VERSION`.

### Helm Chart / Addon Failures

**cloud-provider-azure not deployed:**
- Cluster missing `cloud-provider: azure` label
- HelmChartProxy not matching → check `kubectl get helmchartproxies`
- HelmReleaseProxy failed → check CAAPH controller logs

**cloud-provider-azure deployed but CCM/CNM image empty:**
- The Helm chart's built-in version mapping doesn't cover the cluster's k8s minor
- Fix: use the CI variant (`cloud-provider: azure-ci` label) with explicit image tags

**cloud-node-manager CrashLoopBackOff:**
- Usually cascading: kube-proxy not running → no ClusterIP routing → CNM can't
  reach API server. Fix kube-proxy first.

### Windows-Specific Failures

- HNS (Host Networking Service) crash on Windows nodes
- CSI proxy not running → volume mount failures
- Containerd logger issues → missing logs
- Windows kube-proxy image mismatch (different from Linux)
- Server version mismatch (Windows Server 2019 vs 2022)

## Dalec Custom Builds (Specific)

The `dalec-custom-builds` flavor tests dalec-built Kubernetes images. It has
additional complexity beyond standard CAPZ tests:

### Architecture

- Creates both Ubuntu (`-md-0`) and Azure Linux 3 (`-azl3-md-0`) worker nodes
- Custom init scripts replace gallery-shipped k8s binaries with dalec-built ones:
  - Ubuntu: downloads `.deb` packages from Azure storage, installs via `dpkg -i`
  - azl3: downloads `.rpm` packages, extracts via `rpm2cpio | cpio -idmv`, copies binaries
- Control plane nodes also get custom binaries and container images
- Uses `prow-apiserver-ilb` as base (internal load balancer for API server)
- kube-proxy image is pre-pulled and re-tagged in `preKubeadmCommands`

### Dalec-Specific Failure Patterns

**LTS version image pull failure:**
LTS versions use patch >= 100 (e.g. `v1.31.100`). Tags like
`registry.k8s.io/kube-proxy:v1.31.100` don't exist upstream. The template must
pre-pull the image from the dalec registry and re-tag it.

**Gallery version mismatch for LTS:**
LTS patch numbers (100+) sort higher than any real upstream release. Gallery
lookup must select the highest version within the same major.minor series, not
the "lowest >= target" (which would jump to the next minor).

**Stale /etc/sysconfig/kubelet on azl3:**
The azl3 gallery image ships `/etc/sysconfig/kubelet` with
`KUBELET_EXTRA_ARGS=--pod-infra-container-image=...`. This flag was removed in
k8s v1.35. If the install script replaces the kubelet binary but not the config,
kubelet v1.35+ crashes with `unknown flag: --pod-infra-container-image`.

**Silent binary replacement failure:**
The download/install script may fail without causing cloud-init to abort (e.g.
pipeline errors not caught by `set -o errexit`). The gallery binaries remain,
causing version skew. Always verify installed binary versions on the VM.

**Conformance image for LTS:**
The conformance test runner pulls `registry.k8s.io/conformance:<cluster-version>`.
For LTS this tag doesn't exist. Must set `CONFORMANCE_IMAGE` env var to an
upstream version (e.g. the gallery version).

### Dalec Template Structure

```
templates/test/ci/prow-dalec-custom-builds/
  kustomization.yaml
  patches/
    azl3-machine-deployment.yaml         # azl3 KubeadmConfigTemplate + AzureMachineTemplate
    kubeadm-bootstrap-custom-builds.yaml # Replaces k8s binaries on Ubuntu nodes
    control-plane-custom-builds.yaml     # Replaces k8s binaries on control plane
    azure-machine-template-gallery-image.yaml  # Uses CAPZ_GALLERY_VERSION for base image
    delete-machine-health-check.yaml     # Removes CP MachineHealthCheck
```

Kustomization targets patches by GVK and name regex:
- `^[^-]*-md-0$` → Ubuntu worker KubeadmConfigTemplate (not azl3)
- `.*-azl3-md-0` → azl3 worker KubeadmConfigTemplate
- `.*-control-plane` → KubeadmControlPlane

## Transient Errors to Ignore

These appear frequently in logs but are usually harmless during cluster bootstrap.
Do NOT treat them as root causes unless they persist for more than 5-10 minutes:

**API server startup (normal during first few minutes):**
- `connection refused` to port 6443
- `the server was unable to return a response in the time allotted`
- `TLS handshake error` — initial certificate rotation
- `failed to list *v1.Node: Unauthorized` — RBAC not yet bootstrapped
- `Waiting for the Kubernetes API server`

**Node registration (normal until node object exists):**
- `node "..." not found` in kubelet logs
- `failed to get provider ID` — CCM hasn't set providerID yet
- `Failed to get nodeLease` — lease controller not yet running

**etcd formation (normal during initial cluster):**
- `etcdserver: no leader`
- `waiting for leader`
- `etcdserver: request timed out`

**DNS and service discovery (normal until CoreDNS is running):**
- `dial tcp: lookup ... no such host` for internal services
- `configmaps "kubeadm-config" not found` — created during `kubeadm init`
- `unable to fetch certificate signer` — cert controllers starting up

**Cloud-init (normal on Azure):**
- `url_helper.py` retry warnings — metadata service retries are expected
- Progress dots in cloud-init-output.log

**CAPI status messages (informational, not errors):**
- `MachineDeployment is scaling up`
- `Machine is being created`
- `Waiting for infrastructure provider to report ready`
- `Waiting for control plane provider to indicate the control plane has been initialized`
