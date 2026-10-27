# Jobs

This document provides an overview of our jobs running via Prow and Github actions.

## Builds and Tests running on the default branch

<aside class="note">

<h1> Note </h1>

To see which test jobs execute which e2e tests, you can click on the links which lead to the respective test overviews in testgrid.

</aside>


### Legend

    🟢 REQUIRED - Jobs that have to run successfully to get the PR merged.


### Presubmits

  Prow Presubmits:

  - 🟢 [pull-cluster-api-provider-azure-build]  `./scripts/ci-build.sh`
  - 🟢 [pull-cluster-api-provider-azure-test]  `./scripts/ci-test.sh`
  - 🟢 [pull-cluster-api-provider-azure-e2e]
       * `GINKGO_FOCUS="Workload cluster creation" GINKGO_SKIP="Creating a GPU-enabled cluster|.*Windows.*|.*AKS.*|Creating a cluster that uses the external cloud provider" ./scripts/ci-e2e.sh`
  - 🟢 [pull-cluster-api-provider-azure-windows]
       * `GINKGO_FOCUS=".*Windows.*" GINKGO_SKIP="" ./scripts/ci-e2e.sh`
  - 🟢 [pull-cluster-api-provider-azure-verify]   `make verify`
  - [pull-cluster-api-provider-azure-e2e-exp]
       * `GINKGO_FOCUS=".*AKS.*" GINKGO_SKIP="" ./scripts/ci-e2e.sh`
  - [pull-cluster-api-provider-azure-apidiff]  `./scripts/ci-apidiff.sh`
  - [pull-cluster-api-provider-azure-coverage]  `./scripts/ci-test-coverage`
  - [pull-cluster-api-provider-e2e-full]
       * `GINKGO_FOCUS="Workload cluster creation" GINKGO_SKIP="" ./scripts/ci-e2e.sh`
  - [pull-cluster-api-provider-capi-e2e]
       * `GINKGO_FOCUS="Cluster API E2E tests" GINKGO_SKIP="" ./scripts/ci-e2e.sh`
  - [pull-cluster-api-provider-azure-conformance-v1beta1]  `./scripts/ci-conformance.sh`
  - [pull-cluster-api-provider-azure-upstream-v1beta1-windows]
       * `WINDOWS="true" CONFORMANCE_NODES="4" ./scripts/ci-conformance.sh`
  - [pull-cluster-api-provider-azure-conformance-with-ci-artifacts]
       * `E2E_ARGS="-kubetest.use-ci-artifacts" ./scripts/ci-conformance.sh`
  - [pull-cluster-api-provider-azure-windows-upstream-with-ci-artifacts]
       * `E2E_ARGS="-kubetest.use-ci-artifacts" WINDOWS="true" CONFORMANCE_NODES="4" ./scripts/ci-conformance.sh`
  - [pull-cluster-api-provider-azure-ci-entrypoint]
      * Validates cluster creation with `./scripts/ci-entrypoint.sh` - does not run any tests


  Github Presubmits Workflows:

  - Markdown-link-check  `find . -name \*.md | xargs -I{} markdown-link-check -c .markdownlinkcheck.json {}`


### Postsubmits
  
  Prow Postsubmits:
  
  - [post-cluster-api-provider-azure-push-images]  `/run.sh`
       * args: 
           - project=`k8s-staging-cluster-api-azure`
           - scratch-bucket=`gs://k8s-staging-cluster-api-azure-gcb`
           - env-passthrough=`PULL_BASE_REF`
  - [postsubmits-cluster-api-provider-azure-e2e-full-main]
       * `GINKGO_FOCUS="Workload cluster creation" GINKGO_SKIP="" ./scripts/ci-e2e.sh`


  Github Postsubmits Workflows:

  - Code-coverage-check  `make test-cover`


### Periodics
  
  Prow Periodics:
  
  - [periodic-cluster-api-provider-azure-conformance-v1beta1]  `./scripts/ci-conformance.sh`
  - [periodic-cluster-api-provider-azure-conformance-v1beta1-with-ci-artifacts]  `./scripts/ci-conformance.sh`
      * `E2E_ARGS="-kubetest.use-ci-artifacts" ./scripts/ci-conformance.sh`
  - [periodic-cluster-api-provider-azure-capi-e2e]
      * `GINKGO_FOCUS="Cluster API E2E tests" GINKGO_SKIP="" ./scripts/ci-e2e.sh`
  - [periodic-cluster-api-provider-azure-coverage] `bash` , `./scripts/ci-test-coverage.sh`
  - [periodic-cluster-api-provider-azure-e2e-full]
      * `GINKGO_FOCUS="Workload cluster creation" GINKGO_SKIP="" ./scripts/ci-e2e.sh`

## KubeRay coverage

The `[KubeRay]` scenarios run on both AKS (`aks-aso-kuberay`) and self-managed
Azure Kubernetes clusters (`ci-version`). Each scenario provisions an isolated
cluster and installs the pinned OSS KubeRay Helm chart once before exercising
the API contract, RayCluster readiness, or RayJob completion.

The shared Ray head spec pins `runtime-env-agent-port` to `52366`, distinct from
the dashboard agent's `52365`. Ray 2.56 starts Ray Client before the raylet
registers with GCS; discovering an automatically assigned agent port can exceed
the client's 30-second startup timeout and crash a slowly starting head. The
explicit port avoids that dependency without disabling Ray Client or relaxing
the workload readiness checks.

The `[KubeRayAPI]` scenario checks the `ray.io/v1` resources shipped by the pinned
chart: RayCluster, RayJob, RayService, and RayCronJob. It verifies discovery
(kind, namespace scope, and CRUD verbs), served CRD schemas and baseline field
types, persisted create/read/update field round-trips, and deletion through
`NotFound`. It also submits an incorrect type for a known string field in each
resource and requires an API `Invalid` response naming that field; transport,
authorization, or unrelated validation failures do not count as rejection.
Explicit fixture fields must survive unchanged, while additional server defaults
are allowed.

These are API checks, not workload-readiness checks. Fixtures use suspension
where available and a dedicated namespace. Deletion remains mandatory with
`-e2e.skip-resource-cleanup`; that option only retains the namespace and ordinary
environment cleanup. Failed fixtures remain available for CAPZ diagnostics.
The `kuberay-api/wait-api` interval is a harness timeout, not a latency SLO.
The assertions accept a workload-cluster REST config and do not install or
take ownership of KubeRay. Managed-extension installation and attach-mode CLI
plumbing are not included.

To run only this scenario in both Azure lanes with the usual e2e prerequisites:

```bash
GINKGO_FOCUS='\[KubeRayAPI\]' GINKGO_SKIP='' ./scripts/ci-e2e.sh
```

The same API assertions can be exercised locally with envtest, without Azure or
a running operator. Set `KUBEBUILDER_ASSETS` to your envtest binary directory and
use the KubeRay version pinned in `test/e2e/azure_kuberay.go`:

```bash
helm show crds kuberay-operator \
  --repo https://ray-project.github.io/kuberay-helm/ --version 1.6.2 > /tmp/kuberay-crds.yaml
KUBERAY_CRD_PATH=/tmp/kuberay-crds.yaml \
  go test -tags=e2e ./test/e2e -run '^TestKubeRay' -count=1
```

Without `KUBERAY_CRD_PATH`, the API-server test is skipped; the field-comparison
and rejection-classification unit tests still run.

<!-- links -->

[pull-cluster-api-provider-azure-build]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-azure#capz-pr-build-main
[pull-cluster-api-provider-azure-test]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-azure#capz-pr-test-main
[pull-cluster-api-provider-azure-e2e]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-azure#capz-pr-e2e-main
[pull-cluster-api-provider-azure-windows]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-azure#capz-pr-e2e-windows-main
[pull-cluster-api-provider-azure-verify]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-azure#capz-verify-main
[pull-cluster-api-provider-azure-e2e-exp]:  https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-azure#capz-pr-e2e-exp-main
[pull-cluster-api-provider-azure-apidiff]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-azure#capz-pr-apidiff-main
[pull-cluster-api-provider-azure-coverage]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-azure#pr-coverage
[pull-cluster-api-provider-azure-ci-entrypoint]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-azure#capz-pr-ci-entrypoint
[pull-cluster-api-provider-e2e-full]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-azure#capz-pr-e2e-full-main
[pull-cluster-api-provider-capi-e2e]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-azure#capz-pr-capi-e2e-main
[pull-cluster-api-provider-azure-conformance-v1beta1]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-azure#capz-pull-conformance-v1beta1-main
[pull-cluster-api-provider-azure-upstream-v1beta1-windows]:  https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-azure#capz-pull-conformance-v1beta1-main
[pull-cluster-api-provider-azure-conformance-with-ci-artifacts]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-azure#capz-pr-conformance-v1beta1-k8s-main
[pull-cluster-api-provider-azure-windows-upstream-with-ci-artifacts]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-azure#capz-pr-conformance-v1beta1-k8s-main
[post-cluster-api-provider-azure-push-images]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-azure#post-cluster-api-provider-azure-push-images
[postsubmits-cluster-api-provider-azure-e2e-full-main]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-azure#capz-postsubmit-capi-e2e-full-main
[periodic-cluster-api-provider-azure-conformance-v1beta1]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-azure#capz-periodic-conformance-v1beta1-main
[periodic-cluster-api-provider-azure-conformance-v1beta1-with-ci-artifacts]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-azure#capz-periodic-conformance-v1beta1-k8s-main
[periodic-cluster-api-provider-azure-capi-e2e]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-azure#capz-periodic-capi-e2e-main
[periodic-cluster-api-provider-azure-coverage]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-azure#periodic-cluster-api-provider-azure-coverage
[periodic-cluster-api-provider-azure-e2e-full]: https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-azure#capz-periodic-capi-e2e-full-main
