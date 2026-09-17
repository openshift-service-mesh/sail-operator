[Return to OSSM user documentation](../)

# Skipping a version during an upgrade

Normally, the operator and the mesh are upgraded one minor version at a time. With the `RevisionBased` update strategy you can skip one operator minor version and upgrade directly from `n-2` to `n`, for example from OpenShift Service Mesh 3.0 to 3.2. This is possible because the old and the new control plane run side by side and the existing proxies stay attached to the old revision until you restart the workloads.

Skipping versions is **not** supported with the `InPlace` update strategy. With `InPlace` you must upgrade one minor version at a time and restart the workloads after each step.

This document describes the procedure using an `IstioRevisionTag` named `default`. The tag is a stable alias for the control plane revision, so the workload namespaces keep the same injection label for the whole upgrade and only the deployments have to be restarted.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Procedure](#procedure)
  - [1. Switch the operator subscription channel](#1-switch-the-operator-subscription-channel)
  - [2. Update the Istio control plane](#2-update-the-istio-control-plane)
  - [3. Restart the workloads](#3-restart-the-workloads)
  - [4. Update IstioCNI](#4-update-istiocni)
- [Notes and limitations](#notes-and-limitations)

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

- An `IstioRevisionTag` named `default` whose `targetRef` references the `Istio` resource. Because the tag references the `Istio` resource rather than a single `IstioRevision`, the operator repoints the tag for you when the underlying revision changes, and you only need to restart your deployments to get the new proxies injected. A tag named `default` also enables the legacy `istio-injection=enabled` label in addition to `istio.io/rev=default`; the `istio-injection` label can only be used with revisions and revision tags named `default`. If you prefer to control the switch yourself, set `targetRef` to an `IstioRevision` instead and update the tag manually when you want the workloads to move.

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

The examples below upgrade the operator from OpenShift Service Mesh 3.0 to 3.2 and the mesh from Istio `1.24.6`, the latest version available in 3.0, to Istio `1.27.9`, the latest version available in 3.2, using the `bookinfo` application in the `bookinfo` namespace. Substitute the versions that apply to your environment.

## Procedure

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

Alternatively, use the `v1.27-latest` alias instead of a specific patch version. The operator then keeps the control plane on the most recent `1.27` patch release available in the installed operator:

```bash
$ oc patch istio default --type='merge' -p '{"spec":{"version":"v1.27-latest"}}'
```

Note that with the `RevisionBased` strategy and an alias, every new patch release creates a new revision, so the workloads have to be restarted to move to it.

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

## Notes and limitations

- Only one operator minor version can be skipped (`n-2` to `n`). To move further, repeat the whole procedure.
- Skipping versions requires the `RevisionBased` update strategy. With `InPlace`, upgrade one minor version at a time.
- To reduce the risk of interruptions, avoid adding workloads to the mesh or removing them from it during the upgrade procedure.
- Review the release notes of the versions you skip. Their behavioral changes, deprecations, and removals still apply to your configuration.
- Increase `spec.updateStrategy.inactiveRevisionDeletionGracePeriodSeconds` if you want more time to validate the new control plane before the old revision is removed.

For the general upgrade documentation, see [Versioning and upgrades](../versioning-and-upgrades/README.md).
