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
