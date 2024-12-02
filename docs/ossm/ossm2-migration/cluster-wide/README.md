# OpenShift Service Mesh 2.6 Cluster wide --> 3 Migration guide
This guide is for users who are currently running `ClusterWide` OpenShift Service Mesh 2.6 migrating to OpenShift Service Mesh 3.x. You should first read [this document comparing OpenShift Service Mesh 2 vs. OpenShift Service Mesh 3](../../ossm2-vs-ossm3.md) to familiarize yourself with the concepts between the two versions and the differences in managing the workloads and addons.

## Migrating OpenShift Service Mesh 2.6 Cluster wide to OpenShift Service Mesh 3

### Prerequisites
- you have completed all the steps in the [pre-migration checklist](../README.md#pre-migration-checklist)
- you have read [OpenShift Service Mesh 2 vs. OpenShift Service Mesh 3](../../ossm2-vs-ossm3.md)
- you have verified that your 2.6 `ServiceMeshControlPlane` is using `ClusterWide` mode
- Red Hat OpenShift Service Mesh 3 operator is installed
- `IstioCNI` is installed
- `istioctl` is installed

### Procedure
In this example, we'll be using the [bookinfo demo](https://raw.githubusercontent.com/Maistra/istio/maistra-2.6/samples/bookinfo/platform/kube/bookinfo.yaml) but you can follow these same steps with your own workloads.

TODO: should we add a note about `minReplicas` for workload deployments?
TODO: include also ingress gateway deployment migration?

#### Plan the migration
There will be two cluster wide Istio control planes running during the migration process so it's necessary to plan the migration steps in advance in order to avoid possible conflicts between the two control planes.

There is a few conditions which must be assured to achieve the seamless migration:
- both control planes must share the same root certificate

  This can be achieved simply by installing the 3.0 control plane to the same namespace as 2.6 control plane
- both control planes have access to all migrated namespaces and namespaces running addons (e.g. otel collector)
  1. verify there are no Network Policies blocking the traffic
  1. `discoverySelectors`  defined in 3.0 `Istio` resource must match migrated namespaces. See [Scoping the service mesh with DiscoverySelectors](../../create-mesh/README.md)
- only one control plane will try to inject a side car

To achieve last condition, it's necessary to understand how the injection works. Please see [Installing the Sidecar](../../injection/README.md) for details.
> [!CAUTION]
> In case you decide to use a `default` name for 3.0 `Istio` resource with `InPlace` upgrade strategy, 3.0 control plane will try to inject side cars to all pods in namespaces with `istio-injection=enabled` label and all pods with `sidecar.istio.io/inject="true"` label after next restart of the workloads. Without using `maistra.io/ignore-namespace: "true"` label, it will result in a conflict with 2.6 control plane injector.

In case you are using `istio-injection=enabled` label on your data plane namespaces and planning to keep it, you should first read revision tag documentation. TODO: add link. TODO: describe cases where the migration will work with `istio-injection=enabled` (default revision, default revision tag)


#### Install OpenShift Service Mesh 3

1. Create your `Istio` resource.

    Here we are not using any `discoverySelectors` so the control plane will have access to all namespaces. In case you want to define `discoverySelectors`, keep in mind that all data plane namespaces you are planning to migrate must be matched.
    > [!CAUTION]
    > it is important your `Istio` resource's `spec.namespace` field is the **same** namespace as your `ServiceMeshControlPlane`. If you set your `Istio` resource's `spec.namespace` field to a different namespace than your `ServiceMeshControlPlane`, the migration will not work properly.
    ```yaml
   apiVersion: sailoperator.io/v1alpha1
   kind: Istio
   metadata:
     name: ossm-3
   spec:
     namespace: istio-system
     version: v1.23.2
   ```

#### Migrate Workloads

1. Update injection labels on the data plane namespace.

    Here we're adding two labels to the namespace:

    1. The `istio.io/rev: ossm-3` label which ensures that any new pods that get created in that namespace will connect to the 3.0 proxy.
    2. The `maistra.io/ignore-namespace: "true"` label which will disable sidecar injection for 2.6 proxies in the namespace. This ensures that 2.6 will stop injecting proxies in this namespace and any new proxies will be injected by the 3.0 control plane. Without this, the 2.6 injection webhook will try to inject the pod and it will connect to the 2.6 proxy as well as refuse to start since it will have the 2.6 cni annotation.

    > [!NOTE]
    > that once you apply the `maistra.io/ignore-namespace` label, any new pod that gets created in the namespace will be connected to the 3.0 proxy. Workloads will still be able to communicate with each other though regardless of which control plane they are connected to.

    ```sh
    oc label ns bookinfo istio.io/rev=ossm-3 maistra.io/ignore-namespace="true" --overwrite=true
    ```
    > [!IMPORTANT]
    > In case you are using `istio-injection=enabled` label on the namespace being migrated, remove that label via `oc label ns bookinfo istio-injection-` or add a default revision tag. TODO: add more when the revision tag doc is ready

1. Migrate workloads

    You can now restart the workloads so that the new pod will be injected with the 3.0 proxy.

    This can be done all at once:

    ```sh
    oc rollout restart deployments -n bookinfo
    ```
    or individually:
    ```sh
    oc rollout restart deployments productpage-v1 -n bookinfo
    ```

1. Wait for the productpage app to restart.

    ```sh
    oc rollout status deployment productpage-v1 -n bookinfo
    ```

#### Validate Workload Migration

#### Clean 2.6 control plane