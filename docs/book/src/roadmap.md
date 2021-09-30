# Cluster API Azure Roadmap

This roadmap is a constant work in progress, subject to frequent revision. Dates are approximations. Features are listed in no particular order.

## v0.5 (v1alpha4) ~ Q1 2021

|Area|Description|Issue/Proposal|
|--|--|--|
|OS|Windows worker nodes|[#153](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/153)|
|Identity|Multi-tenancy within one manager instance|[#586](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/586)|
|UX|Bootstrap failure detection|[#603](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/603)|
|UX|Add tracing and metrics|[#311](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/311)|

## v1.0 v1beta1/v1 ~ Q2 2021

|Area|Description|Issue/Proposal|
|--|--|--|
|Network|Allow multiple subnets of role "node"|[#664](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/664)|
|Network|Azure Bastion hosts|[#165](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/165)|
|Identity|AAD Support|[#481](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/481)|
 ---

## Backlog

> Items within this category have been identified as potential candidates for the project
> and can be moved up into a milestone if there is enough interest.

|Area|Description|Issue/Proposal|
|--|--|--|
|Network|Peering of Cluster VNet to Existing VNet|[#532](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/532)|
|OS|Flatcar Support|[#629](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/629)|
|Compute|SGX-enabled VMs|[#488](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/488)|
|Compute|Azure Dedicated hosts|[#675](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/675)|
|AKS|Integrate AzureMachine with AzureManagedControlPlane|[#826](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/826)|
|Cloud Provider|Use Out of Tree cloud-controller-manager and Storage Drivers|[#715](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/715)|

