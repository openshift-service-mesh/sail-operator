# Using OpenShift Service Mesh Console plugin

The OpenShift Service Mesh Console (OSSMC) plugin extends the OpenShift web console with a **Service Mesh** menu and enhanced tabs for workloads and services.

## About the OpenShift Service Mesh Console plugin

The OpenShift Service Mesh Console (OSSMC) plugin is an extension to the OpenShift web console that provides visibility into your Service Mesh.

[!WARNING]
The OSSMC plugin supports only one Kiali instance, regardless of its project access scope.

The OSSMC plugin provides a new category, **Service Mesh**, in the main OpenShift web console navigation with the following menu options:

* **Overview:** Provides a summary of your mesh, displayed as cards that represent the namespaces in the mesh.

* **Traffic Graph:** Provides a full topology view of your mesh, represented by nodes and edges. Each node represents a component of the mesh and each edge represents traffic flowing through the mesh between components.

* **Istio config:** Provides a list of all Istio configuration files in your mesh, with a column that provides a quick way to know if the configuration for each resource is valid.

* **Mesh:** Provides detailed information about the Istio infrastructure status. It shows an infrastructure topology view with core and add-on components, their health, and how they are connected to each other.

In the web console **Workloads** details page, the OSSMC plugin adds a **Service Mesh** tab that has the following subtabs:

* **Overview:** Shows a summary of the selected workload, including a localized topology graph showing the workload with all inbound and outbound edges and nodes.

* **Traffic:** Shows information about all inbound and outbound traffic to the workload.

* **Logs:** Shows the logs for the workload's containers. You can see container logs individually ordered by log time and how the Envoy sidecar proxy logs relate to your workload's application logs. You can enable the tracing span integration, which allows you to see which logs correspond to trace spans.

* **Metrics:** Shows inbound and outbound metric graphs in the corresponding subtabs. All the workload metrics are here, providing a detailed view of the performance of your workload. You can enable the tracing span integration, which allows you to see which spans occurred at the same time as the metrics. With the span marker in the graph, you can see the specific spans associated with that timeframe.

* **Traces:** Provides a chart showing the trace spans collected over the given timeframe. The trace spans show the most low-level detail within your workload application. The trace details further show heatmaps that provide a comparison of one span in relation to other requests and spans in the same timeframe.

* **Envoy:** Shows information about the Envoy sidecar configuration.

In the web console **Networking** details page, the OSSMC plugin adds a **Service Mesh** tab similar to the **Workloads** details page.

In the web console **Projects** details page, the OSSMC plugin adds a **Service Mesh** tab that provides traffic graph information about that project. It is the same information shown in the **Traffic Graph** page but specific to that project.

## Installing the OpenShift Service Mesh Console plugin

You can install the OSSMC plugin with the Kiali Operator by creating a `OSSMConsole` resource with the corresponding plugin settings. It is recommended to install the latest version of the Kiali Operator, even while installing a previous OSSMC plugin version, as it includes the latest z-stream release.

### OSSM version compatibility

| OSSM Version    | Kiali Server Version | OSSMC Plugin Version | OCP Version |
| --------------- | -------------------- | -------------------- | ----------- |
| 3.0             | v2.4                 | v2.4                 | 4.15+       |
| 2.6             | v1.73                | v1.73                | 4.14 - 4.18 |
| 2.5             | v1.73                | v1.73                | 4.14 - 4.18 |

[!NOTE]
The OSSMC plugin is only supported on OpenShift 4.15 and above. For OCP 4.14 users, only the standalone Kiali console is accessible.

You can install the OSSMC plugin by using the OpenShift web console or the OpenShift CLI (`oc`).

### Installing the OSSMC plugin by using the OpenShift web console

You can install the OSSMC plugin by using the OpenShift web console.

**Prerequisites**

* You have the administrator access to the OpenShift web console.
* You have installed the Red Hat OpenShift Service Mesh (OSSM).
* You have installed the `Istio` control plane from OSSM 3.0.
* You have installed the Kiali Server 2.4.

**Procedure**

1. Navigate to **Installed Operators**.

2. Click **Kiali Operator**.

3. Click **Create instance** on the **Red Hat OpenShift Service Mesh Console** tile. You can also click **Create OSSMConsole** button under the **OpenShift Service Mesh Console** tab.

4. Use the **Create OSSMConsole** form to create an instance of the `OSSMConsole` custom resource (CR). **Name** and **Version** are the required fields.

    [!NOTE]
    The **Version** field must match with the `spec.version` field in your Kiali custom resource (CR). If **Version** value is the string `default`, the Kiali Operator installs the OSSMC plugin with the same version as the operator. The `spec.version` field requires the `v` prefix in the version number. The version number must only include the major and minor version numbers (not the patch number); for example: `v1.73`.

5. Click **Create**.

**Verification**

1. Wait until the web console notifies you that the OSSMC plugin is installed and prompts you to refresh.

2. Verify that the **Service Mesh** category is added in the main OpenShift web console navigation.

### Installing the OSSMC plugin by using the CLI

You can install the OSSMC plugin by using the OpenShift CLI (`oc`).

**Prerequisites**

* You have access to the OpenShift CLI on the cluster as an administrator.
* You have installed the Red Hat OpenShift Service Mesh (OSSM).
* You have installed the `Istio` control plane from OSSM 3.0.
* You have installed the Kiali Server 2.4.

**Procedure**

1. Create a `OSSMConsole` custom resource (CR) to install the plugin by running the following command:

    ```yaml
    cat <<EOM | oc apply -f -
    apiVersion: kiali.io/v1alpha1
    kind: OSSMConsole
    metadata:
      namespace: openshift-operators
      name: ossmconsole
    spec:
      version: default
    EOM
    ```

    [!NOTE]
    The OSSMC plugin version must match with the Kiali Server version. If `spec.version` field value is the string `default` or is not specified, the Kiali Operator installs OSSMC with the same version as the operator. The `spec.version` field requires the `v` prefix in the version number. The version number must only include the major and minor version numbers (not the patch number); for example: `v1.73`.

    The plugin resources deploy in the same namespace as the `OSSMConsole` CR.

2. Optional: If more than one Kiali Server is installed in the cluster, specify the `spec.kiali` setting in the OSSMC CR by running a command similar to the following example:

    ```yaml
    cat <<EOM | oc apply -f -
    apiVersion: kiali.io/v1alpha1
    kind: OSSMConsole
    metadata:
      namespace: openshift-operators
      name: ossmconsole
    spec:
      kiali:
        serviceName: kiali
        serviceNamespace: istio-system-two
        servicePort: 20001
    EOM
    ```

**Verification**

1. Go to the OpenShift web console.

2. Verify that the **Service Mesh** category is added in the main OpenShift web console navigation.

3. If the OSSMC plugin is not installed yet, wait until the web console notifies you that the OSSMC plugin is installed and prompts you to refresh.

## Uninstalling the OpenShift Service Mesh Console plugin

You can uninstall the OSSMC plugin by using the OpenShift web console or the OpenShift CLI (`oc`).

You must uninstall the OSSMC plugin before removing the Kiali Operator. Deleting the Operator first may leave OSSMC and Kiali CRs stuck, requiring manual removal of the finalizer. Use the following command with `<custom_resource_type>` as `kiali` or `ossmconsole` to remove the finalizer, if needed:

```console
$ oc patch <custom_resource_type> <custom_resource_name> -n <custom_resource_namespace> -p '{"metadata":{"finalizers": []}}' --type=merge
```

### Uninstalling the OSSMC plugin by using the web console

You can uninstall the OSSMC plugin by using the OpenShift web console.

**Procedure**

1. Navigate to **Installed Operators**.

2. Click **Kiali Operator**.

3. Select the **OpenShift Service Mesh Console** tab.

4. Click **Delete OSSMConsole** option from the entry menu.

5. Confirm that you want to delete the plugin.

### Uninstalling the OSSMC plugin by using the CLI

You can uninstall the OSSMC plugin by using the OpenShift CLI (`oc`).

**Procedure**

1. Remove the OSSMC custom resource (CR) by running the following command:

    ```console
    $ oc delete ossmconsoles <custom_resource_name> -n <custom_resource_namespace>
    ```

**Verification**

1. Verify all the CRs are deleted from all namespaces by running the following command:

    ```console
    $ for r in $(oc get ossmconsoles --ignore-not-found=true --all-namespaces -o custom-columns=NS:.metadata.namespace,N:.metadata.name --no-headers | sed 's/ */:/g'); do oc delete ossmconsoles -n $(echo $r|cut -d: -f1) $(echo $r|cut -d: -f2); done
    ```
