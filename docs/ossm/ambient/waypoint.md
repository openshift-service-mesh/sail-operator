# Getting Started with Istio waypoint in Ambient Mode (Tech Preview)

Once you have gotten started with Istio's ambient mode using ztunnel proxies, you may want to introduce waypoint proxies to take advantage of the many Layer 7 (L7) processing features that Istio provides. This section provides an overview and procedure for adding waypoint proxies to workloads running in your Istio ambient mode service mesh.

---

Istio's ambient mode splits Istio's functionality into two distinct layers, a secure overlay layer 4 (L4) and a Layer 7 (L7). The waypoint proxy is an optional component that is Envoy-based and handles L7 processing for workloads it manages. It acts as a gateway to a resource (a namespace, service or pod). Waypoint proxies are installed, upgraded and scaled independently from applications. They can be configured using the Kubernetes [Gateway API](https://gateway-api.sigs.k8s.io/).

## 1. What is a waypoint proxy for Istio's ambient mode?

A **waypoint proxy** is an optional Envoy-based proxy that provides Layer 7 (L7) processing for specific workloads that are part of a service mesh running in ambient mode. Unlike the traditional sidecar model, where each workload gets its own Envoy proxy, waypoint proxies significantly reduce the number of proxies needed by allowing a single waypoint or set of waypoints to be shared across applications within a similar security boundary (e.g., all workloads in a namespace).

Waypoint proxies operate independently of applications, meaning application owners don't need to be aware of their existence. In **ambient mode**, policies are enforced by the destination waypoint, which acts as a gateway, ensuring all incoming traffic to a resource (namespace, service, or pod) passes through it for policy enforcement.

### 1.1 Do You Need a Waypoint Proxy?

The **ztunnel node proxy** handles Layer 4 (L4) features in ambient mode, including mutual TLS encryption, L4 traffic processing and telemetry. However, if your applications require any of the following L7 mesh functionalities, you'll need to use a waypoint proxy:

* **Traffic Management:** This includes advanced HTTP routing and load balancing, circuit breaking, rate limiting, fault injection, retries, and timeouts.
* **Security:** For rich authorization policies based on L7 primitives like request type or HTTP headers.
* **Observability:** To collect HTTP metrics, enable access logging, and implement tracing for your applications.

---

## 2. Pre-requisites to Using Waypoint Ambient Mode with OSSM 3

Before installing Istio's ambient mode with OpenShift Service Mesh, ensure the following prerequisites are met:

* **OpenShift Container Platform 4.19+:** This version of OpenShift is required for supported Kubernetes Gateway API CRDs, which are essential for ambient mode functionalities.
* **OpenShift Service Mesh 3.1.0+ operator is installed:** Ensure that the OSSM operator version 3.1.0 or later is installed on your OpenShift cluster.
* **Istio deployed in Ambient Mode:** Refer to OSSM Ambient [initial doc](README.md).
* **Labeling:** Make sure the workloads/namespaces have the appropriate labels for ztunnel traffic redirection.

**Note**: Istio's ambient mode is completely incompatible with clusters containing the OpenShift Service Mesh 2.6 or earlier versions of the operator and they should not be used together.

---

## 3. Waypoint Proxies

### 3.1 Using Kubernetes Gateway API

Waypoint proxies are deployed using Kubernetes Gateway resources.  

**Note**: The following command will install community maintained Kubernetes Gateway API CRDs that are not supported by Red Hat. This is only required for OpenShift 4.18 and earlier releases. Upgrading to OpenShift 4.19, which includes supported Gateway API CRDs may result in application disruptions.

```bash
oc get crd gateways.gateway.networking.k8s.io &> /dev/null || \
  { oc apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.0/standard-install.yaml; }
```

### 3.2 Set up Istio Ambient Mode Resources and a Sample Application

1. If you have not already done so, install the OpenShift Service Mesh Operator along with Istio in ambient mode using the following [steps](README.md#3-procedure-to-install-istios-ambient-mode). 

2. The example below will use the sample Bookinfo application. If you have not already done so, deploy the sample Bookinfo applications. The steps can be found [here](README.md#36-about-the-bookinfo-application). 

Before you deploy a waypoint proxy in the application namespace, confirm the namespace is labeled with `istio.io/dataplane-mode: ambient`.

### 3.3 Deploy a Waypoint Proxy

1. Deploy a waypoint proxy in the bookinfo application namespace:

```bash
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  labels:
    istio.io/waypoint-for: service
  name: waypoint
  namespace: bookinfo
spec:
  gatewayClassName: istio-waypoint
  listeners:
  - name: mesh
    port: 15008
    protocol: HBONE
```

2. Apply the Waypoint CR.

```bash
$ oc apply -f waypoint.yaml
```

The `istio.io/waypoint-for: service` label on the Gateway resource specifies that it processes traffic for services, which is the default behavior. The type of traffic a waypoint handles is determined by this label. For more details you can refer to Istio [documentation](https://istio.io/latest/docs/ambient/usage/waypoint/#waypoint-traffic-types).

3. Enroll the bookinfo namespace to use the waypoint:

```bash
$ oc label namespace bookinfo istio.io/use-waypoint=waypoint
```

After a namespace is enrolled to use a waypoint, any requests from any pods using the ambient data plane mode, to any service running in the `bookinfo` namespace, will be routed through the waypoint for L7 processing and policy enforcement. 

If you prefer more granularity than using a waypoint for an entire namespace, you can enroll only a specific service or pod to use a waypoint by labeling the respective service or the pod. When enrolling a pod explicitly, you must also add the `istio.io/waypoint-for: workload` label to the Gateway resource.

### 3.3 Cross-namespace Waypoint

1. By default, a waypoint is usable only within the same namespace, but it also supports cross-namespace usage. The following Gateway allows resources in the `bookinfo` namespace to use `waypoint-foo` from the `foo` namespace:

```bash
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: waypoint-foo
  namespace: foo
spec:
  gatewayClassName: istio-waypoint
  listeners:
  - name: mesh
    port: 15008
    protocol: HBONE
    allowedRoutes:
      namespaces:
        from: Selector
        selector:
          matchLabels:
            kubernetes.io/metadata.name: bookinfo
```

2. Apply the cross namespace waypoint

```bash
$ oc apply -f waypoint-foo.yaml
```

3. By default, the Istio control plane will look for a waypoint specified using the `istio.io/use-waypoint` label in the same namespace as the resource which the label is applied to. You can add labels `istio.io/use-waypoint-namespace` and `istio.io/use-waypoint` together to start using the cross-namespace waypoint.

```bash
$ oc label namespace bookinfo istio.io/use-waypoint-namespace=foo
$ oc label namespace bookinfo istio.io/use-waypoint=waypoint-foo
```

---

## 4. Layer 7 Features in Ambient Mode

The following section describes the stable features using Gateway API resource `HTTPRoute` and Istio resource `AuthorizationPolicy`. Other L7 features using a waypoint proxy will be covered once they reach to Beta status.

### 4.1 Traffic Routing

With a waypoint proxy deployed, you can split traffic between different versions of the bookinfo reviews service. This is useful for testing new features or performing A/B testing.

1. For example, let’s configure traffic routing to send 90% of requests to reviews-v1 and 10% to reviews-v2:

```bash
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: reviews
  namespace: bookinfo
spec:
  parentRefs:
  - group: ""
    kind: Service
    name: reviews
    port: 9080
  rules:
  - backendRefs:
    - name: reviews-v1
      port: 9080
      weight: 90
    - name: reviews-v2
      port: 9080
      weight: 10
```

2. Apply the traffic routing configuration CR.

```bash
$ oc apply -f traffic-route.yaml
```

If you open the Bookinfo application in your browser and refresh the page multiple times, you'll notice that most requests (90%) go to `reviews-v1`, which doesn't show any stars, while a smaller portion (10%) go to `reviews-v2`, which display black stars.

### 4.2 Security Authorization

The `AuthorizationPolicy` resource can be used in both sidecar mode and ambient mode. In ambient mode, authorization policies can either be targeted (for ztunnel enforcement) or attached (for waypoint enforcement). For an authorization policy to be attached to a waypoint it must have a `targetRef` which refers to the waypoint, or a Service which uses that waypoint.

When a waypoint proxy is added to a workload, you may have two possible places where you can enforce L4 policy (L7 policy can only be enforced at the waypoint proxy). Ideally you should attach your policy to the waypoint proxy, because the destination ztunnel will see traffic with the waypoint’s identity, not the source identity once you have introduced a waypoint to the traffic path.

1. For example, let's add a L7 authorization policy that will explicitly allow a curl service to send `GET` requests to the `productpage` service, but perform no other operations:

```bash
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: productpage-waypoint
  namespace: bookinfo
spec:
  targetRefs:
  - kind: Service
    group: ""
    name: productpage
  action: ALLOW
  rules:
  - from:
    - source:
        principals:
        - cluster.local/ns/default/sa/curl
    to:
    - operation:
        methods: ["GET"]
```

2. Apply the authorization policy CR.

```bash
$ oc apply -f authorization-policy.yaml
```

Note the targetRefs field is used to specify the target service for the authorization policy of a waypoint proxy. 

### 4.3 Security Authentication

Istio’s peer authentication policies, which configure mutual TLS (mTLS) modes, are supported by ztunnel.
The key difference of that between sidecar mode and ambient mode is that `DISABLE` mode policies are ignored in ambient mode because ztunnel and HBONE implies the use of mTLS.
More information about Istio's peer authentication behavior can be found [here](https://istio.io/latest/docs/concepts/security/#peer-authentication). 
