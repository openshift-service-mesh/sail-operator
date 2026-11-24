# Rollback limitations

Understand the limitations of rolling back an OpenShift Service Mesh 3.0 migration before you begin migrating your workloads from OpenShift Service Mesh 2.6 to OpenShift Service Mesh 3.0.

> [!IMPORTANT]
> Rolling back from OpenShift Service Mesh 3.0 to OpenShift Service Mesh 2.6 is **not supported** and is not recommended. If you encounter problems during the migration, the recommended approach is to fix the problems and continue forward with the migration. If you get stuck during the migration, create a support case instead of attempting a rollback.

After the OpenShift Service Mesh 3 Operator is installed, all Istio custom resource definitions (CRDs) on the cluster are updated. It is not possible to revert the Istio CRDs to the OpenShift Service Mesh 2.6 versions without reinstalling OpenShift Service Mesh 2.6 from scratch.

If, despite rollback not being supported or recommended, you must roll back workloads that were already migrated to the OpenShift Service Mesh 3.0 control plane, complete the following steps.

> [!IMPORTANT]
> At any given time, exactly one control plane version must inject side cars into workloads. The 2.6 and 3.0 control planes must never inject side cars into workloads at the same time.

## Procedure

1. In the namespace of an already migrated workload, remove the sidecar injection labels for the 3.0 control plane, remove the `maistra.io/ignore-namespace: "true"` label, and re-apply the sidecar injection label for the OpenShift Service Mesh 2.6 control plane. You must apply all of these label changes at the same time by running a single command.
2. Restart the workloads in the namespace so that the OpenShift Service Mesh 2.6 control plane injects the side cars.
3. Repeat steps 1-2 for all other already migrated workloads.
4. After all workloads are managed by the OpenShift Service Mesh 2.6 control plane, remove the OpenShift Service Mesh 3.x control plane.
