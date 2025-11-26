# OSSM 2 Federation to OSSM 3 Multi-Cluster Migration Guide

## Introduction

This document provides instructions for migrating from OpenShift Service Mesh 2.6 federation to OpenShift Service Mesh 3.x multi-cluster architecture.

The key differences between the two approaches are:

- **OSSM 2.6 Federation**: Used `ServiceMeshPeer`, `ExportedServiceSet`, and `ImportedServiceSet` resources to establish mesh federation, and terminated mTLS at egress and ingress gateways for cross-network traffic.
- **OSSM 3.x Multi-cluster**: Uses `Gateway`, `ServiceEntry` and `WorkloadEntry` to manage exported and imported services.

This guide will help you transition from the federation model to the multi-cluster model with minimal disruption to your services.

## Prerequisites

Before beginning the migration, ensure you have:

- Two or more OpenShift clusters with both OSSM 2.6 and OSSM 3.x installed side-by-side
  - See [Running OSSM 2 and OSSM 3 side by side](../../ossm-2-and-ossm-3-side-by-side/README.md) for installation instructions
- Cluster admin access to all clusters
- Network connectivity between clusters for cross-cluster communication
