# Release Process

## Change milestone
 -  Create a new GitHub milestone for the next release
 - Change milestone applier so new changes can be applied to the appropriate release
      - Open a PR in https://github.com/kubernetes/test-infra to change this [line](https://github.com/kubernetes/test-infra/blob/25db54eb9d52e08c16b3601726d8f154f8741025/config/prow/plugins.yaml#L344)
        - Example PR: https://github.com/kubernetes/test-infra/pull/16827

## Prepare branch, tag and release notes
 - Identify a known good commit on the main branch
 - Fast-forward the release branch to the selected commit. :warning: Always release from the release branch and not from master!
   - `git checkout release-0.x`
   - `git fetch upstream`
   - `git merge --ff-only upstream/master`
   - `git push`
 - Create tag with git
   - `export RELEASE_TAG=v0.4.6` (the tag of the release to be cut)
   - `git tag -s ${RELEASE_TAG} -m "${RELEASE_TAG}"`
	 - `-s` creates a signed tag, you must have a GPG key [added to your GitHub account](https://docs.github.com/en/enterprise/2.16/user/github/authenticating-to-github/generating-a-new-gpg-key)
   - `git push upstream ${RELEASE_TAG}`
 - Update the file `metadata.yaml` if is a major or minor release
 - `make release` from repo, this will create the release artifacts in the `out/` folder
 - Install the `release-notes` tool according to [instructions](https://github.com/kubernetes/release/blob/master/cmd/release-notes/README.md)
 - Export GITHUB_TOKEN
 - Run the release-notes tool with the appropriate commits. Commits range from the first commit after the previous release to the new release commit.
  ```
  release-notes --github-org kubernetes-sigs --github-repo cluster-api-provider-azure \
  --start-sha 1cf1ec4a1effd9340fe7370ab45b173a4979dc8f  \
  --end-sha e843409f896981185ca31d6b4a4c939f27d975de
  ```
 - Manually format and categorize the release notes

## Promote image to prod repo
 Promote image
 - Images are built by the [post push images job](https://testgrid.k8s.io/sig-cluster-lifecycle-cluster-api-provider-azure#post-cluster-api-provider-azure-push-images)
 - Create a PR in https://github.com/kubernetes/k8s.io to add the image and tag
   - Example PR: https://github.com/kubernetes/k8s.io/pull/1030/files
 - Location of image: https://console.cloud.google.com/gcr/images/k8s-staging-cluster-api-azure/GLOBAL/cluster-api-azure-controller?rImageListsize=30

## Release in GitHub
 Create the GitHub release in the UI
 - Create a draft release in GitHub and associate it with the tag that was created
 - Copy paste the release notes
 - Upload [artifacts](#expected-artifacts) from the `out/` folder
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
