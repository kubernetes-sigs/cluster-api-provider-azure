# Release Process

## Update metadata.yaml (skip for patch releases)

- Make sure the [metadata.yaml](https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/main/metadata.yaml) file is up to date and contains the new release with the correct cluster-api contract version.
  - If not, open a [PR](https://github.com/kubernetes-sigs/cluster-api-provider-azure/pull/1928) to add it.

## Change milestone (skip for patch releases)

- Create a new GitHub milestone for the next release
- Change milestone applier so new changes can be applied to the appropriate release
  - Open a PR in https://github.com/kubernetes/test-infra to change this [line](https://github.com/kubernetes/test-infra/blob/25db54eb9d52e08c16b3601726d8f154f8741025/config/prow/plugins.yaml#L344)
    - Example PR: https://github.com/kubernetes/test-infra/pull/16827

## Update test capz provider metadata.yaml (skip for patch releases)

Using that same next release version used to create a new milestone, update the the capz provider [metadata.yaml](https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/main/test/e2e/data/shared/v1beta1_provider/metadata.yaml) that we use to run PR and periodic cluster E2E tests against the main branch templates.

For example, if the latest stable API version of capz that we run E2E tests against is `v1beta`, and we're releasing `v1.4.0`, and our next release version is `v1.5.0`, then we want to ensure that the `metadata.yaml` defines a contract between `1.5` and `v1beta1`:

```yaml
apiVersion: clusterctl.cluster.x-k8s.io/v1alpha3
releaseSeries:
  - major: 1
    minor: 5
    contract: v1beta1
```

Additionally, we need to update the `type: InfrastructureProvider` spec in [azure-dev.yaml](https://github.com/kubernetes-sigs/cluster-api-provider-azure/blob/main/test/e2e/config/azure-dev.yaml) to express that our intent is to test (using the above example) `1.5`. By convention we use a sentinel patch version "99" to express "any patch version". In this example we want to look for the `type: InfrastructureProvider` with a `name` value of `v1.4.99` and update it to `v1.5.99`:

```
    - name: v1.5.99 # "vNext"; use manifests from local source files
```

## Create a tag

- Prepare the release branch. :warning: Always release from the release branch and not from main!
  - If releasing a patch release, check out the existing release branch and make sure you have the latest changes:
    - `git checkout release-1.x`
    - `git fetch upstream`
    - `git rebase upstream/release-1.x`
  - If releasing a minor release, create a new release branch from the main branch:
    - `git fetch upstream`
    - `git rebase upstream/main`
    - `git checkout -b release-1.x`
    - `git push upstream release-1.x`
- Create tag with git
  - `export RELEASE_TAG=v1.2.3` (the tag of the release to be cut)
  - `git tag -s ${RELEASE_TAG} -m "${RELEASE_TAG}"`
  - `-s` creates a signed tag, you must have a GPG key [added to your GitHub account](https://docs.github.com/en/authentication/managing-commit-signature-verification/adding-a-new-gpg-key-to-your-github-account)
  - `git push upstream ${RELEASE_TAG}`

This will automatically trigger a [Github Action](https://github.com/kubernetes-sigs/cluster-api-provider-azure/actions) to create a draft release.

## Promote image to prod repo

- Images are built by the [post push images job](https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-azure#post-cluster-api-provider-azure-push-images). This will push the image to a [staging repository](https://console.cloud.google.com/gcr/images/k8s-staging-cluster-api-azure/GLOBAL/cluster-api-azure-controller?rImageListsize=30).
- If you don't have a GitHub token, create one by going to your GitHub settings, in [Personal access tokens](https://github.com/settings/tokens). Make sure you give the token the `repo` scope.
- Wait for the above job to complete for the tag commit and for the image to exist in the staging directory, then create a PR to promote the image and tag:
  - `export GITHUB_TOKEN=<your GH token>`
  - `make promote-images`

This will automatically create a PR in [k8s.io](https://github.com/kubernetes/k8s.io) and assign the CAPZ maintainers. Example PR: https://github.com/kubernetes/k8s.io/pull/3007.

## Release in GitHub

- Manually format and categorize the release notes
- Publish release
- [Announce][release-announcement] the release

## Versioning

cluster-api-provider-azure follows the [semantic versionining][semver] specification.

Example versions:

- Pre-release: `v0.1.1-alpha.1`
- Minor release: `v0.1.0`
- Patch release: `v0.1.1`
- Major release: `v1.0.0`

## Expected artifacts

1. A release yaml file `infrastructure-components.yaml` containing the resources needed to deploy to Kubernetes
2. A `cluster-templates.yaml` for each supported flavor
3. A `metadata.yaml` which maps release series to cluster-api contract version
4. Release notes

## Update Upstream Tests

For major and minor releases we will need to update the set of capz-dependent `test-infra` jobs so that they use our latest release branch. For example, if we cut a new `1.3.0` minor release, from a newly created `release-1.3` git branch, then we need to update all test jobs to use capz at `release-1.3` instead of `release-1.2`.

Here is a reference PR that applied the required test job changes following the `1.3.0` minor release described above:

- https://github.com/kubernetes/test-infra/pull/26200

## Communication

### Patch Releases

1. Announce the release in Kubernetes Slack on the #cluster-api-azure channel.

### Minor/Major Releases

1. Follow the communications process for [pre-releases](#pre-releases)
2. An announcement email is sent to `kubernetes-sig-azure@googlegroups.com` and `kubernetes-sig-cluster-lifecycle@googlegroups.com` with the subject `[ANNOUNCE] cluster-api-provider-azure <version> has been released`

[release-announcement]: #communication
[semver]: https://semver.org/#semantic-versioning-200
[template]: /docs/release-notes-template.md
[versioning]: #versioning
