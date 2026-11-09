# ARO HCP E2E

`cluster-template.yaml` is the 2026-09-01-preview CAPI template used by the ARO HCP lifecycle E2E. The test renders its Azure IDs, location, namespace, and cluster identity from the regular CAPZ E2E configuration. It creates the `AzureClusterIdentity` and the ASO per-resource credential Secret before applying the cluster.

The test creates and deletes a temporary namespace for its resources. ASO uses Workload Identity and no client secret. ExternalAuth requires a pre-registered Microsoft Entra application; set `ARO_HCP_E2E_EXTERNAL_AUTH_CLIENT_ID` to its client ID.

## Run against the production ARO HCP endpoint

Use an ARO HCP-supported production region and an Azure subscription with the permissions required by CAPZ E2E and ARO HCP:

```bash
export AZURE_SUBSCRIPTION_ID=<subscription-id>
export AZURE_TENANT_ID=<tenant-id>
export AZURE_LOCATION=uksouth
export CAPZ_GALLERY_LOCATION=uksouth
export ARO_HCP_E2E_EXTERNAL_AUTH_CLIENT_ID=<external-auth-client-id>
az account set --subscription "$AZURE_SUBSCRIPTION_ID"
make test-e2e-aro-hcp
```

The target enables `EXP_ARO` and opts ASO into the additional ARO CRDs through `ADDITIONAL_ASO_CRDS`. It uses the published upstream ASO v2.22.0 image from the provider manifest. `ASO_IMAGE` can be set only when intentionally testing a different image that the Kind cluster can pull.

If the CAPZ E2E Workload Identity infrastructure and user-assigned identity already exist, set `AZURE_CLIENT_ID_USER_ASSIGNED_IDENTITY`; otherwise the normal E2E bootstrap creates them. The identity's federated credentials must trust the CAPZ manager and ASO service accounts used by the bootstrap. The identity must have the subscription permissions used by CAPZ E2E, and the ExternalAuth application must be configured for the target ARO HCP environment.
