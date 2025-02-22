# Tilt with AKS as Management Cluster with Internal Load Balancer

## Introduction
This guide explains how to set up and use Azure Kubernetes Service (AKS) as a management cluster for Cluster API Azure (CAPZ) development using Tilt and an internal load balancer (ILB). 

While the default Tilt setup uses a KIND cluster as the local management cluster, this approach demonstrates how to leverage AKS with stricter networking policies for secure cluster communication. This solution is particularly useful for organizations that need to maintain all cluster communications within their Azure Virtual Network (VNet) infrastructure with enhanced access controls.

### Who is this for?
- Developers who want to use AKS as the management cluster for CAPZ development.
- Developers working in environments with strict security requirements.
- Teams that need to keep all Kubernetes API traffic within Azure VNet

**Note:** This is not a production ready solution and should not be used in a production environment. This is only meant to be used for development/testing purposes.

### Prerequisites
- All [general CAPZ prerequisites](../getting-started.md#prerequisites) must be satisfied
- Basic understanding of Azure networking concepts.
- Familiarity with Cluster API and CAPZ.
- Tilt installed on your development machine.
- `export REGISTRY=<registry_of_your-choice>` set to the registry of your choice.

## Using Tilt with AKS as the Management Cluster

To use Tilt with AKS as the management cluster, you need to run the following commands:
- `make clean`
- `make generate`
- `make acr-login`
- `make aks-create`
- `make tilt-up`

Using the tilt UI, click on the flavors you want to deploy and CAPZ will deploy the workload cluster with the selected flavor.

## Leveraging internal load balancer   

By default, when using Tilt with Cluster API Provider Azure (CAPZ), the management cluster is exposed via a public endpoint. This works well for many development scenarios but presents challenges in environments with strict network security requirements.

### Flavors leveraging internal load balancer
- apiserver-ilb
- windows-apiserver-ilb

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
      - This is achieved setting `EXP_INTERNAL_LB=true`. `EXP_INTERNAL_LB` set to `true` by default when running `make tilt-up`.
   - Create private DNS zone with workload cluster's apiserver FQDN as its record to route management cluster calls to workload cluster's API server private IP.
      - As of current release, a private DNS zone is automatically created in the tilt workflow for `apiserver-ilb` and `windows-apiserver-ilb` flavors.

2. **Workload Cluster Node Communication**
   
   **Scenario:**
   - Workload cluster worker nodes should not be able to communicate with their workload cluster's API server's FQDN.

   **Solution:**
   - Update `/etc/hosts` on worker nodes via preKubeadmCommands in the `KubeadmConfigTemplate` with `- echo '${AZURE_INTERNAL_LB_PRIVATE_IP}   ${CLUSTER_NAME}-${APISERVER_LB_DNS_SUFFIX}.${AZURE_LOCATION}.cloudapp.azure.com' >> /etc/hosts`
      - This essentially creates a static route (using the ILB's private IP) to the workload cluster's API server (FQDN) in the worker nodes.
      - Deploying `apiserver-ilb` or `windows-apiserver-ilb` flavor will deploy worker nodes of the workload cluster with the private IP of the ILB as their default route.

4. **Network Security Restrictions**
   
   **Scenario:**
   - Critical ports needed for management cluster to communicate with workload cluster.
   - ports:
     - `TCP: 443, 6443`
     - `UDP: 53`

   **Solution:**
   - Once tilt UI is up and running, click on the `allow required ports on mgmt cluster` task to allow the required ports on the management cluster's API server. 

5. **Load Balancer Hairpin Routing**
   
   **Challenge:**
   - Woekload cluster's control plane fails to bootstrap due to internal LB connectivity timeouts.
   - Single control plane node setup causes hairpin routing issues.

   **Solution:**
   - Use 3 control plane nodes in a stacked etcd setup.
      - Using aks as management cluster sets `CONTROL_PLANE_MACHINE_COUNT` to 3 by default.

#### Tilt Workflow for AKS as Management Cluster with Internal Load Balancer

- In tilt-settings.yaml, set subscription_type to "corporate". Example:
   ```
   .
   .
   kustomize_substitutions:
   "SUBSCRIPTION_TYPE": "corporate"
   .
   ```
- `make clean`
- `make generate`
- `make acr-login`
- `make aks-create`
- `make tilt-up`

Once the tilt UI is up and running
- Click on the `allow required ports on mgmt cluster` task to allow the required ports on the management cluster's API server. 
   - Note: This task will wait for the NSG rules to be created and then update them to allow the required ports.
   - This task will take a few minutes to complete.
- Click on the flavors you want to deploy and CAPZ will deploy the workload cluster with the selected flavor.
   - Flavors that leverage internal load balancer are:
     - `apiserver-ilb`
     - `windows-apiserver-ilb`
