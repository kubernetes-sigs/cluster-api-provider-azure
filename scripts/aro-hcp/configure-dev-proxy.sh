#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

if [[ -z "${ARO_HCP_DEV_ENDPOINT:-}" ]]; then
	echo "ARO_HCP_DEV_ENDPOINT is not set; skipping ARO HCP dev proxy setup"
	exit 0
fi

: "${CLUSTER_API_INSTALLER_DIR:?Set CLUSTER_API_INSTALLER_DIR to the cluster-api-installer checkout}"
: "${KIND_CLUSTER_NAME:?Set KIND_CLUSTER_NAME to the CAPZ E2E management cluster name}"

if [[ "${ARO_HCP_DEV_ENDPOINT}" != http://* && "${ARO_HCP_DEV_ENDPOINT}" != https://* ]]; then
	echo "ARO_HCP_DEV_ENDPOINT must use http:// or https://, for example http://172.18.0.1:8443" >&2
	exit 1
fi

installer_dir="$(cd "${CLUSTER_API_INSTALLER_DIR}" && pwd)"
chart_dir="${installer_dir}/charts/aro-mockup-proxy-kind"
deploy_charts="${installer_dir}/scripts/deploy-charts.sh"
context="kind-${KIND_CLUSTER_NAME}"

[[ -f "${chart_dir}/Chart.yaml" ]] || { echo "ARO HCP mock proxy chart not found: ${chart_dir}" >&2; exit 1; }
[[ -x "${deploy_charts}" ]] || { echo "Installer deploy script not found: ${deploy_charts}" >&2; exit 1; }
command -v helm >/dev/null || { echo "helm is required to render the ARO HCP mock proxy chart" >&2; exit 1; }
command -v kubectl >/dev/null || { echo "kubectl is required to configure the ARO HCP mock proxy" >&2; exit 1; }

[[ -n "${KUBECONFIG:-}" ]] || { echo "KUBECONFIG must point to the CAPZ E2E management cluster" >&2; exit 1; }

kubectl --context="${context}" -n capz-system wait --for=condition=Ready issuer/azureserviceoperator-selfsigned-issuer --timeout=5m

echo "Deploying ARO HCP mock proxy to ${context}; forwarding cluster operations to ${ARO_HCP_DEV_ENDPOINT}"
helm template aro-mockup-proxy "${chart_dir}" --include-crds --namespace capz-system \
	--set-string "config.devEndpoint=${ARO_HCP_DEV_ENDPOINT}" \
	--set kubeconfig.secretName= \
	--set persistence.enabled=false \
	--set "Release.Namespace=capz-system" \
	| kubectl --context="${context}" -n capz-system apply -f - --server-side --force-conflicts

kubectl --context="${context}" -n capz-system wait --for=condition=Ready certificate/aro-mockup-proxy-tls --timeout=5m
kubectl --context="${context}" -n capz-system rollout status deployment/aro-mockup-proxy --timeout=5m

# Reuse the installer script's ASO/CAPZ endpoint and CA configuration without
# applying its CAPI or CAPZ charts over the manifests installed by clusterctl.
(
	cd "${installer_dir}"
	USE_KIND=true \
	KIND_CLUSTER_NAME="${KIND_CLUSTER_NAME}" \
	INIT_KIND=false \
	DO_DEPLOY=false \
	DO_CHECK=false \
	ARO_NULL_PROVISIONING=true \
	DEV_ENDPOINT="${ARO_HCP_DEV_ENDPOINT}" \
	./scripts/deploy-charts.sh aro-mockup-proxy cluster-api-provider-azure
)

kubectl --context="${context}" -n capz-system rollout status deployment/azureserviceoperator-controller-manager --timeout=5m
kubectl --context="${context}" -n capz-system rollout status deployment/capz-controller-manager --timeout=5m

echo "Waiting for the CAPZ webhook service to have a ready endpoint"
webhook_endpoints=""
for _ in $(seq 1 60); do
	webhook_endpoints="$(kubectl --context="${context}" -n capz-system get endpoints capz-webhook-service -o jsonpath='{.subsets[*].addresses[*].ip}')"
	if [[ -n "${webhook_endpoints}" ]]; then
		break
	fi
	sleep 1
done
[[ -n "${webhook_endpoints}" ]] || {
	kubectl --context="${context}" -n capz-system get endpoints capz-webhook-service -o yaml >&2
	echo "CAPZ webhook service has no ready endpoints after 60 seconds" >&2
	exit 1
}
echo "CAPZ webhook service endpoints are ready: ${webhook_endpoints}"

echo "ARO HCP dev endpoint proxy is ready"
