# Kiali

Kiali can be used to view the data flow through your application once it has been added to the service mesh.

## About Kiali

Kiali provides observability into the service mesh running on OpenShift. Kiali helps you define, validate, and observe your Istio service mesh. It helps you to understand the structure of your service mesh by inferring the topology, and also provides information about the health of your service mesh.

Kiali provides an interactive graph view of your namespace in real time that provides visibility into features like circuit breakers, request rates, latency, and even graphs of traffic flows. Kiali offers insights about components at different levels, from Applications to Services and Workloads, and can display the interactions with contextual information and charts on the selected graph node or edge. Kiali also provides the ability to validate your Istio configurations, such as gateways, destination rules, virtual services, mesh policies, and more. Kiali provides detailed metrics, and a basic Grafana integration is available for advanced queries. Distributed tracing is provided by integrating Tempo into the Kiali console.

### Kiali Architecture

Kiali is based on the open source [Kiali project](https://www.kiali.io/). Kiali is composed of two components: the Kiali application and the Kiali console.

* **Kiali application (back end)** – This component runs in the container application platform and communicates with the service mesh components, retrieves and processes data, and exposes this data to the console. The Kiali application does not need storage. When deploying the application to a cluster, configurations are set in ConfigMaps and secrets.

* **Kiali console (front end)** – The Kiali console is a web application. The Kiali application serves the Kiali console, which then queries the back end for data to present it to the user.

In addition, Kiali depends on external services and components provided by the container application platform and Istio.

* **Red Hat Service Mesh (Istio)** - Istio is a Kiali requirement. Istio is the component that provides and controls the service mesh. Although Kiali and Istio can be installed separately, Kiali depends on Istio and will not work if it is not present. Kiali needs to retrieve Istio data and configurations, which are exposed through Prometheus and the cluster API.

* **Prometheus** - When Istio telemetry is enabled, metrics data are stored in Prometheus. Kiali uses this Prometheus data to determine the mesh topology, display metrics, calculate health, show possible problems, and so on. Kiali communicates directly with Prometheus and assumes the data schema used by Istio Telemetry. Prometheus is an Istio dependency and a hard dependency for Kiali, and many of Kiali's features will not work without Prometheus.

* **Cluster API** - Kiali uses the API of the OpenShift (cluster API) to fetch and resolve service mesh configurations. Kiali queries the cluster API to retrieve, for example, definitions for namespaces, services, deployments, pods, and other entities. Kiali also makes queries to resolve relationships between the different cluster entities. The cluster API is also queried to retrieve Istio configurations like virtual services, destination rules, route rules, gateways, quotas, and so on.

* **Tempo (optional)** - When you install Tempo, the Kiali console includes a tab to display distributed tracing data. Note that tracing data will not be available if you disable Istio's distributed tracing feature. Also note that user must have access to the namespace where the Istio control plane is installed to view tracing data.

* **Grafana (optional)** - When available, the metrics pages of Kiali display links to direct the user to the same metric in Grafana. Note that user must have access to the namespace where the Istio control plane is installed to view links to the Grafana dashboard and view Grafana data.

### Kiali Features

The Kiali console is integrated with Red Hat Service Mesh and provides the following capabilities:

* **Health** – Quickly identify issues with applications, services, or workloads.

* **Traffic** – Visualize how your applications, services, or workloads communicate via the Kiali traffic graph.

* **Metrics** – Predefined metrics dashboards let you chart service mesh and application performance for Go, Node.js. Quarkus, Spring Boot, Thorntail and Vert.x. You can also create your own custom dashboards.

* **Tracing** – Integration with Jaeger lets you follow the path of a request through various microservices that make up an application.

* **Validations** – Perform advanced validations on the most common Istio objects (Destination Rules, Service Entries, Virtual Services, and so on).

* **Mesh structure** – Detailed information about the Istio infrastructure status is displayed on the mesh page. It shows an infrastructure topology view with core and add-on components, their health, and how they are connected to each other.

* **Configuration** – Optional ability to create, update and delete Istio routing configuration using wizards or directly in the YAML editor in the Kiali Console.

## Installing the Kiali

Kiali can be installed in two different ways: via the OpenShift web console or the OpenShift CLI.

### Via the OpenShift web console

The following steps show how to install the Kiali via the OpenShift web console.

[!WARNING]
Do not install the Community version of the Operator. The Community version is not supported.

**Prerequisites**

* Access to the OpenShift web console with administrator access

**Procedure**

1. Log in to the OpenShift web console.

2. Navigate to **Operators** -> **OperatorHub**.

3. Type **Kiali** into the filter box to find the Kiali.

4. Click **Kiali** to display information about the Operator.

5. Click **Install**.

6. On the **Operator Installation** page, select the **stable** Update Channel.

7. Select **All namespaces on the cluster (default)**. This installs the Operator in the default `openshift-operators` project and makes the Operator available to all projects in the cluster.

8. Select the **Automatic** Approval Strategy.

    [!NOTE]
    The Manual approval strategy requires a user with appropriate credentials to approve the Operator installation and subscription process.

9. Click **Install**.

10. The **Installed Operators** page displays the Kiali Operator's installation progress.

11. Once the Kiali operator is installed successfully, click the Kiali Operator item to access to the operator details page.

12. Select the **Kiali** tab and click **Create Kiali** button.

13. Change any default Kiali settings in the **Form** or **Yaml** view if needed, and click **Create** button

14. The new Kiali instance appears in the Kialis list with the installation status. When the Kiali condition status value is running and successful, Kiali application can be accessed.

**Verification**

1. In the OpenShift web console, navigate to **Networking** -> **Routes**.

2. On the **Routes** page, select the Istio control plane project, for example `istio-system`, from the **Namespace** menu.

    The **Location** column displays the linked address for each route.

3. If necessary, use the filter to find the route for the Kiali console. Click the route **Location** to launch the Kiali console.

4. Click **Log In With OpenShift**.

    When you first log in to the Kiali Console, you see the **Overview** page which displays all the namespaces in your service mesh that you have permission to view. When there are multiple namespaces shown on the **Overview** page, Kiali shows namespaces with health or validation problems first.

    The tile for each namespace displays the number of labels, the **Istio Config** health, the number of and **Applications** health, and **Traffic** for the namespace. If you are validating the console installation and namespaces have not yet been added to the mesh, there might not be any data to display other than `istio-system`.

### Via the OpenShift CLI

The following steps show how to install the Kiali via the OpenShift CLI.

**Prerequisites**

* Access to the OpenShift cluster via CLI

**Procedure**

1. Create a Subscription object in the openshift-operators namespace:

    ```yaml
    cat <<EOM | oc apply -f -
    apiVersion: operators.coreos.com/v1alpha1
    kind: Subscription
    metadata:
      name: kiali
      namespace: openshift-operators
    spec:
      channel: stable
      installPlanApproval: Automatic
      name: kiali-ossm
      source: redhat-operators
      sourceNamespace: openshift-marketplace
    EOM
    ```

2. Wait for the Kiali Operator installation to be complete by using the standard oc wait command:

    ```console
    oc wait deployment/kiali-operator -n openshift-operators --for=condition=available --timeout=600s
    ```

3. Once the Kiali operator is installed, create a Kiali custom resource to install the Kiali server.

    ```yaml
    cat <<EOM | oc apply -f -
    apiVersion: kiali.io/v1alpha1
    kind: Kiali
    metadata:
      name: kiali
      namespace: istio-system
    EOM
    ```

    [!NOTE]
    The `openshift` authentication strategy is the only supported authentication configuration when Kiali is deployed with OpenShift Service Mesh (OSSM). The `openshift` strategy controls access based on the individual’s role-based access control (RBAC) roles of the OpenShift.

    By default, the Kiali Operator will install the Kiali Server whose version is the same as the operator itself. You can ask the operator to install an earlier version of the Kiali Server by specifying the “spec.version” field to indicate which version of the Kiali Server to install (check the documentation for the valid versions that are supported by the operator and which Istio versions work with which Kiali versions):

    ```yaml
    cat <<EOM | oc apply -f -
    apiVersion: kiali.io/v1alpha1
    kind: Kiali
    metadata:
      name: kiali
      namespace: istio-system
    spec:
      version: v2.4
    EOM
    ```

    The Kiali Server is highly customizable through the Kiali CR configuration. For example, to support Kiali observing only a specific set of namespaces, you can define a list of discovery selectors:

    ```yaml
    cat <<EOM | oc apply -f -
    apiVersion: kiali.io/v1alpha1
    kind: Kiali
    metadata:
      name: kiali
      namespace: istio-system
    spec:
      deployment:
        discovery_selectors:
          default:
          - matchExpressions:
            - key: kubernetes.io/metadata.name
              operator: In
              values:
              - my-mesh-apps
              - more-apps
    EOM
    ```

4. Once the Kiali CR is created, the Kiali Operator will shortly be notified and will process it (called “reconcilation”) which performs the Kiali installation. Wait for the Kiali Operator to finish the reconcilation by using the standard oc wait command:

    ```console
    oc wait --for=condition=Successful kiali kiali -n istio-system
    ```

5. When the reconciliation process is finished successfully, run the following command to get the Kiali route:

    ```console
    echo "https://$(oc get routes -n istio-system kiali -o jsonpath='{.spec.host}')"
    ```

6. Follow the URL to open Kiali in your web browser.

## Configuring OpenShift Monitoring with Kiali

The following steps show how to integrate the Kiali with user-workload monitoring.

**Prerequisites**

* OpenShift Service Mesh is installed.

* User-workload monitoring is enabled. See [Enabling monitoring for user-defined projects](https://docs.openshift.com/container-platform/4.16/observability/monitoring/enabling-monitoring-for-user-defined-projects.html).

* OpenShift Monitoring has been configured with Service Mesh. See "Configuring OpenShift Monitoring with Service Mesh".

* Kiali 2.4 is installed.

**Procedure**

1. Create a `ClusterRoleBinding` resource for Kiali:

    **Example `ClusterRoleBinding` configuration**

    ```yaml
    apiVersion: rbac.authorization.k8s.io/v1
    kind: ClusterRoleBinding
    metadata:
      name: kiali-monitoring-rbac
    roleRef:
      apiGroup: rbac.authorization.k8s.io
      kind: ClusterRole
      name: cluster-monitoring-view
    subjects:
    - kind: ServiceAccount
      name: kiali-service-account
      namespace: istio-system
    ```

2. Create a Kiali resource and point it to your Istio instance:

    **Example Kiali resource configuration**

    ```yaml
    apiVersion: kiali.io/v1alpha1
    kind: Kiali
    metadata:
      name: kiali-user-workload-monitoring
      namespace: istio-system
    spec:
    external_services:
      prometheus:
        auth:
          type: bearer
          use_kiali_token: true
        thanos_proxy:
          enabled: true
        url: https://thanos-querier.openshift-monitoring.svc.cluster.local:9091
    ```

3. When the Kiali resource is ready, get the Kiali URL from the Route by running the following command:

    ```console
    echo "https://$(oc get routes -n istio-system kiali -o jsonpath='{.spec.host}')"
    ```

4. Follow the URL to open Kiali in your web browser.

## Configuring OpenShift distributed tracing platform with Kiali

You can integrate OpenShift distributed tracing platform with Kiali, which enables the following features:

* Display trace overlays and details on the graph. These traces can provide insight into the communication between services within the service mesh, helping you understand how requests are flowing through your system and where potential issues might be.
* Display scatterplot charts and in-depth trace/span information on detail pages.
* Integrated span information in logs and metric charts.
* Offer links to the external tracing UI.

**Prerequisites**

* You installed OpenShift Service Mesh.

* You configured distributed tracing platform with OpenShift Service Mesh.

**Procedure**

1. Update the `Kiali` resource `spec` configuration for tracing:

    **Example `Kiali` resource `spec` configuration for tracing**

    ```yaml
    spec:
      external_services:
        tracing:
        enabled: true #(1)
        provider: tempo
        use_grpc: false
        internal_url: http://tempo-sample-query-frontend.tempo:3200
        external_url: https://tempo-sample-query-frontend-tempo.apps-crc.testing #(2)
    ```

    1. Enable tracing.

    2. The OpenShift route for Jaeger UI must be created in the Tempo namespace. You can either manually create it for the `tempo-sample-query-frontend` service, or update the `Tempo` custom resource with `.spec.template.queryFrontend.jaegerQuery.ingress.type: route`.

2. Save the updated `spec` in `kiali_cr.yaml`.

3. Run the following command to apply the configuration:

    ```console
    oc patch -n istio-system kiali kiali --type merge -p "$(cat kiali_cr.yaml)"
    ```

    **Example output:**

    ```console
    kiali.kiali.io/kiali patched
    ```

**Verification**

1. Run the following command to get the Kiali route:

    ```console
    echo "https://$(oc get routes -n istio-system kiali -o jsonpath='{.spec.host}')"
    ```

2. Navigate to the Kiali UI.

3. Navigate to **Workload** → **Traces** tab to see traces in the Kiali UI.

## Viewing service mesh data in the Kiali console

The Kiali Graph offers a powerful visualization of your mesh traffic. The topology combines real-time request traffic with your Istio configuration information to present immediate insight into the behavior of your service mesh, letting you quickly pinpoint issues. Multiple Graph Types let you visualize traffic as a high-level service topology, a low-level workload topology, or as an application-level topology.

There are several graphs to choose from:

* The **App graph** shows an aggregate workload for all applications that are labeled the same.

* The **Service graph** shows a node for each service in your mesh but excludes all applications and workloads from the graph. It provides a high level view and aggregates all traffic for defined services.

* The **Versioned App graph** shows a node for each version of an application. All versions of an application are grouped together.

* The **Workload graph** shows a node for each workload in your service mesh. This graph does not require you to use the application and version labels. If your application does not use version labels, use this the graph.

Graph nodes are decorated with a variety of information, pointing out various route routing options like virtual services and service entries, as well as special configuration like fault-injection and circuit breakers. It can identify mTLS issues, latency issues, error traffic and more. The Graph is highly configurable, can show traffic animation, and has powerful Find and Hide abilities.

Click the **Legend** button to view information about the shapes, colors, arrows, and badges displayed in the graph.

To view a summary of metrics, select any node or edge in the graph to display its metric details in the summary details panel.

### Changing graph layouts in Kiali

The layout for the Kiali graph can render differently depending on your application architecture and the data to display. For example, the number of graph nodes and their interactions can determine how the Kiali graph is rendered. Because it is not possible to create a single layout that renders nicely for every situation, Kiali offers a choice of several different layouts.

**Prerequisites**

* If you do not have your own application installed, install the Bookinfo sample application.  Then generate traffic for the Bookinfo application by entering the following command several times.

    ```console
    curl "http://$GATEWAY_URL/productpage"
    ```

    This command simulates a user visiting the `productpage` microservice of the application.

**Procedure**

1. Launch the Kiali console.

2. Click **Log In With OpenShift**.

3. In Kiali console, click **Traffic Graph** to view a namespace graph.

4. From the **Namespace** menu, select your application namespace, for example, `bookinfo`.

5. To choose a different graph layout, do either or both of the following:

* Select different graph data groupings from the menu at the top of the graph.

  * App graph
  * Service graph
  * Versioned App graph (default)
  * Workload graph

* Select a different graph layout from the Legend at the bottom of the graph.
  * Layout default dagre
  * Layout 1 cose-bilkent
  * Layout 2 cola

## Viewing logs in the Kiali console

You can view logs for your workloads in the Kiali console.  The **Workload Detail** page includes a **Logs** tab which displays a unified logs view that displays both application and proxy logs. You can select how often you want the log display in Kiali to be refreshed.

To change the logging level on the logs displayed in Kiali, you change the logging configuration for the workload or the proxy.

**Prerequisites**

* Service Mesh installed and configured.
* Kiali installed and configured.
* The address for the Kiali console.
* Application or Bookinfo sample application added to the mesh.

**Procedure**

1. Launch the Kiali console.

2. Click **Log In With OpenShift**.

    The Kiali Overview page displays namespaces that have been added to the mesh that you have permissions to view.

3. Click **Workloads**.

4. On the **Workloads** page, select the project from the **Namespace** menu.

5. If necessary, use the filter to find the workload whose logs you want to view.  Click the workload **Name**.  For example, click **ratings-v1**.

6. On the **Workload Details** page, click the **Logs** tab to view the logs for the workload.

[!NOTE]
If you do not see any log entries, you may need to adjust either the Time Range or the Refresh interval.

## Viewing metrics in the Kiali console

You can view inbound and outbound metrics for your applications, workloads, and services in the Kiali console.  The Detail pages include the following tabs:

* Inbound Application metrics
* Outbound Application metrics
* Inbound Workload metrics
* Outbound Workload metrics
* Inbound Service metrics

These tabs display predefined metrics dashboards, tailored to the relevant application, workload or service level. The application and workload detail views show request and response metrics such as volume, duration, size, or TCP traffic. The service detail view shows request and response metrics for inbound traffic only.

Kiali lets you customize the charts by choosing the charted dimensions. Kiali can also present metrics reported by either source or destination proxy metrics. And for troubleshooting, Kiali can overlay trace spans on the metrics.

**Prerequisites**

* Service Mesh installed and configured.
* Kiali installed and configured.
* The address for the Kiali console.
* (Optional) Distributed tracing installed and configured.

**Procedure**

1. Launch the Kiali console.

2. Click **Log In With OpenShift**.

    The Kiali Overview page displays namespaces that have been added to the mesh that you have permissions to view.

3. Click either **Applications**, **Workloads**, or **Services**.

4. On the **Applications**, **Workloads**, or **Services** page, select the project from the **Namespace** menu.

5. If necessary, use the filter to find the application, workload, or service whose logs you want to view.  Click the **Name**.

6. On the **Application Detail**, **Workload Details**, or **Service Details** page, click either the **Inbound Metrics** or **Outbound Metrics** tab to view the metrics.
