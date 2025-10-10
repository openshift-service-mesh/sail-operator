# Coexistence of ambient and sidecar mode in the same mesh (Dev Preview)

The OpenShift Container Platform (OCP) 3.2 supports running sidecar pods and ambient pods together within the same mesh. 
While basic connectivity and Layer 4 (L4) features work seamlessly between sidecar and ambient workloads, there is a
significant limitation for Layer 7 (L7) capabilities in the hybrid model.

## Requirements for Coexistence

- You have deployed Istio with `ambient` profile or have migrated your existing OSSM deployment to OSSM 3.2 with ambient profile.
- For a sidecar proxy to communicate with a workload in ambient mode, it must be ambient-aware. This requires the sidecar to support
  `HBONE` (HTTP-Based Overlay Network Environment) connections, which older versions do not. The feature is controlled by the
  `ISTIO_META_ENABLE_HBONE` proxy metadata setting, applied by Istiod during injection and enabled by default in the `ambient` profile.
  After upgrading the control plane, any existing pods must be restarted so they are reinjected with the updated, HBONE-aware configuration.

## Supported Use Cases in Coexistence Mode

The following table describes the functionality that is expected to work when running sidecar and ambient modes together:

| Category | Functionality | Description |
|----------|---------------|-------------|
| **Basic connectivity** | East-west communication | Pods operating in sidecar mode can communicate with those in ambient mode, and vice versa. |
| **L4 Security** | mTLS | When a sidecar proxy detects that the destination is an HBONE endpoint, it automatically uses the HBONE protocol. Similarly, when a pod runs in ambient mode, its source ztunnel uses HBONE to communicate with the destination's sidecar proxy. |
| **L4 Authentication/Authorization** | Layer 4 policies | Layer 4 (L4) authentication and authorization policies remain supported in coexistence mode. A `PeerAuthentication` policy with mTLS mode set to `STRICT` allows traffic from pods running in either sidecar or ambient mode. |
| **Gateways** | Ingress and Egress Gateways | Pods operating in ambient mode interoperate with Istio egress gateways. Ingress gateways deployed in non-ambient namespaces can expose services hosted in both ambient and sidecar modes. |
| **L4 Observability** | Basic telemetry | Only basic L4 TCP metrics are supported. |

> **_NOTE:_** By default, traffic from ingress gateways to ambient services does not traverse waypoints unless the `istio.io/ingress-use-waypoint: "true"` label is applied.

## Limitations and Unsupported Use Cases

The major limitations are around L7 features (like L7 based traffic routing, L7 auth policies and L7 telemetry) when traffic has to traverse between the two dataplane modes.

### Layer 7 Policy Enforcement

Currently, sidecar proxies cannot communicate with waypoint proxies. As a result, when a sidecar pod sends traffic to
an ambient pod, the traffic bypasses the Ambient waypoint, preventing Layer 7 (L7) policy enforcement. This means:

- L7 authorization policies do not apply
- Only Layer 4 (L4) authorization policies can be used
- All routing decisions are made by the client-side sidecar rather than the waypoint proxy

## Best Practices

When running sidecar and ambient modes together, follow these recommendations:

- **Use separate namespaces**: Preferably use separate namespaces for ambient and sidecar pods to avoid configuration issues.
- **Avoid mixed labeling**: Avoid labeling pods/namespaces with both sidecar and ambient injection labels (although sidecar takes preference if both are present).
