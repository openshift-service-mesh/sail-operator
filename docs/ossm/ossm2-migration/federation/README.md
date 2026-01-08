# OSSM 2 Federation to OSSM 3 Multi-Cluster Migration Guide

## Introduction

This document provides instructions for migrating from OpenShift Service Mesh 2.6 federation to OpenShift Service Mesh 3.x multi-cluster architecture.

> [!NOTE]
> While "federation" is a type of multi-cluster topology in the general Istio context, we avoid using this term in the OSSM 3 project to avoid confusion with the OSSM 2 federation feature, which used specific custom resources that are no longer used in OSSM 3.

The key differences between the two approaches are:

- **OSSM 2.6 Federation**: Used custom resources - `ServiceMeshPeer`, `ExportedServiceSet`, and `ImportedServiceSet` - to establish mesh federation, and terminated mTLS at egress and ingress gateways for cross-network traffic.
- **OSSM 3.x Multi-cluster**: Relies on basic Istio resources - `Gateway`, `ServiceEntry`, and `WorkloadEntry` - to establish federation between clusters and manage exported and imported services. This approach provides a more standardized and simplified configuration model.

This guide will help you transition from the federation model to the multi-cluster model with minimal disruption to your services.

## Prerequisites

Before beginning the migration, ensure you have:

- Two or more OpenShift clusters with both OSSM 2.6 and OSSM 3.x installed side-by-side
  - See [Running OSSM 2 and OSSM 3 side by side](../../ossm-2-and-ossm-3-side-by-side/README.md) for installation instructions
- Cluster admin access to all clusters
- Network connectivity between clusters for cross-cluster communication

## Lab Setup

This section provides step-by-step instructions for setting up a lab environment to demonstrate the migration from OSSM 2.6 federation to OSSM 3.x multi-cluster.

### Environment Setup

For this lab, we'll use two clusters referred to as "East" and "West". You'll need to set up environment variables pointing to the kubeconfig files for each cluster.

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

### Setup Certificates

We create different root and intermediate certificate authorities (CAs) for each mesh, as it was allowed in OSSM 2 federation.

1. Download the certificate generation tools from the Istio repository:

   ```bash
   wget https://raw.githubusercontent.com/istio/istio/release-1.22/tools/certs/common.mk -O common.mk
   wget https://raw.githubusercontent.com/istio/istio/release-1.22/tools/certs/Makefile.selfsigned.mk -O Makefile.selfsigned.mk
   ```

1. Generate certificates for each cluster:

   ```bash
   # east
   make -f Makefile.selfsigned.mk \
     ROOTCA_CN="East Root CA" \
     ROOTCA_ORG=my-company.org \
     root-ca
   make -f Makefile.selfsigned.mk \
     INTERMEDIATE_CN="East Intermediate CA" \
     INTERMEDIATE_ORG=my-company.org \
     east-cacerts
   make -f common.mk clean
   # west
   make -f Makefile.selfsigned.mk \
     ROOTCA_CN="West Root CA" \
     ROOTCA_ORG=my-company.org \
     root-ca
   make -f Makefile.selfsigned.mk \
     INTERMEDIATE_CN="West Intermediate CA" \
     INTERMEDIATE_ORG=my-company.org \
     west-cacerts
   make -f common.mk clean
   ```

1. Create the certificate secrets in both clusters:

   ```bash
   # east
   keast create namespace istio-system
   keast create secret generic cacerts -n istio-system \
     --from-file=root-cert.pem=east/root-cert.pem \
     --from-file=ca-cert.pem=east/ca-cert.pem \
     --from-file=ca-key.pem=east/ca-key.pem \
     --from-file=cert-chain.pem=east/cert-chain.pem
   # west
   kwest create namespace istio-system
   kwest create secret generic cacerts -n istio-system \
     --from-file=root-cert.pem=west/root-cert.pem \
     --from-file=ca-cert.pem=west/ca-cert.pem \
     --from-file=ca-key.pem=west/ca-key.pem \
     --from-file=cert-chain.pem=west/cert-chain.pem
   ```

## Installing OSSM 2.6 Control Planes with Federation

Now we'll install OSSM 2.6 control planes in both clusters with federation ingress and egress gateways configured. This setup allows the two meshes to communicate across clusters.

### Deploy ServiceMeshControlPlane in East Cluster

1. Create the ServiceMeshControlPlane in the east cluster with federation gateways:

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
     techPreview:
       meshConfig:
         trustDomainAliases:
         - west.local
     proxy:
       accessLogging:
         file:
           name: /dev/stdout
     runtime:
       components:
         pilot:
           container:
             env:
               PILOT_MULTI_NETWORK_DISCOVER_GATEWAY_API: "true"
     cluster:
       network: network-east-mesh
     security:
       identity:
         type: ThirdParty
       trust:
         domain: east.local
       manageNetworkPolicy: false
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

### Deploy ServiceMeshControlPlane in West Cluster

1. Create the ServiceMeshControlPlane in the west cluster with federation gateways:

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
     techPreview:
       meshConfig:
         trustDomainAliases:
         - east.local
     proxy:
       accessLogging:
         file:
           name: /dev/stdout
     cluster:
       network: network-west-mesh
     security:
       identity:
         type: ThirdParty
       trust:
         domain: west.local
       manageNetworkPolicy: false
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

### Verify the Installation

1. Verify that the control planes are running in both clusters:

   ```bash
   keast get smcp -n istio-system
   kwest get smcp -n istio-system
   ```

1. Verify that the federation gateways are deployed:

   ```bash
   # east
   keast get pods -n istio-system -l federation.maistra.io/egress-for=west-mesh
   keast get pods -n istio-system -l federation.maistra.io/ingress-for=east-mesh
   # west
   kwest get pods -n istio-system -l federation.maistra.io/egress-for=east-mesh
   kwest get pods -n istio-system -l federation.maistra.io/ingress-for=west-mesh
   ```

### Configure mesh federation

1. In cluster `west`:

   ```shell
   kwest create cm east-mesh-ca-root-cert -n istio-system \
     --from-file=root-cert.pem=east/root-cert.pem
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
     --from-file=root-cert.pem=west/root-cert.pem
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

#### Verify ServiceMeshPeer Status

1. Check the status of the ServiceMeshPeer in the west cluster:

   ```shell
   kwest get servicemeshpeer east-mesh -n istio-system -o jsonpath='{.status}'
   ```

1. Check the status of the ServiceMeshPeer in the east cluster:

   ```shell
   keast get servicemeshpeer west-mesh -n istio-system -o jsonpath='{.status}'
   ```

### Deploy apps

1. Deploy apps in `west` cluster:

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

1. Export services:

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

1. Import services:

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

1. Verify connectivity:

```shell
keast exec -n client deploy/curl -c curl -- curl -v httpbin.a.svc.west-mesh-imports.local:8000/headers
keast exec -n client deploy/curl -c curl -- curl -v httpbin.b.svc.cluster.local:8000/headers
```

### Migration steps

#### Export

1. Create a ServiceEntry for the exported service:

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

1. Deploy custom federation ingress gateway for the exported services:

   ```shell
   kwest apply -f - <<EOF
   apiVersion: gateway.networking.k8s.io/v1
   kind: Gateway
   metadata:
     name: eastwestgateway
     namespace: istio-system
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

#### Import

1. Create remote Gateway:

   ```shell
   WEST_REMOTE_IP=$(kwest get svc eastwestgateway-istio -n istio-system -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
   ```
   ```shell
   keast apply -f - <<EOF
   apiVersion: gateway.networking.k8s.io/v1beta1
   kind: Gateway
   metadata:
     name: remote-gateway-network-west-mesh
     namespace: istio-system
     labels:
       topology.istio.io/network: network-west-mesh
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

1. Create ServiceEntry for imported services:

   ```shell
   keast apply -f - <<EOF
   apiVersion: networking.istio.io/v1beta1
   kind: ServiceEntry
   metadata:
     name: httpbin-b-svc-cluster-local
     namespace: client
   spec:
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
     namespace: client
     labels:
       app: httpbin-cluster-local
       security.istio.io/tlsMode: istio
   spec:
     network: network-west-mesh
   EOF
   ```
   
   ```shell
   keast apply -f - <<EOF
   apiVersion: networking.istio.io/v1beta1
   kind: ServiceEntry
   metadata:
     name: httpbin-west-mesh-imports
     namespace: client
   spec:
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
     namespace: client
     labels:
       app: httpbin-west-imports
       security.istio.io/tlsMode: istio
   spec:
     network: network-west-mesh
   EOF
   ```

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

1. Verify connectivity

   ```shell
   keast exec -n client deploy/curl -c curl -- curl -v httpbin.a.svc.west-mesh-imports.local:8000/headers
   keast exec -n client deploy/curl -c curl -- curl -v httpbin.b.svc.cluster.local:8000/headers
   ```
