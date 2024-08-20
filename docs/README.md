# Documentation Index

## Quick start

- [Getting started](https://capz.sigs.k8s.io/topics/getting-started.html)
- [Cluster API quick start](https://cluster-api.sigs.k8s.io/user/quick-start.html)

## Features

- [Topics](https://capz.sigs.k8s.io/topics/topics.html)
 
## Roadmap
 
- [Roadmap](https://capz.sigs.k8s.io/roadmap.html)

## Development

- [Development guide](https://capz.sigs.k8s.io/developers/development.html)
- [Kubernetes developers](https://capz.sigs.k8s.io/developers/kubernetes-developers.html)
- [Proposals](proposals)
- [Releasing](https://capz.sigs.k8s.io/developers/releasing.html)

## Troubleshooting

- [Troubleshooting guide](https://capz.sigs.k8s.io/topics/troubleshooting.html)

## Docs contributors

To run the link check linter, execute the following command from the root of the repository:

`find . -name '*.md' -not -path './node_modules/*' -exec markdown-link-check '{}' --config .markdownlinkcheck.json -q ';'`
