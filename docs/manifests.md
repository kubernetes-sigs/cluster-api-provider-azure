# Manifests

Cluster API Provider Azure provides several manifest templates to assist with provisioning Kubernetes clusters.

- `cluster.yaml.template`
- `cluster-network-spec.yaml.template`
- `controlplane-machine.yaml.template`
- `controlplane-machines-ha.yaml.template`
- `credentials.yaml.template`
- `machines.yaml.template`
- `machine-deployment.yaml.template`

After running `generate-yaml.sh`, the provided templates will be transformed into usable Kubernetes manifests.

You will not require all of the manifests to successfully create a cluster.
This will provide some guidance on which manifests to use and when.

## Cluster

### Default network configuration

Use `cluster.yaml`.

### (Unsupported) Existing virtual network

Use `cluster-network-spec.yaml`.


## Machine

### Single control plane, single node

Use `machines.yaml`.

### Single control plane, without nodes

Use `controlplane-machine.yaml`.

### Multi-node control plane

Use `controlplane-machines-ha.yaml`.

### Machine deployment (single replica)

**This manifest should only be applied to a cluster _after_ a control plane has been deployed.**

Use `machine-deployment.yaml`.
