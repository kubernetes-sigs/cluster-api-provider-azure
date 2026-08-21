# Downstream context (stolostron / ARO-HCP)

Entry point for AI coding agents (and new humans) working in the
**stolostron** fork of Cluster API Provider Azure (CAPZ).

This file is a **table of contents**, not a manual. It describes only what is
*specific to this fork* and points at the authoritative docs for everything
else — it does not duplicate them. Read the linked document for detail before
making changes.

> **This is a downstream fork.** The engineering source of truth — architecture,
> build/test commands, controller patterns, code generation — lives in the
> upstream-maintained docs in this repo. Start with
> [`AGENTS.md`](../AGENTS.md) (the full CAPZ agent manual, synced from upstream)
> and [`README.md`](../README.md). This page adds only the stolostron/ARO-HCP
> layer on top.

## What this fork is

`stolostron/cluster-api-provider-azure` is a downstream fork of the upstream
project [`kubernetes-sigs/cluster-api-provider-azure`](https://github.com/kubernetes-sigs/cluster-api-provider-azure).
It packages CAPZ for **Azure Red Hat OpenShift Hosted Control Planes (ARO-HCP)**
and ships it as part of **multicluster engine (MCE) / backplane** via
[Konflux](https://konflux-ci.dev/) builds.

The Go module path stays `sigs.k8s.io/cluster-api-provider-azure` — this fork
does not rename the module.

## How it differs from upstream

The Go source is kept as close to upstream as possible; the downstream-only
pieces are the release-branch model, the automated upstream sync, and the
Konflux build/pipeline configuration.

| Concern | Where it lives | Notes |
|---------|----------------|-------|
| Automated upstream sync | [`.github/workflows/upstream-sync.yml`](../.github/workflows/upstream-sync.yml) | Weekly (Mon 08:17 UTC) + manual dispatch. Opens draft PRs that merge upstream release branches into the downstream branches. |
| Konflux build pipelines | [`.tekton/`](../.tekton) | Tekton `PipelineRun` definitions for MCE releases (`mce-217`, `mce-51`, `mce-50`), split into `-pull-request` and `-push` variants. |
| Release branches | see sync matrix below | Downstream branches track specific upstream release branches. |

### Branch model

| Downstream branch | Tracks upstream | Ships in |
|-------------------|-----------------|----------|
| `main` | `main` | MCE 5.1 dev; PRs also trigger the `mce-51` Konflux build |
| `backplane-5.1` | `main` (via `main` sync) | MCE 5.1 |
| `backplane-5.0` | `release-1.26` (via `release-1.26` sync) | MCE 5.0 |
| `backplane-2.17` | `release-1.22` | MCE 2.17 |
| `backplane-2.11` | `release-1.22` | MCE 2.11 |

Intermediate sync branches (`release-1.23`, `release-1.26`) exist solely to
receive upstream syncs and are merged into the corresponding shipping branches
— they are not development targets.

The authoritative matrix is the `strategy.matrix` in
[`upstream-sync.yml`](../.github/workflows/upstream-sync.yml) — consult it there
rather than trusting this table if they disagree.

### Working with the fork

- **Keep changes upstream-friendly.** Prefer contributing fixes upstream and
  letting them flow down via the sync. Downstream-only patches make every future
  sync harder to merge.
- **Do not edit upstream-tracked files to add downstream notes.** Files such as
  [`AGENTS.md`](../AGENTS.md), [`CLAUDE.md`](../CLAUDE.md), [`README.md`](../README.md),
  and [`CONTRIBUTING.md`](../CONTRIBUTING.md) are synced from upstream; edits to
  them create recurring merge conflicts. Put downstream-specific context here
  instead.
- **PR target.** Open PRs against the appropriate downstream branch
  (`main` / `backplane-5.1` / `backplane-5.0` / `backplane-2.17` / `backplane-2.11`),
  not against upstream.

## Start here

| Document | What it covers |
|----------|----------------|
| [AGENTS.md](../AGENTS.md) | **Upstream CAPZ agent manual** — architecture, dev/build/test commands, controller & service patterns, code generation, testing strategy. The primary source of truth. |
| [CLAUDE.md](../CLAUDE.md) | Claude Code entry point (defers to `AGENTS.md`). |
| [README.md](../README.md) | Project overview, prerequisites, quick start. |
| [CONTRIBUTING.md](../CONTRIBUTING.md) | Upstream contribution workflow and conventions. |
| [docs/README.md](README.md) | Documentation index (links into the CAPZ book at capz.sigs.k8s.io). |
| [SECURITY.md](../SECURITY.md) / [SUPPORT.md](../SUPPORT.md) | Vulnerability reporting and support channels. |
