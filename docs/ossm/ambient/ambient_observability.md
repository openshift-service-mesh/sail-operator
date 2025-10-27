
# Getting Started with Observability in Service Mesh Ambient mode

Red Hat OpenShift Observability provides real-time visibility, monitoring, and analysis of various system metrics, logs, and events to help you quickly diagnose and troubleshoot issues before they impact systems or applications.

Red Hat OpenShift Service Mesh connects open-source observability tools and technologies to create a unified Observability solution. The components of this Observability stack work together to help you collect, store, deliver, analyze, and visualize data.

The following components in Service Mesh Ambient mode generate detailed telemetry for all service communications within a mesh.

| Component        | Description   |
| -----------      | ------------- |
| ztunnel          | generates L4 telemetry such as TCP metrics.                                           |
| waypoint proxies | generates L7 telemetry for HTTP, HTTP/2, gRPC traffic metrics and distributed traces. |

Red Hat OpenShift Service Mesh integrates with the following Red Hat OpenShift Observability components in ambient mode:

- OpenShift Cluster Monitoring components
- Red Hat OpenShift Distributed tracing platform

OpenShift Service Mesh also integrates with:

- Kiali provided by Red Hat, a powerful console for visualizing and managing your service mesh.
- OpenShift Service Mesh Console (OSSMC) plugin, an OpenShift Container Platform console plugin that seamlessly integrates Kiali console features into your OpenShift console.

## Configuring Metrics with OpenShift Cluster Monitoring and Service Mesh Ambient mode

Monitoring stack components are deployed by default in every OpenShift Container Platform installation. These components include Prometheus, Alertmanager, Thanos Querier, and others.

When you have enrolled your application in the Service Mesh Ambient mesh mode, you can monitor the [Istio Standard Metrics](https://istio.io/latest/docs/reference/config/metrics/) of your application from the ztunnel resource or the waypoint proxies. The ztunnel also exposes [a variety of DNS and debugging metrics](https://github.com/istio/ztunnel/blob/master/README.md#metrics).

### Prerequisites

- Red Hat OpenShift Service Mesh 3 resources are installed.
- OpenShift Cluster Monitoring User-workload monitoring is enabled. You can enable that by applying the following ConfigMap change.

```sh
cat <<EOF | kubectl apply -f-
apiVersion: v1
kind: ConfigMap
metadata:
  name: cluster-monitoring-config
  namespace: openshift-monitoring
data:
  config.yaml: |
    enableUserWorkload: true
EOF

oc wait --for=condition=Ready pods --all -n openshift-user-workload-monitoring --timeout 60s
```

See [Enabling monitoring for user-defined projects](https://docs.openshift.com/container-platform/4.19/observability/monitoring/enabling-monitoring-for-user-defined-projects.html).

### Procedure

1. Verify that Red Hat OpenShift Service Mesh 3 Ambient mode CRs are created. For example,

```yaml
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: default
spec:
  version: v1.27-latest
  namespace: istio-cni
  profile: ambient
---
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  version: v1.27-latest
  namespace: istio-system
  profile: ambient
  values:
    pilot:
      trustedZtunnelNamespace: ztunnel
---
apiVersion: sailoperator.io/v1alpha1
kind: ZTunnel
metadata:
  name: default
spec:
  version: v1.27-latest
  namespace: ztunnel
  profile: ambient
```

2. Create a `Service` resource to use the metrics exposed by the ztunnel.

```sh
cat <<EOF | kubectl apply -f-
apiVersion: v1
kind: Service
metadata:
  name: ztunnel
  namespace: ztunnel
  labels:
    app: ztunnel
    service: ztunnel
spec:
  selector:
    app: ztunnel
  ports:
    - name: http-monitoring
      protocol: TCP
      port: 15020
      targetPort: 15020
EOF
```

3. Create `ServiceMonitor` resources to monitor the Istio contorl plane and ztunnel pods:

```sh
cat <<EOF | kubectl apply -f-
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: istiod-monitor
  namespace: istio-system
spec:
  targetLabels:
  - app
  selector:
    matchLabels:
      istio: pilot
  endpoints:
  - port: http-monitoring
    interval: 30s
EOF
```

```sh
cat <<EOF | kubectl apply -f-
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: ztunnel-monitor
  namespace: ztunnel
spec:
  targetLabels:
  - app
  selector:
    matchLabels:
      service: ztunnel
  endpoints:
  - port: http-monitoring
    interval: 30s
EOF
```

If you need to monitor a waypoint proxy metrics, you can create a similar service to use the metrics exposed by the waypoint proxy and then apply a `ServiceMonitor` resource.


## Configuring OpenShift Distributed tracing platform with Service Mesh Ambient mode

Integrating Red Hat OpenShift distributed tracing platform with Red Hat OpenShift Service Mesh depends on two parts: Red Hat OpenShift distributed tracing platform (Tempo) and Red Hat build of OpenTelemetry collector.

For more information about the distributed tracing platform (Tempo), its features, installation, and configuration, see: [Red Hat OpenShift distributed tracing platform (Tempo)](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/distributed_tracing/distr-tracing-architecture).

For more information about Red Hat build of OpenTelemetry collector, its features, installation, and configuration, see: [Red Hat build of OpenTelemetry](https://docs.redhat.com/en/documentation/openshift_container_platform/4.19/html/red_hat_build_of_opentelemetry/index).

NOTE: The `ztunnel` component generates only Layer 4 data. Distributed tracing with Layer 7 spans is supported only when the workload or service has an attached `waypoint` or `gateway` proxy. Trace spans will include telemetry data from those waypoint and/or gateway proxies.

### Prerequisites

- A Tempo Operator is installed. See: [Installing the Tempo Operator](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/distributed_tracing/distr-tracing-tempo-installing#installing-the-tempo-operator_distr-tracing-tempo-installing).
- A Red Hat build of OpenTelemetry Operator is installed. See: [Installing the Red Hat build of OpenTelemetry Operator](https://docs.redhat.com/en/documentation/openshift_container_platform/4.19/html/red_hat_build_of_opentelemetry/install-otel).
- A TempoStack is installed and configured in a namespace such as `tempo`. See: [Installing a TempoStack instance](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/distributed_tracing/distr-tracing-tempo-installing#installing-a-tempostack-instance_distr-tracing-tempo-installing).

### Procedure

1. Navigate to the Red Hat build of OpenTelemetry Operator and install the `OpenTelemetryCollector` custom resource in the `istio-system` namespace. For example,

```sh
cat <<EOF | kubectl apply -f-
kind: OpenTelemetryCollector
apiVersion: opentelemetry.io/v1beta1
metadata:
  name: otel
  namespace: istio-system
spec:
  mode: deployment
  observability:
    metrics: {}
  deploymentUpdateStrategy: {}
  config:
    exporters:
      otlp:
        endpoint: 'tempo-sample-distributor.tempo.svc.cluster.local:4317'
        tls:
          insecure: true
    receivers:
      otlp:
        protocols:
          grpc:
            endpoint: '0.0.0.0:4317'
          http: {}
    service:
      pipelines:
        traces:
          exporters:
            - otlp
          receivers:
            - otlp
EOF
```

NOTE: The `exporters.otlp.endpoint` field is the Tempo sample distributor service in a namespace such as `tempo`.

2. Configure Red Hat OpenShift Service Mesh `Istio` custom resource to define a tracing provider in the spec.values.meshConfig:

```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
#  ...
  name: default
spec:
  namespace: istio-system
  profile: ambient
#  ...
  values:
    meshConfig:
      enableTracing: true
      extensionProviders:
      - name: otel
        opentelemetry:
          port: 4317
          service: otel-collector.istio-system.svc.cluster.local
    pilot:
      trustedZtunnelNamespace: ztunnel
```

NOTE: The `service` field is the OpenTelemetry collector service in the `istio-system` namespace.

3. Create an Istio Telemetry custom resource to enable the tracing provider defined in the spec.values.meshConfig.ExtensionProviders:

```sh
cat <<EOF | kubectl apply -f-
apiVersion: telemetry.istio.io/v1
kind: Telemetry
metadata:
  name: otel-demo
  namespace: istio-system
spec:
  tracing:
    - providers:
        - name: otel
      randomSamplingPercentage: 100
EOF
```

NOTE: Once you verify that you can see traces, lower the randomSamplingPercentage value or set it to default to reduce the number of requests.

NOTE: You may use a spec.targetRefs field to enable tracing at a gateway or a waypoint level.

## Validation

1. You can find the status of `Metrics Targets` in the OpenShift Console. Click the `Observe` and `Targets` from the left panel and search targets such as `istiod-mointor`, `ztunnel-monitor` etc. Wait for their `Up` status.

NOTE: The `SerivceMonitor` resource configuration can take up to 5 minutes for showing new `Metrics Targets` results.


2. Create a `bookinfo` namespace and enable ambient mode by running the following commands:

```sh
oc new-project bookinfo
oc label namespace bookinfo istio.io/dataplane-mode=ambient
```

3. Deploy the bookinfo application in the `bookinfo` namespace by running the following command:

```sh
oc apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/master/samples/bookinfo/platform/kube/bookinfo.yaml
oc wait --for=condition=Ready pods --all -n bookinfo --timeout 60s
```

4. Create a bookinfo gateway to manage inbound bookinfo traffic:

```sh
oc get crd gateways.gateway.networking.k8s.io &> /dev/null ||  { oc kustomize "github.com/kubernetes-sigs/gateway-api/config/crd?ref=v1.0.0" | oc apply -f -; }
oc apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/master/samples/bookinfo/gateway-api/bookinfo-gateway.yaml
oc wait --for=condition=Ready pods --all -n bookinfo --timeout 60s
export INGRESS_HOST=$(oc get -n bookinfo gtw bookinfo-gateway -o jsonpath='{.status.addresses[0].value}')
export INGRESS_PORT=$(oc get -n bookinfo gtw bookinfo-gateway -o jsonpath='{.spec.listeners[?(@.name=="http")].port}')
export GATEWAY_URL=$INGRESS_HOST:$INGRESS_PORT
echo "http://${GATEWAY_URL}/productpage"
```

5. Deploy a waypoint proxy and use it to handle all service traffic in the `bookinfo` namespace:

```sh
cat <<EOF | oc apply -f-
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
EOF
```

6. Enroll the `bookinfo` namespace to use the waypoint:

```sh
oc label namespace bookinfo istio.io/use-waypoint=waypoint
```

7. Send traffic to the `productpage` service for generating metrics and traces:

```sh
curl "http://${GATEWAY_URL}/productpage" | grep "<title>"
```

8. Validate the bookinfo application metrics in the OpenShift Console. Click the `Observe` and `Metrics` from the left panel and run the query `istio_tcp_received_bytes_total` or `istio_build`.

9. Validate the bookinfo application traces in a Tempo dashboard UI. You can find the dashboard UI route by running the following command:

```sh
oc get routes -n tempo tempo-sample-query-frontend
```

  Select the `bookinfo-gateway-istio.booinfo` or the `waypoint.bookinfo` service from the dashboard UI and then click `Find Traces`.

NOTE: The OpenShift route for Tempo dashboard UI can be created from the TempoStack custom resource with `.spec.template.queryFrontend.jaegerQuery.ingress.type: route`.

