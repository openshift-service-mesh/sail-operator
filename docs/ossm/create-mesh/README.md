# Adding services to a service mesh
This document describes basic concepts how Istio control plane monitors your OCP resources and how to scope your service mesh.

Adding your workload to a mesh is a two steps process:
1. Your workload must be discovered by Istio control plane
1. Your workload must be injected with Istio Proxy sidecar

First step is described in this guide, see [Installing the Sidecar
](../injection/README.md) for second step.

## Concepts
To better understand next chaptes, it's recommended to read Service mesh architecture guide first.

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

This chapter is focusing on `DiscoverySelectors`, see [Single-mesh isolation ("zone") features](../zones/README.md) for details about `Sidecar` and `exportTo`.

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

#### Discovery Selectors in Action
Assuming you know which namespaces to include as part of the service mesh, as a mesh administrator, you can configure `discoverySelectors` at installation time or post-installation by adding your desired discovery selectors to Istio’s MeshConfig resource. For example, you can configure Istio to discover only the namespaces that have the label `istio-discovery=enabled`.

##### Prerequisites
- OpenShift Service Mesh 3 operator is installed
- Istio CNI resource is created

1. Create the `istio-system` system namespace and create the Istio CR with `discoverySelectors` configured:
    ```bash
    oc create ns istio-system
    oc label ns istio-system istio-discovery=enabled
    oc apply -f - <<EOF
    kind: Istio
    apiVersion: sailoperator.io/v1alpha1
    metadata:
      name: default
    spec:
      namespace: istio-system
      values:
        meshConfig:
          discoverySelectors:
            - matchLabels:
                istio-discovery: enabled
      updateStrategy:
        type: InPlace
      version: v1.23.0
    EOF
    ```
1. Create two application namespaces:
    ```bash
    oc create ns app-ns-1
    oc create ns app-ns-2
    ```
1. Label first application namespace to be matched by defined `discoverySelectors` and enable sidecar injection:
    ```bash
    oc label ns app-ns-1 istio-discovery=enabled istio-injection=enabled
    ```
1. Deploy the sleep application to both namespaces:
    ```bash
    oc apply -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/sleep/sleep.yaml -n app-ns-1
    oc apply -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/sleep/sleep.yaml -n app-ns-2
    ```
1. Verify that you don't see any endpoints from the second namespace:
    ```bash
    istioctl pc endpoint deploy/sleep -n app-ns-1
    ENDPOINT                                                STATUS      OUTLIER CHECK     CLUSTER
    10.128.2.197:15010                                      HEALTHY     OK                outbound|15010||istiod.istio-system.svc.cluster.local
    10.128.2.197:15012                                      HEALTHY     OK                outbound|15012||istiod.istio-system.svc.cluster.local
    10.128.2.197:15014                                      HEALTHY     OK                outbound|15014||istiod.istio-system.svc.cluster.local
    10.128.2.197:15017                                      HEALTHY     OK                outbound|443||istiod.istio-system.svc.cluster.local
    10.131.0.32:80                                          HEALTHY     OK                outbound|80||sleep.app-ns-1.svc.cluster.local
    127.0.0.1:15000                                         HEALTHY     OK                prometheus_stats
    127.0.0.1:15020                                         HEALTHY     OK                agent
    unix://./etc/istio/proxy/XDS                            HEALTHY     OK                xds-grpc
    unix://./var/run/secrets/workload-spiffe-uds/socket     HEALTHY     OK                sds-grpc
    ```
1. Verify that after labeling second namespace it also appears on the list of discovered endpoints:
    ```bash
    oc label ns app-ns-2 istio-discovery=enabled
    istioctl pc endpoint deploy/sleep -n app-ns-1
    ENDPOINT                                                STATUS      OUTLIER CHECK     CLUSTER
    10.128.2.197:15010                                      HEALTHY     OK                outbound|15010||istiod.istio-system.svc.cluster.local
    10.128.2.197:15012                                      HEALTHY     OK                outbound|15012||istiod.istio-system.svc.cluster.local
    10.128.2.197:15014                                      HEALTHY     OK                outbound|15014||istiod.istio-system.svc.cluster.local
    10.128.2.197:15017                                      HEALTHY     OK                outbound|443||istiod.istio-system.svc.cluster.local
    10.131.0.32:80                                          HEALTHY     OK                outbound|80||sleep.app-ns-1.svc.cluster.local
    10.131.0.33:80                                          HEALTHY     OK                outbound|80||sleep.app-ns-2.svc.cluster.local
    127.0.0.1:15000                                         HEALTHY     OK                prometheus_stats
    127.0.0.1:15020                                         HEALTHY     OK                agent
    unix://./etc/istio/proxy/XDS                            HEALTHY     OK                xds-grpc
    unix://./var/run/secrets/workload-spiffe-uds/socket     HEALTHY     OK                sds-grpc
    ```

See [Multiple Istio Control Planes in a Single Cluster](../multi-control-planes/README.md) for another example of `discoverySelectors` usage.