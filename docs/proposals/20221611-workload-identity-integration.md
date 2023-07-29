```yaml
title: Workload Identity Integration
authors:
    - @sonasingh46
reviewers:
    - @aramase
    - @CecileRobertMichon
    - @yastij
    - @fabriziopandini 

creation-date: 2022-11-16
last-updated: 2023-05-23
status: implementable
see-also:
    - https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/2205
```

# Workload Identity Integration

## <a name='TableofContents'></a>Table of Contents

<!-- vscode-markdown-toc -->
* [Table of Contents](#TableofContents)
* [Acronyms](#Acronyms)
* [Summary](#Summary)
* [Motivation](#Motivation)
	* [Goals](#Goals)
	* [Future Goals](#FutureGoals)
* [Personas](#Personas)
* [User Stories](#UserStories)
* [Proposal](#Proposal)
	* [Implementation Details/Notes/Constraints](#ImplementationDetailsNotesConstraints)
		* [Key Generation](#KeyGeneration)
		* [OIDC URL Setup](#OidcUrlSetup)
		* [Set Service Account Signing Flags](#SetServiceAccountSigningFlags)
		* [Federated Credential](#FederatedCredential)
		* [Distribute Keys To Management Cluster](#DistributeKeys)
	* [Cloud Provider Azure Integration](#CloudProviderAzureIntegration)
	* [Proposed API Changes](#ProposedApiChanges)
	* [Proposed Deployment Configuration Changes](#ProposedConfigurationChanges)
	* [Proposed Controller Changes](#ProposedControllerChanges)
		* [Identity](#Identity)
	* [Open Questions](#OpenQuestions)
		* [1. How to achieve multi-tenancy?](#Howtomultitenancy)
		* [2. How to distribute key pair to management cluster?](#Howtodistributekeys)
		* [3. User Experience](#UserExperience)
	* [Migration Plan](#MigrationPlan)
	* [Test Plan](#TestPlan)
* [Implementation History](#ImplementationHistory)

<!-- vscode-markdown-toc-config
	numbering=false
	autoSave=false
	/vscode-markdown-toc-config -->
<!-- /vscode-markdown-toc -->

## <a name='Acronyms'></a>Acronyms
| Acronym      | Full Form               |
| ------------ | ------------------------|
| AD           | Active Directory        |
| AAD          | Azure Active Directory  |
| AZWI         | Azure Workload Identity |
| OIDC         | OpenID Connect          |
| JWKS         | JSON Web Key Sets       |

## <a name='Summary'></a>Summary

Workloads deployed in Kubernetes cluster may require Azure Active Directory application credential or managed identities to access azure protected resource e.g. Azure Key Vault, Virtual Machines etc. AAD Pod Identity helps access azure resources without the need of a secret management via Azure Managed Identities. AAD Pod Identity is now deprecated and Azure AD Workload Identity is the next iteration of the former. This design proposal aims to define the way for AZWI integration into capz for self managed clusters with keeping in mind other factor e.g. User Experience, Multi Tenancy, Backwards Compatibility etc.

For more details about AAD Pod Identity please visit this [link](https://github.com/Azure/aad-pod-identity)  

## <a name='Motivation'></a>Motivation

AZWI provides the capability to federate the identity with external identity providers in a Kubernetes native way. This approach overcomes several limitations of AAD Pod Identity as mentioned below.
- Removes scale and performance issues that existed for identity assignment.
- Supports K8s clusters hosted in any cloud or on premise.
- Supports both Linux and Windows workloads and removes the need for CRD and pods that intercept IMDS traffic.

From CAPZ perspective
- The AAD pod identity pod has to be deployed on all the nodes where the capz pod can be potentially scheduled.
- It has a dependency with the CNI and requires modifying iptables on the node.
- AAD pod identity is now deprecated. 

To learn more about AZWI please visit this link https://azure.github.io/azure-workload-identity/docs/introduction.html

### <a name='Goals'></a>Goals

- Enable CAPZ pod to use AZWI on the management cluster to authenticate to Azure to create/update/delete resources as part of workload cluster lifecycle management

### <a name='NonGoals'></a>Non Goals

- Use workload identity for cloud provider azure once supported.

- Migrate to using workload identity in CI pipelines.

- Automation for migration from AAD pod identity to workload identity.

- Bootstrapping components (storage account, discovery document and JWKS) and enabling WI in the workload cluster.

## <a name='Personas'></a>Personas

The following personas are available when writing user stories. 

- John - Cloud Admin
	- Installs, configures and maintains, management and workload clusters on Azure using CAPZ. 

## <a name='UserStories'></a>User Stories

- [S1] As a cloud admin I want to use workload identity in the management cluster in order to enhance security by not using the static azure credentials. I prefer to use CAPI pivoting to create management cluster which means creating a management cluster on Kind and then create a workload cluster on Azure from that and then convert the workload cluster to a management cluster.

- [S2] I am a cloud admin and I want to install CAP* on an existing Kubernetes cluster and make this a management cluster to create and manage workload clusters by using workload identity. For example, creating a management cluster on an already existing Kubernetes cluster on Azure.

- [S3] As a cloud admin I want to migrate to using workload identity for my management cluster which is still using older AAD pod identity.   

## <a name='Proposal'></a>Proposal

In AZWI model, Kubernetes cluster itself becomes the token issuer issuing tokens to Kubernetes service accounts. These service accounts can be configured to be trusted on Azure AD application or user assigned managed identity. Workload pods can use this service account token which is projected to it's volume and can exchange the projected service account token for an Azure AD access token.

The first step for creating a Kubernetes cluster using capz is creating a management cluster. On a high level the workflow looks the following to be able to use AZWI in the management cluster.

**Management Cluster on Kind**

Notes: 
- Often, management cluster created on Kind is also termed as Bootstrap cluster.
- Cloud Provider azure deployment is not required in this case.
- This is also used in CAPI pivoting i.e creating a management cluster on Kind and then later creating a workload cluster from it and again this workload cluster is then converted to a management cluster and the Kind cluster is decommissioned. Refer to user story S1(#UserStories). 

User Workflow:

- The operator/admin generates signing key pair or BYO key pair. 

- A Kind cluster is created with appropriate flags on kube-apiserver and kube-controller-manager and the key pairs are mounted on the container path for control plane node. See [this](#set-service-account-signing-flags) section for more details on this.
	- kube-apiserver flags
		- --service-account-issuer
		- --service-account-signing-key-file
		- --service-account-key-file
	- kube-controller-manager
		- --service-account-private-key-file

- The operator/admin uploads the following two documents in a blob storage container. These documents are accessible publicly on a URL and this URL is commonly referred as issuer URL in this context. More about this [here](https://azure.github.io/azure-workload-identity/docs/installation/self-managed-clusters/oidc-issuer.html)  
  - Generate and upload the Discovery Document.
  - Generate and upload the JWKS Document.

- A federated identity credential should be created between the identity and <Issuer URL, service account namespace, service account name>. More on this [here](https://azure.github.io/azure-workload-identity/docs/topics/federated-identity-credential.html)

- CAPI and CAPZ are deployed on the Kind cluster that supports workload identity.

- CAPZ pod uses the client ID and tenant ID of the Azure AD or user-assigned Identity by passing it in AzureClusterIdentity CR. The AzureClusterIdentity also has a capability to specify type of identity as `WorkloadIdentity` on the field `type`. 

- The management cluster is now configured to use workload identity. A workload cluster can now be created by referencing the AzureClusterIdentity.


**Management Cluster on Azure Via Pivoting**

Notes:
- Cloud provider azure will be required to run on Kubernetes cluster in this case as the management cluster is created on Azure cloud.

User Workflow:

- All the steps are followed as described above in `Management Cluster on Kind` with a exception that a secret is created with name `<cluster-name>-sa` encompassing the key pairs that is generated in the previous step. This is done so that the key pairs gets distributed on the control plane node. More details on it [here](https://cluster-api.sigs.k8s.io/tasks/certs/using-custom-certificates.html)

- A workload cluster is created using Azure static credentials or aad pod identity. 

- After the workload cluster is created, to convert it into a management cluster `clusterctl init` and `clusterctl move` commands are executed. More on this [here](https://cluster-api.sigs.k8s.io/clusterctl/commands/move.html?highlight=pivot#bootstrap--pivot)

- The workload cluster is now converted into a management cluster but still using static credentials. A migration step can be followed to migrate to using workload identity.

### <a name='ImplementationDetailsNotesConstraints'></a>Implementation Details/Notes/Constraints

- AAD pod identity can co exist with AZWI
- Migration plan to AZWI for existing cluster. Refer to the [MigrationPlan](#migration-plan) section at the bottom of the document.
- For AZWI to work the following prerequisites must be met for self managed cluster. This is not required for managed cluster and follow this [link](https://azure.github.io/azure-workload-identity/docs/installation/managed-clusters.html) to know more about managed cluster setup.
  - Key Generation
  - OIDC URL Setup
  - Set Service Account Signing Flags

#### <a name='KeyGeneration'></a>Key Generation

Admin should generate signing key pairs by using a tool such as openssl or bring their own public and private keys. 
These keys will be mounted on a path on the containers running on the control plane node. These keys are required for signing the service account tokens that will be used by the capz pod. 

#### <a name='OidcUrlSetup'></a>OIDC URL Setup

Two documents i.e Discovery and JWKS json documents needs to be generated and published to a public URL. The OIDC discovery document contains the metadata of the issuer. The JSON Web Key Sets (JWKS) document contains the public signing key(s) that allows AAD to verify the authenticity of the service account token.

Refer to [link](https://azure.github.io/azure-workload-identity/docs/installation/self-managed-clusters/oidc-issuer.html) for steps to setup and OIDC issuer URL.

The steps on a high level to setup is the following
- Create an azure blob storage account.
- Create a storage container.
- Generate the OIDC and JWKS document.
- The document should be accessible on the public accessible URL which will be used later.

#### <a name='SetServiceAccountSigningFlags'></a>Set Service Account Signing Flags

Setup the flags on the kind cluster. An example is shown below

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraMounts:
      # path on node where the public key exists
    - hostPath: ${SERVICE_ACCOUNT_KEY_FILE}
      containerPath: /etc/kubernetes/pki/sa.pub
      # path on node where the private key exists
    - hostPath: ${SERVICE_ACCOUNT_SIGNING_KEY_FILE}
      containerPath: /etc/kubernetes/pki/sa.key
  kubeadmConfigPatches:
  - |
    kind: ClusterConfiguration
    apiServer:
      extraArgs:
        # the oidc url after it has been set up
        service-account-issuer: ${SERVICE_ACCOUNT_ISSUER}
        service-account-key-file: /etc/kubernetes/pki/sa.pub
        service-account-signing-key-file: /etc/kubernetes/pki/sa.key
    controllerManager:
      extraArgs:
        service-account-private-key-file: /etc/kubernetes/pki/sa.key
```

#### <a name='FederatedCredential'></a>Federated Credential

A federated identity should be created using the azure cli ( or via the azure portal).
Please see [this](https://azure.github.io/azure-workload-identity/docs/topics/federated-identity-credential.html) for reference.

```bash
az identity federated-credential create \
  --name "kubernetes-federated-credential" \
  --identity-name "${USER_ASSIGNED_IDENTITY_NAME}" \
  --resource-group "${RESOURCE_GROUP}" \
  --issuer "${SERVICE_ACCOUNT_ISSUER}" \
  --subject "system:serviceaccount:${SERVICE_ACCOUNT_NAMESPACE}:${SERVICE_ACCOUNT_NAME}"
```

#### <a name='DistributeKeys'></a>Distribute Keys

Key pair can be distributed to a workload cluster by creating a secret with the name as `<cluster-name>-sa` encompassing the key pair. 
Follow this [link](https://cluster-api.sigs.k8s.io/tasks/certs/using-custom-certificates.html) for more details.

### <a name='CloudProviderAzureIntegration'></a>Cloud Provider Azure Integration

- Cloud provider azure should be deployed with projected service account token config in the config YAML.

- CAPZ should create cloud config by setting the following values if workload identity is used. 
```go
	AADFederatedTokenFile string `json:"aadFederatedTokenFile,omitempty" yaml:"aadFederatedTokenFile,omitempty"`
	UseFederatedWorkloadIdentityExtension bool `json:"useFederatedWorkloadIdentityExtension,omitempty" yaml:"useFederatedWorkloadIdentityExtension,omitempty"`
```


### <a name='ProposedApiChanges'></a>Proposed API Changes

The AzureClusterIdentity spec has a `Type` field that can be used to define what type of Azure identity should be used.

```go
// AzureClusterIdentitySpec defines the parameters that are used to create an AzureIdentity.
type AzureClusterIdentitySpec struct {
	// Type is the type of Azure Identity used.
	// ServicePrincipal, ServicePrincipalCertificate, UserAssignedMSI or ManualServicePrincipal.
	Type IdentityType `json:"type"`

	// ...
	// ...
}
```

- Introducing one more acceptable value for `Type` in AzureClusterIdentity spec for workload identity is proposed.

```go
// IdentityType represents different types of identities.
// +kubebuilder:validation:Enum=ServicePrincipal;UserAssignedMSI;ManualServicePrincipal;ServicePrincipalCertificate;WorkloadIdentity
type IdentityType string

const (
	// UserAssignedMSI represents a user-assigned managed identity.
	UserAssignedMSI IdentityType = "UserAssignedMSI"

	// ServicePrincipal represents a service principal using a client password as secret.
	ServicePrincipal IdentityType = "ServicePrincipal"

	// ManualServicePrincipal represents a manual service principal.
	ManualServicePrincipal IdentityType = "ManualServicePrincipal"

	// ServicePrincipalCertificate represents a service principal using a certificate as secret.
	ServicePrincipalCertificate IdentityType = "ServicePrincipalCertificate"
	
	//[Proposed Change] WorkloadIdentity represents  azure workload identity.
	WorkloadIdentity IdentityType = "WorkloadIdentity"
)

```
- Making the field `type` on AzureClusterIdentity immutable.

### <a name='ProposedConfigurationChanges'></a>Proposed Deployment Configuration Changes

- Service account token projected volume and volume mount config should be added in the CAPZ manager deployment config as described below: 

```yaml
          volumeMounts:
            - mountPath: /var/run/secrets/azure/tokens
              name: azure-identity-token
              readOnly: true
...
      volumes:
      - name: azure-identity-token
        projected:
          defaultMode: 420
          sources:
          - serviceAccountToken:
              audience: api://AzureADTokenExchange
              expirationSeconds: 3600
              path: azure-identity-token
```

### <a name='ProposedControllerChanges'></a>Proposed Controller Changes

The identity code workflow in capz should use `azidentity` module to exchange token from AAD as displayed in the next section.


#### <a name='Identity'></a>Identity

Azure client and tenant ID are injected as env variables by the azwi webhook. But for the azwi workflow, client ID and tenant ID will be fetched from AzureClusterIdentity first and will use the env variables as a fallback option. 

Following is a sample code that should be made into capz identity workflow.

```go

			// see the next code section for details on this function
			cred, err := newWorkloadIdentityCredential(tenantID, clientID, tokenFilePath, wiCredOptions)
			if err != nil {
				return nil, errors.Wrap(err, "failed to setup workload identity")
			}

			client := subscriptions.NewClient()

			// setCredentialsForWorkloadIdentity just setups the 
			// PublicCloud env URLs 
			params.AzureClients.setCredentialsForWorkloadIdentity(ctx, params.AzureCluster.Spec.SubscriptionID, params.AzureCluster.Spec.AzureEnvironment)
			client.Authorizer = azidext.NewTokenCredentialAdapter(cred, []string{"https://management.azure.com//.default"})
			params.AzureClients.Authorizer = client.Authorizer

```

**NOTE:**
`azidext.NewTokenCredentialAdapter` is used to get a authorizer in order to add to the existing code workflow to adapt an azcore.TokenCredential type to an autorest.Authorizer type.

Also a go file e.g `workload_identity.go` in the `identity` package dealing with AZWI functionality.

```go

type workloadIdentityCredential struct {
	assertion string
	file      string
	cred      *azidentity.ClientAssertionCredential
	lastRead  time.Time
}

type workloadIdentityCredentialOptions struct {
	azcore.ClientOptions
}

func newWorkloadIdentityCredential(tenantID, clientID, file string, options *workloadIdentityCredentialOptions) (*workloadIdentityCredential, error) {
	w := &workloadIdentityCredential{file: file}
	cred, err := azidentity.NewClientAssertionCredential(tenantID, clientID, w.getAssertion, &azidentity.ClientAssertionCredentialOptions{ClientOptions: options.ClientOptions})
	if err != nil {
		return nil, err
	}
	w.cred = cred
	return w, nil
}

func (w *workloadIdentityCredential) GetToken(ctx context.Context, opts policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return w.cred.GetToken(ctx, opts)
}

func (w *workloadIdentityCredential) getAssertion(context.Context) (string, error) {
	if now := time.Now(); w.lastRead.Add(5 * time.Minute).Before(now) {
		content, err := os.ReadFile(w.file)
		if err != nil {
			return "", err
		}
		w.assertion = string(content)
		w.lastRead = now
	}
	return w.assertion, nil
}

```

### <a name='OpenQuestions'></a>Open Questions

#### <a name='Howtomultitenancy'></a>1. How to achieve multi-tenancy?

The identity is tied to the client ID which can be supplied via AzureClusterIdentity.

#### <a name='Howtodistributekeys'></a>2. How to distribute key pair to management cluster?

Keys can be distributed by using the CABPK feature by creating a secret with name `<cluster-name>-sa` encompassing the key pair. More details on it [here](https://cluster-api.sigs.k8s.io/tasks/certs/using-custom-certificates.html)

#### <a name='UserExperience'></a>3. User Experience

Though AZWI has a lot of advantages as compared to AAD pod identity, setting up AZWI involves couple of manual step for self managed clusters and it can impact the user experience. Though, for managed clusters the configurations steps are not required.

### <a name='MigrationPlan'></a>Migration Plan

Management clusters using AAD pod identity should have a seamless migration process which is well documented.

For migrating an existing cluster to use AZWI following steps should be taken.

**Migrating Greenfield Clusters**
- The steps in this section applies to greenfield cluster creation that uses CAPI pivoting to use workload identity. As in this case the key pairs are already distributed to the control plane nodes.

- Create a `UserAssignedIdentity` and create a new machine template to use user assigned identity. This step will become optional once cloud provider azure supports workload identity.

- Patch the control plane to include the following flag on the api server.
	- `service-account-issuer: <value-is-service-account-issuer-url>

- Perform node rollout to propagate the changes by patching KCP and MachineDeployment objects to include the reference on the new AzureMachineTemplate created in the previous step.

- Create a new `AzureClusterIdentity` and specify the client ID, tenant ID and `type` to `WorkloadIdentity`.

- Patch AzureCluster to use the new `AzureClusterIdentity`.

**Migrating Brownfield Clusters**

- The steps in this section applies to existing clusters. 

- Generate a key pair or use the same key pairs as present in `/etc/kubernetes/pki`

- Perform the following pre-requistes as discussed earlier in the **Bootstrap Cluster** section of the [document](#proposal):
	- Generate and upload the JWKS and Discovery Document.
	- Install the AZWI mutating admission webhook controller.
	- Establish federated credential for the identity and `<Issuer, service account namespace, service account name>`.

- If you are not using the existing key pairs then perform a service account rotation using this guide. 
https://azure.github.io/azure-workload-identity/docs/topics/self-managed-clusters/service-account-key-rotation.html

- If you are using the existing key pairs or if you just configure workload identity using the keys present in /etc/kubernetes/pki then rotation is not required.
 	 
- Create a `UserAssignedIdentity` and create a new machine template to use user assigned identity. This step will become optional once cloud provider azure supports workload identity.

- This step is not required if you are using the existing key pairs. Patch the control plane to include the following flags on the api server.
	- `service-account-key-file:<public-key-path-on-controlplane-node>`
	- `service-account-signing-key-file:<private-key-path-on-controlplane-node>`
	- **Note:** The key pairs are present in `/etc/kubernetes/pki` directory if using the `<cluster-name>-sa` key pair.

- This step is not required if you are using the existing key pairs. Patch the control plane to include the following flags on the kube controller manager.
	- `service-account-signing-key-file:<private-key-path-on-controlplane-node>`

- Patch the control plane to include the following flag on the api server.
	- `service-account-issuer: <value-is-service-account-issuer-url>`

- Perform node rollout to propagate the changes by patching KCP and MachineDeployment objects to include the reference on the new AzureMachineTemplate created in the previous step.

- Upgrade to the CAPZ version that supports AZWI. 

- Create a new `AzureClusterIdentity` and specify the client ID, tenant ID and `type` to `WorkloadIdentity`.

- Patch AzureCluster to use the new `AzureClusterIdentity`.

**Note:** Migration to AZWI is a tedious manual process and poses risk of human error. It should be better done via an automation.

### <a name='TestPlan'></a>Test Plan

* Unit tests to validate newer workload identity functions and helper functions.  
* Using AZWI for existing e2e tests for create, upgrade, scale down/up, and delete.

## <a name='ImplementationHistory'></a>Implementation History

- Enabling workload identity feature in CAPZ
Ref Issue: https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/3588
Ref PR: https://github.com/kubernetes-sigs/cluster-api-provider-azure/pull/3583

- [ToDo]Integrating with cloud provider azure to use workload identity. 
Ref Issue: https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/3589

- [ToDo] Moving all the CI jobs to be using workload identity.
Ref Issue: https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/3590
