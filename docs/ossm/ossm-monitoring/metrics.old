# OSSM Operator Metrics

This document describes the custom telemetry metrics for the OSSM operator and CRD usage. It aims to provide a list of those metrics that are collected and exposed by the operator.

The following section outlines the usage and limitations on metric counts and cardinality. The last section provides a development guide about how to add additional metrics for new functionalities.

## Standard Controller-runtime Metrics

The `controller-runtime` library automatically builds and exports a standardized set of metrics. Those metrics are broken down into reconciler loop performance, work queue mechanics, webhook interactions, and runtime behavior. See more details in the [Default exported metrics references](https://book.kubebuilder.io/reference/metrics-reference).

**Note:** These standard metrics are not sent via Telemetry to the [in-cluster monitoring stack](https://rhobs-handbook.netlify.app/products/openshiftmonitoring/telemetry.md/#in-cluster-monitoring-stack).

## OSSM Operator Custom Metrics List

*Limitations*:
- The metrics with the `app.kubernetes.io/version` label only record OSSM 3.x versions to keep cardinality low.

### ossm_3_istiod_total

Total number of Istio control planes at each Istio version. The label `app.kubernetes.io/version` helps filter the number at each OSSM 3.x version.

*Cardinality*: It equals the total number of OSSM 3.x releases.

### ossm_3_envoy_proxy_total

Total number of Envoy Proxies managed by an Istio control plane. 

*Limitation*:
- This metric is counted from an Istiod control plane's `pilot_xds` connections. It includes sidecar proxies, gateways, waypoint proxies, and ztunnel proxies. A fine-grained filter requires changes to the upstream Istio metric code.

*Cardinality*: 1

### ossm_3_sidecar_namespace_total

Total number of namespaces enrolled in Istio sidecar mode.

*Cardinality*: 1

### ossm_3_ztunnel_total

Total number of ZTunnel Proxies managed by an Istio control plane in Ambient mode. The label `app.kubernetes.io/version` helps filtering the number at each OSSM 3.x version.

*Limitation*:
- This metric is counted from the `ZTunnel` custom resource creation using the OSSM operator. A fine-grained filter of counting an Istio control plane's `pilot_xds` connections will be revisited in a later phase.

*Cardinality*: It equals the total number of OSSM 3.x releases.

### ossm_3_ambient_namespace_total

Total number of namespaces enrolled in Istio Ambient mode.

*Cardinality*: 1

## Next Phase metric implementation Plan

The following Multi-cluster mesh and external VM custom metrics will be implemented in those features' tech preview releases.

### ossm_3_multicluster_mesh_cluster_total

Total number of OpenShift clusters involved in a multi-cluster mesh.

*Cardinality*: 1

### ossm_3_multicluster_mesh_primary_total

Total number of OpenShift clusters involved in a multi-cluster mesh primary role.

*Cardinality*: 1

### ossm_3_multicluster_mesh_remote_total

Total number of OpenShift clusters involved in a multi-cluster mesh remote role.

*Cardinality*: 1

### ossm_3_external_vm_total

Total number of external virtual machines and OpenShift `VirtualMachine` workloads managed by an Istio control plane.

*Cardinality*: 1

## Developing New Metrics

This section describes the development process for continually adding custom metrics when new features are added in the operator.

### Define and Register Metrics

Developers can add new custom Prometheus metrics to the operator by defining them with the Prometheus client library and registering them through the [controller-runtime metrics registry](https://book.kubebuilder.io/reference/metrics.html). A separate metrics file or package should be created for adding custom metric declarations. The standard Prometheus metric types and data format should be mentioned in a design doc. 

When it comes to aggregating metrics using the Red Hat Telemetry system, each metric type requires specific functions to prevent data distortion or inaccurate math. Developers should follow Prometheus documentation about metric aggregation behaviors and rules.

### Record Metrics in a Reconcile Loop

The metric methods such as `.Inc()`, `Add()`, or `.Observe()` should be called inside a controller's `Reconcile` function.

### Verify and Expose Metrics

Ensure the operator has granted permissions and created RBAC resources so that Prometheus can scrape the metric endpoint such as `https://servicemesh-operator3-metrics-service.openshift-operators.svc.cluster.local:8443/metrics`.

Recording rules are required to reduce the cardinality of the metrics being shipped. Developers should follow the [monitoring group handbook](https://rhobs-handbook.netlify.app/products/openshiftmonitoring/telemetry.md/). The `PrometheusRule`, `ServiceMonitor` and/or `PodMonitor` objects should be managed by the operator.

Verify the metrics being shipped on an OpenShift in-cluster monitoring stack. The Prometheus pods are running in the `openshift-monitoring` namespace. New metrics should be collected in the in-cluster Prometheus.
