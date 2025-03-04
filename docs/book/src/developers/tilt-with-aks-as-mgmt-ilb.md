# Tilt with AKS as Management Cluster with Internal Load Balancer

## Introduction
This guide is explaining how to set up and use Azure Kubernetes Service (AKS) as a management cluster for Cluster API Provider Azure (CAPZ) development using Tilt and an internal load balancer (ILB).

While the default Tilt setup recommends using a KIND cluster as the management cluster for faster development and experimentation, this guide demonstrates using AKS as an alternative management cluster. We also cover additional steps for working with stricter network policies - particularly useful for organizations that need to maintain all cluster communications within their Azure Virtual Network (VNet) infrastructure with enhanced access controls.

### Who is this for?
- Developers who want to use AKS as the management cluster for CAPZ development.
- Developers working in environments with strict network security requirements.
- Teams that need to keep all Kubernetes API traffic within Azure VNet


**Note:** This is not a production ready solution and should not be used in a production environment. This is only meant to be used for development/testing purposes.

### Prerequisites
- All [general CAPZ prerequisites](../getting-started.md#prerequisites) should be satisfied
- Basic understanding of Azure networking concepts.
- Familiarity with Cluster API and CAPZ.
- Tilt installed on your development machine.
- `export REGISTRY=<registry_of_your-choice>` set to the registry of your choice.
- If `tilt-settings.yaml` file exists in the root of your repo, clear out any values in `kustomize_settings` unless you want to use them instead of the values that will be set by running `make aks-create`.

## Using Tilt with AKS as the Management Cluster

To use Tilt with AKS as the management cluster, you need to run the following commands:
- `make clean`
- `make generate`
- `make acr-login`
- `make aks-create`
- `make tilt-up`

Using the tilt UI, click on the flavors you want to deploy and CAPZ will deploy the workload cluster with the selected flavor.

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

##### Flavors leveraging internal load balancer

There are two flavors available for development in CAPZ for MSFT Tenant:
- [apiserver-ilb](../../../../templates/cluster-template-apiserver-ilb.yaml): VM based default flavor that brings up native K8s clusters with Linux nodes.
- [apiserver-ilb-windows](../../../../templates/cluster-template-windows-apiserver-ilb.yaml): VM based flavor that brings up native K8s clusters with Linux and Windows nodes.

#### Tilt Workflow for AKS as Management Cluster with Internal Load Balancer

- In tilt-settings.yaml, set subscription_type to "corporate" and remove any other env values unless you want to override env variables created by `make aks-create`. Example:
   ```
   .
   .
   kustomize_substitutions:
   "SUBSCRIPTION_TYPE": "corporate"
   .
   ```
- `make clean`
  - This make target does not need to be run every time. Run it to remove bin and kubeconfigs.
- `make generate`
  - This make target does not need to be run every time. Run it to update your go related targets, manifests, flavors, e2e-templates and addons.
  - Run it periodically upon updating your branch or if you want to make changes in your templates.
- `make acr-login`
  - Run this make target if you have `REGISTRY` set to an Azure Container Registry.
- `make aks-create`
  - Run this target to bring up an AKS cluster.
  - Once the AKS cluster is created, you can reuse the cluster as many times as you like. Tilt ensures that the new image gets deployed every time there are changes in the Tiltfile and/or dependent files.
  - Running `make aks-create` cleans up any existing `aks_as_mgmt_settings`.
- `make tilt-up`
  - Run this target to use underlying cluster being pointed by your `KUBECONFIG`.

Once the tilt UI is up and running
- Click on the `allow required ports on mgmt cluster` task to allow the required ports on the management cluster's API server.
   - Note: This task will wait for the NSG rules to be created and then update them to allow the required ports.
   - This task will take a few minutes to complete.
   - This target can run in parallel and independent of deploying any cluster flavors.
- Click on the flavors you want to deploy and CAPZ will deploy the workload cluster with the selected flavor.
   - Flavors that leverage internal load balancer are:
     - `apiserver-ilb`
     - `windows-apiserver-ilb`
