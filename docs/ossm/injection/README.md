## Installing the Sidecar
### Injection
In order to take advantage of all of Istio’s features, pods in the mesh must be running an Istio sidecar proxy.

The following sections describe automatic Istio sidecar injection in the pod’s namespace. Manual injection using the `istioctl` command is not supported by OpenShift Service Mesh.

When enabled in a pod’s namespace, automatic injection injects the proxy configuration at pod creation time using an admission controller.

### Automatic sidecar injection
Sidecars can be automatically added to applicable OpenShift pods using a [mutating webhook admission controller](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/) provided by Istio.

When you set the `istio-injection=enabled` or `istio.io/rev=<revision>` (depending if you use control plane revisions) label on a namespace, any new pods that are created in that namespace will automatically have a sidecar added to them.

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