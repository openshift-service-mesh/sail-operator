[Return to OSSM user documentation](../)

# Skipping a version during an upgrade

Normally, the operator and the mesh are upgraded one minor version at a time. With the `RevisionBased` update strategy you can skip one operator minor version and upgrade directly from `n-2` to `n`, for example from OpenShift Service Mesh 3.0 to 3.2. This is possible because the old and the new control plane run side by side and the existing proxies stay attached to the old revision until you restart the workloads.

This document describes the procedure using an `IstioRevisionTag` named `default`. The tag is a stable alias for the control plane revision, so the workload namespaces keep the same injection label for the whole upgrade and only the deployments have to be restarted.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Notes and limitations](#notes-and-limitations)
- [Procedure](#procedure)
  - [1. Switch the operator subscription channel](#1-switch-the-operator-subscription-channel)
  - [2. Update the Istio control plane](#2-update-the-istio-control-plane)
  - [3. Restart the workloads](#3-restart-the-workloads)
  - [4. Update IstioCNI](#4-update-istiocni)
- [Rollback](#rollback)
  - [Operator downgrade limitations](#operator-downgrade-limitations)
  - [Rollback procedure](#rollback-procedure)

## Prerequisites

- You are logged in to OpenShift Container Platform as `cluster-admin`.
- `istioctl` is installed on your local machine. For more information, see the [istioctl documentation](../istioctl/README.md).
- The `Istio` resource uses the `RevisionBased` update strategy:

  ```yaml
  apiVersion: sailoperator.io/v1
  kind: Istio
  metadata:
    name: default
  spec:
    namespace: istio-system
    version: v1.24.6
    updateStrategy:
      type: RevisionBased
  ```

- An `IstioRevisionTag` named `default` whose `targetRef` references the `Istio` resource. Because the tag references the `Istio` resource rather than a single `IstioRevision`, the operator repoints the tag for you when the underlying revision changes, and you only need to restart your deployments to get the new proxies injected. A tag named `default` also enables the legacy `istio-injection=enabled` label in addition to `istio.io/rev=default`; the `istio-injection` label can only be used with revisions and revision tags named `default`. If you prefer to control the switch yourself, set `targetRef` to an `IstioRevision` instead and update the tag manually when you want the workloads to move. We are using a default tag to avoid a known issue with missing validating webhook - https://github.com/istio-ecosystem/sail-operator/issues/1889

  ```yaml
  apiVersion: sailoperator.io/v1
  kind: IstioRevisionTag
  metadata:
    name: default
  spec:
    targetRef:
      kind: Istio
      name: default
  ```

- `IstioCNI` is installed with the same version as the `Istio` resource:

  ```yaml
  apiVersion: sailoperator.io/v1
  kind: IstioCNI
  metadata:
    name: default
  spec:
    namespace: istio-cni
    version: v1.24.6
  ```

- Workload namespaces are labeled for injection through the default tag, for example:

  ```bash
  $ oc label namespace bookinfo istio-injection=enabled
  ```

## Notes and limitations

Review the following limitations before you start:

- Only one operator minor version can be skipped (`n-2` to `n`). To move further, repeat the whole procedure.
- Skipping versions requires the `RevisionBased` update strategy. With `InPlace`, upgrade one minor version at a time and restart the workloads after each step.
- Skipping versions is supported only for sidecar mode. Meshes using ambient mode must be upgraded one minor version at a time.
- To reduce the risk of interruptions, avoid adding workloads to the mesh or removing them from it during the upgrade procedure.
- Review the release notes of the versions you skip. Their behavioral changes, deprecations, and removals still apply to your configuration.
- Review the minimum OpenShift Container Platform version required by the target operator version.
- Review the minimum Gateway API CRD version required by the target Istio version.
- Increase `spec.updateStrategy.inactiveRevisionDeletionGracePeriodSeconds` if you want more time to validate the new control plane before the old revision is removed.
- A rollback moves the operands back only. The operator cannot be downgraded, see [Rollback](#rollback).
- Known issue [istio/istio#61095](https://github.com/istio/istio/issues/61095): up to and including Istio 1.27, a gateway created with the Kubernetes Gateway API can stay on the old revision after the control plane is updated, because the new revision fails to reconcile it with a `PushContext not initialized` error and gives up. The gateway then serves no traffic. As a workaround, write to the `Gateway` resource, for example `oc annotate gateway <name> -n <namespace> --overwrite nudge="$(date +%s)"`, to make the new revision reconcile it again.

## Procedure

The examples below upgrade the operator from OpenShift Service Mesh 3.0 to 3.2 and the mesh from Istio `1.24.6`, the latest version available in 3.0, to Istio `1.27.9`, the latest version available in 3.2, using the `bookinfo` application in the `bookinfo` namespace. Substitute the versions that apply to your environment.

**Note:** Steps in this procedure applies also to multicluster deployments.

### 1. Switch the operator subscription channel

Change the channel of the operator subscription from the current version to the target version, for example from `stable-3.0` to `stable-3.2`:

```bash
$ oc patch subscription servicemeshoperator3 -n openshift-operators --type='merge' -p '{"spec":{"channel":"stable-3.2"}}'
```

If the subscription uses the `Manual` approval strategy, approve the resulting install plan.

Wait until the operator upgrade is complete and the new `ClusterServiceVersion` reports `Succeeded`:

```bash
$ oc get csv -n openshift-operators
```

```console
NAME                           DISPLAY                            VERSION   REPLACES   PHASE
servicemeshoperator3.v3.2.10   Red Hat OpenShift Service Mesh 3   3.2.10               Succeeded
```

The operator upgrade does not change the running control plane. The mesh keeps running Istio `1.24.6` until you update the `Istio` resource in the next step.

List the Istio versions that the new operator supports:

```bash
$ oc explain istio.spec.version
```

### 2. Update the Istio control plane

Set `spec.version` of the `Istio` resource to the latest version available in the new operator, `v1.27.9` in this example:

```bash
$ oc patch istio default --type='merge' -p '{"spec":{"version":"v1.27.9"}}'
```

**Note:** Alternatively, use the `v1.27-latest` alias instead of a specific patch version. The operator then keeps the control plane on the most recent `1.27` patch release available in the installed operator. With the `RevisionBased` strategy and an alias, every new patch release creates a new revision, so the workloads have to be restarted to move to it.

```bash
$ oc patch istio default --type='merge' -p '{"spec":{"version":"v1.27-latest"}}'
```

The operator deploys a new control plane next to the existing one and moves the default `IstioRevisionTag` to the new revision automatically. Existing proxies stay connected to the old control plane until they are restarted.

Confirm that both revisions are healthy:

```bash
$ oc get istiorevisions
```

```console
NAME              TYPE    READY   STATUS    IN USE   VERSION   AGE
default-v1-24-6   Local   True    Healthy   True     v1.24.6   35m
default-v1-27-9   Local   True    Healthy   True     v1.27.9   50s
```

Confirm that the tag points at the new revision:

```bash
$ oc get istiorevisiontags
```

```console
NAME      STATUS    IN USE   REVISION          AGE
default   Healthy   True     default-v1-27-9   35m
```

### 3. Restart the workloads

Restart the application workloads and the gateways so that the new proxy version is injected and the proxies connect to the new control plane:

```bash
$ oc rollout restart deployment -n bookinfo
```

Confirm that the workloads are running:

```bash
$ oc get pods -n bookinfo
```

Confirm that the proxies are connected to the new control plane. The `VERSION` column must show the new version for all proxies:

```bash
$ istioctl proxy-status
```

```console
NAME                                       CLUSTER        CDS              LDS              EDS              RDS              ECDS        ISTIOD                                     VERSION
details-v1-766844796b-2w7hv.bookinfo       Kubernetes     SYNCED (11s)     SYNCED (11s)     SYNCED (11s)     SYNCED (11s)     IGNORED     istiod-default-v1-27-9-5c8f7d6b49-qk4zt    1.27.9
productpage-v1-54bb874995-9xnvd.bookinfo   Kubernetes     SYNCED (14s)     SYNCED (14s)     SYNCED (14s)     SYNCED (14s)     IGNORED     istiod-default-v1-27-9-5c8f7d6b49-qk4zt    1.27.9
ratings-v1-5dc79b6bcd-7jr4c.bookinfo       Kubernetes     SYNCED (13s)     SYNCED (13s)     SYNCED (13s)     SYNCED (13s)     IGNORED     istiod-default-v1-27-9-5c8f7d6b49-qk4zt    1.27.9
reviews-v1-598b896c9d-pk8mn.bookinfo       Kubernetes     SYNCED (12s)     SYNCED (12s)     SYNCED (12s)     SYNCED (12s)     IGNORED     istiod-default-v1-27-9-5c8f7d6b49-qk4zt    1.27.9
reviews-v2-556d6457d-tz6sq.bookinfo        Kubernetes     SYNCED (12s)     SYNCED (12s)     SYNCED (12s)     SYNCED (12s)     IGNORED     istiod-default-v1-27-9-5c8f7d6b49-qk4zt    1.27.9
reviews-v3-564544b4d6-lr8xh.bookinfo       Kubernetes     SYNCED (12s)     SYNCED (12s)     SYNCED (12s)     SYNCED (12s)     IGNORED     istiod-default-v1-27-9-5c8f7d6b49-qk4zt    1.27.9
```

**Note:** newer `istioctl` versions may require the revision to be named explicitly while more than one control plane is running. If the output is empty or lists only part of the proxies, repeat the command for each revision:

```bash
$ istioctl proxy-status --istioNamespace istio-system --revision default-v1-27-9
```

Once no proxy uses the old revision, the old `IstioRevision` is no longer in use and is deleted after the grace period defined by `spec.updateStrategy.inactiveRevisionDeletionGracePeriodSeconds` (30 seconds by default):

```bash
$ oc get istiorevisions
```

```console
NAME              TYPE    READY   STATUS    IN USE   VERSION   AGE
default-v1-27-9   Local   True    Healthy   True     v1.27.9   6m
```

### 4. Update IstioCNI

Set `spec.version` of the `IstioCNI` resource to the same version as the `Istio` resource:

```bash
$ oc patch istiocni default -n istio-cni --type='merge' -p '{"spec":{"version":"v1.27.9"}}'
```

The CNI plugin is always updated in place: the `DaemonSet` is rolled out and the `istio-cni-node` pods are replaced.

Confirm that the new version is ready:

```bash
$ oc get istiocni default
```

```console
NAME      READY   STATUS    VERSION   AGE
default   True    Healthy   v1.27.9   38m
```

The upgrade is complete.

## Rollback

If the new version does not behave as expected, you can move the mesh back to the version you upgraded from. The rollback covers the operands only:

- the control plane, by setting `spec.version` of the `Istio` resource back,
- the data plane, by restarting the workloads so that the proxies are replaced with the older proxy version,
- the CNI plugin, by setting `spec.version` of the `IstioCNI` resource back.

The operator itself stays on the new version.

### Operator downgrade limitations

OLM does not support downgrading an installed operator: the subscription cannot be moved to a channel that is older than the current one. For details, see the "Updating installed Operators" section of the OpenShift Container Platform documentation.

The state you end up in after a rollback is therefore the new operator reconciling the operands of the older version: an older `Istio`, `IstioCNI`, and data plane kept running by a newer operator. This is a transitional state meant for the duration of the upgrade window, not a configuration to stay on. Fix whatever made you roll back and move the mesh to the latest version the installed operator offers as soon as possible.

### Rollback procedure

The rollback walks the upgrade backwards, so the steps are those of the upgrade in reverse order. The only step that changes its place is `IstioCNI`: during the upgrade it is updated after the workload restart, during the rollback it is moved first. As a result, the CNI plugin is never newer than the control plane at any point, in either direction.

The example below rolls the mesh back from Istio `1.27.9` to `1.24.6`, the version used before the upgrade.

#### 1. Roll back IstioCNI

```bash
$ oc patch istiocni default -n istio-cni --type='merge' -p '{"spec":{"version":"v1.24.6"}}'
```

Wait until the `DaemonSet` has rolled out and the resource is ready again:

```bash
$ oc get istiocni default
```

```console
NAME      READY   STATUS    VERSION   AGE
default   True    Healthy   v1.24.6   52m
```

#### 2. Roll back the Istio control plane

```bash
$ oc patch istio default --type='merge' -p '{"spec":{"version":"v1.24.6"}}'
```

The operator deploys the older control plane next to the current one. Revision names are derived from the `Istio` resource name and the version, so the revision name that was used before the upgrade is recreated.

Confirm that the `Istio` resource reconciled and that the older revision is the active one:

```bash
$ oc get istio default
```

```console
NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
default   2           2       2        default-v1-24-6   Healthy   v1.24.6   57m
```

Confirm that both revisions are healthy:

```bash
$ oc get istiorevisions
```

```console
NAME              TYPE    READY   STATUS    IN USE   VERSION   AGE
default-v1-24-6   Local   True    Healthy   True     v1.24.6   40s
default-v1-27-9   Local   True    Healthy   True     v1.27.9   21m
```

#### 3. Wait for the IstioRevisionTag to follow

The operator repoints the `default` tag at the older revision:

```bash
$ oc get istiorevisiontags
```

```console
NAME      STATUS    IN USE   REVISION          AGE
default   Healthy   True     default-v1-24-6   56m
```

#### 4. Restart the workloads

This is the step that rolls the data plane back. Restarting the deployments replaces the proxies with the older proxy version and connects them to the older control plane:

```bash
$ oc rollout restart deployment -n bookinfo
```

Confirm that all proxies reconnected and that the `VERSION` column shows the older version:

```bash
$ istioctl proxy-status
```

```console
NAME                                       CLUSTER        CDS              LDS              EDS              RDS              ECDS        ISTIOD                                     VERSION
details-v1-7d4b8f6c55-h6bq2.bookinfo       Kubernetes     SYNCED (15s)     SYNCED (15s)     SYNCED (15s)     SYNCED (15s)     IGNORED     istiod-default-v1-24-6-6f7b58d4c8-w2ndp    1.24.6
productpage-v1-6c9b48d7f4-sk3vj.bookinfo   Kubernetes     SYNCED (17s)     SYNCED (17s)     SYNCED (17s)     SYNCED (17s)     IGNORED     istiod-default-v1-24-6-6f7b58d4c8-w2ndp    1.24.6
ratings-v1-84975bc778-mn5qc.bookinfo       Kubernetes     SYNCED (16s)     SYNCED (16s)     SYNCED (16s)     SYNCED (16s)     IGNORED     istiod-default-v1-24-6-6f7b58d4c8-w2ndp    1.24.6
reviews-v1-5b5d6494f4-x9td7.bookinfo       Kubernetes     SYNCED (16s)     SYNCED (16s)     SYNCED (16s)     SYNCED (16s)     IGNORED     istiod-default-v1-24-6-6f7b58d4c8-w2ndp    1.24.6
reviews-v2-5b667bcbf8-2hcfl.bookinfo       Kubernetes     SYNCED (16s)     SYNCED (16s)     SYNCED (16s)     SYNCED (16s)     IGNORED     istiod-default-v1-24-6-6f7b58d4c8-w2ndp    1.24.6
reviews-v3-5b9bd44f4-vq7rk.bookinfo        Kubernetes     SYNCED (16s)     SYNCED (16s)     SYNCED (16s)     SYNCED (16s)     IGNORED     istiod-default-v1-24-6-6f7b58d4c8-w2ndp    1.24.6
```

As during the upgrade, newer `istioctl` versions may require the revision to be named explicitly while both control planes are still running:

```bash
$ istioctl proxy-status --istioNamespace istio-system --revision default-v1-24-6
```

#### 5. Wait until the newer revision is removed

Once no proxy uses it, the revision you rolled back from is deleted after the grace period defined by `spec.updateStrategy.inactiveRevisionDeletionGracePeriodSeconds`:

```bash
$ oc get istiorevisions
```

```console
NAME              TYPE    READY   STATUS    IN USE   VERSION   AGE
default-v1-24-6   Local   True    Healthy   True     v1.24.6   7m
```

The rollback is complete. To move forward again, repeat the procedure from [2. Update the Istio control plane](#2-update-the-istio-control-plane); the operator subscription is already on the new channel.

For the general upgrade documentation, see [Versioning and upgrades](../versioning-and-upgrades/README.md).
