# AAD Integration

CAPZ can be configured to use Azure Active Directory (AD) for user authentication. In this configuration, you can log into a CAPZ cluster using an Azure AD token. Cluster operators can also configure Kubernetes role-based access control (Kubernetes RBAC) based on a user's identity or directory group membership.

<aside class="note warning">

<h1> Warning </h1>

The following operations to configure Azure AD applications must be completed by an Azure tenant administrator.

</aside>

## Create Azure AD server component

### Create the Azure AD application

```bash
export CLUSTER_NAME=my-aad-cluster
```

```bash
export AZURE_SERVER_APP_ID=$(az ad app create \
    --display-name "${CLUSTER_NAME}Server" \
    --identifier-uris "https://${CLUSTER_NAME}Server" \
    --query appId -o tsv)
```

### Update the application group membership claims
```bash
az ad app update --id ${AZURE_SERVER_APP_ID} --set groupMembershipClaims=All
```

### Create a service principal
```bash
az ad sp create --id ${AZURE_SERVER_APP_ID}
```

## Create Azure AD client component
```bash
AZURE_CLIENT_APP_ID=$(az ad app create \
    --display-name "${CLUSTER_NAME}Client" \
    --native-app \
    --reply-urls "https://${CLUSTER_NAME}Client" \
    --query appId -o tsv)
```

### Create a service principal
```bash
az ad sp create --id ${AZURE_CLIENT_APP_ID}
```

### Grant the application API permissions
```bash
oAuthPermissionId=$(az ad app show --id ${AZURE_SERVER_APP_ID} --query "oauth2Permissions[0].id" -o tsv)
az ad app permission add --id ${AZURE_CLIENT_APP_ID} --api ${AZURE_SERVER_APP_ID} --api-permissions ${oAuthPermissionId}=Scope
az ad app permission grant --id ${AZURE_CLIENT_APP_ID} --api ${AZURE_SERVER_APP_ID}
```

## Create the cluster

To deploy a cluster with support for AAD, use the [aad flavor](https://raw.githubusercontent.com/kubernetes-sigs/cluster-api-provider-azure/main/templates/cluster-template-aad.yaml).

Make sure that `AZURE_SERVER_APP_ID` is set to the ID of the server AD application created above.

### Get the admin kubeconfig
```bash
clusterctl get kubeconfig ${CLUSTER_NAME} > ./${CLUSTER_NAME}.kubeconfig
export KUBECONFIG=./${CLUSTER_NAME}.kubeconfig
```

## Create Kubernetes RBAC binding

Get the user principal name (UPN) for the user currently logged in using the az ad signed-in-user show command. This user account is enabled for Azure AD integration in the next step:

<aside class="note">

<h1> Note </h1>

If the user you grant the Kubernetes RBAC binding for is in the same Azure AD tenant, assign permissions based on the userPrincipalName. If the user is in a different Azure AD tenant, query for and use the objectId property instead.

</aside>

```bash
az ad signed-in-user show --query objectId -o tsv
```

Create a YAML manifest `my-azure-ad-binding.yaml`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: my-cluster-admin
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: your_objectId
```

Create the ClusterRoleBinding using the kubectl apply command and specify the filename of your YAML manifest:

```bash
kubectl apply -f my-azure-ad-binding.yaml
```

## Accessing the cluster

### Install kubelogin
kubelogin is a client-go credential (exec) plugin implementing Azure authentication. Follow the setup instructions [here](https://github.com/Azure/kubelogin/blob/master/README.md).

### Set the config user context
```bash
kubectl config set-credentials ad-user --exec-command kubelogin --exec-api-version=client.authentication.k8s.io/v1beta1 --exec-arg=get-token --exec-arg=--environment --exec-arg=$AZURE_ENVIRONMENT --exec-arg=--server-id --exec-arg=$AZURE_SERVER_APP_ID --exec-arg=--client-id --exec-arg=$AZURE_CLIENT_APP_ID --exec-arg=--tenant-id --exec-arg=$AZURE_TENANT_ID
kubectl config set-context ${CLUSTER_NAME}-ad-user@${CLUSTER_NAME} --user ad-user --cluster ${CLUSTER_NAME}
```

To verify it works, run:

```bash
kubectl config use-context ${CLUSTER_NAME}-ad-user@${CLUSTER_NAME}
kubectl get pods -A
```

You will receive a sign in prompt to authenticate using Azure AD credentials using a web browser. After you've successfully authenticated, the kubectl command should display the pods in the CAPZ cluster.

## Adding AAD Groups

To add a group to the admin role run:

```bash
AZURE_GROUP_OID=<Your Group ObjectID>
kubectl create clusterrolebinding aad-group-cluster-admin-binding --clusterrole=cluster-admin --group=${AZURE_GROUP_OID}
```

## Adding users

To add another user, create a additional role binding for that user:

```bash
USER_OID=<Your User ObjectID or UserPrincipalName>
kubectl create clusterrolebinding aad-user-binding --clusterrole=cluster-admin --user ${USER_OID}
```

You can update the cluster role bindings to suit your needs for that user or group. See the [default role bindings](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#default-roles-and-role-bindings) for more details, and the [general guide to Kubernetes RBAC](https://kubernetes.io/docs/reference/access-authn-authz/rbac/).

## Known Limitations

- The user must not be a member of more than 200 groups.
