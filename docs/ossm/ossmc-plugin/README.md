# Using OpenShift Service Mesh Console plugin

The OpenShift Service Mesh Console (OSSMC) plugin extends the OpenShift web console, allowing you to monitor and manage your service mesh directly without switching to a separate application.

## About the OpenShift Service Mesh Console plugin

The OpenShift Service Mesh Console (OSSMC) plugin is an OpenShift dynamic plugin that integrates Kiali functionality into the OpenShift web console, providing full visibility into your Service Mesh. It offers the same features as the standalone Kiali console but organized to fit the OpenShift web console experience.

[!WARNING]
The OSSMC plugin supports only one Kiali instance, regardless of its project access scope.

The plugin integrates into the OpenShift web console through the Main Navigation Menu and enhanced detail pages for workloads, services, applications, Istio configuration, and projects.

### Main Navigation Menu

The OSSMC plugin adds a **Service Mesh** category to the main OpenShift web console navigation with the following menu options:

* **Overview:** Provides a summary of your mesh, displayed as cards that represent the namespaces in the mesh.

* **Traffic Graph:** Provides a full topology view of your mesh, represented by nodes and edges. Each node represents a component of the mesh and each edge represents traffic flowing through the mesh between components.

* **Mesh:** Provides detailed information about the service mesh infrastructure status. It shows an infrastructure topology view with core and add-on components, their health, and how they are connected to each other.

* **Namespaces:** Provides a list of namespaces participating in the mesh with information about type, data plane mode, cluster, health, mTLS status, Istio configuration status, and labels.

* **Applications:** Provides a list of applications detected in the mesh with information about health status and associated workloads.

* **Services:** Provides a list of services in the mesh with information about health, Istio configuration status, and labels.

* **Workloads:** Provides a list of workloads in the mesh with information about health status, type (Deployment, ReplicaSet, DaemonSet, StatefulSet, etc.), and associated Istio configuration.

* **Istio Config:** Provides a list of all service mesh configuration resources, with a column that provides a quick way to know if the configuration for each resource is valid. The list page supports full filtering capabilities, including type, name, and validation status filters. You can also create new configuration resources from this page.

### Application, Workload, and Service Details

The **Applications**, **Workloads** (Deployments, ReplicaSets, Pods, StatefulSets, DaemonSets), and **Services** detail pages provide the following subtabs:

| Subtab | Description |
| ------ | ----------- |
| **Overview** | Shows a summary of the selected application, workload, or service, including a localized topology graph with all inbound and outbound edges and nodes. |
| **Traffic** | Shows information about all inbound and outbound traffic. |
| **Logs** | Shows the logs for the workload's containers. You can see container logs individually ordered by log time and how the Envoy sidecar proxy logs relate to your workload's application logs. You can enable the tracing span integration, which allows you to see which logs correspond to trace spans. Only available for workloads. |
| **Inbound Metrics** | Shows inbound metric graphs, providing a detailed view of performance. You can enable the tracing span integration, which allows you to see which spans occurred at the same time as the metrics. With the span marker in the graph, you can see the specific spans associated with that timeframe. |
| **Outbound Metrics** | Shows outbound metric graphs. Not available for services. |
| **Traces** | Provides a chart showing the trace spans collected over the given timeframe. The trace spans show the most low-level detail within your application. The trace details further show heatmaps that provide a comparison of one span in relation to other requests and spans in the same timeframe. |
| **Envoy** | Shows information about the Envoy sidecar configuration. Only available for workloads. |

### Istio Configuration Details

In the web console detail pages for **Istio configuration resources** (such as VirtualService, DestinationRule, Gateway, AuthorizationPolicy, and others), the OSSMC plugin adds a **Service Mesh** tab that shows an overview and validation status for the resource.

### Project Details

In the web console **Projects** details page, the OSSMC plugin adds a **Service Mesh** tab that shows the namespace detail page with a split-panel layout: the left panel contains stacked cards showing namespace attributes, resource links, and health information, while the right panel displays a namespace-scoped traffic minigraph.

## Installing the OpenShift Service Mesh Console plugin

You can install the OSSMC plugin with the Kiali Operator by creating a `OSSMConsole` resource with the corresponding plugin settings. It is recommended to install the latest version of the Kiali Operator, even while installing a previous OSSMC plugin version, as it includes the latest z-stream release.

### OSSM version compatibility

| OSSM Version    | Kiali Server Version | OSSMC Plugin Version | OCP Version |
| --------------- | -------------------- | -------------------- | ----------- |
| 3.3             | v2.22                | v2.22                | 4.19+       |
| 3.2             | v2.17                | v2.17                | 4.18+       |
| 3.1             | v2.11                | v2.11                | 4.16+       |
| 3.0             | v2.4                 | v2.4                 | 4.15+       |
| 2.6             | v1.73                | v1.73                | 4.15 - 4.18 |

[!NOTE]
The OSSMC plugin is only supported on OpenShift 4.15 and above. For OCP 4.14 users, only the standalone Kiali console is accessible.

You can install the OSSMC plugin by using the OpenShift web console or the OpenShift CLI (`oc`).

### Installing by using the OpenShift web console

You can install the OSSMC plugin by using the OpenShift web console.

**Prerequisites**

* You have logged in to the OpenShift Container Platform cluster either through the web console as a user with the cluster-admin role, or with the oc login command, depending on the installation method.
* You have installed the OpenShift Service Mesh Operator in the OpenShift Container Platform cluster.
* You have installed the `Istio` Resource from OSSM.
* You have installed the Kiali Server.

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

### Installing by using the CLI

You can install the OSSMC plugin by using the OpenShift CLI (`oc`).

**Prerequisites**

* You have logged in to the OpenShift Container Platform cluster either through the web console as a user with the cluster-admin role, or with the oc login command, depending on the installation method.
* You have installed the OpenShift Service Mesh Operator in the OpenShift Container Platform cluster.
* You have installed the `Istio` Resource from OSSM.
* You have installed the Kiali Server.

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

### Uninstalling using the web console

You can uninstall the OSSMC plugin by using the OpenShift web console.

**Procedure**

1. Navigate to **Installed Operators**.

2. Click **Kiali Operator**.

3. Select the **OpenShift Service Mesh Console** tab.

4. Click **Delete OSSMConsole** option from the entry menu.

5. Confirm that you want to delete the plugin.

### Uninstalling using the CLI

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
