# Adding services to a service mesh
OpenShift Service Mesh 3 is very different compared to OpenShift Service Mesh 2 in a way how one can add a service to your service mesh. OSSM 3 is much closer to the [Istio](https://istio.io/) project. Detailed list of OSSM 3 vs. OSSM 2 changes is available in [Compared to OpenShift Service Mesh 2](../ossm2-vs-ossm3.md) chapter.

This document describes basic concepts how Istio control plane monitors your OCP resources and how to scope your service mesh.

## Concepts

### Cluster wide by default
In order to program the service mesh, the Istio control plane (Istiod) reads a variety of configurations, including core OpenShift types like Service and Node, and Istio’s own types like Gateway. These are then sent to the data plane.

By default, the control plane will read all configuration in all namespaces. Each proxy instance will receive configuration for all namespaces as well. This includes information about workloads that are not enrolled in the mesh.

This default ensures correct behavior out of the box, but comes with a scalability cost. Each configuration has a cost (in CPU and memory, primarily) to maintain and keep up to date. At large scales, it is critical to limit the configuration scope to avoid excessive resource consumption.

### Soft multi-tenancy
Is defined as having a single OpenShift control plane with multiple Istio control planes and multiple meshes, one control plane and one mesh per tenant. The cluster administrator gets control and visibility across all the Istio control planes, while the tenant administrator only gets control of a specific Istio instance. Separation between the tenants is provided by OpenShift namespaces and RBAC.

## Configuration scoping
Istio offers a few tools to help control the scope of a configuration to meet different use cases. Depending on your requirements, these can be used alone or together.

- `Sidecar` provides a mechanism for specific workloads to import a set of configurations
- `exportTo` provides a mechanism to export a configuration to a set of workloads
- `discoverySelectors` provides a mechanism to let Istio completely ignore a set of configurations

### Sidecar import
The `egress.hosts` field in [Sidecar](https://istio.io/latest/docs/reference/config/networking/sidecar/) allows specifying a list of configurations to import. Only configurations matching the specified criteria will be seen by sidecars impacted by the `Sidecar` resource.

For example:
```yaml
apiVersion: networking.istio.io/v1
kind: Sidecar
metadata:
  name: default
spec:
  egress:
  - hosts:
    - "./*" # Import all configuration from our own namespace
    - "bookinfo/*" # Import all configuration from the bookinfo namespace
    - "external-services/example.com" # Import only 'example.com' from the external-services namespace
```
### exportTo
Istio’s `VirtualService`, `DestinationRule`, and `ServiceEntry` provide a `spec.exportTo` field. Similarly, `Service` can be configured with the `networking.istio.io/exportTo` annotation.

Unlike Sidecar which allows a workload owner to control what dependencies it has, exportTo works in the opposite way, and allows the service owners to control their own service’s visibility.

For example, this configuration makes the `details Service` only visible to its own namespace, and the client namespace:
```yaml
apiVersion: v1
kind: Service
metadata:
  name: details
  annotations:
    networking.istio.io/exportTo: ".,client"
spec: ...
```

### DiscoverySelectors
While the previous controls operate on a workload or service owner level, `DiscoverySelectors` provides mesh wide control over configuration visibility. Discovery selectors allows specifying criteria for which namespaces should be visible to the control plane. Any namespaces not matching are ignored by the control plane entirely.

> **_NOTE:_** Istiod will always open a watch to OpenShift for all namespaces. However, discovery selectors will ignore objects that are not selected very early in its processing, minimizing costs.

> **_NOTE:_** `discoverySelectors` is not a security boundary. Istiod will continue to have access to all namespaces even when you have configured your `discoverySelectors`.

#### DiscoverySelectors vs. Sidecar
The `discoverySelectors` configuration enables users to dynamically restrict the set of namespaces that are part of the mesh. A `Sidecar` resource also controls the visibility of sidecar configurations and what gets pushed to the sidecar proxy. What are the differences between them?

- The `discoverySelectors` configuration declares what Istio control plane watches and processes. Without `discoverySelectors` configuration, the Istio control plane watches and processes all namespaces/services/endpoints/pods in the cluster regardless of the sidecar resources you have.
- `discoverySelectors` is configured globally for the mesh by the mesh administrators. While `Sidecar` resources can also be configured for the mesh globally by the mesh administrators in the MeshConfig root namespace, they are commonly configured by service owners for their namespaces.

You can use `discoverySelectors` with `Sidecar` resources. You can use `discoverySelectors` to configure at the mesh-wide level what namespaces the Istio control plane should watch and process. For these namespaces in the Istio service mesh, you can create `Sidecar` resources globally or per namespace to further control what gets pushed to the sidecar proxies.

 #### Using DiscoverySelectors
 `discoverySelectors` field accepts an array of Kubernetes [selectors](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#resources-that-support-set-based-requirements). The exact type is `[]LabelSelector`, as defined [here](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#resources-that-support-set-based-requirements), allowing both simple selectors and set-based selectors. These selectors apply to labels on namespaces.

You can configure each label selector for expressing a variety of use cases, including but not limited to:

- Arbitrary label names/values, for example, all namespaces with label `istio-discovery=enabled`
- A list of namespace labels using set-based selectors which carries OR semantics, for example, all namespaces with label `istio-discovery=enabled` OR `region=us-east1`
- Inclusion and/or exclusion of namespaces, for example, all namespaces with label `istio-discovery=enabled` AND label key `app` equal to `helloworld`

Example of Istio CR with `discoverySelectors` defined:
```yaml
kind: Istio
apiVersion: sailoperator.io/v1alpha1
metadata:
  name: ossm3
spec:
  namespace: istio-system3
  values:
    meshConfig:
      discoverySelectors:
        - matchExpressions:
          - key: maistra.io/member-of
            operator: DoesNotExist
  updateStrategy:
    type: InPlace
  version: v1.23.0
```

## Installing the Sidecar
### Injection
In order to take advantage of all of Istio’s features, pods in the mesh must be running an Istio sidecar proxy.

The following sections describe automatic Istio sidecar injection in the pod’s namespace. Manual injection using the `istioctl` command is not supported by OpenShift Service Mesh 3.

When enabled in a pod’s namespace, automatic injection injects the proxy configuration at pod creation time using an admission controller.

### Automatic sidecar injection
Sidecars can be automatically added to applicable Kubernetes pods using a [mutating webhook admission controller](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/) provided by Istio.

When you set the `istio-injection=enabled` or `istio.io/rev=<revision>` label on a namespace, any new pods that are created in that namespace will automatically have a sidecar added to them.

Automatic injection occurs at the pod-level. You won’t see any change to the deployment itself. Instead, you’ll want to check individual pods (via `oc describe`) to see the injected proxy.

#### Controlling the injection policy
You enabled and disabled injection at the namespace level. Injection can also be controlled on a per-pod basis, by configuring the `sidecar.istio.io/inject` label on a pod:
| Resource | Label | Enabled value | Disabled value |
| --- | --- | --- | --- |
| Namespace | `istio-injection` | `enabled` | `disabled` |
| Pod | `sidecar.istio.io/inject` | `"true"` | `"false"` |

If you are using control plane revisions, revision specific labels are instead used by a matching `istio.io/rev` label. For example, for a revision named `canary`:
| Resource | Enabled Label | Disabled Label |
| --- | --- | --- |
| Namespace | `istio.io/rev=canary` | `istio-injection=disabled` |
| Pod | `istio.io/rev=canary` | `sidecar.istio.io/inject="false"` |

If the `istio-injection` label and the `istio.io/rev` label are both present on the same namespace, the istio-injection label will take precedence.

The injector is configured with the following logic:

1. If either label (`istio-injection` or `sidecar.istio.io/inject`) is disabled, the pod is not injected.
2. If either label (`istio-injection` or `sidecar.istio.io/inject` or `istio.io/rev`) is enabled, the pod is injected.

#### Deploying an app
Prerequisites:
- OpenShift Service Mesh 3 operator is installed
- Istio CNI resource is created

1. Create `default` Istio CR in `istio-system` namespace:
    ```bash
    oc apply -f - <<EOF
    kind: Istio
    apiVersion: sailoperator.io/v1alpha1
    metadata:
      name: default
    spec:
      namespace: istio-system
      updateStrategy:
        type: InPlace
      version: v1.23.0
    EOF
    ```
1. Deploy `sleep` app:
    ```bash
    oc apply -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/sleep/sleep.yaml
    ```
1. Verify both deployment and pod have a single container:
    ```bash
    oc get deployment -o wide
    NAME    READY   UP-TO-DATE   AVAILABLE   AGE   CONTAINERS   IMAGES            SELECTOR
    sleep   1/1     1            1           16s   sleep        curlimages/curl   app=sleep
    oc get pod -l app=sleep
    NAME                     READY   STATUS    RESTARTS   AGE
    sleep-5577c64d7c-ntn9d   1/1     Running   0          16s
    ```
1. Label the `default` namespace with `istio-injection=enabled`:
    ```bash
    oc label namespace default istio-injection=enabled
    ```
1. Injection occurs at pod creation time. Kill the running pod and verify a new pod is created with the injected sidecar. The original pod has `1/1 READY` containers, and the pod with injected sidecar has `2/2 READY` containers.
    ```bash
    oc delete pod -l app=sleep
    oc get pod -l app=sleep
    NAME                     READY   STATUS    RESTARTS   AGE
    sleep-5577c64d7c-w9vpk   2/2     Running   0          12s
    ```
1. View detailed state of the injected pod. You should see the injected `istio-proxy` container.
    ```bash
    oc describe pod -l app=sleep
    ...
    Events:
      Type    Reason          Age   From               Message
      ----    ------          ----  ----               -------
      Normal  Scheduled       50s   default-scheduler  Successfully assigned default/sleep-5577c64d7c-w9vpk to user-rhos-d-1-v8rnx-worker-0-rwjrr
      Normal  AddedInterface  50s   multus             Add eth0 [10.128.2.179/23] from ovn-kubernetes
      Normal  Pulled          50s   kubelet            Container image "registry.redhat.io/openshift-service-mesh-tech-preview/istio-proxyv2-rhel9@sha256:c0170ef9a34869828a5f2fea285a7cda543d99e268f7771e6433c54d6b2cbaf4" already present on machine
      Normal  Created         50s   kubelet            Created container istio-validation
      Normal  Started         50s   kubelet            Started container istio-validation
      Normal  Pulled          50s   kubelet            Container image "curlimages/curl" already present on machine
      Normal  Created         50s   kubelet            Created container sleep
      Normal  Started         50s   kubelet            Started container sleep
      Normal  Pulled          50s   kubelet            Container image "registry.redhat.io/openshift-service-mesh-tech-preview/istio-proxyv2-rhel9@sha256:c0170ef9a34869828a5f2fea285a7cda543d99e268f7771e6433c54d6b2cbaf4" already present on machine
      Normal  Created         50s   kubelet            Created container istio-proxy
      Normal  Started         50s   kubelet            Started container istio-proxy
    ...
    ```

> **_NOTE:_** In case you choose a different name for the control plane (other than `default`), you have to use `istio.io/rev=<revName>` label instead of `istio-injection=enabled`.


