|Status                                  | Authors      | Created    | 
|----------------------------------------|--------------|------------|
|WIP                                     | @yxun        | 2026-08-26 |

# OSSM Operator Usage Metric Analytics

## Overview

While we receive telemetry metrics for each version of the OSSM operator total installations, we do not have any metric for the service mesh CRDs and components usage. For OSSM 3 operator, we should introduce a set of custom metrics that help measure the magnitude of the service mesh usage. 

## Goals

* Define and register custom metrics for the following resources managed by an OSSM operator on a given cluster.
  * Istio version counts (number of Istiod control plane at each Istio version)
  *  Envoy Proxy counts (number of Envoy sidecar proxies managed by an Istiod)
  * ZTunnel Proxy counts (number of ZTunnel proxies managed by an Istiod)
  * Sidecar and Ambient namespace counts
* Record custom metrics in a Reconcile loop
* Expose and send custom metrics to the OpenShift in-cluster monitoring stack
* Verify and ship custom metrics to the Red Hat Telemetry service

## Non-goals

* Registering multi-cluster mesh custom metrics
* Registering external Virtual Machine custom metrics
* Modifying the existing CRDs or APIs
* Aggregating custom metrics data

## Design

A new `analytics` controller will be introduced along with custom metric types. Each type will include members as standard Prometheus metric types. Those custom metric types will be registered in the operator's scheme `init` function. 

The controller will watch `Istio` and `ZTunnel` custom resources creation and increase their counts in the custom metric accordingly. It will watch namespaces with Istio sidecar injection and Ambient mode labels so that it can count the number of namespaces enrolled in either Istio sidecar mode or Ambient mode.

The Envoy Proxy count metric will be collected from an Istiod control plane's `pilot_xds` connections. The controller will fetch an Istiod control plane's log and filter the `pilot_xds` connections. 

A new Role `prometheus-role` will be created and bound to the existing `prometheus-k8s` ServiceAccount in the `openshift-monitoring` namespace. So that the in-cluster monitoring Prometheus can scrape the OSSM operator metric endpoint such as `https://servicemesh-operator3-metrics-service.openshift-operators.svc.cluster.local:8443/metrics`.

The controller will create a `ServiceMonitor` resource for scraping the OSSM operator metric endpoint using port `8443` and path `/metrics`. It will also create a `PrometheusRule` resource for defining recording rules based on the collected metrics.

## Implementation Plan

- [X] Add a custom metric document that outlines the usage, limitation and cardinality
- [X] Add custom metric types in the analytics package
- [X] Implement controller changes for recording metrics and sending them to the OpenShift in-cluster monitoring stack
- [ ] Add integration tests for verifying custom metrics from the operator metric endpoint
- [ ] Define and register multi-cluster mesh custom metrics when the multi-cluster mesh feature is in tech preview
- [ ] Define and register external Virtual Machine custom metrics when the OSSM external VM integration is available

## Change History
