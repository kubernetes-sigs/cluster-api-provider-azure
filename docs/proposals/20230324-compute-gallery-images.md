---
title: Reference images in Azure Compute Gallery
authors:
  - "@mboersma"
reviewers:
creation-date: 2023-03-24
last-updated: 2023-03-24
status: provisional
see-also:
- https://github.com/kubernetes-sigs/cluster-api-provider-azure/issues/2294
- https://learn.microsoft.com/azure/virtual-machines/shared-image-galleries
- https://capz.sigs.k8s.io/topics/custom-images.html
---

# Reference Images in Azure Compute Gallery

## Table of Contents

- [Reference Images in Azure Compute Gallery](#reference-images-in-azure-compute-gallery)
  - [Table of Contents](#table-of-contents)
  - [Glossary](#glossary)
  - [Summary](#summary)
  - [Motivation](#motivation)
    - [Goals](#goals)
    - [Non-Goals / Future Work](#non-goals--future-work)
  - [Proposal](#proposal)
    - [User Stories](#user-stories)
      - [Story 1 - blah blah](#story-1---blah-blah)
    - [Implementation Details/Notes/Constraints](#implementation-detailsnotesconstraints)
    - [Security Considerations](#security-considerations)
    - [Risks and Mitigations](#risks-and-mitigations)
  - [Alternatives](#alternatives)
  - [Graduation Criteria](#graduation-criteria)
    - [Publishing Process](#publishing-process)
    - [Test Plan](#test-plan)
  - [Implementation History](#implementation-history)

## Glossary

- **Custom image** - a disk image built by a user, suitable for use with Cluster API Provider Azure (CAPZ).
- **Reference image** - a public disk image built by the CAPZ team to facilitate testing and to help new users.
- **Virtual Hard Disk (VHD)** - a blob containing a file system, the contents of a disk image.

## Summary

The Cluster API Provider Azure (CAPZ) project should publish reference images to Azure Compute Gallery and by default use those images to provision new clusters. This will simplify and speed up the image publishing process and align the project's practices with the approach we recommend to users.

## Motivation

The Cluster API Provider Azure (CAPZ) project has made *reference images* available in the Azure Marketplace for versions of Kubernetes since at least Kubernetes 1.17 in early 2021. By default, CAPZ uses these Marketplace images to provision new workload clusters.

Without these reference images, users and developers of CAPZ would have to build and host their own *custom images* and to modify their cluster templates. While that process is strongly recommended for use in production, it is a barrier to entry. Allowing new users to ignore the details of image hosting at first is crucial to keeping CAPZ approachable.

Publishing images to the Azure Marketplace is a slow and heavyweight process, with some significant limitations. Although it is mostly automated, it is still error-prone and requires human intervention. An image-builder command such as `make -C images/capi build-azure-vhd-ubuntu-2204` only creates a VHD; the rest of the existing publishing process involves complicated Azure DevOps pipelines and is not easily explained to users.

Azure Compute Gallery is a more efficient and flexible way to publish images (HOW?) with fewer limitations (TRUE?). An image-builder command such as `make -C images/capi build-azure-sig-ubuntu-2204` is sufficient to build an image and publish it to a Compute Gallery, keeping the vast majority of the publishing process transparent to users.

Currently, each current pipeline build prints this:

> **Warning: You are using Azure Packer Builder to create VHDs which is being deprecated, consider using Managed Images. Learn more https://www.packer.io/docs/builders/azure/arm#azure-arm-builder-specific-options**

Using the Azure Packer Builder to populate a Compute Gallery (SIG) is not deprecated and emits no such warning.

In the [CAPZ book](https://capz.sigs.k8s.io/topics/custom-images.html), the project calls Azure Compute Gallery "Recommended" and provides an example of how to publish to it. While the scripts for the current publishing pipeline are available in image-builder, their usage is not documented publicly. The team has gained lots of expertise in a Marketplace process we don't encourage users to use. CAPZ should follow its own advice by publishing to Compute Gallery.

### Goals

### Non-Goals / Future Work

- Publishing a reference image automatically when a new Kubernetes version is released. This is desirable, but out of scope for this proposal.
- Switching to VM Gen2 images. This proposal creates no obstacles to that decision, but it is orthogonal.

## Proposal

The current image publishing pipeline should be phased out in favor of a similar pipeline that publishes to Compute Gallery. CAPZ code should be changed to prefer provisioning with the reference images in Compute Gallery.

### User Stories

#### Story 1 - User updates CAPZ to new version that uses Compute Gallery

#### Story 2 - Developer investigates available reference images

#### Story 3 - Team member publishes a new reference image

#### Story 4 - Published reference image is broken

#### Story 5 - Published reference image goes out of support

### Implementation Details/Notes/Constraints

- How do we publish to a "staging" area so we can test before making generally available?
- How do we control the name of the VM image version so it has the same name as the Kubernetes version? In the Packer Azure ARM [docs](https://developer.hashicorp.com/packer/plugins/builders/azure/arm#image_version-1) it should be `image_version` but I don't seem to able to control that through config/kubernetes.json in image-builder.

### Security Considerations

Overall, this process presents similar security rasks to the current Marketplace publishing process. In both cases, the scripts, code, and pipeline definitions to create and publish images are all open source, but the actual pipelines are run securely at Microsoft by authorized team members.

Image hosting will now be directly from a replicated Azure Compute Gallery, rather than through the Azure Marketplace. Both services are assumed to be secure, but Compute Gallery is a newer service that provides more direct control over access and permissions. (TODO: investigate specific permissions and see if there's any room to lock things down beyond the defaults.)

### Risks and Mitigations

- We have hit hard-coded limits with the Marketplace before. Will this be a problem with Compute Galleries? Specifically, how many Galleries, Image definitions, and Image versions can we have in a single subscription? See https://learn.microsoft.com/azure/virtual-machines/azure-compute-gallery#limits
- Is it faster to download a Marketplace image or a Compute Gallery image? Is the difference significant?
- While no one should be relying on the current reference images, it's possible a user may expect Marketplace publishing to continue apace.

## Alternatives

### Stay the Course

We can continue with the current Marketplace publishing process, which has the benefit of being time-tested. While this would appear to be the "no additional work" option, a not-insignficant amount of time has gone into updating and troubleshooting. It's likely that future problems will occur here that require unknown amounts of time. To be fair, the proposed Compute Gallery process may also require maintenance and fixing.

## Graduation Criteria

### Publishing Process

### Test Plan

## Implementation History

- [x] 03/24/2023: Document first created
