# Federation-to-Multi-Cluster Migration Guide

## Introduction

This document provides instructions for migrating from federation in OpenShift Service Mesh 2 to multi-cluster with **manual service discovery** in OpenShift Service Mesh 3.

> [!NOTE]
> While `federation` is a type of multi-cluster topology, we avoid using this term in Service Mesh 3 to prevent confusion with the Federation feature in OSSM 2, which relied on custom resources that are no longer available in OSSM 3.

The key differences between the two approaches are:

- **OSSM 2 Federation**:
  - relies on custom resources (`ServiceMeshPeer`, `ExportedServiceSet`, and `ImportedServiceSet`) to establish mesh federation
  - automates exporting and importing services using flexible matching and aliasing rules
  - terminates cross-network mTLS at egress and ingress gateways
  - does not support end-to-end client-server authentication and authorization due to mTLS termination at gateways and gateway-based identity impersonation
  - allows exporting a service to a specific mesh
- **OSSM 3 Multi-cluster (with manual service discovery)**:
  - relies on basic Istio resources to export and import services (`Gateway`, `ServiceEntry`, and `WorkloadEntry`)
  - requires manual effort to configure imported and exported services
  - does not require mTLS termination at gateways
  - fully supports end-to-end client-server authentication and authorization
  - exports services to all meshes with network access to the exported service

This guide will help you transition from the federation model to the multi-cluster model with minimal disruption to your services.

## Why manual service discovery?

Automatic service discovery is supported in Istio multi-primary deployments.
However, this guide focuses on scenarios where meshes do not maintain identical namespace and service definitions across clusters.

In a multi-primary setup using automatic service discovery, a service is visible across clusters only if the same namespace and service exist locally.
For example, if cluster A exposes a service `foo` in namespace `bar`, clients in cluster B can access it only if cluster B also defines a local service `foo` in namespace `bar`.
This local service effectively acts as a "stub" that Istio uses to route traffic to the remote endpoints.

Manual service discovery removes the above constraints. Client clusters are not required to mirror the server cluster’s namespace or service configuration.
Instead, remote services are explicitly modeled using `ServiceEntry` and `WorkloadEntry` resources, providing full control over how services are named, addressed, and discovered.
This approach is particularly useful when:
- clusters use different namespace naming conventions
- you want to avoid creating placeholder namespaces and services
- services must be imported under different names or namespaces than their source
- fine-grained control is required over which services are exposed and to which consumers

## Prerequisites

Before beginning the migration, ensure you have:

- Two OpenShift 4.19+ clusters with cluster-admin access. OpenShift 4.19 and newer versions include built-in support for the Kubernetes Gateway API, which is required for this migration.
- Network connectivity between clusters for cross-cluster communication
- OSSM 2.6.13+ and OSSM 3.2.1+ operators installed in both clusters

### Environment Setup

For this demo, we'll use two clusters referred to as "East" and "West". You'll need to set up environment variables pointing to the kubeconfig files for each cluster.

1. Export paths to directories with kubeconfigs for each cluster:

   ```bash
   export EAST_AUTH_PATH=/path/to/east
   export WEST_AUTH_PATH=/path/to/west
   ```

   Ensure that a file named `kubeconfig` exists at both `$EAST_AUTH_PATH` and `$WEST_AUTH_PATH` locations with the appropriate cluster credentials.

1. Set up command aliases for easier cluster access:

   ```bash
   alias keast="KUBECONFIG=$EAST_AUTH_PATH/kubeconfig kubectl"
   alias ieast="istioctl --kubeconfig=$EAST_AUTH_PATH/kubeconfig"
   alias kwest="KUBECONFIG=$WEST_AUTH_PATH/kubeconfig kubectl"
   alias iwest="istioctl --kubeconfig=$WEST_AUTH_PATH/kubeconfig"
   ```

1. Verify connectivity to both clusters:

   ```bash
   keast get nodes
   kwest get nodes
   ```

### CA certificates

For the purpose of this demo, we created **different** root and intermediate CAs for each mesh.

1. Create the certificate secrets in both clusters:

   ```bash
   # east
   EAST_CERT_DIR=docs/ossm/ossm2-migration/federation/east
   keast create namespace istio-system
   keast create secret generic cacerts -n istio-system \
     --from-file=root-cert.pem=$EAST_CERT_DIR/root-cert.pem \
     --from-file=ca-cert.pem=$EAST_CERT_DIR/ca-cert.pem \
     --from-file=ca-key.pem=$EAST_CERT_DIR/ca-key.pem \
     --from-file=cert-chain.pem=$EAST_CERT_DIR/cert-chain.pem
   # west
   WEST_CERT_DIR=docs/ossm/ossm2-migration/federation/west
   kwest create namespace istio-system
   kwest create secret generic cacerts -n istio-system \
     --from-file=root-cert.pem=$WEST_CERT_DIR/root-cert.pem \
     --from-file=ca-cert.pem=$WEST_CERT_DIR/ca-cert.pem \
     --from-file=ca-key.pem=$WEST_CERT_DIR/ca-key.pem \
     --from-file=cert-chain.pem=$WEST_CERT_DIR/cert-chain.pem
   ```

### Install Service Mesh 2

1. Deploy control planes and federation gateways:

   ```bash
   keast apply -f - <<EOF
   apiVersion: maistra.io/v2
   kind: ServiceMeshControlPlane
   metadata:
     name: basic
     namespace: istio-system
   spec:
     mode: ClusterWide
     version: v2.6
     addons:
       grafana:
         enabled: false
       kiali:
         enabled: false
       prometheus:
         enabled: false
     tracing:
       type: None
     general:
       logging:
         componentLevels:
           default: info
     proxy:
       accessLogging:
         file:
           name: /dev/stdout
     security:
       identity:
         type: ThirdParty
       trust:
         domain: east.local
     gateways:
       ingress:
         enabled: false
       egress:
         enabled: false
       additionalEgress:
         federation-egress:
           enabled: true
           requestedNetworkView:
           - network-west-mesh
           service:
             metadata:
               labels:
                 federation.maistra.io/egress-for: west-mesh
             ports:
             - port: 15443
               name: tls
             - port: 8188
               name: http-discovery
       additionalIngress:
         federation-ingress:
           enabled: true
           service:
             type: LoadBalancer
             metadata:
               labels:
                 federation.maistra.io/ingress-for: east-mesh
             ports:
             - port: 15443
               name: tls
             - port: 8188
               name: https-discovery
   EOF
   ```
   ```bash
   kwest apply -f - <<EOF
   apiVersion: maistra.io/v2
   kind: ServiceMeshControlPlane
   metadata:
     name: basic
     namespace: istio-system
   spec:
     mode: ClusterWide
     version: v2.6
     addons:
       grafana:
         enabled: false
       kiali:
         enabled: false
       prometheus:
         enabled: false
     tracing:
       type: None
     general:
       logging:
         componentLevels:
           default: info
     proxy:
       accessLogging:
         file:
           name: /dev/stdout
     security:
       identity:
         type: ThirdParty
       trust:
         domain: west.local
     gateways:
       ingress:
         enabled: false
       egress:
         enabled: false
       additionalEgress:
         federation-egress:
           enabled: true
           requestedNetworkView:
           - network-east-mesh
           service:
             metadata:
               labels:
                 federation.maistra.io/egress-for: east-mesh
             ports:
             - port: 15443
               name: tls
             - port: 8188
               name: http-discovery
       additionalIngress:
         federation-ingress:
           enabled: true
           service:
             type: LoadBalancer
             metadata:
               labels:
                 federation.maistra.io/ingress-for: west-mesh
             ports:
             - port: 15443
               name: tls
             - port: 8188
               name: https-discovery
   EOF
   ```

1. Verify the installation:

   ```shell
   # east
   keast get smcp -n istio-system
   keast get pods -n istio-system -l federation.maistra.io/egress-for=west-mesh
   keast get pods -n istio-system -l federation.maistra.io/ingress-for=east-mesh
   # west
   kwest get smcp -n istio-system
   kwest get pods -n istio-system -l federation.maistra.io/egress-for=east-mesh
   kwest get pods -n istio-system -l federation.maistra.io/ingress-for=west-mesh
   ```

### Configure mesh federation

1. In cluster `west`:

   ```shell
   kwest create cm east-mesh-ca-root-cert -n istio-system \
     --from-file=root-cert.pem=$EAST_CERT_DIR/root-cert.pem
   ```
   ```shell
   EAST_INGRESS_IP=$(keast get svc federation-ingress -n istio-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
   ```
   ```shell
   kwest apply -f - <<EOF
   apiVersion: federation.maistra.io/v1
   kind: ServiceMeshPeer
   metadata:
     name: east-mesh
     namespace: istio-system
   spec:
     remote:
       addresses:
       - "$EAST_INGRESS_IP"
       discoveryPort: 8188
       servicePort: 15443
     gateways:
       ingress:
         name: federation-ingress
       egress:
         name: federation-egress
     security:
       trustDomain: east.local
       clientID: east.local/ns/istio-system/sa/federation-egress-service-account
       certificateChain:
         kind: ConfigMap
         name: east-mesh-ca-root-cert
   EOF
   ```

1. In cluster `east`:

   ```shell
   keast create cm west-mesh-ca-root-cert -n istio-system \
     --from-file=root-cert.pem=$WEST_CERT_DIR/root-cert.pem
   ```
   ```shell
   WEST_INGRESS_IP=$(kwest get svc federation-ingress -n istio-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
   ```
   ```shell
   keast apply -f - <<EOF
   apiVersion: federation.maistra.io/v1
   kind: ServiceMeshPeer
   metadata:
     name: west-mesh
     namespace: istio-system
   spec:
     remote:
       addresses:
       - "$WEST_INGRESS_IP"
       discoveryPort: 8188
       servicePort: 15443
     gateways:
       ingress:
         name: federation-ingress
       egress:
         name: federation-egress
     security:
       trustDomain: west.local
       clientID: west.local/ns/istio-system/sa/federation-egress-service-account
       certificateChain:
         kind: ConfigMap
         name: west-mesh-ca-root-cert
   EOF
   ```

1. Verify the ServiceMeshPeer status:

   ```shell
   keast get servicemeshpeer west-mesh -n istio-system -o jsonpath='{.status}' | jq
   kwest get servicemeshpeer east-mesh -n istio-system -o jsonpath='{.status}' | jq
   ```

### Export and import services

1. Deploy applications:

   ```shell
   # east
   keast create ns client
   keast label ns client istio-injection=enabled
   keast apply -f https://raw.githubusercontent.com/istio/istio/master/samples/curl/curl.yaml -n client
   keast patch deploy curl -n client -p '{"spec":{"template":{"metadata":{"annotations":{"sidecar.istio.io/inject":"true"}}}}}'
   # west
   kwest create ns a
   kwest label ns a istio-injection=enabled
   kwest apply -f https://raw.githubusercontent.com/istio/istio/master/samples/httpbin/httpbin.yaml -n a
   kwest patch deploy httpbin -n a -p '{"spec":{"template":{"metadata":{"annotations":{"sidecar.istio.io/inject":"true"}}}}}'
   kwest create ns b
   kwest label ns b istio-injection=enabled
   kwest apply -f https://raw.githubusercontent.com/istio/istio/master/samples/httpbin/httpbin.yaml -n b
   kwest patch deploy httpbin -n b -p '{"spec":{"template":{"metadata":{"annotations":{"sidecar.istio.io/inject":"true"}}}}}'
   ```

1. Export services from west to east mesh:

   ```shell
   kwest apply -f - <<EOF
   apiVersion: federation.maistra.io/v1
   kind: ExportedServiceSet
   metadata:
     name: east-mesh
     namespace: istio-system
   spec:
     exportRules:
     - type: NameSelector
       nameSelector:
         namespace: a
         name: httpbin
     - type: NameSelector
       nameSelector:
         namespace: b
         name: httpbin
   EOF
   ```

1. Import services from west to east mesh:

   ```shell
   keast apply -f - <<EOF
   apiVersion: federation.maistra.io/v1
   kind: ImportedServiceSet
   metadata:
     name: west-mesh
     namespace: istio-system
   spec:
     importRules:
     - type: NameSelector
       nameSelector:
         namespace: a
     - type: NameSelector
       nameSelector:
         namespace: b
         alias:
           namespace: b
       importAsLocal: true
   EOF
   ```

1. Verify exported and imported services and wait until you see the following statuses:

    ```shell
    kwest get exportedserviceset east-mesh -n istio-system -o jsonpath='{.status}' | jq
    ```
    ```json
    {
      "exportedServices": [
        {
          "exportedName": "httpbin.a.svc.east-mesh-exports.local",
          "localService": {
            "hostname": "httpbin.a.svc.cluster.local",
            "name": "httpbin",
            "namespace": "a"
          }
        },
        {
          "exportedName": "httpbin.b.svc.east-mesh-exports.local",
          "localService": {
            "hostname": "httpbin.b.svc.cluster.local",
            "name": "httpbin",
            "namespace": "b"
          }
        }
      ]
    }
    ```
    ```shell
    keast get importedserviceset west-mesh -n istio-system -o jsonpath='{.status}' | jq
    ```
    ```json
    {
      "importedServices": [
        {
          "exportedName": "httpbin.a.svc.east-mesh-exports.local",
          "localService": {
            "hostname": "httpbin.a.svc.west-mesh-imports.local",
            "name": "httpbin",
            "namespace": "a"
          }
        },
        {
          "exportedName": "httpbin.b.svc.east-mesh-exports.local",
          "localService": {
            "hostname": "httpbin.b.svc.cluster.local",
            "name": "httpbin",
            "namespace": "b"
          }
        }
      ]
    }
    ```

1. Verify connectivity:

    ```shell
    keast exec -n client deploy/curl -c curl -- curl -v httpbin.a.svc.west-mesh-imports.local:8000/headers
    keast exec -n client deploy/curl -c curl -- curl -v httpbin.b.svc.cluster.local:8000/headers
    ```

### Migration steps

#### Configure control plane v2.6

To prepare the mesh for disabling the federation feature, configure the following properties that enable cross-cluster communication without relying on federation-specific resources:

- `techPreview.meshConfig.trustDomainAliases` - allows the mesh to accept identities from a different trust domain.
- `PILOT_MULTI_NETWORK_DISCOVER_GATEWAY_API` - enables Istiod to automatically discover gateway addresses using the Kubernetes Gateway API. This simplifies managing WorkloadEntries for federated services through automatic address injection. When a gateway address changes, updating the Gateway resource automatically propagates the change to all related WorkloadEntries.
- `cluster.network` - defines the local network name. This is required for `istio-remote` Gateway to work correctly.
- `security.manageNetworkPolicy` - must be disabled to allow direct communication to the new east-west gateway, bypassing the federation egress gateway.

1. Patch the control planes in both clusters:

   ```shell
   keast patch smcp basic -n istio-system --type=merge -p '{
      "spec": {
        "techPreview": {
          "meshConfig": {
            "trustDomainAliases": ["west.local"]
          }
        },
        "runtime": {
          "components": {
            "pilot": {
              "container": {
                "env": {
                  "PILOT_MULTI_NETWORK_DISCOVER_GATEWAY_API": "true"
                }
              }
            }
          }
        },
        "cluster": {
          "network": "network-east-mesh"
        },
        "security": {
          "manageNetworkPolicy": false
        }
      }
    }'
    ```
    ```shell
    kwest patch smcp basic -n istio-system --type=merge -p '{
      "spec": {
        "techPreview": {
          "meshConfig": {
            "trustDomainAliases": ["east.local"]
          }
        },
        "runtime": {
          "components": {
            "pilot": {
              "container": {
                "env": {
                  "PILOT_MULTI_NETWORK_DISCOVER_GATEWAY_API": "true"
                }
              }
            }
          }
        },
        "cluster": {
          "network": "network-west-mesh"
        },
        "security": {
          "manageNetworkPolicy": false
        }
      }
    }'
    ```

#### Export services

1. Deploy east-west gateway and expose exported services:

   ```shell
   kwest create ns istio-ingress
   kwest apply -n istio-ingress -f - <<EOF
   apiVersion: gateway.networking.k8s.io/v1
   kind: Gateway
   metadata:
     name: eastwestgateway
     labels:
       topology.istio.io/network: network-west-mesh
   spec:
     gatewayClassName: istio
     listeners:
     - name: cross-network
       hostname: fake-hostname-to-block-all-by-default
       port: 15443
       protocol: TLS
       tls:
         mode: Passthrough
         options:
           gateway.istio.io/listener-protocol: auto-passthrough
   ---
   apiVersion: networking.istio.io/v1alpha3
   kind: Gateway
   metadata:
     name: expose-services
   spec:
     selector:
       istio.io/gateway-name: eastwestgateway
     servers:
     - port:
         number: 15443
         name: tls
         protocol: TLS
       tls:
         mode: AUTO_PASSTHROUGH
       hosts:
         - "httpbin.a.svc.west-mesh-imports.local"
         - "httpbin.b.svc.cluster.local"
   EOF
   ```

> [!NOTE]
> Two gateway configurations are applied here:
>
> 1. A **Kubernetes Gateway** leverages Istio's deployment controller, eliminating the need to manually manage the underlying `Service`, `Deployment`, and related resources.
> 2. An **Istio Gateway** selectively exposes specific services.
>
> The hostname `fake-hostname-to-block-all-by-default` is a placeholder that matches no real FQDN in the cluster. This workaround is necessary because the Kubernetes Gateway API only supports a single hostname per listener.
> If you want to export all local services, you can omit the Istio Gateway and configure the Kubernetes Gateway with `hostname: "*"`.

1. Create ServiceEntry for each service that was imported **without** `importAsLocal` in other clusters:

   ```shell
   kwest apply -f - <<EOF
   apiVersion: networking.istio.io/v1beta1
   kind: ServiceEntry
   metadata:
     name: httpbin-a-svc-east-mesh-exports-local
     namespace: a
   spec:
     hosts:
     - httpbin.a.svc.west-mesh-imports.local
     location: MESH_INTERNAL
     ports:
     - number: 8000
       name: http
       protocol: HTTP
       targetPort: 8080
     resolution: STATIC
     workloadSelector:
       labels:
         app: httpbin
   EOF
   ```

#### Import services

1. Create a remote Gateway for each remote network:

   ```shell
   WEST_REMOTE_IP=$(kwest get svc eastwestgateway-istio -n istio-ingress -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
   keast apply -f - <<EOF
   apiVersion: gateway.networking.k8s.io/v1beta1
   kind: Gateway
   metadata:
     name: remote-gateway-network-west-mesh
     namespace: istio-system
     labels:
       topology.istio.io/network: west-network
   spec:
     gatewayClassName: istio-remote
     addresses:
     - value: "$WEST_REMOTE_IP"
     listeners:
     - name: cross-network
       port: 15443
       protocol: TLS
       tls:
         mode: Passthrough
         options:
           gateway.istio.io/listener-protocol: auto-passthrough
   EOF
   ```

   The `istio-remote` gateway is used by Istio to automatically populate WorkloadEntry addresses that point to the network managed by the gateway.

> [!IMPORTANT]
> The `istio-remote` gateway must use a network name (`topology.istio.io/network` label) that is different from the one used by the remote `federation-ingress` gateway. In this example, we use `west-network` instead of `network-west-mesh`. If the same network name is used, the `federation-ingress` gateway address will be overwritten by the `istio-remote` gateway address in Istio's internal registry. This will break communication with imported services while ServiceEntries and WorkloadEntries are not yet configured, as traffic would be routed to the wrong gateway.
>
> Note that network names do not need to match between clusters. For example, locally we use `west-network`, while in the west cluster it is `network-west-mesh`. This is acceptable in manual service discovery because the network name only needs to be different from the local one to avoid routing traffic from a local client to a local service via the remote gateway.

1. Create a ServiceEntry and WorkloadEntry for each imported service:

   ```shell
   keast apply -f - <<EOF
   apiVersion: networking.istio.io/v1beta1
   kind: ServiceEntry
   metadata:
     name: httpbin-b-svc-cluster-local
     namespace: istio-system
   spec:
     exportTo:
     - client
     hosts:
     - httpbin.b.svc.cluster.local
     location: MESH_INTERNAL
     ports:
     - number: 8000
       name: http
       protocol: HTTP
     resolution: STATIC
     subjectAltNames:
     - "spiffe://west.local/ns/b/sa/httpbin"
     workloadSelector:
       labels:
         app: httpbin-cluster-local
   ---
   apiVersion: networking.istio.io/v1beta1
   kind: WorkloadEntry
   metadata:
     name: httpbin-cluster-local-west-mesh
     namespace: istio-system
     labels:
       app: httpbin-cluster-local
       security.istio.io/tlsMode: istio
   spec:
     network: west-network
   EOF
   ```
   
   ```shell
   keast apply -f - <<EOF
   apiVersion: networking.istio.io/v1beta1
   kind: ServiceEntry
   metadata:
     name: httpbin-west-mesh-imports
     namespace: istio-system
   spec:
     exportTo:
     - client
     hosts:
     - httpbin.a.svc.west-mesh-imports.local
     location: MESH_INTERNAL
     ports:
     - number: 8000
       name: http
       protocol: HTTP
     resolution: STATIC
     subjectAltNames:
     - "spiffe://west.local/ns/a/sa/httpbin"
     workloadSelector:
       labels:
         app: httpbin-west-imports
   ---
   apiVersion: networking.istio.io/v1beta1
   kind: WorkloadEntry
   metadata:
     name: httpbin-west-mesh-imports
     namespace: istio-system
     labels:
       app: httpbin-west-imports
       security.istio.io/tlsMode: istio
   spec:
     network: west-network
   EOF
   ```

> [!NOTE]
> - `security.istio.io/tlsMode: istio` enforces Istio mTLS for the endpoint specified by the WorkloadEntry
> - `subjectAltNames` specifies the expected service identity
> - `network` must match `topology.istio.io/network` specified in the `istio-remote` gateway to ensure the correct address is assigned to the endpoint.

> [!IMPORTANT]
> During migration, endpoints from ServiceEntries may not be immediately pushed to the client proxy due to conflicts with existing entries in Istio's internal service registry.
> To verify that endpoints have been configured, run:
>
> ```shell
> ieast pc endpoints deploy/curl -n client | grep httpbin
> ```
>
> If no endpoints are shown, restart the client pod to trigger endpoint synchronization:
>
> ```shell
> keast rollout restart deploy/curl -n client
> ```

#### Verification steps

1. Verify connectivity

   ```shell
   keast exec -n client deploy/curl -c curl -- curl -v httpbin.a.svc.west-mesh-imports.local:8000/headers
   keast exec -n client deploy/curl -c curl -- curl -v httpbin.b.svc.cluster.local:8000/headers
   ```

1. Remove federation-related resources:

   ```shell
   keast delete importedserviceset west-mesh -n istio-system
   kwest delete exportedserviceset east-mesh -n istio-system
   keast delete servicemeshpeer west-mesh -n istio-system
   kwest delete servicemeshpeer east-mesh -n istio-system
   ```

1. Remove federation gateways:

   ```shell
   keast patch servicemeshcontrolplane basic -n istio-system --type=json -p='[{"op": "remove", "path": "/spec/gateways/additionalIngress"}, {"op": "remove", "path": "/spec/gateways/additionalEgress"}]'
   kwest patch servicemeshcontrolplane basic -n istio-system --type=json -p='[{"op": "remove", "path": "/spec/gateways/additionalIngress"}, {"op": "remove", "path": "/spec/gateways/additionalEgress"}]'
   ```

1. Verify that cross-cluster connectivity still works after removing the federation resources:

   ```shell
   keast exec -n client deploy/curl -c curl -- curl -v httpbin.a.svc.west-mesh-imports.local:8000/headers
   keast exec -n client deploy/curl -c curl -- curl -v httpbin.b.svc.cluster.local:8000/headers
   ```

#### Install OSSM 3 control plane

1. East cluster:

   ```shell
   keast create ns istio-cni
   keast apply -n istio-cni -f - <<EOF
   apiVersion: sailoperator.io/v1
   kind: IstioCNI
   metadata:
     name: default
   spec:
     version: v1.27.3
     namespace: istio-cni
   EOF
   ```
   ```shell
   keast apply -f - <<EOF
   apiVersion: sailoperator.io/v1
   kind: Istio
   metadata:
     name: default
   spec:
     namespace: istio-system
     version: v1.27.3
     updateStrategy:
       type: RevisionBased
     values:
       meshConfig:
         accessLogFile: /dev/stdout
         defaultConfig:
           proxyMetadata:
             ISTIO_META_DNS_CAPTURE: "true"
             PROXY_CONFIG_XDS_AGENT: "true"
         caCertificates:
         # source: west/root-cert.pem
         - pem: |
             -----BEGIN CERTIFICATE-----
             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
             -----END CERTIFICATE-----
         trustDomain: east.local
         trustDomainAliases:
         - west.local
       pilot:
         env:
           ISTIO_MULTIROOT_MESH: "true"
       global:
         network: network-east-mesh
   EOF
   ```

1. West cluster:

   ```shell
   kwest create ns istio-cni
   kwest apply -n istio-cni -f - <<EOF
   apiVersion: sailoperator.io/v1
   kind: IstioCNI
   metadata:
     name: default
   spec:
     version: v1.27.3
     namespace: istio-cni
   EOF
   ```
   ```shell
   kwest apply -f - <<EOF
   apiVersion: sailoperator.io/v1
   kind: Istio
   metadata:
     name: default
   spec:
     namespace: istio-system
     version: v1.27.3
     updateStrategy:
       type: RevisionBased
     values:
       meshConfig:
         accessLogFile: /dev/stdout
         defaultConfig:
           proxyMetadata:
             ISTIO_META_DNS_CAPTURE: "true"
             PROXY_CONFIG_XDS_AGENT: "true"
         caCertificates:
         # source: east/root-cert.pem
         - pem: |
             -----BEGIN CERTIFICATE-----
             MIIFMDCCAxigAwIBAgIUbgLZIA7G4DaJ3GH39KRJtJTRVuMwDQYJKoZIhvcNAQEL
             BQAwMDEXMBUGA1UECgwObXktY29tcGFueS5vcmcxFTATBgNVBAMMDEVhc3QgUm9v
             dCBDQTAeFw0yNjAxMTMxMzU4MzJaFw0zNjAxMTExMzU4MzJaMDAxFzAVBgNVBAoM
             Dm15LWNvbXBhbnkub3JnMRUwEwYDVQQDDAxFYXN0IFJvb3QgQ0EwggIiMA0GCSqG
             SIb3DQEBAQUAA4ICDwAwggIKAoICAQDmDZChjHd/4m/9KcsfwCA4bTY8ueVaRhSs
             fmsjZlBPQ7AfQkrZt/OjklsPjUm9VdCI7E0jHiJ5L5qxg5Aq5CyIyQJ72hYXluRS
             TzHRV4B4UL5VvuFzpTIehdfvDB+Jd8MuX0Z+oBbArbqd//nNqOGPme913hDVnTj0
             vzy/RoCyFeH/zCDYGvGMNAeSrKcsolGP/5ndFh1aloQwygPe/A7TxR+1/DM8LQrQ
             fEzyn6K+3SSmHmB5xZGtdmv2C5S1ph5dbkQh3y6p76Ww+2p225O64NssmC2L5/td
             W6mZBq/ux3CenAyWyJcV+VDlYSCbkQdppzFULR5lM6g6kbQG3VIZv3eHXIs7eJHs
             u7BOP22CBXDXTzUYOu1DThv8/IUrWBZ4k6eABXROpU2za5IrxmMQzJazpJVrwO5r
             bOktrKeF1sYn64l+icyx5DLx7H1hHINac7oOAj+2gmJIFUUYEgihqopSmnzbv+eM
             gBLGqIgqzsyXTYA+Ye1NzRUfMQLsu+T2+kQEETPWFp+3CxH/dpsGT7DRrHBWS1JF
             hWZdCi2ElECM4z07RFE7o+LX813pMPV98UW7m22W3pcPoevWz6VVoEvTKbusKLC3
             VhZdopYpJtgbIHyd8o0FYvR1O+6WCqYLJur4zgFr1Gj3H7DF6kPm//gnR5tF2WNi
             RCrQg3HwwwIDAQABo0IwQDAdBgNVHQ4EFgQUdCPFAsfhZDScug/BqmxDqx90YIYw
             DwYDVR0TAQH/BAUwAwEB/zAOBgNVHQ8BAf8EBAMCAuQwDQYJKoZIhvcNAQELBQAD
             ggIBACpd7oBrSthZv4zCJWONLHRVDgk3etOPMs3g0p8zzxbbr40xKb2aReYj9F2z
             N7MOopqMNYDcs6tSbdJhadQqJzJ8c/DO+SWLos0I55KcDnNhoxJLs/C6lFDmkQiE
             51W8C3wMeXkNuX9eHW/0bWtvKF2RsD3EKxV2WFl+JXpUjLZVEbFbyuWDmbDdcPsD
             RjSxTVIHieLKTSiUQ00L0smK4a7PTwmqYaaF7pO65WNU5aVElI3R3zQxQzA5aQHE
             fqzCL6RDJkbQ+3ttTG5c9731ygZRqmMul7pHml7wk5E2HH9jlG4CmiQCL4M0kUPf
             Lwvv06xiAtcZb9xvKm7nZkXa1YoexKDVyUVf3LpygXBW6hPZA0ovau+krm7jNJRO
             vtgykrRbtXHMKEaIbTbIJbvbbubhiIIZBelaJ9eGb+l6o+8kQxIAiHkjzTdqOqov
             G/J2MrtmPavDyt+t6q52YzPaMJqaVUfi8F+Lf6k7LOWXL4XdTGqb4LcZLeMBFTN0
             OFU6BbmBD4tZCbSGZ/KNOEIs8WI3iltGdNuTJhSyBGshBgWWtS87ENxp3+ZlLeQK
             wOILZCCpVyLDk3xcvPMuXfHaThjAYzIPdW2/m7EiiWPnoPt7OrpahdkZo/3GcPrG
             57lUu9iu52ySo5E7WzjL/l47Ows3NewhXZ99Mj1xyBdvcl5r
             -----END CERTIFICATE-----
         trustDomain: west.local
         trustDomainAliases:
         - east.local
       pilot:
         env:
           ISTIO_MULTIROOT_MESH: "true"
       global:
         network: network-west-mesh
   EOF
   ```

#### Migrate proxies and gateways

1. Update namespace labels and restart deployments to trigger sidecar injection by the new control plane:

    ```shell
    # east
    keast label ns client istio.io/rev=default-v1-27-3 maistra.io/ignore-namespace="true" istio-injection- --overwrite=true
    keast rollout restart deploy/curl -n client
    # west
    kwest label ns a istio.io/rev=default-v1-27-3 maistra.io/ignore-namespace="true" istio-injection- --overwrite=true
    kwest rollout restart deploy/httpbin -n a
    kwest label ns b istio.io/rev=default-v1-27-3 maistra.io/ignore-namespace="true" istio-injection- --overwrite=true
    kwest rollout restart deploy/httpbin -n b
    ```

1. Disable support for Kubernetes Gateway API in SMCP to ensure that the east-west gateway will be managed by the new control plane:

    ```shell
    kwest patch smcp basic -n istio-system --type=merge -p '{
      "spec": {
        "runtime": {
          "components": {
            "pilot": {
              "container": {
                "env": {
                  "PILOT_ENABLE_GATEWAY_API": "false"
                }
              }
            }
          }
        }
      }
    }'
    ```

1. Update labels on `istio-ingress` namespace to trigger gateway reconciliation by the new control plane:

    ```shell
    kwest label ns istio-ingress istio.io/rev=default-v1-27-3 maistra.io/ignore-namespace="true" istio-injection- --overwrite=true
    ```

#### Post-migration steps

1. **Resource cleanup** - once all proxies and gateways are managed by the new control plane, you can delete all OSSM 2-related resources. See [Cleaning up OpenShift Service Mesh 2.6 after migration](../cleaning-2.6/README.md#remove-26-control-planes) for detailed instructions.

1. **Apply authorization policies** - now when you no longer use OSSM 2 Federation, you can apply `AuthorizationPolicy` for fine-grained access control on the server side. It was not possible in OSSM 2 Federation, which terminated TLS at egress and ingress gateways effectively hiding client identities.

   ```shell
   kwest apply -f - <<EOF
   apiVersion: security.istio.io/v1
   kind: AuthorizationPolicy
   metadata:
     name: httpbin-allow-curl-from-east
     namespace: b
   spec:
     selector:
       matchLabels:
         app: httpbin
     action: ALLOW
     rules:
     - from:
       - source:
           principals:
           - "east.local/ns/client/sa/curl"
       to:
       - operation:
           ports:
           - "8080"
           paths:
           - "/headers"
   EOF
   ```

## Caveats

### AuthorizationPolicy limitations with trustDomainAliases

When `trustDomainAliases` is configured, Istio treats identities from aliased trust domains as equivalent to identities from the local trust domain. This means that AuthorizationPolicies cannot distinguish between workloads from different meshes if they share the same namespace and service account name.

For example, if you have two meshes with trust domains `east.local` and `west.local`, and each mesh has a namespace `foo` with a service account `bar`, the following limitation applies:

- An AuthorizationPolicy that allows traffic from `west.local/ns/foo/sa/bar` will **also** allow traffic from `east.local/ns/foo/sa/bar`
- This is because Istio automatically expands the policy to include all trust domain aliases

This behavior is by design in Istio's trust domain aliasing mechanism. If you require strict isolation between identities from different meshes, consider using distinct namespace or service account names across meshes.
