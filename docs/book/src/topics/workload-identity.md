# Workload Identity

Azure AD Workload identity is the next iteration of Azure AD Pod identity 
that enables Kubernetes applications (e.g. CAPZ) to access Azure cloud 
resources securely with Azure Active Directory.

This document describes a quick start guide of using workload identity and 
assumes that you have access to Azure cloud.

Workload identity is currently worked upon and cloud provider azure 
integration is in progress. Please refer to [this](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/3589) issue for details.
For more information, please refer to the [proposal](https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/main/docs/proposals/20221611-workload-identity-integration.md)

## Workload Identity Quick Start Guide

### Setting Up Management Cluster on Kind

- Create a public and private key pair. For example, you can generate the 
  key pairs using OpenSSL.

  Generate a private key called `sa.key` using the following command:
```bash
$ openssl genrsa -out sa.key 2048
```

Set the environment variable `SERVICE_ACCOUNT_SIGNING_KEY_FILE` to the path of the
generated `sa.key`. This ENV var will be used in the upcoming step. 
Note: You can use `readlink -f sa.key` to get the absolute path of the key file.

Generate a public key using the private key.
```bash
$ openssl rsa -in sa.key -pubout -out sa.pub
```
Set the environment variable `SERVICE_ACCOUNT_KEY_FILE` to the path of the
generated `sa.pub`. This ENV var will be used in the upcoming step.

- Create and upload Discovery and JWKS document using this [link](https://azure.github.io/azure-workload-identity/docs/installation/self-managed-clusters/oidc-issuer.html)

- At this stage, you will need to create a federated identity credential.
  - You can create that either with Azure AD application or user-assigned
    identity. Please note that user assigned identity will need to be created
    regardless because cloud provider azure integration is not yet done. The
    steps are mentioned in the next section of workload cluster creation.
  - The next list items links to steps on creating the federated
    identity credential. You will need to set up several environment
    variables:
    - `SERVICE_ACCOUNT_NAMESPACE` : Namespace where the capz-manager pod
      will run.
    - `SERVICE_ACCOUNT_NAME` : Name of the capz-manager k8s service account.
    - `SERVICE_ACCOUNT_ISSUER` : This is the path of the Azure storage
      container which you created in the previous step which is:
      `"https://${AZURE_STORAGE_ACCOUNT}.blob.core.windows.net/${AZURE_STORAGE_CONTAINER}/"`

  - Create a federated identity credential using the steps outlined [here](https://azure.github.io/azure-workload-identity/docs/topics/federated-identity-credential.html)
    You can either use `user-assigned-identity` or `AD application` to create federated identity credential and add `contributor` role to it.

- Create a Kind cluster with necessary flags with the following command:

```bash
cat <<EOF | kind create cluster --name azure-workload-identity --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraMounts:
    - hostPath: ${SERVICE_ACCOUNT_KEY_FILE}
      containerPath: /etc/kubernetes/pki/sa.pub
    - hostPath: ${SERVICE_ACCOUNT_SIGNING_KEY_FILE}
      containerPath: /etc/kubernetes/pki/sa.key
  kubeadmConfigPatches:
  - |
    kind: ClusterConfiguration
    apiServer:
      extraArgs:
        service-account-issuer: ${SERVICE_ACCOUNT_ISSUER}
        service-account-key-file: /etc/kubernetes/pki/sa.pub
        service-account-signing-key-file: /etc/kubernetes/pki/sa.key
    controllerManager:
      extraArgs:
        service-account-private-key-file: /etc/kubernetes/pki/sa.key
EOF
```

- Initialize a management cluster using `clusterctl` using the below command.
  If you do not have `clusterctl` installed, then follow this [link](https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl)
  to install.
```bash
$ clusterctl init --infrastructure azure
```

### Creating a Workload Cluster

- Create a user-assigned identity using the below steps:
  - [Create](https://learn.microsoft.com/en-gb/azure/active-directory/managed-identities-azure-resources/how-manage-user-assigned-managed-identities?pivots=identity-mi-methods-azp#create-a-user-assigned-managed-identity)
a user-assigned managed identity in Azure. Save its name which will be used later.
  - [Create a role assignment](https://learn.microsoft.com/en-us/azure/role-based-access-control/role-assignments-portal-managed-identity)
to give the identity Contributor access to the Azure subscription where the workload cluster will be created.

- Before generating a workload cluster YAML configuration set the
  following environment variables.
```bash
export AZURE_SUBSCRIPTION_ID=<your-azure-subscription-id>
# This is the client ID of the AAD app or user-assigned identity that you used to created the federated identity.
export AZURE_CLIENT_ID=<your-azure-client-id>
export AZURE_TENANT_ID=<your-azure-tenant-id>
export AZURE_CONTROL_PLANE_MACHINE_TYPE="Standard_B2s"
export AZURE_NODE_MACHINE_TYPE="Standard_B2s"
export AZURE_LOCATION="eastus"

# Identity secret. Though these are not used in workload identity, we still
# need to set them for the sake of generating the workload cluster YAML configuration
export AZURE_CLUSTER_IDENTITY_SECRET_NAME="cluster-identity-secret"
export CLUSTER_IDENTITY_NAME="cluster-identity"
export AZURE_CLUSTER_IDENTITY_SECRET_NAMESPACE="default"
```
- Generate a workload cluster template using the following command.

```bash
clusterctl generate cluster azwi-quickstart --kubernetes-version v1.27.3  --worker-machine-count=3 > azwi-quickstart.yaml
```

- Edit the generated `azwi-quickstart.yaml` to make the following changes for
  workload identity to the `AzureClusterIdentity` object.
  - Change the type to `WorkloadIdentity`.
  - Remove the `clientSecret` spec.

The AzureClusterIdentity specification should look like the following.
```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureClusterIdentity
metadata:
  name: cluster-identity
spec:
  type: WorkloadIdentity
  allowedNamespaces:
    list:
    - <cluster-namespace>
  tenantID: <your-tenant-id>
  clientID: <your-client-id>
```

- Change the `AzureMachineTemplate` for both control plane and worker to include user-assigned-identity by
  adding the following in its `spec`.
```yaml
      identity: UserAssigned
      userAssignedIdentities:
      - providerID: /subscriptions/${AZURE_SUBSCRIPTION_ID}/resourceGroups/${RESOURCE_GROUP}/providers/Microsoft.ManagedIdentity/userAssignedIdentities/${USER_ASSIGNED_IDENTITY_NAME}
```
A sample `AzureMahineTemplate` after the edit should look like the below:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: AzureMachineTemplate
metadata:
  name: ${CLUSTER_NAME}-md-0
  namespace: default
spec:
  template:
    spec:
      osDisk:
        diskSizeGB: 128
        osType: Linux
      sshPublicKey: ${AZURE_SSH_PUBLIC_KEY_B64:=""}
      identity: UserAssigned
      userAssignedIdentities:
      - providerID: /subscriptions/${AZURE_SUBSCRIPTION_ID}/resourceGroups/${RESOURCE_GROUP}/providers/Microsoft.ManagedIdentity/userAssignedIdentities/${USER_ASSIGNED_IDENTITY_NAME}
      vmSize: ${AZURE_NODE_MACHINE_TYPE}
```

- At this stage, you can apply this yaml to create a workload cluster.

Notes:
- Please follow this [link](https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/main/templates/test/ci/cluster-template-prow-workload-identity.yaml)
to see a workload cluster yaml configuration that uses workload identity.
- Creating a workload cluster via workload identity will be
  simplified after [this](https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/3589) issue is resolved.

## Debugging

### No matching federated identity record found

If you see logs like below, double check if the service account URL is exactly same on apiserver as
that in the federated credential.
```bash
"error": "invalid_request",
"error_description": "AADSTS70021: No matching federated identity record found for presented assertion. Assertion
```

### Authorization failed when using user-assigned identity

If you see error message similar to the following in the `AzureCluster` object,
this can be because the user-assigned identity does not have required permission.

```bash
Message: group failed to create or update. err: failed to get existing resource
demo/demo(service: group): resources.GroupsClient#Get: Failure
responding to request: StatusCode=403 -- Original Error: autorest/azure:
Service returned an error. Status=403 Code="AuthorizationFailed"
Message="The client '<id-redacted>' with object id '<id-redacted>' does not have
authorization to perform action 'Microsoft.Resources/subscriptions/resourcegroups/read'
over scope '/subscriptions/<sub-id-redacted>/resourcegroups/ashu-test' or the
scope is invalid. If access was recently granted, please refresh your
credentials.". Object will be requeued after 15s
```

Add `contributor` role to the user-assigned identity and this should fix it.
