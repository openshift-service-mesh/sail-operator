# Using Kiali

Once you have added your application to the mesh, you can use Kiali to view the data flow through your application.

## About Kiali

You can use Kiali to view configurations, monitor traffic, and analyze traces in a single console. It is based on the open source [Kiali](https://www.kiali.io/) project.

Kiali is the management console for OpenShift Service Mesh. It provides dashboards, observability, and robust configuration and validation capabilities. It shows the structure of your service mesh by inferring traffic topology and displays the health of your mesh. Kiali provides detailed metrics, powerful validation, access to Grafana, and strong integration with the OpenShift distributed tracing platform (Tempo).

## Installing the Kiali

The following steps show how to install the Kiali.

[!WARNING]
Do not install the Community version of the Operator. The Community version is not supported.

**Prerequisites**

* Access to the OpenShift web console.

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
    $ echo "https://$(oc get routes -n istio-system kiali -o jsonpath='{.spec.host}')"
    ```

4. Follow the URL to open Kiali in your web browser.

## Integrating OpenShift distributed tracing platform with Kiali

You can integrate OpenShift distributed tracing platform with Kiali, which enables the following features:

* Display trace overlays and details on the graph.
* Display scatterplot charts and in-depth trace/span information on detail pages.
* Integrated span information in logs and metric charts.
* Offer links to the external tracing UI.

## Configuring OpenShift with Kiali

After Kiali is integrated with OpenShift distributed tracing platform, you can view distributed traces in the Kiali console. Viewing these traces can provide insight into the communication between services within the service mesh, helping you understand how requests are flowing through your system and where potential issues might be.

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
        enabled: true [1]
        provider: tempo
        use_grpc: false
        internal_url: http://tempo-sample-query-frontend.tempo:3200
        external_url: https://tempo-sample-query-frontend-tempo.apps-crc.testing [2]
    ```

    [1] Enable tracing.

    [2] The OpenShift route for Jaeger UI must be created in the Tempo namespace. You can either manually create it for the `tempo-sample-query-frontend` service, or update the `Tempo` custom resource with `.spec.template.queryFrontend.jaegerQuery.ingress.type: route`.

2. Save the updated `spec` in `kiali_cr.yaml`.

3. Run the following command to apply the configuration:

    ```console
    $ oc patch -n istio-system kiali kiali --type merge -p "$(cat kiali_cr.yaml)"
    ```

    **Example output:**

    ```console
    $ kiali.kiali.io/kiali patched
    ```

4. Verification

5. Run the following command to get the Kiali route:

    ```console
    $ oc get route kiali ns istio-system
    ```

6. Navigate to the Kiali UI.

7. Navigate to **Workload** → **Traces** tab to see traces in the Kiali UI.
