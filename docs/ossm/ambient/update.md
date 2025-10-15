# Chapter 1. Updating OpenShift Service Mesh with Istio Ambient Mode

The strategy you use to deploy a service mesh affects how you can update the mesh. This document provides procedures for updating OpenShift Service Mesh when using Istio in ambient mode. The upgrade process includes updating the Istio control plane, IstioCNI, and ZTunnel components.

**Note:** Istio's ambient mode is a Technology Preview feature in OpenShift Service Mesh 3.1.x and is subject to change in future releases.

---

## 1.1. Understanding versioning

Red Hat OpenShift Service Mesh follows Semantic Versioning for all product releases. Semantic Versioning uses a three-part version number in the format X.Y.Z to communicate the nature of changes in each release.

**X (Major version)**
indicates significant updates that might include breaking changes, such as architectural shifts, API changes, or schema modifications.

**Y (Minor version)**
introduces new features and enhancements while maintaining backward compatibility.

**Z (Patch version or z-stream release)**
delivers critical bug fixes and security updates, such as Common Vulnerabilities and Exposures (CVEs) resolutions. Patch versions do not include new features.

---

## 1.2. Understanding Service Mesh and Istio versions

This document covers the upgrade path from OpenShift Service Mesh 3.1.1 to 3.1.2. The OpenShift Service Mesh Operator includes additional Istio releases for upgrades but supports only the latest Istio version available for each Operator version.

**OpenShift Service Mesh 3.1.1 supported versions:**

| Feature | Supported versions |
|---------|-------------------|
| OpenShift Service Mesh 3 Operator | 3.1.1 |
| OpenShift Service Mesh Istio control plane resource | 1.26.3 |
| OpenShift Container Platform | 4.16 and later |
| Envoy proxy | 1.34.3 |
| IstioCNI resource | 1.26.3 |
| Kiali Operator | 2.11.2 |

**OpenShift Service Mesh 3.1.2 supported versions:**

| Feature | Supported versions |
|---------|-------------------|
| OpenShift Service Mesh 3 Operator | 3.1.2 |
| OpenShift Service Mesh Istio control plane resource | 1.26.4 |
| OpenShift Container Platform | 4.16 and later |
| Envoy proxy | 1.34.6 |
| IstioCNI resource | 1.26.4 |
| Kiali Operator | 2.11.3 |
| Kiali control plane resource | 2.11.3 |

**Additional resources**
* Service Mesh 3.1.1 feature support tables
* Service Mesh 3.1.2 feature support tables
* Service Mesh version support tables

---

## 1.3. Understanding Operator updates and channels

The Operator Lifecycle Manager (OLM) manages Operators and their associated services by using channels to organize and distribute updates. Channels are a way to group related updates.

To ensure that your OpenShift Service Mesh stays current with the latest security patches, bug fixes, and software updates, keep the OpenShift Service Mesh Operator up to date. The upgrade process depends on the configured channel and approval strategy.

OLM provides the following channels for the OpenShift Service Mesh Operator:

**Stable channel:** tracks the most recent version of the OpenShift Service Mesh 3 Operator and the latest supported version of Istio. This channel enables upgrades to new operator versions and corresponding Istio updates as soon as they are released. Use the stable channel to stay current with the latest features, bug fixes, and security updates.

**Versioned channel:** restricts updates to patch-level releases within a specific minor version. For example, stable-3.0 provides access to the latest 3.1.1 patch version. When a new patch release becomes available, you can upgrade the Operator to the newer patch version. To move to a newer minor release, you must manually switch to a different channel. You can use a versioned channel to maintain a consistent minor version while applying only patch updates.

**Note:** You can find the update strategy field in the Install Operator section and under the sub-section update approval. The default value for the update strategy is Automatic.

### 1.3.1. About Operator update process

The OpenShift Service Mesh Operator will upgrade automatically to the latest available version based on the selected channel when the approval strategy field is set to Automatic (default). If the approval strategy field is set to Manual, Operator Lifecycle Manager (OLM) will generate an update request, which a cluster administrator must approve to update the Operator to the latest version.

The Operator update process does not automatically update the Istio control plane unless the Istio resource version is set to an alias (for example, vX.Y-latest) and the updateStrategy is set to InPlace. This triggers a control plane update when a new version is available in the Operator. By default, the Operator will not update the Istio control plane unless the Istio resource is updated with a new version.

---

## 1.4. About Istio update process

After updating the OpenShift Service Mesh Operator, update the Istio control plane to the latest supported version. The Istio resource configuration determines how the control plane upgrade is performed, including which steps require manual action and which are handled automatically.

In ambient mode, the upgrade process involves three main components:
* **Istio Control Plane** - Manages configuration and certificate distribution
* **IstioCNI** - Handles traffic redirection configuration
* **ZTunnel** - Per-node L4 proxy running as a DaemonSet

The Istio resource configuration includes the following fields that are relevant to the upgrade process:

**spec.version**
specifies the version of Istio to install. Use the format vX.Y.Z, where X.Y.Z is the desired Istio release. For example, set the field to v1.24.4 to install Istio 1.24.4. Alternatively, set the value to an alias such as vX.Y-latest to automatically install the latest supported patch version for the specified minor release.

**spec.updateStrategy**
defines the strategy for updating the Istio control plane. The available update strategies are InPlace and RevisionBased.

**Note:** To enable automatic patch upgrades, set the approval strategy of the Operator to Automatic. When the Operator detects a new patch release and the version field uses the vX.Y-latest alias, the control plane is updated based on the configured updateStrategy type.

### 1.4.1. About Istio control plane update strategies

The update strategy affects how the update process is performed. The spec.updateStrategy field in the Istio resource configuration determines how the OpenShift Service Mesh Operator updates the Istio control plane. When the Operator detects a change in the spec.version field or identifies a new minor release with a configured vX.Y-latest alias, it initiates an upgrade procedure. For each mesh, you select one of two strategies:

* InPlace
* RevisionBased

InPlace is the default strategy for updating OpenShift Service Mesh in ambient mode.

When the InPlace strategy is used in ambient mode, the existing Istio control plane, IstioCNI, and ZTunnel components are replaced with new versions. The ambient workloads immediately connect to the new control plane. The workloads therefore don't need to be moved from one control plane instance to another.

When the RevisionBased strategy is used in ambient mode, a new Istio control plane instance is created for every change to the Istio.spec.version field. The old control plane remains in place until all workloads have been moved to the new control plane instance. This canary upgrade approach allows for gradual migration and rollback if needed. IstioCNI and ZTunnel components support both control plane versions during migration.

---

## 1.5. About InPlace strategy

The InPlace update strategy runs only one revision of the control plane at a time. During an update, all the workloads immediately connect to the new control plane version. To maintain compatibility between the ambient data plane components and the control plane, you can upgrade only one minor version at a time.

The InPlace strategy updates and restarts the existing Istio control plane in place. During this process, only one instance of the control plane exists, eliminating the need to move workloads to a new control plane instance. For ambient mode, the IstioCNI and ZTunnel components are also updated using rolling updates.

While the InPlace strategy offers simplicity and efficiency, there's a slight possibility of application traffic interruption if a workload pod updates, restarts, or scales while the control plane is restarting. You can mitigate this risk by running multiple replicas of the Istio control plane (istiod).

### 1.5.1. Selecting InPlace strategy

To select the InPlace strategy, set the spec.updateStrategy.type value in the Istio resource to InPlace.

**Example specification to select InPlace update strategy**

```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
  version: v1.26.3
  updateStrategy:
    type: InPlace
```

You can set this value while creating the resource or edit it later. If you edit the resource after creation, make the change before updating the Istio control plane.

When using ambient mode with the InPlace strategy, ensure that all three components (Istio, IstioCNI, and ZTunnel) are updated to compatible versions. The update should be performed in the following order:
1. Istio control plane
2. IstioCNI
3. ZTunnel

Running the Istio resource in High Availability mode to minimize traffic disruptions requires additional property settings. For more information, see "About Istio High Availability".

### 1.5.2. Updating Istio control plane with InPlace strategy

When updating Istio using the InPlace strategy in ambient mode, you can increment the version by only one minor release at a time. To update by more than one minor version, you must increment the version after each update. The update process is complete after all ambient components (Istio control plane, IstioCNI, and ZTunnel) have been updated.

#### Update the Istio Control Plane

You can update the Istio control plane version by changing the version in the Istio resource. When using the InPlace update strategy, the Service Mesh Operator replaces the existing istiod deployment with a new version. The old istiod pod is terminated and a new pod with the updated version is created. Ambient workloads automatically reconnect to the new control plane once it becomes ready.

**Prerequisites:**

* You are logged in to OpenShift Container Platform as a user with the `cluster-admin` role.
* You have installed the Red Hat OpenShift Service Mesh Operator.
* You have deployed Istio in ambient mode with the InPlace update strategy.
* The Istio resource named `default` is deployed in the `istio-system` namespace with the InPlace update strategy.
* You have verified the current version and the Istio resource is in a Healthy state by running the following command:

```bash
$ oc get istio -n istio-system
NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
default   1           1       1        default           Healthy   v1.26.3   7d
```

**Procedure:**

1. Update the Istio resource version. For example, to update to Istio 1.26.4, set the `spec.version` field to `v1.26.4` by running the following command:

```bash
$ oc patch istio default -n istio-system --type='merge' -p '{"spec":{"version":"v1.26.4"}}'
```

2. Monitor the istiod pod replacement. The old pod will be terminated and a new pod will be created:

```bash
$ oc get pods -n istio-system -l app=istiod -w
```

3. Wait for the Istio control plane to become ready:

```bash
$ oc wait --for=condition=Ready istios/default -n istio-system --timeout=5m
```

4. Verify the Istio resource shows the new version and is in a Healthy state:

```bash
$ oc get istio -n istio-system
NAME      REVISIONS   READY   IN USE   ACTIVE REVISION   STATUS    VERSION   AGE
default   1           1       1        default           Healthy   v1.26.4   7d1h
```

5. Confirm the istiod pod is running with the new version:

```bash
$ oc get pods -n istio-system -l app=istiod
NAME                             READY   STATUS    RESTARTS   AGE
istiod-default-6bd6b8664b-x7k2m  1/1     Running   0          2m15s
```

6. Verify the control plane is functioning correctly by checking the istiod logs:

```bash
$ oc logs -n istio-system -l app=istiod --tail=50 | grep -i "version\|ready"
```

#### Update IstioCNI

After updating the Istio control plane, update the IstioCNI component. The Service Mesh Operator deploys a new version of the CNI plugin that replaces the old version. The `istio-cni-node` DaemonSet pods are updated using a rolling update strategy, and traffic redirection rules are maintained during the update process.

**Prerequisites:**

* You are logged in to OpenShift Container Platform as a user with the `cluster-admin` role.
* You have successfully updated the Istio control plane to the desired version.
* The IstioCNI resource named `default` is deployed in the `istio-cni` namespace.

**Procedure:**

1. Update the IstioCNI resource version. For example, to update to Istio 1.26.4, set the `spec.version` field to `v1.26.4` by running the following command:

```bash
$ oc patch istiocni -n istio-cni default --type='merge' -p '{"spec":{"version":"v1.26.4"}}'
```

2. Wait for the IstioCNI DaemonSet to be updated. The Service Mesh Operator updates the IstioCNI pods one node at a time:

```bash
$ oc wait --for=condition=Ready istiocnis/default --timeout=5m
```

3. Verify the IstioCNI resource shows the new version:

```bash
$ oc get istiocni
NAME      READY   STATUS    VERSION   AGE
default   True    Healthy   v1.26.4   7d1h
```

4. Confirm all IstioCNI DaemonSet pods are running with the new version:

```bash
$ oc get pods -n istio-cni
NAME                   READY   STATUS    RESTARTS   AGE
istio-cni-node-abc12   1/1     Running   0          3m
istio-cni-node-def34   1/1     Running   0          3m
istio-cni-node-ghi56   1/1     Running   0          3m
```

5. Verify the IstioCNI pods are healthy by checking the logs:

```bash
$ oc logs -n istio-cni -l k8s-app=istio-cni-node --tail=20
```

The IstioCNI pods should show successful initialization and no errors.

#### Update ZTunnel

After updating IstioCNI, update the ZTunnel component. The Service Mesh Operator updates the ZTunnel DaemonSet, which runs the L4 node proxies. The ZTunnel pods are updated using a rolling update strategy, updating one node at a time to maintain mesh connectivity during the upgrade. Existing connections are maintained while new connections use the updated ZTunnel proxies.

**Prerequisites:**

* You are logged in to OpenShift Container Platform as a user with the `cluster-admin` role.
* You have successfully updated the Istio control plane to the desired version.
* You have successfully updated the IstioCNI resource to the desired version.
* The ZTunnel resource named `default` is deployed in the `ztunnel` namespace.

**Procedure:**

1. Update the ZTunnel resource version. For example, to update to Istio 1.26.4, set the `spec.version` field to `v1.26.4` by running the following command:

```bash
$ oc patch ztunnel -n ztunnel default --type='merge' -p '{"spec":{"version":"v1.26.4"}}'
```

2. Monitor the ZTunnel DaemonSet rollout. The Service Mesh Operator updates the ZTunnel pods one node at a time:

```bash
$ oc rollout status daemonset/ztunnel -n ztunnel
```

**Note:** The ZTunnel DaemonSet update may take several minutes as pods are updated node-by-node to minimize disruption to ambient workloads.

3. Wait for the ZTunnel resource to become ready:

```bash
$ oc wait --for=condition=Ready ztunnel/default --timeout=10m
```

4. Verify the ZTunnel resource shows the new version:

```bash
$ oc get ztunnel
NAME      READY   STATUS    VERSION   AGE
default   True    Healthy   v1.26.4   7d1h
```

5. Confirm all ZTunnel pods are running with the new version on all nodes:

```bash
$ oc get pods -n ztunnel -o wide
NAME              READY   STATUS    RESTARTS   AGE   NODE
ztunnel-2w5mj     1/1     Running   0          5m    node1.example.com
ztunnel-6njq8     1/1     Running   0          4m    node2.example.com
ztunnel-96j7k     1/1     Running   0          3m    node3.example.com
```

6. Verify the ZTunnel pods are healthy by checking the logs:

```bash
$ oc logs -n ztunnel -l app=ztunnel --tail=20
```

The ZTunnel pods should show successful startup and connection to the Istio control plane.

#### Verify Ambient Workloads

1. Verify that your ambient workloads are still functioning correctly:

```bash
$ oc get pods -n bookinfo
NAME                             READY   STATUS    RESTARTS   AGE
details-v1-54ffdd5947-8gk5h      1/1     Running   0          7d
productpage-v1-d49bb79b4-cb9sl   1/1     Running   0          7d
ratings-v1-856f65bcff-h6kkf      1/1     Running   0          7d
reviews-v1-848b8749df-wl5br      1/1     Running   0          7d
reviews-v2-5fdf9886c7-8xprg      1/1     Running   0          7d
reviews-v3-bb6b8ddc7-bvcm5       1/1     Running   0          7d
```

2. Verify ZTunnel is processing traffic for your ambient workloads:

```bash
$ istioctl ztunnel-config workloads --namespace ztunnel | grep bookinfo
NAMESPACE    POD NAME                       ADDRESS      NODE                        WAYPOINT PROTOCOL
bookinfo     details-v1-54ffdd5947-8gk5h    10.131.0.69  node1.example.com           None     HBONE
bookinfo     productpage-v1-d49bb79b4-cb9sl 10.128.2.80  node2.example.com           None     HBONE
bookinfo     ratings-v1-856f65bcff-h6kkf    10.131.0.70  node1.example.com           None     HBONE
bookinfo     reviews-v1-848b8749df-wl5br    10.131.0.72  node1.example.com           None     HBONE
bookinfo     reviews-v2-5fdf9886c7-8xprg    10.128.2.78  node2.example.com           None     HBONE
bookinfo     reviews-v3-bb6b8ddc7-bvcm5     10.128.2.79  node2.example.com           None     HBONE
```

3. Test connectivity within your mesh:

```bash
$ oc exec "$(oc get pod -l app=ratings -n bookinfo -o jsonpath='{.items[0].metadata.name}')" -c ratings -n bookinfo -- curl -sS productpage:9080/productpage | grep -o "<title>.*</title>"
<title>Simple Bookstore App</title>
```

#### Update Waypoint Proxies (If Deployed)

If you have deployed waypoint proxies in your ambient mesh, they should be updated after the control plane upgrade:

1. List existing waypoint proxies:

```bash
$ oc get gateway -n bookinfo
NAME       CLASS              ADDRESS        PROGRAMMED   AGE
waypoint   istio-waypoint     10.96.123.45   True         7d
```

2. Waypoint proxies should automatically update to use the new control plane. Verify the waypoint proxy pods are running:

```bash
$ oc get pods -n bookinfo -l gateway.networking.k8s.io/gateway-name=waypoint
NAME                       READY   STATUS    RESTARTS   AGE
waypoint-5d9c8b7f9-abc12   1/1     Running   0          5m
```

### 1.5.3. Recommendations for InPlace strategy in Ambient Mode

* **High Availability:** Configure the istiod deployment with multiple replicas for high availability during updates. See the [Istiod HA guide](../../general/istiod-ha.md) for more information.
* **ZTunnel Updates:** The ZTunnel DaemonSet uses a RollingUpdate strategy by default, which updates pods one node at a time. Monitor the rollout to ensure it completes successfully.
* **Maintenance Window:** While ambient mode is designed to minimize disruption, it's recommended to perform upgrades during a maintenance window.
* **Testing:** Always test the upgrade process in a non-production environment first.

---

## 1.6. About RevisionBased strategy

The RevisionBased strategy runs two revisions of the control plane during an upgrade. This approach supports gradual workload migration from the old control plane to the new one, enabling canary upgrades. It also supports upgrades across more than one minor version.

The RevisionBased strategy creates a new Istio control plane instance for each change to the spec.version field. The existing control plane remains active until all workloads transition to the new instance. In ambient mode, workloads automatically connect to the active control plane revision.

Although the RevisionBased strategy involves additional steps and requires multiple control plane instances to run concurrently during the upgrade, it allows for gradual migration of workloads. This approach enables validation of the updated control plane before completing the migration, making it useful for large meshes with mission-critical workloads.

### 1.6.1. Selecting RevisionBased strategy

To deploy Istio with the RevisionBased strategy, create the Istio resource with the following spec.updateStrategy value:

**Example specification to select RevisionBased strategy**

```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
  version: v1.26.3
  updateStrategy:
    type: RevisionBased
    inactiveRevisionDeletionGracePeriodSeconds: 30
```

After you select the strategy for the Istio resource, the Operator creates a new IstioRevision resource with the name `<istio_resource_name>-<version>`. For example, if the Istio resource is named `default` and the version is `v1.26.3`, the IstioRevision resource name would be `default-v1-26-3`.

When using ambient mode with the RevisionBased strategy, IstioCNI and ZTunnel components are compatible with multiple control plane versions and continue to function during the workload migration period.

### 1.6.2. Updating Istio control plane with RevisionBased strategy

When updating Istio using the RevisionBased strategy in ambient mode, you can upgrade by more than one minor version at a time. The Red Hat OpenShift Service Mesh Operator creates a new IstioRevision resource for each change to the .spec.version field and deploys a corresponding control plane instance.

#### Update the Istio Control Plane

You can update the Istio control plane version by changing the version in the Istio resource. When using the RevisionBased update strategy, the Service Mesh Operator creates a new istiod deployment alongside the existing one, allowing for a canary upgrade. Both control planes run simultaneously until all workloads are migrated to the new version. The new control plane is created with a revision name in the format `<istio-name>-<version>`.

**Prerequisites:**

* You are logged in to OpenShift Container Platform as a user with the `cluster-admin` role.
* You have installed the Red Hat OpenShift Service Mesh Operator.
* You have deployed Istio in ambient mode with the RevisionBased update strategy.
* The Istio resource named `default` is deployed in the `istio-system` namespace with the RevisionBased update strategy.
* You have verified the current version and the Istio resource is in a Healthy state by running the following commands:

```bash
$ oc get istio default -n istio-system -o yaml | grep -A 3 updateStrategy
  updateStrategy:
    type: RevisionBased
    inactiveRevisionDeletionGracePeriodSeconds: 30
```

```bash
$ oc get istio -n istio-system
NAME      REVISIONS   READY   IN USE   ACTIVE REVISION     STATUS    VERSION   AGE
default   1           1       1        default-v1-26-3     Healthy   v1.26.3   7d
```

```bash
$ oc get istiorevision -n istio-system
NAME              TYPE    READY   STATUS    IN USE   VERSION   AGE
default-v1-26-3   Local   True    Healthy   True     v1.26.3   7d
```

* The `inactiveRevisionDeletionGracePeriodSeconds` is configured in the Istio resource.

**Procedure:**

1. Update the Istio resource version. For example, to update to Istio 1.26.4, set the `spec.version` field to `v1.26.4` by running the following command:

```bash
$ oc patch istio default -n istio-system --type='merge' -p '{"spec":{"version":"v1.26.4"}}'
```

This command creates a new IstioRevision resource and a new istiod deployment for the new version.

2. Monitor the new istiod pod creation:

```bash
$ oc get pods -n istio-system -l app=istiod -w
```

3. Wait for the new control plane revision to become ready:

```bash
$ oc wait --for=condition=Ready istios/default -n istio-system --timeout=5m
```

4. Verify both revisions are now running. The Istio resource should show 2 revisions:

```bash
$ oc get istio -n istio-system
NAME      REVISIONS   READY   IN USE   ACTIVE REVISION     STATUS    VERSION   AGE
default   2           2       1        default-v1-26-4     Healthy   v1.26.4   7d1h
```

5. List the IstioRevision resources to see both versions:

```bash
$ oc get istiorevision -n istio-system
NAME              TYPE    READY   STATUS    IN USE   VERSION   AGE
default-v1-26-3   Local   True    Healthy   True     v1.26.3   7d
default-v1-26-4   Local   True    Healthy   False    v1.26.4   2m
```

The old revision shows `IN USE: True` because workloads are still connected to it. The new revision shows `IN USE: False` until workloads are migrated.

6. Confirm both control plane pods are running:

```bash
$ oc get pods -n istio-system -l app=istiod
NAME                                      READY   STATUS    RESTARTS   AGE
istiod-default-v1-26-3-6bd6b8664b-x7k2m   1/1     Running   0          7d
istiod-default-v1-26-4-7c8e9d775c-y8l3n   1/1     Running   0          2m
```

7. Verify the new control plane is functioning by checking its logs:

```bash
$ oc logs -n istio-system istiod-default-v1-26-4-7c8e9d775c-y8l3n --tail=50 | grep -i "version\|ready"
```

#### Update IstioCNI

After creating the new Istio control plane revision, update the IstioCNI component. The Service Mesh Operator deploys a new version of the CNI plugin that replaces the old version. The IstioCNI component is compatible with multiple control plane versions and continues to handle traffic redirection for both the old and new control planes during the migration period.

**Prerequisites:**

* You are logged in to OpenShift Container Platform as a user with the `cluster-admin` role.
* You have successfully created the new Istio control plane revision.
* Both control plane revisions are running and healthy.
* The IstioCNI resource named `default` is deployed in the `istio-cni` namespace.

**Procedure:**

1. Update the IstioCNI resource version. For example, to update to Istio 1.26.4, set the `spec.version` field to `v1.26.4` by running the following command:

```bash
$ oc patch istiocni -n istio-cni default --type='merge' -p '{"spec":{"version":"v1.26.4"}}'
```

2. Wait for the IstioCNI DaemonSet to be updated:

```bash
$ oc wait --for=condition=Ready istiocnis/default --timeout=5m
```

3. Verify the IstioCNI resource shows the new version:

```bash
$ oc get istiocni
NAME      READY   STATUS    VERSION   AGE
default   True    Healthy   v1.26.4   7d1h
```

4. Confirm all IstioCNI pods are running with the new version:

```bash
$ oc get pods -n istio-cni
NAME                   READY   STATUS    RESTARTS   AGE
istio-cni-node-abc12   1/1     Running   0          3m
istio-cni-node-def34   1/1     Running   0          3m
istio-cni-node-ghi56   1/1     Running   0          3m
```

**Note:** IstioCNI is compatible with multiple control plane versions and continues to work with both the old and new control planes during the workload migration.

#### Update ZTunnel

After updating IstioCNI, update the ZTunnel component. The Service Mesh Operator updates the ZTunnel DaemonSet, which runs the L4 node proxies. The ZTunnel component is compatible with multiple control plane versions and can communicate with both the old and new control planes simultaneously, allowing for smooth workload migration between revisions.

**Prerequisites:**

* You are logged in to OpenShift Container Platform as a user with the `cluster-admin` role.
* You have successfully created the new Istio control plane revision.
* You have successfully updated the IstioCNI resource to the desired version.
* Both control plane revisions are running and healthy.
* The ZTunnel resource named `default` is deployed in the `ztunnel` namespace.

**Procedure:**

1. Update the ZTunnel resource version. For example, to update to Istio 1.26.4, set the `spec.version` field to `v1.26.4` by running the following command:

```bash
$ oc patch ztunnel default --type='merge' -p '{"spec":{"version":"v1.26.4"}}'
```

2. Monitor the ZTunnel DaemonSet rollout:

```bash
$ oc rollout status daemonset/ztunnel -n ztunnel
```

3. Wait for the ZTunnel resource to become ready:

```bash
$ oc wait --for=condition=Ready ztunnel/default --timeout=10m
```

4. Verify the ZTunnel resource shows the new version:

```bash
$ oc get ztunnel
NAME      READY   STATUS    VERSION   AGE
default   True    Healthy   v1.26.4   7d1h
```

5. Confirm all ZTunnel pods are running with the new version:

```bash
$ oc get pods -n ztunnel -o wide
NAME              READY   STATUS    RESTARTS   AGE   NODE
ztunnel-2w5mj     1/1     Running   0          5m    node1.example.com
ztunnel-6njq8     1/1     Running   0          4m    node2.example.com
ztunnel-96j7k     1/1     Running   0          3m    node3.example.com
```

**Note:** ZTunnel can communicate with multiple control plane versions, allowing ambient workloads to migrate between revisions smoothly without disruption.

#### Migrate Ambient Workloads to New Revision

Unlike sidecar mode, ambient mode workloads don't use namespace labels like `istio.io/rev` for version selection. Instead, ambient workloads automatically connect to the active control plane revision. However, to ensure proper migration:

1. Verify that your ambient namespaces are still labeled correctly:

```bash
$ oc get namespace bookinfo --show-labels | grep istio
NAME       STATUS   AGE   LABELS
bookinfo   Active   7d    istio-discovery=enabled,istio.io/dataplane-mode=ambient
```

2. The ambient workloads automatically use the new control plane. Verify connectivity:

```bash
$ istioctl ztunnel-config workloads --namespace ztunnel | grep bookinfo
```

3. For more controlled migration, you can temporarily restart application pods to ensure they pick up any configuration changes:

```bash
$ oc rollout restart deployment -n bookinfo
```

4. Wait for the rollout to complete:

```bash
$ oc rollout status deployment -n bookinfo
```

5. Verify the workloads are functioning correctly:

```bash
$ oc exec "$(oc get pod -l app=ratings -n bookinfo -o jsonpath='{.items[0].metadata.name}')" -c ratings -n bookinfo -- curl -sS productpage:9080/productpage | grep -o "<title>.*</title>"
<title>Simple Bookstore App</title>
```

#### Verify Old Revision Cleanup

1. After the grace period (specified in `inactiveRevisionDeletionGracePeriodSeconds`), verify that the old revision has been cleaned up:

```bash
$ oc get istiorevision -n istio-system
NAME              TYPE    READY   STATUS    IN USE   VERSION   AGE
default-v1-26-4   Local   True    Healthy   True     v1.26.4   35m
```

2. Confirm only the new control plane pods are running:

```bash
$ oc get pods -n istio-system -l app=istiod
NAME                                      READY   STATUS    RESTARTS   AGE
istiod-default-v1-26-4-7c8e9d775c-y8l3n   1/1     Running   0          35m
```

3. Verify the Istio resource reflects the single active revision:

```bash
$ oc get istio -n istio-system
NAME      REVISIONS   READY   IN USE   ACTIVE REVISION     STATUS    VERSION   AGE
default   1           1       1        default-v1-26-4     Healthy   v1.26.4   7d1h
```

#### Update Waypoint Proxies (If Deployed)

If you have deployed waypoint proxies, they should be verified after the upgrade:

1. Verify waypoint proxies are functioning with the new control plane:

```bash
$ oc get gateway -n bookinfo
NAME       CLASS              ADDRESS        PROGRAMMED   AGE
waypoint   istio-waypoint     10.96.123.45   True         7d
```

2. Check the waypoint proxy pods:

```bash
$ oc get pods -n bookinfo -l gateway.networking.k8s.io/gateway-name=waypoint
NAME                       READY   STATUS    RESTARTS   AGE
waypoint-5d9c8b7f9-abc12   1/1     Running   0          5m
```

### 1.6.3. Rollback Procedure

If you encounter issues during the RevisionBased upgrade, you can roll back before the old revision is deleted:

1. Verify the old revision is still available:

```bash
$ oc get istiorevision -n istio-system
NAME              TYPE    READY   STATUS    IN USE   VERSION   AGE
default-v1-26-3   Local   True    Healthy   False    v1.26.3   7d
default-v1-26-4   Local   True    Healthy   True     v1.26.4   10m
```

2. Roll back the Istio resource to the previous version:

```bash
$ oc patch istio default -n istio-system --type='merge' -p '{"spec":{"version":"v1.26.3"}}'
```

3. Roll back IstioCNI and ZTunnel if needed:

```bash
$ oc patch istiocni default --type='merge' -p '{"spec":{"version":"v1.26.3"}}'
$ oc patch ztunnel default --type='merge' -p '{"spec":{"version":"v1.26.3"}}'
```

4. Restart application pods:

```bash
$ oc rollout restart deployment -n bookinfo
```

---

## 1.7. About the Istio CNI update process

The Istio Container Network Interface (CNI) update process uses in-place updates. When the IstioCNI resource changes, the daemonset automatically replaces the existing istio-cni-node pods with the specified version of the CNI plugin.

You can use the following field to manage version updates:

**spec.version**
defines the CNI plugin version to install. Specify the value in the format vX.Y.Z, where X.Y.Z represents the desired version. For example, use v1.26.4 to install the CNI plugin version 1.26.4.

To update the CNI plugin, modify the spec.version field with the target version. The IstioCNI resource also includes a values field that exposes configuration options from the istio-cni chart.

In ambient mode, the IstioCNI component is responsible for traffic redirection. The component is compatible with multiple control plane versions during RevisionBased upgrades and continues to handle traffic redirection for both old and new control planes during the migration period.

### 1.7.1. Updating the Istio CNI resource version

You can update the Istio CNI resource version by changing the version in the resource. Then, the Service Mesh Operator deploys a new version of the CNI plugin that replaces the old version of the CNI plugin. The istio-cni-node pods automatically reconnect to the new CNI plugin.

**Prerequisites:**

* You are logged in to OpenShift Container Platform as a user with the `cluster-admin` role.
* You installed the Red Hat OpenShift Service Mesh Operator and deployed Istio in ambient mode.
* You installed the Istio CNI plugin with the desired version. In the following example, the IstioCNI resource named `default` is deployed in the `istio-cni` namespace.

**Procedure:**

1. Change the version in the Istio resource. For example, to update to Istio 1.26.4, set the `spec.version` field to `v1.26.4` by running the following command:

```bash
$ oc patch istiocni default --type='merge' -p '{"spec":{"version":"v1.26.4"}}'
```

2. Confirm that the new version of the CNI plugin is ready by running the following command:

```bash
$ oc get istiocni default
```

**Example Output**

```console
NAME      READY   STATUS    VERSION   AGE
default   True    Healthy   v1.26.4   91m
```

---

## 1.8. Special Considerations for Ambient Mode Upgrades

### 1.8.1. ZTunnel DaemonSet Updates

The ZTunnel component runs as a DaemonSet on every node in the cluster. During upgrades:

* **Rolling Updates:** ZTunnel uses a RollingUpdate strategy, updating one node at a time by default.
* **Minimal Disruption:** While a node's ZTunnel pod is restarting, new connections may experience brief latency, but existing connections are maintained.
* **Node-by-Node:** The update process ensures that at least one ZTunnel pod is always available on each node before proceeding to the next.
* **Monitoring:** Monitor the ZTunnel DaemonSet rollout status:

```bash
$ oc rollout status daemonset/ztunnel -n ztunnel
```

### 1.8.2. IstioCNI Considerations

The IstioCNI component is responsible for traffic redirection in ambient mode:

* **Version Compatibility:** IstioCNI must be compatible with the ZTunnel version. Always upgrade IstioCNI before or alongside ZTunnel.
* **DaemonSet Updates:** Like ZTunnel, IstioCNI runs as a DaemonSet and uses rolling updates.
* **Traffic Redirection:** During the update, existing traffic redirection rules remain in place until the new version is applied.
* **Verification:** After updating, verify IstioCNI pods are running on all nodes:

```bash
$ oc get pods -n istio-cni -o wide
```

### 1.8.3. Control Plane and Data Plane Version Skew

In ambient mode, version skew between components is handled differently than in sidecar mode:

* **Supported Skew:** The control plane (Istio) can typically be N+1 or N-1 relative to the data plane (ZTunnel).
* **Testing Required:** Always test your specific version combinations in a non-production environment.
* **Recommendation:** Keep all components (Istio, IstioCNI, ZTunnel) at the same version when possible.

### 1.8.4. Waypoint Proxy Compatibility

If you have deployed waypoint proxies for L7 features:

* **Automatic Updates:** Waypoint proxies automatically reference the active control plane revision.
* **Gateway API:** Waypoint proxies are deployed using Kubernetes Gateway resources and will continue to function during upgrades.
* **Recreation:** In rare cases, you may need to recreate waypoint Gateway resources if there are breaking changes between versions.
* **Verification:** Test L7 features after the upgrade to ensure waypoint proxies are functioning correctly.

### 1.8.5. Impact on Existing Ambient Workloads

During ambient mode upgrades:

* **No Pod Restarts Required:** Unlike sidecar mode, ambient workloads don't require pod restarts to pick up the new mesh version.
* **Automatic Reconnection:** Workloads automatically reconnect to the upgraded control plane.
* **mTLS Continuity:** Mutual TLS connections are maintained throughout the upgrade process.
* **Optional Restart:** You may choose to restart workloads to ensure they receive the latest configuration, but this is not required.

### 1.8.6. Discovery Selectors Impact

If you're using discovery selectors to scope your mesh:

* **Label Verification:** Ensure that all required namespaces (istio-system, istio-cni, ztunnel) retain their discovery selector labels during upgrades.
* **Namespace Discovery:** The control plane must discover all necessary namespaces for proper operation.
* **Verification Command:**

```bash
$ oc get namespace -l istio-discovery=enabled
NAME           STATUS   AGE
istio-system   Active   7d
istio-cni      Active   7d
ztunnel        Active   7d
bookinfo       Active   7d
```

### 1.8.7. Troubleshooting Common Issues

**Issue: ZTunnel pods stuck in update**
* Check the DaemonSet rollout status: `oc rollout status daemonset/ztunnel -n ztunnel`
* Review ZTunnel pod logs: `oc logs -n ztunnel -l app=ztunnel --tail=100`
* Verify node resources are sufficient for pod scheduling

**Issue: Workloads not connecting to new control plane**
* Verify the control plane is in Ready state: `oc get istio -n istio-system`
* Check istiod logs for errors: `oc logs -n istio-system -l app=istiod --tail=100`
* Ensure discovery selectors include all necessary namespaces

**Issue: IstioCNI not redirecting traffic**
* Verify IstioCNI pods are running: `oc get pods -n istio-cni`
* Check IstioCNI logs: `oc logs -n istio-cni -l k8s-app=istio-cni-node --tail=100`
* Ensure the namespace has the correct label: `istio.io/dataplane-mode=ambient`

**Issue: Waypoint proxies not functioning**
* Verify Gateway resource exists: `oc get gateway -n <namespace>`
* Check waypoint pod logs: `oc logs -n <namespace> -l gateway.networking.k8s.io/gateway-name=waypoint`
* Ensure the namespace has the waypoint label: `istio.io/use-waypoint=<waypoint-name>`

---

## Additional Resources

* **Upstream Istio Ambient Documentation:** [https://istio.io/latest/docs/ambient/](https://istio.io/latest/docs/ambient/)
* **Istio Upgrade Documentation:** [https://istio.io/latest/docs/setup/upgrade/](https://istio.io/latest/docs/setup/upgrade/)
* **OpenShift Service Mesh Ambient Mode Installation:** [../README.md](../README.md)
* **OpenShift Service Mesh Waypoint Proxy Guide:** [../waypoint.md](../waypoint.md)
* **General Update Strategy Documentation:** [../../update-strategy/update-strategy.md](../../update-strategy/update-strategy.md)
