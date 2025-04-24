# Tilt with AKS as Management Cluster with Internal Load Balancer

## Introduction
This guide is explaining how to set up and use Azure Kubernetes Service (AKS) as a management cluster for Cluster API Provider Azure (CAPZ) development using Tilt and an internal load balancer (ILB).

While the default Tilt setup recommends using a KIND cluster as the management cluster for faster development and experimentation, this guide demonstrates using AKS as an alternative management cluster. We also cover additional steps for working with stricter network policies - particularly useful for organizations that need to maintain all cluster communications within their Azure Virtual Network (VNet) infrastructure with enhanced access controls.

### Who is this for?
- Developers who want to use AKS as the management cluster for CAPZ development.
- Developers working in environments with strict network security requirements.
- Teams that need to keep all Kubernetes API traffic within Azure VNet

**Note:** This is not a production ready solution and should not be used in a production environment. This is only meant to be used for development/testing purposes.

## Prerequisites

### Requirements
<!-- markdown-link-check-disable-next-line -->
- A [Microsoft Azure account](https://azure.microsoft.com/)
  - Note: If using a new subscription, make sure to [register](https://learn.microsoft.com/azure/azure-resource-manager/management/resource-providers-and-types) the following resource providers:
    - `Microsoft.Compute`
    - `Microsoft.Network`
    - `Microsoft.ContainerService`
    - `Microsoft.ManagedIdentity`
    - `Microsoft.Authorization`
    - `Microsoft.ResourceHealth` (if the `EXP_AKS_RESOURCE_HEALTH` feature flag is enabled)
- Install the [Azure CLI](https://learn.microsoft.com/cli/azure/install-azure-cli?view=azure-cli-latest)
- A [supported version](https://github.com/kubernetes-sigs/cluster-api-provider-azure#compatibility) of [clusterctl](https://cluster-api.sigs.k8s.io/user/quick-start#install-clusterctl)
- Basic understanding of Azure networking concepts, Cluster API, and CAPZ.
- `go`, `wget`, and `tilt` installed on your development machine.
- If `tilt-settings.yaml` file exists in the root of your repo, clear out any values in `kustomize_settings` unless you want to use them instead of the values that will be set by running `make aks-create`.

### Managed Identity & Registry Setup
1. Have a managed identity created from Azure Portal.
2. Add the following lines to your shell config such as `~/.bashrc` or `~/.zshrc`
   ```shell
   export USER_IDENTITY="<user-assigned-managed-identity-name>"
   export AZURE_CLIENT_ID_USER_ASSIGNED_IDENTITY="<user-assigned-managed-identity-client-id>"
   export AZURE_CLIENT_ID="${AZURE_CLIENT_ID_USER_ASSIGNED_IDENTITY}"
   export AZURE_OBJECT_ID_USER_ASSIGNED_IDENTITY="<user-assigned-managed-identity--object-id>"
   export AZURE_USER_ASSIGNED_IDENTITY_RESOURCE_ID="<resource-id-of-user-assigned-managed-id-from-json-view>"
   export AZURE_LOCATION="<azure-location-having-quota-for-B2s-and-D4s_v3-SKU>"
   export REGISTRY=<your-container-registry>
   ```
3. Be sure to reload with `source ~/.bashrc` or `source ~/.zshrc` and then verify the correct env vars values return with `echo $AZURE_CLIENT_ID` and `echo $REGISTRY`.

## Steps to Use Tilt with AKS as the Management Cluster

1. Ensure that the tilt-settings.yaml in root of the repository looks like below
   ```yaml
      kustomize_substitutions: {}
      allowed_contexts:
      - "kind-capz"
      container_args:
         capz-controller-manager:
            - "--v=4"
   ```
      - Add env variables in `kustomize_substitutions` if you want the added env variables to take precedence over the env values exported by running `make aks-create`
2. `make clean`
   - This make target does not need to be run every time. Run it to remove bin and kubeconfigs.
3. `make generate`
   - This make target does not need to be run every time. Run it to update your go related targets, manifests, flavors, e2e-templates and addons.
   - Run it periodically upon updating your branch or if you want to make changes in your templates.
4. `make acr-login`
   - Run this make target only if you have `REGISTRY` set to an Azure Container Registry. If you used DockerHub like we recommend, you can skip this step.
5. `make aks-create`
   - Run this target to bring up an AKS cluster.
   - Once the AKS cluster is created, you can reuse the cluster as many times as you like. Tilt ensures that the new image gets deployed every time there are changes in the Tiltfile and/or dependent files.
   - Running `make aks-create` cleans up any existing variables from `aks_as_mgmt_settings` from the `tilt-settings.yaml`.
6. `make tilt-up`
   - Run this target to use underlying cluster being pointed by your `KUBECONFIG`.

7. [Optional for 1P users] Once the tilt UI is up and running click on the `allow required ports on mgmt cluster` task (checkmark the box and reload) to allow the required ports on the management cluster's API server.
   - Note: This task will wait for the NSG rules to be created and then update them to allow the required ports.
   - This task will take a few minutes to complete. Wait for this to finish to avoid race conditions.
8. Check the flavors you want to deploy and CAPZ will deploy the workload cluster with the selected flavor.
      - Flavors that leverage internal load balancer and are available for development in CAPZ for MSFT Tenant:
         - [apiserver-ilb](https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/main/templates/cluster-template-apiserver-ilb.yaml): VM-based default flavor that brings up native K8s clusters with Linux nodes.
         - [apiserver-ilb-windows](https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/main/templates/cluster-template-windows-apiserver-ilb.yaml): VM-based flavor that brings up native K8s clusters with Linux and Windows nodes.

## Running e2e tests locally using API Server ILB's networking solution

Running an e2e test locally in a restricted environment calls for some workarounds in the prow templates and the e2e test itself.

1. We need to add the apiserver ILB with private endpoints and predeterimined CIDRs to the workload cluster's VNet & Subnets, and pre-kubeadm commands updating the `/etc/hosts` file of the nodes of the workload cluster.

2. Once the template has been modified to be run in local environment using AKS as management cluster, we need to be able to peer the vnets, create private DNS zone for the FQDN of the workload cluster and re-enable blocked NSG ports.

**Note:**

- The following guidance is only for debugging, and is not a recommendation for any production environment.

- The below steps are for self-managed templates only and do not apply to AKS workload clusters.

- If you are going to run the local tests from a dev machine in Azure, you will have to use user-assigned managed identity and assign it to the management cluster. Follow the below steps before proceeding.
  1. Create a user-assigned managed identity
  2. Assign that managed identity a contributor role to your subscription
  3. Set `AZURE_CLIENT_ID_USER_ASSIGNED_IDENTITY`, `AZURE_OBJECT_ID_USER_ASSIGNED_IDENTITY`, and `AZURE_USER_ASSIGNED_IDENTITY_RESOURCE_ID` to the user-assigned managed identity.

#### Update prow template with apiserver ILB networking solution

There are three sections of a prow template that need an update.

1. AzureCluster
   - `/spec/networkSpec/apiServerLB`
      - Add FrontendIP
      - Add an associated private IP to be leveraged by an internal ILB
   - `/spec/networkSpec/vnet/cidrBlocks`
      - Add VNet CIDR
   - `/spec/networkSpec/subnets/0/cidrBlocks`
      - Add Subnet CIDR for the control plane
   - `/spec/networkSpec/subnets/1/cidrBlocks`
      - Add Subnet CIDR for the worker node
2. `KubeadmConfigTemplate` - linux node; Identifiable by `name: .*-md-0`
   - `/spec/template/spec/preKubeadmCommands/0`
      - Add a prekubeadm command updating the `/etc/hosts` of worker nodes of type "linux".
3. `KubeadmConfigTemplate` - windows node; Identifiable by `name: .*-md-win`
   - `/spec/template/spec/preKubeadmCommands/0`
      - Add a prekubeadm command updating the `/etc/hosts` of worker nodes of type "windows".

A sample kustomize command for updating a prow template via its kustomization.yaml is pasted below.

```yaml
- target:
    kind: AzureCluster
  patch: |-
    - op: add
      path: /spec/networkSpec/apiServerLB
      value:
        frontendIPs:
        - name: ${CLUSTER_NAME}-api-lb
          publicIP:
            dnsName: ${CLUSTER_NAME}-${APISERVER_LB_DNS_SUFFIX}.${AZURE_LOCATION}.cloudapp.azure.com
            name: ${CLUSTER_NAME}-api-lb
        - name: ${CLUSTER_NAME}-internal-lb-private-ip
          privateIP: ${AZURE_INTERNAL_LB_PRIVATE_IP}
- target:
    kind: AzureCluster
  patch: |-
    - op: add
      path: /spec/networkSpec/vnet/cidrBlocks
      value: []
    - op: add
      path: /spec/networkSpec/vnet/cidrBlocks/-
      value: ${AZURE_VNET_CIDR}
- target:
    kind: AzureCluster
  patch: |-
    - op: add
      path: /spec/networkSpec/subnets/0/cidrBlocks
      value: []
    - op: add
      path: /spec/networkSpec/subnets/0/cidrBlocks/-
      value: ${AZURE_CP_SUBNET_CIDR}
- target:
    kind: AzureCluster
  patch: |-
    - op: add
      path: /spec/networkSpec/subnets/1/cidrBlocks
      value: []
    - op: add
      path: /spec/networkSpec/subnets/1/cidrBlocks/-
      value: ${AZURE_NODE_SUBNET_CIDR}
- target:
    kind: KubeadmConfigTemplate
    name: .*-md-0
  patch: |-
    - op: add
      path: /spec/template/spec/preKubeadmCommands
      value: []
    - op: add
      path: /spec/template/spec/preKubeadmCommands/-
      value: echo '${AZURE_INTERNAL_LB_PRIVATE_IP}   ${CLUSTER_NAME}-${APISERVER_LB_DNS_SUFFIX}.${AZURE_LOCATION}.cloudapp.azure.com' >> /etc/hosts
- target:
    kind: KubeadmConfigTemplate
    name: .*-md-win
  patch: |-
    - op: add
      path: /spec/template/spec/preKubeadmCommands/-
      value:
        powershell -Command "Add-Content -Path 'C:\\Windows\\System32\\drivers\\etc\\hosts' -Value '${AZURE_INTERNAL_LB_PRIVATE_IP} ${CLUSTER_NAME}-${APISERVER_LB_DNS_SUFFIX}.${AZURE_LOCATION}.cloudapp.azure.com'"
```

#### Peer Vnets of the management cluster and the workload cluster

Peering VNets, creating a private DNS zone with the FQDN of the workload cluster, and updating NSGs of the management and workload clusters can be achieved by running `scripts/peer-vnets.sh`.

This script, `scripts/peer-vnets.sh`, should be run after triggering the test run locally and from a separate terminal.

#### Running the test locally

We recommend running each test individually while debugging the test failure. This implies that `GINKGO_FOCUS` is as unique as possible. So for instance if you want to run `periodic-cluster-api-provider-azure-e2e-main`'s "With 3 control-plane nodes and 2 Linux and 2 Windows worker nodes" test,

1. We first need to add the following environment variables to the test itself. For example:

   ```go
   Expect(os.Setenv("EXP_APISERVER_ILB", "true")).To(Succeed())
   Expect(os.Setenv("AZURE_INTERNAL_LB_PRIVATE_IP", "10.0.0.101")).To(Succeed())
   Expect(os.Setenv("AZURE_VNET_CIDR", "10.0.0.0/8")).To(Succeed())
   Expect(os.Setenv("AZURE_CP_SUBNET_CIDR", "10.0.0.0/16")).To(Succeed())
   Expect(os.Setenv("AZURE_NODE_SUBNET_CIDR", "10.1.0.0/16")).To(Succeed())
   ```

   The above lines should be added before the `clusterctl.ApplyClusterTemplateAndWait()` is invoked.


2. Open the terminal and run the below command:

   ```bash
   GINKGO_FOCUS="With 3 control-plane nodes and 2 Linux and 2 Windows worker nodes" USE_LOCAL_KIND_REGISTRY=false SKIP_CLEANUP="true" SKIP_LOG_COLLECTION="true" REGISTRY="<>" MGMT_CLUSTER_TYPE="aks" EXP_APISERVER_ILB=true AZURE_LOCATION="<>" ARCH="amd64" scripts/ci-e2e.sh
   ```

   **Note:**

   - Set `MGMT_CLUSTER_TYPE` to `"aks"` to leverage `AKS` as the management cluster.
   - Set `EXP_APISERVER_ILB` to `true` to enable the API Server ILB feature gate.
   - Set `AZURE_CLIENT_ID_USER_ASSIGNED_IDENTITY`, `AZURE_OBJECT_ID_USER_ASSIGNED_IDENTITY` and `AZURE_USER_ASSIGNED_IDENTITY_RESOURCE_ID` to use the user-assigned managed identity instead of the AKS-created managed identity.

3. In a new terminal, wait for AzureClusters to be created by the above command. Check it using `kubectl get AzureClusters -A`. Note that this command will fail or will not output anything unless the above command, `GINKGO_FOCUS...`, has deployed the worker template and initiated workload cluster creation.

   Once the worker cluster has been created, `export` the `CLUSTER_NAME` and `CLUSTER_NAMESPACE`.
   It is recommended that `AZURE_INTERNAL_LB_PRIVATE_IP` is set an IP of `10.0.0.x`, say `10.0.0.101`, to avoid any test updates.

   Then open a new terminal at the root of the cluster api provider azure repo and run the below command.

   ```bash
   AZURE_INTERNAL_LB_PRIVATE_IP="<Internal-IP-from-the-e2e-test>" CLUSTER_NAME="<e2e workload-cluster-name>" CLUSTER_NAMESPACE="<e2e-cluster-namespace>" ./scripts/peer-vnets.sh ./tilt-settings.yaml
   ```

You will see that the test progresses in the first terminal window that invoked `GINKGO_FOCUS=....`


## Leveraging internal load balancer

By default using Tilt with Cluster API Provider Azure (CAPZ), the management cluster is exposed via a public endpoint. This works well for many development scenarios but presents challenges in environments with strict network security requirements.

### Challenges and Solutions

1. **Management to Workload Cluster Connectivity**

   **Scenario:**
   - Management cluster cannot connect to workload cluster's API server via workload cluster's FQDN.

   **Solution:**
   - Peer management and workload cluster VNets.
   - Set Workload cluster API server replica count to 3. (Default is 1 when using KIND as the management cluster).
      - This is done by setting `CONTROL_PLANE_MACHINE_COUNT` to 3 in the tilt-settings.yaml file.
      - `make aks-create` will set this value to 3 for you.
   - Create a internal load balancer (ILB) with the workload cluster's apiserver VMs as its backend pool in the workload cluster's VNet.
      - This is achieved by setting `EXP_INTERNAL_LB=true`. `EXP_INTERNAL_LB` is set to `true` by default when running `make tilt-up`.
   - Create private DNS zone with workload cluster's apiserver FQDN as its record to route management cluster calls to workload cluster's API server private IP.
      - As of current release, a private DNS zone is automatically created in the tilt workflow for `apiserver-ilb` and `windows-apiserver-ilb` flavors.

2. **Workload Cluster Node Communication**

   **Scenario:**
   - Workload cluster worker nodes should not be able to communicate with their workload cluster's API server's FQDN.

   **Solution:**
   - Update `/etc/hosts` on worker nodes via preKubeadmCommands in the `KubeadmConfigTemplate` with `- echo '${AZURE_INTERNAL_LB_PRIVATE_IP}   ${CLUSTER_NAME}-${APISERVER_LB_DNS_SUFFIX}.${AZURE_LOCATION}.cloudapp.azure.com' >> /etc/hosts`
      - This essentially creates a static route (using the ILB's private IP) to the workload cluster's API server (FQDN) in the worker nodes.
      - Deploying `apiserver-ilb` or `windows-apiserver-ilb` flavor will deploy worker nodes of the workload cluster with the private IP of the ILB as their default route.

3. **Network Security Restrictions**

   **Scenario:**
   - Critical ports needed for management cluster to communicate with workload cluster.
   - ports:
     - `TCP: 443, 6443`
     - `UDP: 53`

   **Solution:**
   - Once tilt UI is up and running, click on the `allow required ports on mgmt cluster` task to allow the required ports on the management cluster's API server.

4. **Load Balancer Hairpin Routing**

   **Challenge:**
   - Workload cluster's control plane fails to bootstrap due to internal LB connectivity timeouts.
   - Single control plane node setup causes hairpin routing issues.

   **Solution:**
   - Use 3 control plane nodes in a stacked etcd setup.
      - Using aks as management cluster sets `CONTROL_PLANE_MACHINE_COUNT` to 3 by default.
