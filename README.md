# Kubernetes Cluster API Provider Azure

[![Go Report Card](https://goreportcard.com/badge/kubernetes-sigs/cluster-api-provider-azure)](https://goreportcard.com/report/kubernetes-sigs/cluster-api-provider-azure)

<img src="https://github.com/kubernetes/kubernetes/raw/master/logo/logo.png"  width="100">

------

Kubernetes-native declarative infrastructure for Azure.

## What is the Cluster API Provider Azure

The [Cluster API][cluster_api] brings declarative, Kubernetes-style APIs to cluster creation, configuration and management.

The API itself is shared across multiple cloud providers allowing for true Azure
hybrid deployments of Kubernetes.

## Launching a Kubernetes cluster on Azure

Check out the [getting started guide](docs/getting-started.md) for launching a
cluster on Azure.

## Features

TODO

------

## Documentation

Documentation is in the `/docs` directory, and the [index is here](docs/README.md).

## Getting involved and contributing

Are you interested in contributing to cluster-api-provider-azure? We, the
maintainers and community, would love your suggestions, contributions, and help!
Also, the maintainers can be contacted at any time to learn more about how to get
involved.

In the interest of getting more new people involved we tag issues with
[`good first issue`][good_first_issue].
These are typically issues that have smaller scope but are good ways to start
to get acquainted with the codebase.

We also encourage ALL active community participants to act as if they are
maintainers, even if you don't have "official" write permissions. This is a
community effort, we are here to serve the Kubernetes community. If you have an
active interest and you want to get involved, you have real power! Don't assume
that the only people who can get things done around here are the "maintainers".

We also would love to add more "official" maintainers, so show us what you can
do!

This repository uses the Kubernetes bots.  See a full list of the commands [here][prow].

### Implementer office hours

TODO. Coming soon!

### Other ways to communicate with the contributors

Please check in with us in the [#cluster-api-azure][slack] channel on Slack.

## Github issues

### Bugs

If you think you have found a bug please follow the instructions below.

- Please spend a small amount of time giving due diligence to the issue tracker. Your issue might be a duplicate.
- Get the logs from the cluster controllers. Please paste this into your issue.
- Open a [bug report][bug_report].
- Remember users might be searching for your issue in the future, so please give it a meaningful title to helps others.
- Feel free to reach out to the cluster-api community on [kubernetes slack][slack_info].

### Tracking new features

We also use the issue tracker to track features. If you have an idea for a feature, or think you can help kops become even more awesome follow the steps below.

- Open a [feature request][feature_request].
- Remember users might be searching for your issue in the future, so please
  give it a meaningful title to helps others.
- Clearly define the use case, using concrete examples. EG: I type `this` and
  cluster-api-provider-azure does `that`.
- Some of our larger features will require some design. If you would like to
  include a technical design for your feature please include it in the issue.
- After the new feature is well understood, and the design agreed upon we can
  start coding the feature. We would love for you to code it. So please open
  up a **WIP** *(work in progress)* pull request, and happy coding.

<!-- References -->

[slack]: https://kubernetes.slack.com/messages/CEX9HENG7
[good_first_issue]: https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues?q=is%3Aissue+is%3Aopen+sort%3Aupdated-desc+label%3A%22good+first+issue%22
[prow]:
https://github.com/kubernetes/test-infra/blob/master/commands.md
[bug_report]: https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/new?template=bug_report.md
[feature_request]: https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/new?template=feature_request.md
[slack_info]: https://github.com/kubernetes/community/blob/master/communication.md#social-media
[cluster_api]: https://github.com/kubernetes-sigs/cluster-api
