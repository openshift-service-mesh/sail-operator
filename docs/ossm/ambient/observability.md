
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

When you have enrolled your application in the Service Mesh Ambient mesh mode, you can monitor the [Istio Standard Metrics](https://istio.io/latest/docs/reference/config/metrics/) of your application from the ztunnel resource and the waypoint proxies. The ztunnel also exposes [a variety of DNS and debugging metrics](https://github.com/istio/ztunnel/blob/master/README.md#metrics).

Because the Service Mesh Ambient mode has two levels of proxies, there are two sets of metrics that can be collected for each application service. The layer 4 TCP related metrics can be collected from the ztunnel resource and the waypoint proxies. The layer 7 metrics such as HTTP traffic metrics can be collected from the waypoint proxies.

### Prerequisites

1. If you have not already done so, install the OpenShift Service Mesh Operator along with Istio in ambient mode using the following [steps](https://github.com/openshift-service-mesh/sail-operator/blob/main/docs/ossm/ambient/README.md#3-procedure-to-install-istios-ambient-mode).

2. OpenShift Cluster Monitoring User-workload monitoring is enabled. You can enable that by applying the following ConfigMap change.

  This ConfigMap change is the only command you need from the doc page [Enabling monitoring for user-defined projects](https://docs.redhat.com/en/documentation/openshift_container_platform/4.19/html/monitoring/configuring-user-workload-monitoring#enabling-monitoring-for-user-defined-projects_preparing-to-configure-the-monitoring-stack-uwm). It's required for both sidecar mode and ambient mode metrics integration.

```sh
cat <<EOF | oc apply -f-
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

### Procedure

1. The example below will use the sample Bookinfo application. If you have not already done so, deploy the sample Bookinfo applications. The steps can be found [here](https://github.com/openshift-service-mesh/sail-operator/blob/main/docs/ossm/ambient/README.md#36-about-the-bookinfo-application).

2. Create a `Telemetry` resource in the Istio Control Plane namespace to ensure Prometheus is a metrics provider:

```sh
cat <<EOF | oc apply -f-
apiVersion: telemetry.istio.io/v1
kind: Telemetry
metadata:
  name: enable-prometheus-metrics
  namespace: istio-system
spec:
  metrics:
  - providers:
    - name: prometheus
EOF
```

3. Create a `ServiceMonitor` resource for collecting the Istio control plane metrics:

```sh
cat <<EOF | oc apply -f-
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

4. Create a `PodMonitor` resource in the `ztunnel` namespace for collecting the ztunnel metrics:

```sh
cat <<EOF | oc apply -f-
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: istio-ztunnel-monitor
  namespace: ztunnel
spec:
  selector:
    matchExpressions:
    - key: istio-prometheus-ignore
      operator: DoesNotExist
  podMetricsEndpoints:
  - path: /stats/prometheus
    interval: 30s
    relabelings:
    - action: keep
      sourceLabels: [__meta_kubernetes_pod_container_name]
      regex: "istio-proxy"
    - action: keep
      sourceLabels: [__meta_kubernetes_pod_annotationpresent_prometheus_io_scrape]
    - action: replace
      regex: (\\d+);(([A-Fa-f0-9]{1,4}::?){1,7}[A-Fa-f0-9]{1,4})
      replacement: '[\$2]:\$1'
      sourceLabels: [__meta_kubernetes_pod_annotation_prometheus_io_port, __meta_kubernetes_pod_ip]
      targetLabel: __address__
    - action: replace
      regex: (\\d+);((([0-9]+?)(\.|$)){4})
      replacement: \$2:\$1
      sourceLabels: [__meta_kubernetes_pod_annotation_prometheus_io_port, __meta_kubernetes_pod_ip]
      targetLabel: __address__
    # Set the 'app' label from 'app.kubernetes.io/name' or fallback to 'app'
    - sourceLabels: ["__meta_kubernetes_pod_label_app_kubernetes_io_name", "__meta_kubernetes_pod_label_app"]
      separator: ";"
      targetLabel: "app"
      action: replace
      regex: "(.+);.*|.*;(.+)"
      replacement: "\${1}\${2}"  # Use the first non-empty value
    # Set the 'version' label from 'app.kubernetes.io/version' or fallback to 'version'
    - sourceLabels: ["__meta_kubernetes_pod_label_app_kubernetes_io_version", "__meta_kubernetes_pod_label_version"]
      separator: ";"
      targetLabel: "version"
      action: replace
      regex: "(.+);.*|.*;(.+)"
      replacement: "\${1}\${2}"  # Use the first non-empty value
    # add some labels we want
    - sourceLabels: [__meta_kubernetes_namespace]
      action: replace
      targetLabel: namespace
    - action: replace
      replacement: "mesh_id"
      targetLabel: mesh_id
EOF
```

5. A waypoint is an optional proxy that can be deployed to provide layer 7 (e.g. HTTP) features, such as metrics and traces in ambient mode. Deploy a waypoint proxy. The steps can be found [here](https://github.com/openshift-service-mesh/sail-operator/blob/main/docs/ossm/ambient/waypoint.md#3-waypoint-proxies).

6. Create a `PodMonitor` resource for collecting waypoint proxies metrics in an application namespace such as `bookinfo`:

```sh
cat <<EOF | oc apply -f-
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: istio-waypoint-monitor
  namespace: bookinfo
spec:
  selector:
    matchExpressions:
    - key: istio-prometheus-ignore
      operator: DoesNotExist
  podMetricsEndpoints:
  - path: /stats/prometheus
    interval: 30s
    relabelings:
    - action: keep
      sourceLabels: [__meta_kubernetes_pod_container_name]
      regex: "istio-proxy"
    - action: keep
      sourceLabels: [__meta_kubernetes_pod_annotationpresent_prometheus_io_scrape]
    - action: replace
      regex: (\\d+);(([A-Fa-f0-9]{1,4}::?){1,7}[A-Fa-f0-9]{1,4})
      replacement: '[\$2]:\$1'
      sourceLabels: [__meta_kubernetes_pod_annotation_prometheus_io_port, __meta_kubernetes_pod_ip]
      targetLabel: __address__
    - action: replace
      regex: (\\d+);((([0-9]+?)(\.|$)){4})
      replacement: \$2:\$1
      sourceLabels: [__meta_kubernetes_pod_annotation_prometheus_io_port, __meta_kubernetes_pod_ip]
      targetLabel: __address__
    # Set the 'app' label from 'app.kubernetes.io/name' or fallback to 'app'
    - sourceLabels: ["__meta_kubernetes_pod_label_app_kubernetes_io_name", "__meta_kubernetes_pod_label_app"]
      separator: ";"
      targetLabel: "app"
      action: replace
      regex: "(.+);.*|.*;(.+)"
      replacement: "\${1}\${2}"  # Use the first non-empty value
    # Set the 'version' label from 'app.kubernetes.io/version' or fallback to 'version'
    - sourceLabels: ["__meta_kubernetes_pod_label_app_kubernetes_io_version", "__meta_kubernetes_pod_label_version"]
      separator: ";"
      targetLabel: "version"
      action: replace
      regex: "(.+);.*|.*;(.+)"
      replacement: "\${1}\${2}"  # Use the first non-empty value
    # add some labels we want
    - sourceLabels: [__meta_kubernetes_namespace]
      action: replace
      targetLabel: namespace
    - action: replace
      replacement: "mesh_id"
      targetLabel: mesh_id
EOF
```

  A waypoint proxy generates layer 4 and layer 7 statistics as metrics. It scopes the statistics by Envoy proxy functions. Examples include:

  - [Upstream connection](https://www.envoyproxy.io/docs/envoy/latest/configuration/upstream/cluster_manager/cluster_stats)
  - [Listener](https://www.envoyproxy.io/docs/envoy/latest/configuration/listeners/stats)
  - [HTTP Connection Manager](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_conn_man/stats)
  - [TCP proxy](https://www.envoyproxy.io/docs/envoy/latest/configuration/listeners/network_filters/tcp_proxy_filter#statistics)
  - [Router](https://www.envoyproxy.io/docs/envoy/latest/configuration/http/http_filters/router_filter.html?highlight=vhost#statistics)

### Metrics Validation

1. You can find the status of `Metrics Targets` in the OpenShift Console. Click the `Observe` and `Targets` from the left panel and search targets such as `istiod-monitor`, `istio-ztunnel-monitor` and `istio-waypoint-monitor`. Wait for their `Up` status.

  NOTE: The `ServiceMonitor` and `PodMonitor` resources configuration can take up to 5 minutes for showing new `Metrics Targets` results.

2. Send some traffic to the Bookinfo `productpage` service for generating metrics:

```sh
curl "http://${GATEWAY_URL}/productpage" | grep "<title>"
```

3. Validate the Bookinfo application metrics in the OpenShift Console. Click the `Observe` and `Metrics` from the left panel and run a query such as `istio_build`, `istio_tcp_received_bytes_total` or `istio_requests_total`.


## Configuring OpenShift Distributed tracing platform with Service Mesh Ambient mode

Integrating Red Hat OpenShift distributed tracing platform with Red Hat OpenShift Service Mesh depends on two parts: Red Hat OpenShift distributed tracing platform (Tempo) and Red Hat build of OpenTelemetry collector.

For more information about the distributed tracing platform (Tempo), its features, installation, and configuration, see: [Red Hat OpenShift distributed tracing platform (Tempo)](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/distributed_tracing/distr-tracing-architecture).

For more information about Red Hat build of OpenTelemetry collector, its features, installation, and configuration, see: [Red Hat build of OpenTelemetry](https://docs.redhat.com/en/documentation/openshift_container_platform/4.19/html/red_hat_build_of_opentelemetry/index).

NOTE: The `ztunnel` component generates only layer 4 data. Distributed tracing with layer 7 spans is supported only when the workload or service has an attached `waypoint` or `gateway` proxy. Trace spans will include telemetry data from those waypoint and/or gateway proxies.

### Prerequisites

- A Tempo Operator is installed. See: [Installing the Tempo Operator](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/distributed_tracing/distr-tracing-tempo-installing#installing-the-tempo-operator_distr-tracing-tempo-installing).
- A Red Hat build of OpenTelemetry Operator is installed. See: [Installing the Red Hat build of OpenTelemetry Operator](https://docs.redhat.com/en/documentation/openshift_container_platform/4.19/html/red_hat_build_of_opentelemetry/install-otel).
- A TempoStack is installed and configured in a namespace such as `tempo`. See: [Installing a TempoStack instance](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/distributed_tracing/distr-tracing-tempo-installing#installing-a-tempostack-instance_distr-tracing-tempo-installing).

### Procedure

1. Navigate to the Red Hat build of OpenTelemetry Operator and install the `OpenTelemetryCollector` custom resource in the `istio-system` namespace. For example,

```sh
cat <<EOF | oc apply -f-
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
cat <<EOF | oc apply -f-
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

  NOTE: When you need to use a single Istio Telemetry custom resource for a metrics `prometheus` provider and a tracing provider, you can set the `spec.metrics.overrides.disabled: false` to enable a metrics `prometheus` provider. This step is not needed if you follow the `Configuring Metrics with OpenShift Cluster Monitoring` approach above.

### Tracing Validation

1. The example below will use the sample Bookinfo application. If you have not already done so, deploy the sample Bookinfo applications. The steps can be found [here](https://github.com/openshift-service-mesh/sail-operator/blob/main/docs/ossm/ambient/README.md#36-about-the-bookinfo-application).

2. If you have not already done so, deploy a waypoint proxy and enroll the `bookinfo` namespace to use the waypoint:

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

oc label namespace bookinfo istio.io/use-waypoint=waypoint
```

3. Send some traffic to the Bookinfo `productpage` service for generating traces:

```sh
curl "http://${GATEWAY_URL}/productpage" | grep "<title>"
```

4. Validate the Bookinfo application traces in a Tempo dashboard UI. You can find the dashboard UI route by running the following command:

```sh
oc get routes -n tempo tempo-sample-query-frontend
```

  Select the `bookinfo-gateway-istio.booinfo` or the `waypoint.bookinfo` service from the dashboard UI and then click `Find Traces`.

  NOTE: The OpenShift route for Tempo dashboard UI can be created from the TempoStack custom resource with `.spec.template.queryFrontend.jaegerQuery.ingress.type: route`.

