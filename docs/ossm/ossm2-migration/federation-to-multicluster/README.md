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
