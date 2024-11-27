## Installing the Sidecar
### Injection
To include workloads as part of the service mesh and to begin using Istio's many features, pods must be injected with a sidecar proxy that will be configured by an Istio control plane.

Sidecar injection can be enabled via labels at the namespace or pod level. That also serves to identify the control plane managing the sidecar proxy(ies). By adding a valid injection label on a `Deployment`, pods created through that deployment will automatically have a sidecar added to them. By adding a valid pod injection label on a namespace, any new pods that are created in that namespace will automatically have a sidecar added to them.

The proxy configuration is injected at pod creation time using an [admission controller](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/). As sidecar injection occurs at the pod-level, you won’t see any change to `Deployment` resources. Instead, you’ll want to check individual pods (via `oc describe`) to see the injected Istio proxy container.

### Identifying the revision name

The correct label used to enable sidecar injection depends on the control plane instance being used. A control plane instance is called a "revision" and is managed by the `IstioRevision` resource. The `Istio` control plane resource creates and manages `IstioRevision` resources, thus users do not typically have to create or modify them. 

When the `Istio` resources's `spec.updateStrategy.type` is set to `InPlace`, the `IstioRevision` will have the same name as the `Istio` resource. When the `Istio` resources's `spec.updateStrategy.type` is set to `RevisionBased`, the `IstioRevision` will have the format `<Istio resource name>-v<version>`.

In most cases, there will be a single `IstioRevision` resource per `Istio` resource. During a revision based upgrade, there may be multiple `IstioRevision` instances present, each representing an independent control plane. 

The available revision names can be checked with the command:

```console
$ oc get istiorevision
NAME              READY   STATUS    IN USE   VERSION   AGE
my-mesh-v1-23-0   True    Healthy   False    v1.23.0   114s
```

### Enabling sidecar injection - "default" revision

When the service mesh's `IstioRevision` name is "default", it's possible to use following labels on a namespace or a pod to enable sidecar injection:
| Resource | Label | Enabled value | Disabled value |
| --- | --- | --- | --- |
| Namespace | `istio-injection` | `enabled` | `disabled` |
| Pod | `sidecar.istio.io/inject` | `"true"` | `"false"` |

### Enabling sidecar injection - other revisions

When the `IstioRevision` name is not "default", then the specific `IstioRevision` name must be used with the `istio.io/rev` label to map the pod to the desired control plane while enabling sidecar injection. 

For example, with the revision shown above, the following labels would enable sidecar injection:
| Resource | Enabled Label | Disabled Label |
| --- | --- | --- |
| Namespace | `istio.io/rev=my-mesh-v1-23-0` | `istio-injection=disabled` |
| Pod | `istio.io/rev=my-mesh-v1-23-0` | `sidecar.istio.io/inject="false"` |

### Sidecar injection logic

If the `istio-injection` label and the `istio.io/rev` label are both present on the same namespace, the `istio-injection` label (mapping to the "default" revision) will take precedence.

The injector is configured with the following logic:

1. If either label (`istio-injection` or `sidecar.istio.io/inject`) is disabled, the pod is not injected.
2. If either label (`istio-injection` or `sidecar.istio.io/inject` or `istio.io/rev`) is enabled, the pod is injected.

### Example: Enabling sidecar injection with namespace labels
Prerequisites:
- You have installed the Red Hat OpenShift Service Mesh Operator, created the Istio resource, and the Operator has deployed Istio.
- You have created IstioCNI resource, and the Operator has deployed the necessary IstioCNI pods.
- You have created the namespaces or workloads to be part of the mesh, and they are [discoverable by the Istio control plane](https://docs.openshift.com/service-mesh/3.0.0tp1/install/ossm-installing-openshift-service-mesh.html#ossm-scoping-service-mesh-with-discoveryselectors_ossm-creating-istiocni-resource). In this example, the [bookinfo application](https://docs.openshift.com/service-mesh/3.0.0tp1/install/ossm-installing-openshift-service-mesh.html#deploying-book-info_ossm-about-bookinfo-application).

1. Verify the revision name of the Istio control plane:

    ```console
    $ oc get istiorevision 
    NAME      TYPE    READY   STATUS    IN USE   VERSION   AGE
    default   Local   True    Healthy   False    v1.23.0   4m57s
    ```
    Since the revision name is `default`, we can used the default injection labels above and do not need to reference the specific revision. 

2. Apply the injection label to the bookinfo namespace by entering the following command at the CLI:
    ```bash
    $ oc label namespace bookinfo istio-injection=enabled
    namespace/bookinfo labeled
    ```

3. Workloads that were already running when the injection label was added will need to be restarted for sidecar injection to occur:
    ```bash
    oc rollout restart deployment details-v1
    oc rollout restart deployment productpage-v1
    oc rollout restart deployment ratings-v1
    oc rollout restart deployment reviews-v1
    oc rollout restart deployment reviews-v2
    oc rollout restart deployment reviews-v3
    ```

4. Verify that the deployed pods now show "2/2" containers "READY", indicating that the sidecars have been successfully injected:

    ```bash
    $ oc get pods -n bookinfo
    NAME                              READY   STATUS    RESTARTS   AGE
    details-v1-7548fcd748-dfqlt       2/2     Running   0          3m26s
    productpage-v1-76885c4d7f-c8jhg   2/2     Running   0          3m18s
    ratings-v1-6b87d45487-mfx9q       2/2     Running   0          3m12s
    reviews-v1-5745d75947-69697       2/2     Running   0          3m6s
    reviews-v2-7d48d755f9-stdjx       2/2     Running   0          3m4s
    reviews-v3-df57bf666-vw7vb        2/2     Running   0          3m2s
    ```

### Example: Enabling sidecar injection with pod labels
Prerequisites:
- You have installed the Red Hat OpenShift Service Mesh Operator, created the Istio resource, and the Operator has deployed Istio.
- You have created IstioCNI resource, and the Operator has deployed the necessary IstioCNI pods.
- You have created the namespaces or workloads to be part of the mesh, and they are [discoverable by the Istio control plane](https://docs.openshift.com/service-mesh/3.0.0tp1/install/ossm-installing-openshift-service-mesh.html#ossm-scoping-service-mesh-with-discoveryselectors_ossm-creating-istiocni-resource). In this example, the [bookinfo application](https://docs.openshift.com/service-mesh/3.0.0tp1/install/ossm-installing-openshift-service-mesh.html#deploying-book-info_ossm-about-bookinfo-application).

1. Verify the revision name of the Istio control plane:

    ```console
    $ oc get istiorevision
    NAME      TYPE    READY   STATUS    IN USE   VERSION   AGE
    my-mesh   Local   True    Healthy   False    v1.23.0   47s
    ```
    Since the revision name is `my-mesh`, we must use the a revision label to enable sidecar injection. In this case, `istio.io/rev=my-mesh`.

2. To find your deployments use the oc get command. For example, to view the Deployment YAML file for the 'ratings-v1' microservice in the bookinfo namespace, use the following command to see the resource in YAML format.

    ```bash
    oc get deployment -n bookinfo ratings-v1 -o yaml
    ```

3. Open the application’s Deployment YAML file in an editor.

4. Update the `spec.template.metadata.labels` section of your Deployment YAML file to include the appropriate pod injection or revision label. In this case, `istio.io/rev=my-mesh`:

    ```yaml
    kind: Deployment
    apiVersion: apps/v1
    metadata:
    name: ratings-v1
    namespace: bookinfo
    labels:
      app: ratings
      version: v1
    spec:
      template:
        metadata:
          labels:
            istio.io/rev: my-mesh
    ```
5. If the Deployment was already running, it will need to be restarted for sidecar injection to occur:
    ```bash
    oc rollout restart deployment ratings-v1
    ```
6. Verify that only the `ratings-v1` pod now shows "2/2" containers "READY", indicating that the sidecar has been successfully injected:
    ```
    oc get pods
    NAME                              READY   STATUS    RESTARTS   AGE
    details-v1-559cd49f6c-b89hw       1/1     Running   0          42m
    productpage-v1-5f48cdcb85-8ppz5   1/1     Running   0          42m
    ratings-v1-848bf79888-krdch       2/2     Running   0          9s
    reviews-v1-6b7444ffbd-7m5wp       1/1     Running   0          42m
    reviews-v2-67876d7b7-9nmw5        1/1     Running   0          42m
    reviews-v3-84b55b667c-x5t8s       1/1     Running   0          42m
    ```

7. Repeat for other workloads that you wish to include in the mesh.