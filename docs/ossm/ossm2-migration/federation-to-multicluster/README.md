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

1. Export the kubeconfig paths for both clusters:

   ```bash
   export EAST_AUTH_PATH=/path/to/east/kubeconfig
   export WEST_AUTH_PATH=/path/to/west/kubeconfig
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
   # Verify East cluster
   keast get nodes

   # Verify West cluster
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
