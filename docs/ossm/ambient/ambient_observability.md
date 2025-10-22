
# Getting Started with Observability in Service Mesh Ambient mode

Red Hat OpenShift Observability provides real-time visibility, monitoring, and analysis of various system metrics, logs, and events to help you quickly diagnose and troubleshoot issues before they impact systems or applications.

Red Hat OpenShift Service Mesh connects open-source observability tools and technologies to create a unified Observability solution. The components of this Observability stack work together to help you collect, store, deliver, analyze, and visualize data.

The following components in Service Mesh Abmient mode generate detailed telemetry for all service communications within a mesh.

| Component        | Description   |
| -----------      | ------------- |
| ztunnel          | generates L4 telemetry such as TCP metrics.                                           |
| waypoint proxies | generates L7 telemetry for HTTP, HTTP/2, gRPC traffic metrics and distributed traces. |

Red Hat OpenShift Service Mesh integrates with the following Red Hat OpenShift Observability components in ambient mode:

- OpenShift Cluster Monitoring Prometheus
- Red Hat OpenShift distributed tracing platform

OpenShift Service Mesh also integrates with:

- Kiali provided by Red Hat, a powerful console for visualizing and managing your service mesh.
- OpenShift Service Mesh Console (OSSMC) plugin, an OpenShift Container Platform console plugin that seamlessly integrates Kiali console features into your OpenShift console.

## Configuring Metrics with OpenShift Cluster Monitoring and Service Mesh Ambient mode


## Configuring OpenShift distributed tracing platform with Service Mesh Ambient mode

Integrating Red Hat OpenShift distributed tracing platform with Red Hat OpenShift Service Mesh depends on two parts: Red Hat OpenShift distributed tracing platform (Tempo) and Red Hat build of OpenTelemetry collector.

For more information about the distributed tracing platform (Tempo), its features, installation, and configuration, see: [Red Hat OpenShift distributed tracing platform (Tempo)](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/distributed_tracing/distr-tracing-architecture).

For more information about Red Hat build of OpenTelemetry collector, its features, installation, and configuration, see: [Red Hat build of OpenTelemetry](https://docs.redhat.com/en/documentation/openshift_container_platform/4.16/html/red_hat_build_of_opentelemetry/index).

NOTE: Red Hat OpenShift Service Mesh Ambient mode does not install sidecar proxies by default. The `ztunnel` component can only generate L4 data and traces can only be generated from L7 at the moment. So Distributed tracing is only supported when a workload or a service has an attached waypoint proxy or a Gateway proxy. A trace span will include those waypoint and/or Gateway proxies' telemetry data.

### Prerequisites

- A Tempo Operator is installed. See: [Installing the Tempo Operator](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/distributed_tracing/distr-tracing-tempo-installing#installing-the-tempo-operator_distr-tracing-tempo-installing).
- A Red Hat build of OpenTelemetry Operator is installed. See: [Installing the Red Hat build of OpenTelemetry Operator](https://docs.redhat.com/en/documentation/openshift_container_platform/4.16/html/red_hat_build_of_opentelemetry/install-otel).
- A TempoStack is installed and configured in a namespace such as `tempo`. See: [Installing a TempoStack instance](https://docs.redhat.com/en/documentation/openshift_container_platform/latest/html/distributed_tracing/distr-tracing-tempo-installing#installing-a-tempostack-instance_distr-tracing-tempo-installing).

### Procedure

1. Navigate to the Red Hat build of OpenTelemetry Operator and install the `OpenTelemetryCollector` custom resource in the `istio-system` namespace:

Example of an OpenTelemetry collector in the `istio-system` namespace

```yaml
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

```yaml
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
```

NOTE: Once you verify that you can see traces, lower the randomSamplingPercentage value or set it to default to reduce the number of requests.
NOTE: You may use a spec.targetRefs field to enable tracing at a gateway or a waypoint level.

### Validation

1. Create a `bookinfo` namespace and enable ambient mode by running the following commands:

```sh
$ oc new-project bookinfo
$ oc label namespace bookinfo istio.io/dataplane-mode=ambient
```

2. Deploy the bookinfo application in the `bookinfo` namespace by running the following command:

```sh
$ oc apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/master/samples/bookinfo/platform/kube/bookinfo.yaml
$ oc wait --for=condition=Ready pods --all -n bookinfo --timeout 60s
```

3. Create a bookinfo gateway to manage inbound bookinfo traffic:

```sh
$ oc get crd gateways.gateway.networking.k8s.io &> /dev/null ||  { oc kustomize "github.com/kubernetes-sigs/gateway-api/config/crd?ref=v1.0.0" | oc apply -f -; }
$ oc apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/master/samples/bookinfo/gateway-api/bookinfo-gateway.yaml
$ export INGRESS_HOST=$(oc get -n bookinfo gtw bookinfo-gateway -o jsonpath='{.status.addresses[0].value}')
$ export INGRESS_PORT=$(oc get -n bookinfo gtw bookinfo-gateway -o jsonpath='{.spec.listeners[?(@.name=="http")].port}')
$ export GATEWAY_URL=$INGRESS_HOST:$INGRESS_PORT
$ echo "http://${GATEWAY_URL}/productpage"
```

4. Deploy a waypoint proxy and use it to handle all service traffic in the `bookinfo` namespace:

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

5. Enroll the `bookinfo` namespace to use the waypoint:

```sh
$ oc label namespace bookinfo istio.io/use-waypoint=waypoint
```

6. Send traffic to the `productpage` service for generating traces:

```sh
$ curl "http://${GATEWAY_URL}/productpage" | grep "<title>"
```

Validate the bookinfo application traces in a Tempo dashboard UI. You can find the dashboard UI route by running the following command:

```sh
$ oc get routes -n tempo tempo-sample-query-frontend
```

Select the `bookinfo-gateway-istio.booinfo` or the `waypoint.bookinfo` service from the dashboard UI and then click `Find Traces`.

NOTE: The OpenShift route for Tempo dashboard UI can be created from the TempoStack custom resource with .spec.template.queryFrontend.jaegerQuery.ingress.type: route.

