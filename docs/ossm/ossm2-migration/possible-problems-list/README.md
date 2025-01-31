# OpenShift Service Mesh 2 --> 3 Migration risks and recommendations
Given the nature of the migration (not upgrading the operator but migrating to brand new operator), there are a few problems which can't be handled by any of the OpenShift Service Mesh 2 or OpenShift Service Mesh 3 operators. We are listing those problems and recommendations here so users can prepare in advance.

## Recommendations
Following are recommendations to keep the risk of misconfigurations or possible conflicts to minimum.

1. Do not upgrade OpenShift Service Mesh 2 operator or control planes in the middle of the migration

    After the OpenShift Service Mesh 3 operator is installed and migration of data plane is in progress, it's not recommended to upgrade OpenShift Service Mesh 2 operator or control planes. This can be achieved by switching from `Automatic` Operator Update approval to `Manual`.
1. Keep the amount of the service mesh configuration changes to minimum during the migration

    It's not recommended to change Istio resources for traffic management or security or add new workloads to the service mesh in the middle of the migration.
1. Finish the migration without unnecessary delays

    To easier follow the recommendations above, when started the migration should be finished as soon as possible without unnecessary delays.
