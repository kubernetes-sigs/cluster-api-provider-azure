# Workload Identity

Azure AD Workload identity is the next iteration of Azure AD Pod identity
that enables Kubernetes applications such as CAPZ to access Azure cloud
resources securely with Azure Active Directory.

Let's help you get started using workload identity. We assume
you have access to Azure cloud services.

## Quick start

### Set up a management cluster with kind

- Create a private and public key pair. For example, using OpenSSL:

  ```bash
  openssl genrsa -out sa.key 2048
  openssl rsa -in sa.key -pubout -out sa.pub
  ```

  Set the environment variable `SERVICE_ACCOUNT_SIGNING_KEY_FILE` to the full path
  of the `sa.key` private key file you just generated, and set `SERVICE_ACCOUNT_KEY_FILE`
  to the generated `sa.pub` public key file.

  ```bash
  export SERVICE_ACCOUNT_SIGNING_KEY_FILE=$(realpath sa.key)
  export SERVICE_ACCOUNT_KEY_FILE=$(realpath sa.pub)
  ```

  These environment variables will be used later, when creating the kind cluster.

- Create and upload a discovery document

  Create and upload a JWKS discovery document by following [these instructions](https://azure.github.io/azure-workload-identity/docs/installation/self-managed-clusters/oidc-issuer.html).

- Create two federated identity credentials

  Export environment variables used for creating a federated identity credential:

  - `SERVICE_ACCOUNT_NAMESPACE`: Namespace where the capz-manager and
    azureserviceoperator-controller-manager pods will run. Default is `capz-system`.
  - `SERVICE_ACCOUNT_NAME`: Name of the capz-manager or azureserviceoperator-default k8s service account. Default is `capz-manager` for CAPZ and `azureserviceoperator-default` for ASO.
  - `SERVICE_ACCOUNT_ISSUER`: Path of the Azure storage container created in the previous step, specifically:
    - `"https://${AZURE_STORAGE_ACCOUNT}.blob.core.windows.net/${AZURE_STORAGE_CONTAINER}/"`

  Create two federated identity credentials, one for CAPZ and one for ASO, by following [these instructions](https://azure.github.io/azure-workload-identity/docs/topics/federated-identity-credential.html). You'll need to set `SERVICE_ACCOUNT_NAME` and `SERVICE_ACCOUNT_NAMESPACE` to different values for each credential.
  Use either `user-assigned-identity` or `AD application` when creating the credentials, and add the `contributor` role to each.

- Create a kind cluster with the following command:

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

- Initialize the kind cluster as a CAPZ management cluster using `clusterctl`:

  ```bash
  clusterctl init --infrastructure azure
  ```

  If you don't have `clusterctl` installed, follow [these instructions](https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl)
  to install it.

### Creating a Workload Cluster

- Create a user-assigned identity using the below steps:
  - [Create](https://learn.microsoft.com/en-gb/azure/active-directory/managed-identities-azure-resources/how-manage-user-assigned-managed-identities?pivots=identity-mi-methods-azp#create-a-user-assigned-managed-identity)
a user-assigned managed identity in Azure. Save its name which will be used later.
  - [Create a role assignment](https://learn.microsoft.com/en-us/azure/role-based-access-control/role-assignments-portal-managed-identity)
to give the identity Contributor access to the Azure subscription where the workload cluster will be created.

- Before generating a workload cluster YAML configuration, set the
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
  workload identity to the `AzureClusterIdentity` object:

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

  A sample `AzureMachineTemplate` after the edit should look like the below:

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
