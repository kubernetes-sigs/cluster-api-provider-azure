# ARO HCP E2E test

`cluster-template.yaml` is the 2026-09-01-preview CAPI template used by the ARO
HCP lifecycle E2E. The test renders its Azure IDs, location, namespace, and
cluster identity from the regular CAPZ E2E configuration. It creates the
`AzureClusterIdentity` and the ASO per-resource credential Secret in the test
namespace before applying the cluster. No credential YAML or `kubectl create secret`
command needs to be run by hand: the test namespace is generated for each run,
and the test creates the Secret in that namespace before applying the cluster
template.

The Secret defaults to `aso-credentials` (override with
`ASO_CREDENTIAL_SECRET_NAME`) and contains the subscription ID, tenant ID, the
configured user-assigned identity's client ID, and `workloadidentity` as the
ASO authentication mode. It contains no client secret. The Secret and
`AzureClusterIdentity` are removed with the temporary namespace during test
cleanup.

The ASO Secret uses the E2E workload identity. CAPZ uses the same configured
user-assigned identity through `AzureClusterIdentity` when managing the etcd
encryption key. For ExternalAuth, set
`ARO_HCP_E2E_EXTERNAL_AUTH_CLIENT_ID` to the same client ID configured for
ExternalAuth (the installer's `EA_AZURE_CLIENT_ID`, normally its
`AZURE_CLIENT_ID`).

## Run against the ARO HCP DEV environment

In one terminal, forward the development ARO HCP frontend to the host:

```bash
KUBECONFIG=<dev-ARO-HCP-kubeconfig> kubectl port-forward \
  --address 172.18.0.1 -n aro-hcp svc/aro-hcp-frontend 8443:8443
```

In another terminal, set the E2E credentials and DEV proxy settings, register
the subscription with the DEV frontend, and start the test:

```bash
export AZURE_SUBSCRIPTION_ID=...
export AZURE_TENANT_ID=...
export AZURE_LOCATION=westus3
export CAPZ_GALLERY_LOCATION=uksouth
export ARO_HCP_E2E_EXTERNAL_AUTH_CLIENT_ID=...
export ARO_HCP_DEV_ENDPOINT=http://172.18.0.1:8443
export CLUSTER_API_INSTALLER_DIR=/path/to/cluster-api-installer
export ASO_IMAGE=localhost:5000/capz/azure-service-operator-rhel9:upstream
az account set --subscription "$AZURE_SUBSCRIPTION_ID"
curl --fail-with-body --silent --show-error --request PUT \
  "${ARO_HCP_DEV_ENDPOINT%/}/subscriptions/${AZURE_SUBSCRIPTION_ID}?api-version=2.0" \
  --header 'Content-Type: application/json' \
  --data "{\"state\":\"Registered\",\"registrationDate\":\"now\",\"properties\":{\"tenantId\":\"${AZURE_TENANT_ID}\"}}"
make test-e2e-aro-hcp
```

The DEV run requires `ARO_HCP_DEV_ENDPOINT`; `AZURE_LOCATION` selects the
resource region (`westus3`), while `CAPZ_GALLERY_LOCATION` selects the
community gallery region (`uksouth`).

`ASO_IMAGE` selects the ASO image in the provider manifest. Make it available
to the Kind cluster before running the test. The `upstream` tag is mutable; use
a pinned tag or digest to reproduce a specific run.

The suite hook deploys `aro-mockup-proxy` into the Kind management cluster
after clusterctl initializes CAPI/CAPZ/ASO, then configures ASO and CAPZ to
trust and use the proxy. The proxy forwards ARO HCP cluster operations to the
development frontend; leave the port-forward running until the test ends.
When `ARO_HCP_DEV_ENDPOINT` is unset, the suite skips proxy setup and uses the
regular endpoint path. The tracked `scripts/aro-hcp/configure-dev-proxy.sh`
helper also exits successfully if invoked without the endpoint.
Other Azure resources created by CAPZ/ASO still use the configured Azure
subscription, so this mode does not make the E2E test a mock-only run.

## Run against the production ARO HCP endpoint

Use the production subscription and `uksouth`. Leave the DEV endpoint unset so
the suite uses the regular ARO HCP endpoint and does not install the mock proxy:

```bash
export AZURE_SUBSCRIPTION_ID=<production-subscription-id>
export AZURE_TENANT_ID=<tenant-id>
export AZURE_LOCATION=uksouth
export CAPZ_GALLERY_LOCATION=uksouth
export ARO_HCP_E2E_EXTERNAL_AUTH_CLIENT_ID=<external-auth-client-id>
unset ARO_HCP_DEV_ENDPOINT
az account set --subscription "$AZURE_SUBSCRIPTION_ID"
make test-e2e-aro-hcp
```

The account must have the permissions required by CAPZ E2E and ARO HCP. Keep
`ASO_IMAGE` set to an image that the Kind cluster can pull if overriding the
ASO image from the provider manifest.

If CAPZ's E2E Workload Identity infrastructure and user-assigned identity
already exist, set `AZURE_CLIENT_ID_USER_ASSIGNED_IDENTITY`; otherwise the
normal E2E bootstrap creates them and makes the identity ID available to the
test. For a supplied identity, its federated credentials must trust the CAPZ
manager and ASO service accounts used by the bootstrap. The identity must have
the subscription permissions used by CAPZ E2E, and the ExternalAuth application
must be configured for the target ARO HCP environment. No Azure client secret
is needed: the test uses Workload Identity.
