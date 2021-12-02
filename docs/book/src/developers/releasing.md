# Release Process

## Change milestone

- Create a new GitHub milestone for the next release
- Change milestone applier so new changes can be applied to the appropriate release
  - Open a PR in https://github.com/kubernetes/test-infra to change this [line](https://github.com/kubernetes/test-infra/blob/25db54eb9d52e08c16b3601726d8f154f8741025/config/prow/plugins.yaml#L344)
    - Example PR: https://github.com/kubernetes/test-infra/pull/16827

## Create a tag

- Identify a known good commit on the main branch
- Fast-forward the release branch to the selected commit. :warning: Always release from the release branch and not from main!
  - `git checkout release-0.x`
  - `git fetch upstream`
  - `git merge --ff-only upstream/main`
  - `git push`
- Create tag with git
  - `export RELEASE_TAG=v0.4.6` (the tag of the release to be cut)
  - `git tag -s ${RELEASE_TAG} -m "${RELEASE_TAG}"`
  - `-s` creates a signed tag, you must have a GPG key [added to your GitHub account](https://docs.github.com/en/enterprise/2.16/user/github/authenticating-to-github/generating-a-new-gpg-key)
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

## Communication

### Patch Releases

1. Announce the release in Kubernetes Slack on the #cluster-api-azure channel.

### Minor/Major Releases

1. Follow the communications process for [pre-releases](#pre-releases)
2. An announcement email is sent to `kubernetes-sig-azure@googlegroups.com` and `kubernetes-sig-cluster-lifecycle@googlegroups.com` with the subject `[ANNOUNCE] cluster-api-provider-azure <version> has been released`

[release-announcement]: #communication
[semver]: https://semver.org/#semantic-versioning-200
[support-policy]: /README.md#support-policy
[template]: /docs/release-notes-template.md
[versioning]: #versioning
