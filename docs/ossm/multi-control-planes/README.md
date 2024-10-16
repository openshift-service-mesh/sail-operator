# Multiple Istio Control Planes in a Single Cluster
By default, the control plane will read all configuration in all namespaces - see [Cluster wide by default](../create-mesh/README.md#cluster-wide-by-default) for details. To achieve [soft multi-tenancy](../create-mesh/README.md#soft-multi-tenancy), Istio provides [discoverySelectors](../create-mesh/README.md#discoveryselectors)  which together with revisions capability allows to install multiple control planes in a Single Cluster.

## Prerequisites
- OpenShift Service Mesh 3 operator is installed
- Istio CNI resource is created

## Deploying multiple control planes
The cluster will host two control planes installed in two different system namespaces. The mesh application workloads will run in multiple application-specific namespaces, each namespace associated with one or the other control plane based on revision and discovery selector configurations.

1. Create the first system namespace, `usergroup-1`, and create the Istio CR:
    ```bash
    oc create ns usergroup-1
    oc label ns usergroup-1 usergroup=usergroup-1
    oc apply -f - <<EOF
    kind: Istio
    apiVersion: sailoperator.io/v1alpha1
    metadata:
      name: usergroup-1
    spec:
      namespace: usergroup-1
      values:
        meshConfig:
          discoverySelectors:
            - matchLabels:
                usergroup: usergroup-1
      updateStrategy:
        type: InPlace
      version: v1.23.0
    EOF
    ```
1. Create the second system namespace, `usergroup-2`, and create the Istio CR:
    ```bash
    oc create ns usergroup-2
    oc label ns usergroup-2 usergroup=usergroup-2
    oc apply -f - <<EOF
    kind: Istio
    apiVersion: sailoperator.io/v1alpha1
    metadata:
      name: usergroup-2
    spec:
      namespace: usergroup-2
      values:
        meshConfig:
          discoverySelectors:
            - matchLabels:
                usergroup: usergroup-2
      updateStrategy:
        type: InPlace
      version: v1.23.0
    EOF
    ```
1. Deploy a policy for workloads in the `usergroup-1` namespace to only accept mutual TLS traffic:
    ```bash
    oc apply -f - <<EOF
    apiVersion: security.istio.io/v1
    kind: PeerAuthentication
    metadata:
      name: "usergroup-1-peerauth"
      namespace: "usergroup-1"
    spec:
      mtls:
        mode: STRICT
    EOF
    ```
1. Deploy a policy for workloads in the `usergroup-2` namespace to only accept mutual TLS traffic:
    ```bash
    oc apply -f - <<EOF
    apiVersion: security.istio.io/v1
    kind: PeerAuthentication
    metadata:
      name: "usergroup-2-peerauth"
      namespace: "usergroup-2"
    spec:
      mtls:
        mode: STRICT
    EOF
    ```
1. Verify the control planes are deployed and running:
    ```bash
    oc get pods -n usergroup-1
    NAME                                  READY   STATUS    RESTARTS   AGE
    istiod-usergroup-1-747fddfb56-xzpkj   1/1     Running   0          5m1s
    oc get pods -n usergroup-2
    NAME                                  READY   STATUS    RESTARTS   AGE
    istiod-usergroup-2-5b9cbb7669-lwhgv   1/1     Running   0          3m41s
    ```

## Deploy application workloads per usergroup
1. Create three application namespaces:
    ```bash
    oc create ns app-ns-1
    oc create ns app-ns-2
    oc create ns app-ns-3
    ```
1. Label each namespace to associate them with their respective control planes:
    ```bash
    oc label ns app-ns-1 usergroup=usergroup-1 istio.io/rev=usergroup-1
    oc label ns app-ns-2 usergroup=usergroup-2 istio.io/rev=usergroup-2
    oc label ns app-ns-3 usergroup=usergroup-2 istio.io/rev=usergroup-2
    ```
1. Deploy one `sleep` and `bookinfo` application per namespace:
    ```bash
    oc apply -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/sleep/sleep.yaml -n app-ns-1
    oc apply -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/bookinfo/platform/kube/bookinfo.yaml -n app-ns-1
    oc apply -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/sleep/sleep.yaml -n app-ns-2
    oc apply -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/bookinfo/platform/kube/bookinfo.yaml -n app-ns-2
    oc apply -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/sleep/sleep.yaml -n app-ns-3
    oc apply -f https://raw.githubusercontent.com/istio/istio/release-1.23/samples/bookinfo/platform/kube/bookinfo.yaml -n app-ns-3
    ```
1. Wait a few seconds for the `bookinfo` and `sleep` pods to be running with sidecars injected:
    ```yaml
    oc get pods -n app-ns-1
    NAME                             READY   STATUS    RESTARTS   AGE
    details-v1-65cfcf56f9-gsrzs      2/2     Running   0          3m20s
    productpage-v1-d5789fdfb-9x42p   2/2     Running   0          3m17s
    ratings-v1-7c9bd4b87f-ww82z      2/2     Running   0          3m20s
    reviews-v1-6584ddcf65-8wd54      2/2     Running   0          3m19s
    reviews-v2-6f85cb9b7c-fwg4j      2/2     Running   0          3m18s
    reviews-v3-6f5b775685-kgftg      2/2     Running   0          3m18s
    sleep-5577c64d7c-b5wd2           2/2     Running   0          42m
    ```
    Repeat for other namespaces.

## Verify the application to control plane mapping
Now that the applications are deployed, you can use the `istioctl ps` command to confirm that the application workloads are managed by their respective control plane, i.e., `app-ns-1` is managed by `usergroup-1`, `app-ns-2` and `app-ns-3` are managed by `usergroup-2`:
```yaml
istioctl ps -i usergroup-1
NAME                                        CLUSTER        CDS               LDS               EDS                RDS               ECDS        ISTIOD                                  VERSION
details-v1-65cfcf56f9-gsrzs.app-ns-1        Kubernetes     SYNCED (6m2s)     SYNCED (6m2s)     SYNCED (5m54s)     SYNCED (6m2s)     IGNORED     istiod-usergroup-1-747fddfb56-xzpkj     1.23.0
productpage-v1-d5789fdfb-9x42p.app-ns-1     Kubernetes     SYNCED (6m)       SYNCED (6m)       SYNCED (5m54s)     SYNCED (6m)       IGNORED     istiod-usergroup-1-747fddfb56-xzpkj     1.23.0
ratings-v1-7c9bd4b87f-ww82z.app-ns-1        Kubernetes     SYNCED (6m2s)     SYNCED (6m2s)     SYNCED (5m54s)     SYNCED (6m2s)     IGNORED     istiod-usergroup-1-747fddfb56-xzpkj     1.23.0
reviews-v1-6584ddcf65-8wd54.app-ns-1        Kubernetes     SYNCED (6m2s)     SYNCED (6m2s)     SYNCED (5m54s)     SYNCED (6m2s)     IGNORED     istiod-usergroup-1-747fddfb56-xzpkj     1.23.0
reviews-v2-6f85cb9b7c-fwg4j.app-ns-1        Kubernetes     SYNCED (6m1s)     SYNCED (6m1s)     SYNCED (5m54s)     SYNCED (6m1s)     IGNORED     istiod-usergroup-1-747fddfb56-xzpkj     1.23.0
reviews-v3-6f5b775685-kgftg.app-ns-1        Kubernetes     SYNCED (6m1s)     SYNCED (6m1s)     SYNCED (5m54s)     SYNCED (6m1s)     IGNORED     istiod-usergroup-1-747fddfb56-xzpkj     1.23.0
sleep-5577c64d7c-b5wd2.app-ns-1             Kubernetes     SYNCED (13m)      SYNCED (13m)      SYNCED (5m54s)     SYNCED (13m)      IGNORED     istiod-usergroup-1-747fddfb56-xzpkj     1.23.0
```
```yaml
istioctl ps -i usergroup-2
NAME                                        CLUSTER        CDS                LDS                EDS                RDS                ECDS        ISTIOD                                  VERSION
details-v1-65cfcf56f9-m8hct.app-ns-3        Kubernetes     SYNCED (6m10s)     SYNCED (6m10s)     SYNCED (5m56s)     SYNCED (6m10s)     IGNORED     istiod-usergroup-2-5b9cbb7669-lwhgv     1.23.0
details-v1-65cfcf56f9-qsh8b.app-ns-2        Kubernetes     SYNCED (6m4s)      SYNCED (6m4s)      SYNCED (5m56s)     SYNCED (6m4s)      IGNORED     istiod-usergroup-2-5b9cbb7669-lwhgv     1.23.0
productpage-v1-d5789fdfb-dfkkk.app-ns-2     Kubernetes     SYNCED (6m2s)      SYNCED (6m2s)      SYNCED (5m56s)     SYNCED (6m2s)      IGNORED     istiod-usergroup-2-5b9cbb7669-lwhgv     1.23.0
productpage-v1-d5789fdfb-hwdj6.app-ns-3     Kubernetes     SYNCED (6m7s)      SYNCED (6m7s)      SYNCED (5m56s)     SYNCED (6m7s)      IGNORED     istiod-usergroup-2-5b9cbb7669-lwhgv     1.23.0
ratings-v1-7c9bd4b87f-8jwvk.app-ns-2        Kubernetes     SYNCED (6m3s)      SYNCED (6m3s)      SYNCED (5m56s)     SYNCED (6m3s)      IGNORED     istiod-usergroup-2-5b9cbb7669-lwhgv     1.23.0
ratings-v1-7c9bd4b87f-x8hr4.app-ns-3        Kubernetes     SYNCED (6m10s)     SYNCED (6m10s)     SYNCED (5m56s)     SYNCED (6m10s)     IGNORED     istiod-usergroup-2-5b9cbb7669-lwhgv     1.23.0
reviews-v1-6584ddcf65-4vfkt.app-ns-3        Kubernetes     SYNCED (6m8s)      SYNCED (6m8s)      SYNCED (5m56s)     SYNCED (6m8s)      IGNORED     istiod-usergroup-2-5b9cbb7669-lwhgv     1.23.0
reviews-v1-6584ddcf65-nt4qt.app-ns-2        Kubernetes     SYNCED (6m3s)      SYNCED (6m3s)      SYNCED (5m56s)     SYNCED (6m3s)      IGNORED     istiod-usergroup-2-5b9cbb7669-lwhgv     1.23.0
reviews-v2-6f85cb9b7c-2sswv.app-ns-3        Kubernetes     SYNCED (6m8s)      SYNCED (6m8s)      SYNCED (5m56s)     SYNCED (6m8s)      IGNORED     istiod-usergroup-2-5b9cbb7669-lwhgv     1.23.0
reviews-v2-6f85cb9b7c-g76r4.app-ns-2        Kubernetes     SYNCED (6m3s)      SYNCED (6m3s)      SYNCED (5m56s)     SYNCED (6m3s)      IGNORED     istiod-usergroup-2-5b9cbb7669-lwhgv     1.23.0
reviews-v3-6f5b775685-2xd5f.app-ns-3        Kubernetes     SYNCED (6m8s)      SYNCED (6m8s)      SYNCED (5m56s)     SYNCED (6m8s)      IGNORED     istiod-usergroup-2-5b9cbb7669-lwhgv     1.23.0
reviews-v3-6f5b775685-w25qk.app-ns-2        Kubernetes     SYNCED (6m2s)      SYNCED (6m2s)      SYNCED (5m56s)     SYNCED (6m2s)      IGNORED     istiod-usergroup-2-5b9cbb7669-lwhgv     1.23.0
sleep-5577c64d7c-wjnxc.app-ns-3             Kubernetes     SYNCED (12m)       SYNCED (12m)       SYNCED (5m56s)     SYNCED (12m)       IGNORED     istiod-usergroup-2-5b9cbb7669-lwhgv     1.23.0
sleep-5577c64d7c-xk27f.app-ns-2             Kubernetes     SYNCED (12m)       SYNCED (12m)       SYNCED (5m56s)     SYNCED (12m)       IGNORED     istiod-usergroup-2-5b9cbb7669-lwhgv     1.23.0
```
## Verify the application connectivity is ONLY within the respective usergroup
1. Send a request from the `sleep` pod in `app-ns-1` in `usergroup-1` to the `productpage` service in `app-ns-2` in `usergroup-2`. The communication should fail:
    ```yaml
    oc -n app-ns-1 exec "$(oc -n app-ns-1 get pod -l app=sleep -o jsonpath={.items..metadata.name})" -c sleep -- curl -sIL http://productpage.app-ns-2.svc.cluster.local:9080
    HTTP/1.1 503 Service Unavailable
    content-length: 95
    content-type: text/plain
    date: Wed, 16 Oct 2024 11:18:30 GMT
    server: envoy
    ```
1. Send a request from the `sleep` pod in `app-ns-2` in `usergroup-2` to the `productpage` service in `app-ns-3` in `usergroup-2`. The communication should work:
    ```yaml
    oc -n app-ns-2 exec "$(oc -n app-ns-2 get pod -l app=sleep -o jsonpath={.items..metadata.name})" -c sleep -- curl -sIL http://productpage.app-ns-3.svc.cluster.local:9080
    HTTP/1.1 200 OK
    server: envoy
    date: Wed, 16 Oct 2024 11:20:42 GMT
    content-type: text/html; charset=utf-8
    content-length: 2080
    x-envoy-upstream-service-time: 5
    ```