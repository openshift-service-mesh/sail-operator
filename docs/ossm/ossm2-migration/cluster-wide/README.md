# OpenShift Service Mesh 2.6 Cluster wide --> 3 Migration guide
This guide is for users who are currently running `ClusterWide` OpenShift Service Mesh 2.6 migrating to OpenShift Service Mesh 3.0. You should first read [this document comparing OpenShift Service Mesh 2 vs. OpenShift Service Mesh 3](../../ossm2-vs-ossm3.md) to familiarize yourself with the concepts between the two versions and the differences in managing the workloads and addons.

## Migrating OpenShift Service Mesh 2.6 Cluster wide to OpenShift Service Mesh 3

### Prerequisites
- you have read [OpenShift Service Mesh 2 vs. OpenShift Service Mesh 3](../../ossm2-vs-ossm3.md)
- you have completed all the steps from the [pre-migration checklist](../README.md#pre-migration-checklist)
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

  This can be achieved simply by installing the 3.0 control plane to the same namespace as 2.6 control plane. The migration procedure below shows how to verify the root cert is shared.
- 3.0 control plane must have access to all namespaces in 2.6 mesh

  During the migration, some proxies will be controlled by the 3.0 control plane while others will still be controlled by the 2.6 control plane. To assure the communication still works, both control planes must be aware of the same set of services. You must verify that:
  1. there are no Network Policies blocking the traffic

     OpenShift Service Mesh 3.0 is no longer managing Network Policies and it's up to users to assure that existing Network Policies are not blocking the traffic for OpenShift Service Mesh 3.0 components. See Migration of Network Policies documentation for details.
     <!--TODO: add a link when the doc is ready https://issues.redhat.com/browse/OSSM-8520-->
  1. Ensure that the `discoverySelectors` defined in your OpenShift Service Mesh 3.0 `Istio` resource will match the namespaces that make up your OpenShift Service Mesh 2.6 mesh. You may need to add additional labels onto your OpenShift Service Mesh 2.6 application namespaces to ensure that they are captured by your OpenShift Service Mesh 3.0 `Istio` `discoverySelectors`. See [Scoping the service mesh with DiscoverySelectors](../../create-mesh/README.md)
- only one control plane will try to inject a side car

  This can be achieved by correct use of injection labels. Please see [Installing the Sidecar](../../injection/README.md) for details.
  > **_NOTE:_** Due to specific behavior of OpenShift Service Mesh 2.6 it's necessary to disable 2.6 injector when migrating the data plane namespace. We will use `maistra.io/ignore-namespace: "true"` label in this guide.

Apart from the conditions above, it's necessary to decide which injection labels will be used. See [Installing the Sidecar](../../injection/README.md) explaining relation between Istio revisions and injection labels.

Once it's clear how the `istio.io/rev` and `istio-injection` labels work in OpenShift Service Mesh 3, it's also necessary to revisit your OpenShift Service Mesh 2.6 installation and understand consequences of different injection configurations. Typically, following configurations might be used:

- by default the `spec.memberSelectors` in your `ServiceMeshMemberRoll` is configured to match `istio-injection=enabled` label and all of your 2.6 data plane namespaces are already labeled with `istio-injection=enabled`

    With this configuration, you will be able to keep the `istio-injection=enabled` on your data plane namespaces during the migration.
- `spec.memberSelectors` in your `ServiceMeshMemberRoll` is not configured to match `istio-injection=enabled` and your 2.6 data plane namespaces are using some other label

    With this configuration, it will be necessary to add either the `istio.io/rev` or `istio-injection` label during the migration. The label defined in the `spec.memberSelectors` in your `ServiceMeshMemberRoll` will have no effect on injection in OpenShift Service Mesh 3
- [adding projects using label selectors](https://docs.openshift.com/container-platform/4.16/service_mesh/v2x/ossm-create-mesh.html#ossm-about-adding-projects-using-label-selectors_ossm-create-mesh) feature is not used at all and all projects were added to the mesh manually by creating `ServiceMeshMember`

    With this configuration, it will be necessary to add either the `istio.io/rev` or `istio-injection` label during the migration.

Based on your 2.6 configuration listed above, you should follow one of the procedures bellow.

#### Migration of 2.6 installation with istio-injection=enabled label
In this procedure it's expected that all 2.6 data plane namespaces have `istio-injection=enabled` label.
> [!CAUTION]
> This procedure may cause a traffic disruption for workloads which are restarted at unexpected time.

##### Create your Istio resource
1. Find a namespace with 2.6 control plane:

    ```sh
    oc get smcp -A
    NAMESPACE      NAME                   READY   STATUS            PROFILES      VERSION   AGE
    istio-system   install-istio-system   6/6     ComponentsReady   ["default"]   2.6.4     115m
    ```
1. Prepare the `Istio` resource yaml named `ossm-3.yaml` to be deployed to the same namespace as the 2.6 control plane:

    Here we are not using any `discoverySelectors` so the control plane will have access to all namespaces. In case you want to define `discoverySelectors`, keep in mind that all data plane namespaces you are planning to migrate from 2.6 must be matched.
    
    Also note that `default` name with `InPlace` update strategy is used which allows usage of the `istio-injection=enabled` label. In case you want to use different name or `RevisionBased` update strategy, you will have to configure `default` `IstioRevisionTag`.

    ```yaml
   apiVersion: sailoperator.io/v1alpha1
   kind: Istio
   metadata:
     name: default # the name and the updateStrategy is significant for injection labels
   spec:
     updateStrategy:
       type: InPlace # the name and the updateStrategy is significant for injection labels
     namespace: istio-system # the same namespace where we run the 2.6 control plane
     version: v1.24.1
   ```
1. Apply the `Istio` resource yaml:

    > **_NOTE:_** after next step, both 2.6 and 3.0 control planes will try to inject side cars to all pods in namespaces with the `istio-injection=enabled` label and all pods with the `sidecar.istio.io/inject="true"` label after next restart of the workloads. Workloads should be restarted only after the `maistra.io/ignore-namespace: "true"` label is added (see below).
    ```sh
    oc apply -f ossm-3.yaml
    ```
1. Verify that new `istiod` is using existing root certificate:

    ```sh
    oc logs deployments/istiod -n istio-system | grep 'Load signing key and cert from existing secret'
    2024-12-18T08:13:53.788959Z	info	pkica	Load signing key and cert from existing secret istio-system/istio-ca-secret
    ```
##### Migrate Workloads
1. Add `maistra.io/ignore-namespace: "true"` label to the data plane namespace

    The `maistra.io/ignore-namespace: "true"` label will disable sidecar injection for 2.6 proxies in the namespace. This ensures that 2.6 will stop injecting proxies in this namespace and any new proxies will be injected by the 3.0 control plane. Without this, there will be a conflict and the proxy will not start.

      > **_NOTE:_** that once you apply the `maistra.io/ignore-namespace` label, any new pod that gets created in the namespace will be connected to the 3.0 proxy. Workloads will still be able to communicate with each other though regardless of which control plane they are connected to.

    ```sh
    oc label ns bookinfo maistra.io/ignore-namespace="true"
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

##### Validate Workload Migration
1.  Ensure that expected workloads are managed by the new control plane via `istioctl ps -n bookinfo`

    In case you have restarted just `productpage-v1`, you will see that only `productpage` proxy is upgraded and connected to the new control plane:
    ```sh
    $ istioctl ps -n bookinfo
    NAME                                          CLUSTER        CDS             LDS             EDS             RDS             ECDS         ISTIOD                                           VERSION
    details-v1-7f46897b-d497c.bookinfo            Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    productpage-v1-74bfbd4d65-vsxqm.bookinfo      Kubernetes     SYNCED (4s)     SYNCED (4s)     SYNCED (3s)     SYNCED (4s)     IGNORED      istiod-797bb4d78f-xpchx                          1.24.1
    ratings-v1-559b64556-c5ppg.bookinfo           Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    reviews-v1-847fb7c54d-qxt5d.bookinfo          Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    reviews-v2-5c7ff5b77b-8jbhd.bookinfo          Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    reviews-v3-5c5d764c9b-rrx8w.bookinfo          Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    ```
    In case you restarted all deployments, all proxies will be upgraded:
    ```sh
    $ istioctl ps -n bookinfo
    NAME                                           CLUSTER        CDS              LDS              EDS             RDS              ECDS        ISTIOD                             VERSION
    details-v1-7b5c68d756-9v9g4.bookinfo           Kubernetes     SYNCED (13s)     SYNCED (13s)     SYNCED (4s)     SYNCED (13s)     IGNORED     istiod-797bb4d78f-xpchx            1.24.1
    productpage-v1-db9bfdbd4-z5c2l.bookinfo        Kubernetes     SYNCED (9s)      SYNCED (9s)      SYNCED (4s)     SYNCED (9s)      IGNORED     istiod-797bb4d78f-xpchx            1.24.1
    ratings-v1-7684d8d8b8-xzrc6.bookinfo           Kubernetes     SYNCED (12s)     SYNCED (12s)     SYNCED (4s)     SYNCED (12s)     IGNORED     istiod-797bb4d78f-xpchx            1.24.1
    reviews-v1-fb4d48bd8-lzvtx.bookinfo            Kubernetes     SYNCED (12s)     SYNCED (12s)     SYNCED (4s)     SYNCED (12s)     IGNORED     istiod-797bb4d78f-xpchx            1.24.1
    reviews-v2-58bcc78ff6-fcrb8.bookinfo           Kubernetes     SYNCED (11s)     SYNCED (11s)     SYNCED (4s)     SYNCED (11s)     IGNORED     istiod-797bb4d78f-xpchx            1.24.1
    reviews-v3-5d56c9c79b-l6gms.bookinfo           Kubernetes     SYNCED (11s)     SYNCED (11s)     SYNCED (4s)     SYNCED (11s)     IGNORED     istiod-797bb4d78f-xpchx            1.24.1
    ```
You can now proceed with the migration of next namespace.

  > **_NOTE:_** Even with different versions of the proxies, the communication between services should work normally.

#### Migration of 2.6 installation without istio-injection=enabled label
In this procedure, we will use a proper canary upgrade with gradual migration of data plane namespaces.

##### Create your Istio resource
1. Find a namespace with 2.6 control plane:

    ```sh
    oc get smcp -A
    NAMESPACE      NAME                   READY   STATUS            PROFILES      VERSION   AGE
    istio-system   install-istio-system   6/6     ComponentsReady   ["default"]   2.6.4     115m
    ```
1. Prepare the `Istio` resource yaml named `ossm-3.yaml` to be deployed to the same namespace as the 2.6 control plane:

    Here we are not using any `discoverySelectors` so the control plane will have access to all namespaces. In case you want to define `discoverySelectors`, keep in mind that all data plane namespaces you are planning to migrate from 2.6 must be matched.

    ```yaml
   apiVersion: sailoperator.io/v1alpha1
   kind: Istio
   metadata:
     name: ossm-3 # the name, updateStrategy and version are significant for injection labels
   spec:
     updateStrategy:
       type: RevisionBased # the name and the updateStrategy is significant for injection labels
     namespace: istio-system # the name, updateStrategy and version are significant for injection labels
     version: v1.24.1 # the name, updateStrategy and version are significant for injection labels
   ```
1. Apply the `Istio` resource yaml:

    ```sh
    oc apply -f ossm-3.yaml
    ```
1. Verify that new `istiod` is using existing root certificate:

    ```sh
    oc logs deployments/istiod-ossm-3-v1-24-1 -n istio-system | grep 'Load signing key and cert from existing secret'
    2024-12-18T08:13:53.788959Z	info	pkica	Load signing key and cert from existing secret istio-system/istio-ca-secret
    ```
> **_NOTE:_** Unlike in the procedure with `istio-injection=enabled` label, at this point it is still safe to restart 2.6 workloads without any injection conflicts.

##### Migrate Workloads
This guide is not using revision tags but it's recommended to use them for big meshes to avoid re-labeling of namespaces during future 3.y.z upgrades.

1. Update injection labels on the data plane namespace

    Here we're adding two and removing one label:

    1. The `istio.io/rev=ossm-3-v1-24-1` label which ensures that any new pods that get created in that namespace will connect to the 3.0 proxy. In our example, the 3.0 revision is named `ossm-3-v1-24-1`
    1. The `maistra.io/ignore-namespace: "true"` label which will disable sidecar injection for 2.6 proxies in the namespace. This ensures that 2.6 will stop injecting proxies in this namespace and any new proxies will be injected by the 3.0 control plane.
    1. Even though this procedure expects that the `istio-injection` is not used in any of the migrated namespaces, we are removing it here as precaution because the `istio-injection=enabled` label would prevent proxy injection.

    ```sh
    oc label ns bookinfo istio.io/rev=ossm-3-v1-24-1 maistra.io/ignore-namespace="true" istio-injection- --overwrite=true
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

##### Validate Workload Migration
1.  Ensure that expected workloads are managed by the new control plane via `istioctl ps -n bookinfo`

    In case you have restarted just `productpage-v1`, you will see that only `productpage` proxy is upgraded and connected to the new control plane:
    ```sh
    $ istioctl ps -n bookinfo
    NAME                                          CLUSTER        CDS             LDS             EDS             RDS             ECDS         ISTIOD                                           VERSION
    details-v1-7f46897b-d497c.bookinfo            Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    productpage-v1-74bfbd4d65-vsxqm.bookinfo      Kubernetes     SYNCED (4s)     SYNCED (4s)     SYNCED (3s)     SYNCED (4s)     IGNORED      istiod-ossm-3-v1-24-1-797bb4d78f-xpchx           1.24.1
    ratings-v1-559b64556-c5ppg.bookinfo           Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    reviews-v1-847fb7c54d-qxt5d.bookinfo          Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    reviews-v2-5c7ff5b77b-8jbhd.bookinfo          Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    reviews-v3-5c5d764c9b-rrx8w.bookinfo          Kubernetes     SYNCED          SYNCED          SYNCED          SYNCED          NOT SENT     istiod-install-istio-system-866b57d668-6lpcr     1.20.8
    ```
  > **_NOTE:_** Even with different versions of the proxies, the communication between services should work normally.

Now you can proceed with the migration of next namespaces.

#### Remove 2.6 control plane
Once you are done with the migration of all workloads in your mesh, you can remove your 2.6 control plane.

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