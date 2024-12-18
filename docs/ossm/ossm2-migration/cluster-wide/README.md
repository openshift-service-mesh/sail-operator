# OpenShift Service Mesh 2.6 Cluster wide --> 3 Migration guide
This guide is for users who are currently running `ClusterWide` OpenShift Service Mesh 2.6 migrating to OpenShift Service Mesh 3.0. You should first read [this document comparing OpenShift Service Mesh 2 vs. OpenShift Service Mesh 3](../../ossm2-vs-ossm3.md) to familiarize yourself with the concepts between the two versions and the differences in managing the workloads and addons.

## Migrating OpenShift Service Mesh 2.6 Cluster wide to OpenShift Service Mesh 3

### Prerequisites
- you have completed all the steps from the [pre-migration checklist](../README.md#pre-migration-checklist)
- you have read [OpenShift Service Mesh 2 vs. OpenShift Service Mesh 3](../../ossm2-vs-ossm3.md)
- you have verified that your 2.6 `ServiceMeshControlPlane` is using `ClusterWide` mode
- Red Hat OpenShift Service Mesh 3 operator is installed
- `IstioCNI` is installed
- `istioctl` is installed

### Procedure
In this example, we'll be using the [bookinfo demo](https://raw.githubusercontent.com/Maistra/istio/maistra-2.6/samples/bookinfo/platform/kube/bookinfo.yaml) but you can follow these same steps with your own workloads.

#### Plan the migration
There will be two cluster wide Istio control planes running during the migration process so it's necessary to plan the migration steps in advance in order to avoid possible conflicts between the two control planes.

There are a few conditions which must be verified to ensure a successful migration:
- both control planes must share the same root certificate

  This can be achieved simply by installing the 3.0 control plane to the same namespace as 2.6 control plane
- 3.0 control plane must have access to all namespaces in 2.6 mesh

  During the migration, some proxies will be controlled by the 3.0 control plane while others will still be controlled by the 2.6 control plane. To assure the communication still works, both control planes must be aware of the same set of services. You must verify that:
  1. there are no Network Policies blocking the traffic
  1. Ensure that the `discoverySelectors` defined in your OpenShift Service Mesh 3.0 `Istio` resource will match the namespaces that make up your OpenShift Service Mesh 2.6 mesh. You may need to add additional labels onto your OpenShift Service Mesh 2.6 application namespaces to ensure that they are captured by your OpenShift Service Mesh 3.0 `Istio` `discoverySelectors`. See [Scoping the service mesh with DiscoverySelectors](../../create-mesh/README.md)
- only one control plane will try to inject a side car

  This can be achieved by correct use of injection labels. Please see [Installing the Sidecar](../../injection/README.md) for details.
  > **_NOTE:_** To disable 2.6 injector, we will use `maistra.io/ignore-namespace: "true"` label in this guide.

Apart from the conditions above, it's recommended to decide which injection labels will be used. See [Installing the Sidecar](../../injection/README.md) explaining relation between Istio revisions and injection labels. Based on the decision, use correct injection labels later in this guide.

#### Create your Istio resource
See Installation documentation for details.
While creating the `Istio` resource, make sure all conditions from previous chapter are fulfilled.

> [!CAUTION]
> In case you have decided to use a `default` name for 3.0 `Istio` resource with `InPlace` upgrade strategy or you have configured the `default` revision tag, 3.0 control plane will try to inject side cars to all pods in namespaces with `istio-injection=enabled` label and all pods with `sidecar.istio.io/inject="true"` label after next restart of the workloads.

#### Migrate Workloads
This guide will use `istio.io/rev=ossm-3` label. In case you are using the `default` revision or you have configured the `default` revision tag, it's possible to use `istio-injection=enabled` label. Following steps will work with that label (instead of `istio.io/rev=ossm-3`) too.

1. Update injection labels on the data plane namespace

    Here we're adding two labels to the namespace:

    1. The `istio.io/rev=ossm-3` label which ensures that any new pods that get created in that namespace will connect to the 3.0 proxy. In our example, the 3.0 revision is named `ossm-3`
    2. The `maistra.io/ignore-namespace: "true"` label which will disable sidecar injection for 2.6 proxies in the namespace. This ensures that 2.6 will stop injecting proxies in this namespace and any new proxies will be injected by the 3.0 control plane. Without this, the 2.6 injection webhook will try to inject the pod and it will connect to the 2.6 proxy as well as refuse to start since it will have the 2.6 cni annotation.

        > **_NOTE:_** that once you apply the `maistra.io/ignore-namespace` label, any new pod that gets created in the namespace will be connected to the 3.0 proxy. Workloads will still be able to communicate with each other though regardless of which control plane they are connected to.

    ```sh
    oc label ns bookinfo istio.io/rev=ossm-3 maistra.io/ignore-namespace="true" --overwrite=true
    ```

1. Migrate workloads

    You can now restart the workloads so that the new pods will be injected with the 3.0 proxy.

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
1.  Ensure that expected workloads are managed by the new control plane via `istioctl ps -n bookinfo`

    In case you have restarted just `productpage-v1`, you will see that only `productpage` proxy is upgraded and connected to the new control plane:
    ```sh
    $ istioctl ps -n bookinfo
    NAME                                          CLUSTER        CDS             LDS             EDS             RDS             ECDS         ISTIOD                                           VERSION
    details-v1-7f46897b-d497c.bookinfo            Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    productpage-v1-74bfbd4d65-vsxqm.bookinfo      Kubernetes     SYNCED (4s)     SYNCED (4s)     SYNCED (3s)     SYNCED (4s)     IGNORED      istiod-ossm-3-797bb4d78f-xpchx                   1.23.2
    ratings-v1-559b64556-c5ppg.bookinfo           Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    reviews-v1-847fb7c54d-qxt5d.bookinfo          Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    reviews-v2-5c7ff5b77b-8jbhd.bookinfo          Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    reviews-v3-5c5d764c9b-rrx8w.bookinfo          Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    ```
    In case you restarted all deployments, all proxies will be upgraded:
    ```sh
    $ istioctl ps -n bookinfo
    NAME                                           CLUSTER        CDS              LDS              EDS             RDS              ECDS        ISTIOD                             VERSION
    details-v1-7b5c68d756-9v9g4.bookinfo           Kubernetes     SYNCED (13s)     SYNCED (13s)     SYNCED (4s)     SYNCED (13s)     IGNORED     istiod-ossm-3-797bb4d78f-xpchx     1.23.2
    productpage-v1-db9bfdbd4-z5c2l.bookinfo        Kubernetes     SYNCED (9s)      SYNCED (9s)      SYNCED (4s)     SYNCED (9s)      IGNORED     istiod-ossm-3-797bb4d78f-xpchx     1.23.2
    ratings-v1-7684d8d8b8-xzrc6.bookinfo           Kubernetes     SYNCED (12s)     SYNCED (12s)     SYNCED (4s)     SYNCED (12s)     IGNORED     istiod-ossm-3-797bb4d78f-xpchx     1.23.2
    reviews-v1-fb4d48bd8-lzvtx.bookinfo            Kubernetes     SYNCED (12s)     SYNCED (12s)     SYNCED (4s)     SYNCED (12s)     IGNORED     istiod-ossm-3-797bb4d78f-xpchx     1.23.2
    reviews-v2-58bcc78ff6-fcrb8.bookinfo           Kubernetes     SYNCED (11s)     SYNCED (11s)     SYNCED (4s)     SYNCED (11s)     IGNORED     istiod-ossm-3-797bb4d78f-xpchx     1.23.2
    reviews-v3-5d56c9c79b-l6gms.bookinfo           Kubernetes     SYNCED (11s)     SYNCED (11s)     SYNCED (4s)     SYNCED (11s)     IGNORED     istiod-ossm-3-797bb4d78f-xpchx     1.23.2
    ```
  > **_NOTE:_** Even with different versions of the proxies, the communication between services should work normally.

#### Clean 2.6 control plane
Once you are done with the migration of all workloads in your mesh, you can clean 2.6 control plane.

> [!CAUTION]
> Following steps will remove also all NetworkPolicies created by 2.6 control plane. Please make sure you are done with the [pre-migration checklist](../README.md#pre-migration-checklist)

1. Remove your `ServiceMeshControlPlane`
    ```sh
    $ oc delete smcp basic -n istio-system
    ```
1. Remove your `ServiceMeshMemberRoll`
    ```sh
    $ oc delete smmr default -n istio-system
    ```
1. Remove your `ServiceMeshMembers`
    ```sh
    $ oc delete smm default -n bookinfo
    ```
1. Verify that all `ServiceMeshMembers` and `ServiceMeshMemberRolls` were removed:
    ```sh
    $ oc get smm,smmr -A
    No resources found
    ```

> **_NOTE:_** that depending on how you created `ServiceMeshMembers` and `ServiceMeshMemberRoll`, those resources might be removed automatically with removal of `ServiceMeshControlPlane` after step 1.

#### Clean 2.6 operator and CRDs
TODO