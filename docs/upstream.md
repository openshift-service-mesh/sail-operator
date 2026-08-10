# Upstream Relationship

## Overview

The Sail Operator is developed as a community project at
[istio-ecosystem/sail-operator](https://github.com/istio-ecosystem/sail-operator) (upstream).
Red Hat maintains a midstream fork at
[openshift-service-mesh/sail-operator](https://github.com/openshift-service-mesh/sail-operator)
that carries OpenShift Service Mesh (OSSM) specific changes on top of the upstream codebase.

The midstream fork tracks upstream branches and adds patches for OpenShift integration,
CI configuration, and OSSM-specific features that are not suitable for the community project.

## Repository Structure

| Role       | URL                                                          |
|------------|--------------------------------------------------------------|
| Upstream   | https://github.com/istio-ecosystem/sail-operator             |
| Midstream  | https://github.com/openshift-service-mesh/sail-operator      |

## Branch Mapping

Each midstream release branch corresponds to an upstream release branch, targets specific
OpenShift Container Platform (OCP) versions, and tracks a particular Istio release through
the [istio midstream fork](https://github.com/openshift-service-mesh/istio).

| Midstream Branch | Upstream Branch | OCP Versions                        | Istio Version Tracked |
|------------------|-----------------|-------------------------------------|-----------------------|
| `main`           | `main`          | TBD                                 | upstream master       |
| `release-3.0`    | `release-3.0`   | 4.14, 4.15, 4.16, 4.17, 4.18, 4.19, 4.20 | release-1.24    |
| `release-3.1`    | `release-3.1`   | 4.16, 4.17, 4.18, 4.19, 4.20       | release-1.26          |
| `release-3.2`    | `release-3.2`   | 4.18, 4.19, 4.20, 4.21, 4.22       | release-1.27          |
| `release-3.3`    | `release-3.3`   | 4.18, 4.19, 4.20, 4.21, 4.22       | release-1.28          |
| `release-3.4`    | `release-3.4`   | 4.20, 4.21, 4.22                    | release-1.30          |
| `release-3.5`    | `release-3.5`   | TBD                                 | release-1.31          |

OCP version support is sourced from the [Red Hat OpenShift Operators support policy](https://access.redhat.com/support/policy/updates/openshift_operators#platform-agnostic).

The "Istio Version Tracked" column indicates which branch of the
[openshift-service-mesh/istio](https://github.com/openshift-service-mesh/istio)
midstream fork is used for the Istio control plane in that release. See the istio midstream
repository's `docs/upstream.md` for details on the istio upstream-to-midstream relationship.

## Contribution Workflow

1. **Prefer upstream first.** Changes that are not OSSM-specific should be proposed as pull
   requests to the upstream repository. Once merged upstream, they will be synced to the
   midstream fork.
2. **OSSM-specific changes** that have no relevance to the community project (OpenShift CI
   configs, OSSM-only features, vendor defaults) go directly to the midstream repository.
3. Bug fixes that affect both upstream and midstream should land upstream first to avoid
   divergence.

## Sync Process

Upstream changes are brought into midstream through periodic merges and targeted cherry-picks.

- An automator bot performs routine merges from upstream branches into the corresponding
  midstream branches.
- When the bot merge produces conflicts or when specific commits need to be pulled ahead of
  a full merge, maintainers perform manual cherry-picks.
- After a sync, CI (prow) runs on the midstream branch to validate that OSSM-specific patches
  still apply cleanly and tests pass.

## Coding Conventions

- Follow the coding style and conventions established by the upstream project.
- OSSM-specific patches should be kept as small and isolated as possible.
- Do not modify upstream files unnecessarily; prefer additive changes or configuration-based
  customization.

### Labeling permanent OSSM changes

Changes that are intentional, permanent divergences from upstream (not cherry-pick candidates)
must be labeled so they survive future merges without confusion. Use this comment style:

```go
// OSSM-only: <JIRA-KEY> <short reason>
```

Example:

```go
// OSSM-only: OSSM-12345 OpenShift-specific default required by OCP security policy
```

Apply the label on the same line (or the line above for blocks) as the divergent code.
This makes it easy to locate all permanent patches with `git grep "OSSM-only"` and avoids
re-introducing upstream behavior by accident during a merge conflict resolution.

Changes that are intended for upstream contribution but have not landed yet should instead
reference the upstream issue or PR in the commit message, not in a code comment.

## CI Configuration

Prow CI job definitions for the sail-operator midstream are located in the
[openshift/release](https://github.com/openshift/release) repository:

- [ci-operator/config/openshift-service-mesh/sail-operator/](https://github.com/openshift/release/tree/main/ci-operator/config/openshift-service-mesh/sail-operator)

Each file encodes the branch and OCP version it targets:

```
openshift-service-mesh-sail-operator-<branch>__ocp-<version>.yaml
```

For example, `openshift-service-mesh-sail-operator-release-3.4__ocp-4.23.yaml` defines
the prow jobs that run on `release-3.4` against OCP 4.23.

## PR Process

| Target             | Where to open the PR                                         |
|--------------------|--------------------------------------------------------------|
| Community feature  | https://github.com/istio-ecosystem/sail-operator             |
| OSSM-only change   | https://github.com/openshift-service-mesh/sail-operator      |

- All PRs are gated by CI (prow). Ensure `make all` passes before submitting.
- Commit signing (`-s` flag) is required.
- Reference the relevant JIRA issue in the PR description.
