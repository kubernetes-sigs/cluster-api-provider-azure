# Container Networking Interface

This document aims to provide steps to configure your cluster using the below CNI solutions

- [Azure CNI v1](#azure-container-networking-interface-v1)

## Limitations

- We can only configure one subnet per control-plane node. Refer [CAPZ#3506](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/3506)

## Azure Container Networking Interface v1

As of writing this document, Azure CNI needs to be installed in the following steps below.

<!-- TODO : Do you use default Cluster template or create a new and reference it here? -->

<!-- TODO: Do we specify the number of IPs per nodes depending on the VM size? because Refer https://learn.microsoft.com/en-us/azure/aks/configure-azure-cni#maximum-pods-per-node -->

<!-- TODO: Do we specify the number of IPs per nodes depending on the VM size? As a general guideline, Microsoft recommends the following maximum number of pods per node for VM Standard_D2s_v3 and Standard_B2s using Azure CNI V1 in AKS: -->
<!-- Standard_D2s_v3: up to 30 pods per node -->
<!-- Standard_B2s: up to 10 pods per node -->