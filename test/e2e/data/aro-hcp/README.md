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
encryption key. For ExternalAuth, use the same client ID as the installer:
`scripts/aro-hcp/gen.sh` sets `EA_AZURE_CLIENT_ID` from `AZURE_CLIENT_ID`. In its
service-principal path, `AZURE_CLIENT_ID` comes from the `.appId` field in
`sp-$AZURE_SUBSCRIPTION_ID.json`; `test-e2e.sh` reads that value by default. If
the installer uses its user-assigned identity path, set
`ARO_HCP_E2E_EXTERNAL_AUTH_CLIENT_ID` to the resolved `AZURE_CLIENT_ID`.

Run from the CAPZ repository root after setting the normal CAPZ E2E Azure
environment and logging in with `az login`:

```bash
export AZURE_SUBSCRIPTION_ID=...
export AZURE_TENANT_ID=...
export AZURE_LOCATION=...
export ARO_HCP_E2E_EXTERNAL_AUTH_CLIENT_ID=...
export ASO_IMAGE=localhost:5000/capz/azure-service-operator-rhel9:upstream
make test-e2e-aro-hcp
```

If `azureserviceoperator:dev` exists in Docker, the test tags it as `ASO_IMAGE`
and pushes it to Kind's local registry. The Kind node pulls it from
`localhost:5000`; no Quay push is needed. If you already pushed the image to
the local registry, the test uses that image directly.

For a local run using the developer's service-principal JSON to read the tenant
ID and the default test subscription/region, run `./test-e2e.sh` after setting
`ARO_HCP_E2E_EXTERNAL_AUTH_CLIENT_ID`. The script selects the subscription in
Azure CLI and invokes the E2E target. It defaults `ASO_IMAGE` to
`localhost:5000/capz/azure-service-operator-rhel9:upstream` and
uses the local `azureserviceoperator:dev` build when it is present. Override
`ASO_IMAGE` to select a different image:

```bash
ASO_IMAGE=quay.io/capz/azure-service-operator-rhel9:your-tag ./test-e2e.sh
```

Azure CLI must already be logged in with an account that can create the E2E
Workload Identity infrastructure. The `upstream` tag is mutable; use a pinned
tag or digest to reproduce a specific run.

If CAPZ's E2E Workload Identity infrastructure and user-assigned identity
already exist, set `AZURE_CLIENT_ID_USER_ASSIGNED_IDENTITY`; otherwise the
normal E2E bootstrap creates them and makes the identity ID available to the
test. For a supplied identity, its federated credentials must trust the CAPZ
manager and ASO service accounts used by the bootstrap. The identity must have
the subscription permissions used by CAPZ E2E, and the ExternalAuth application
must be configured for the target ARO HCP environment. No Azure client secret
is needed: the test uses Workload Identity.
