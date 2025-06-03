# Getting Started with Istio Ambient Mode (Tech Preview)

This section provides a high-level overview and installation procedure for Istio's ambient mode on OpenShift Container Platform (OCP) using OpenShift Service Mesh 3.1.x.

---

## 1. Overview of Istio Ambient Mode

Istio's ambient mode offers a sidecar-less approach to service mesh, simplifying operations and reducing resource consumption. Instead of injecting a sidecar proxy into every application pod, ambient mode utilizes a node-level **ZTunnel proxy** for secure, mTLS-enabled connections and an optional **Waypoint proxy** for advanced L7 functionalities.

### 1.1 Istio Ambient Mode Architecture

* **ZTunnel Proxy:** A per-node proxy that handles secure, transparent TCP connections (mTLS) for all workloads within its node. It operates at Layer 4, offloading mTLS and L4 policy enforcement from application pods.
* **Waypoint Proxy:** An optional, dedicated proxy deployed per service account or namespace. It provides advanced L7 functionalities like traffic management, policy enforcement, and observability for workloads that require them. This allows for selective application of L7 features without the overhead of sidecars for all services.
* **Istio-CNI:** The Istio CNI plugin is responsible for redirecting traffic to the `ztunnel` proxy on each node, enabling transparent interception without requiring modifications to application pods.

### 1.2 Why Use Istio's Ambient Mode?

Istio's ambient mode offers several benefits:

* **Simplified Operations:** Eliminates the need for sidecar injection and management, reducing operational complexity and cognitive load.
* **Reduced Resource Consumption:** By centralizing mTLS and L4 policy enforcement in the `ztunnel`, ambient mode significantly lowers resource overhead per pod.
* **Incremental Adoption:** Allows for gradual adoption of service mesh features. Workloads can join the mesh at L4 for mTLS and basic policy, and then selectively opt-in for L7 features via `waypoint` proxies as needed.
* **Enhanced Security:** Provides a secure, zero-trust network foundation with mTLS by default for all meshed workloads.

**Trade-offs:**

* Ambient mode is a newer architecture and may have different operational considerations compared to the traditional sidecar model.
* L7 features require the deployment of `waypoint` proxies, which add a small amount of overhead for the services that utilize them.

---

## 2. Pre-requisites to Using Ambient Mode with OSSM 3

Before installing Istio's ambient mode with OpenShift Service Mesh, ensure the following prerequisites are met:

* **OpenShift Container Platform 4.15+:** This version of OpenShift is required for supported Kubernetes Gateway API CRDs, which are essential for ambient mode functionalities.
* **OpenShift Service Mesh 3.1.0+ operator is installed:** Ensure that the OSSM operator version 3.1.0 or later is installed on your OpenShift cluster.

**Pre-existing Service Mesh Installations:**

While the use of properly defined discovery selectors will allow a service mesh to be deployed in ambient mode alongside a service mesh in sidecar mode, this is not a scenario we have thoroughly validated. To avoid potential conflicts, as a technology preview feature, Istio's ambient mode should only be installed on clusters without a pre-existing OpenShift Service Mesh installation.

**Note**: Istio's ambient mode is completely incompatible with clusters containing the OpenShift Service Mesh 2.6 or earlier versions of the operator and they should not be used together.

---

## 3. Procedure to Install Istio's Ambient Mode

This procedure demonstrates how to install Istio's ambient mode on OpenShift with Istio's sample Bookinfo application.

### 3.1 Prerequisites

* You have deployed a cluster on OpenShift Container Platform 4.19 or later.
* You are logged in to the OpenShift Container Platform by web console as a user with the cluster-admin role or with `oc login` command, depends on the installation method.
* The OpenShift Service Mesh operator is [installed](https://docs.redhat.com/en/documentation/red_hat_openshift_service_mesh/3.0/html/installing/ossm-installing-service-mesh#ossm-installing-operator_ossm-about-deploying-istio-using-service-mesh-operator).

### 3.2 Install Istio (Control Plane)

#### 3.2.1 Creating the Istio project

**Using the web console**

1.  In the OpenShift Container Platform web console, click **Home** > **Projects**.
2.  Click **Create Project**.
3.  At the prompt, enter a name for the project in the **Name** field. For example, `istio-system`. The other fields provide supplementary information to the `Istio` resource definition and are optional.
4.  Click **Create**. The Service Mesh Operator deploys Istio to the project you specified.

**Using the CLI**

1. Create a namespace for Istio. For example, `istio-system`.

```bash
$ oc create namespace istio-system
```

#### 3.2.2 Creating the Istio resource

**Using the web console**

1.  In the OpenShift Container Platform web console, click **Operators** > **Installed Operators**.
2.  Select `istio-system` in the **Project** drop-down menu.
3.  Click the Service Mesh Operator.
4.  Click **Istio**.
5.  Click **Create Istio**.
6.  Select the `istio-system` project from the **Namespace** drop-down menu.
7.  Click on the **Helm Values** drop-down menu.
8.  Locate **profile** parameter and enter **ambient** as a value.
9.  Locate and click on **pilot** drop-down menu under the **Helm Values**.
10. Locate **trustedZtunnelNamespace** parameter and enter **ztunnel** as a value.
11. Click **Create**. This action deploys the Istio control plane.  
    When `State: Healthy` appears in the **Status** column, Istio is successfully deployed.

**Using the CLI**

1. Create an Istio resource for creation.

```bash
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: istio-system
  updateStrategy:
    type: InPlace
    inactiveRevisionDeletionGracePeriodSeconds: 30
  values:
    pilot:
      trustedZtunnelNamespace: ztunnel
    profile: ambient
  version: v1.26.0
```

2. Apply the Istio CR.

```bash
oc apply -f istio.yaml
```

3. Watch for the Istio pod to become ready

```bash
oc -n istio-system get pods
```

### 3.3 Install Istio CNI

#### 3.3.1 Creating the Istio CNI project

**Using the web console**

1.  In the OpenShift Container Platform web console, click **Home** > **Projects**.
2.  Click **Create Project**.
3.  At the prompt, enter a name for the project in the **Name** field. For example, `istio-cni`. The other fields provide supplementary information to the `Istio` resource definition and are optional.
4.  Click **Create**. The Service Mesh Operator deploys Istio to the project you specified.

**Using the CLI**

1. Create a namespace for Istio. For example, `istio-cni`.

```bash
$ oc create namespace istio-cni
```

#### 3.3.2 Creating the Istio CNI resource

**Using the web console**

1.  In the OpenShift Container Platform web console, click **Operators** > **Installed Operators**.
2.  Select `istio-cni` in the **Project** drop-down menu.
3.  Click the Service Mesh Operator.
4.  Click **IstioCNI**.
5.  Click **Create IstioCNI**.
6.  Ensure that the name is `default`.
7.  Select the `istio-cni` project from the **Namespace** drop-down menu.
8.  Click on the **Helm Values** -> **cni** -> **ambient** drop-down menu.
9.  Locate **enabled** parameter and mark the checkbox.
10. Click **Create**. This action deploys the Istio CNI plugin.  
    When `State: Healthy` appears in the **Status** column, it implies that Istio CNI is successfully deployed.

**Using the CLI**

1. Create an IstioCNI resource for creation.

```bash
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: default
spec:
  namespace: istio-cni
  values:
    cni:
      ambient:
        enabled: true
  version: v1.26.0
```

2. Apply the IstioCNI CR.

```bash
oc apply -f istio-cni.yaml
```

3. Watch for the IstioCNI pods to become ready

```bash
oc -n istio-cni get pods
```

### 3.4 Install Ztunnel proxy

#### 3.4.1 Creating the Ztunnel project

**Using the web console**

1.  In the OpenShift Container Platform web console, click **Home** > **Projects**.
2.  Click **Create Project**.
3.  At the prompt, enter a name for the project in the **Name** field. For example, `ztunnel`. The other fields provide supplementary information to the `Istio` resource definition and are optional.  
    **Note** - The namespace name for `ztunnel` project must match the `trustedZtunnelNamespace` parameter in **Istio** configuration.
4.  Click **Create**. The Service Mesh Operator deploys Istio to the project you specified.

**Using the CLI**

1. Create a namespace for Ztunnel. For example, `ztunnel`.  
   **Note** - The namespace name for `ztunnel` project must match the `trustedZtunnelNamespace` parameter in **Istio** configuration.

```bash
$ oc create namespace ztunnel
```

#### 3.4.2 Creating the Ztunnel resource

**Using the web console**

1.  In the OpenShift Container Platform web console, click **Operators** > **Installed Operators**.
2.  Select `ztunnel` in the **Project** drop-down menu.
3.  Click the Service Mesh Operator.
4.  Click **ZTunnel**.
5.  Click **Create ZTunnel**.
6.  Ensure that the name is `default`.
7.  Select the `ztunnel` project from the **Namespace** drop-down menu.
8. Click **Create**. This action deploys the Ztunnel component.  
    When `State: Healthy` appears in the **Status** column, it implies that Ztunnel is successfully deployed.

**Using the CLI**

1. Create an Ztunnel resource for creation.

```bash
apiVersion: sailoperator.io/v1alpha1
kind: ZTunnel
metadata:
  name: default
spec:
  namespace: ztunnel
  profile: ambient
  version: v1.26.0
```

2. Apply the Ztunnel CR.

```bash
oc apply -f ztunnel.yaml
```

3. Watch for the Ztunnel pods to become ready

```bash
oc -n ztunnel get pods
```

### 3.5 Scoping the Service Mesh with discovery selectors

Service Mesh in Istio ambient mode includes workloads that meet the following criteria:

* The control plane has discovered the workload.
* The workload has been [labelled appropriately](https://istio.io/latest/docs/ambient/usage/add-workloads/), such that traffic redirection with ZTunnel has been configured.

By default, the control plane discovers workloads in all namespaces across the cluster, with the following results:

* Each proxy instance receives configuration for all namespaces, including workloads not enrolled in the mesh.
* All the workloads with the appropriate labels would become part of the mesh. 

In shared clusters, you might want to limit the scope of Service Mesh to only certain namespaces. This approach is especially useful if multiple service meshes run in the same cluster.

#### 3.5.1 About discovery selectors

With discovery selectors, the mesh administrator can control which namespaces the control plane can access. By using a Kubernetes label selector, the administrator sets the criteria for the namespaces visible to the control plane, excluding any namespaces that do not match the specified criteria.

The `discoverySelectors` field accepts an array of Kubernetes selectors, which apply to labels on namespaces. You can configure each selector for different use cases:

* Custom label names and values. For example, configure all namespaces with the label istio-discovery=enabled.
* A list of namespace labels by using set-based selectors with OR logic. For instance, configure namespaces with istio-discovery=enabled OR region=us-east1.
* Inclusion and exclusion of namespaces. For example, configure namespaces with istio-discovery=enabled AND the label app=helloworld.

#### 3.5.2 Scoping a Service Mesh by using discovery selectors

If you know which namespaces to include in the Service Mesh, configure `discoverySelectors` during or after installation by adding the required selectors to the `meshConfig.discoverySelectors` section of the `Istio` resource. For example, configure Istio to discover only namespaces labeled `istio-discovery=enabled`.

**Prerequisites**

* The OpenShift Service Mesh operator is installed.
* An Istiod resource is created
* An Istio CNI resource is created.
* A Ztunnel resource is created.

**Procedure**

1. Add a label to the namespace containing the Istio control plane, IstioCNI and Ztunnel.

```bash
$ oc label namespace istio-system istio-discovery=enabled
$ oc label namespace istio-cni istio-discovery=enabled
$ oc label namespace ztunnel istio-discovery=enabled
```

2. Modify the `Istio` control plane resource to include a `discoverySelectors` section with the same label.

```bash
$ oc patch istio default --type=merge -p '{
  "spec": {
    "values": {
      "meshConfig": {
        "discoverySelectors": [
          {
            "matchLabels": {
              "istio-discovery": "enabled"
            }
          }
        ]
      }
    }
  }
}'
```

### 3.6 About the Bookinfo Application

Installing the `bookinfo` example application consists of two main tasks: deploying the application and creating a gateway so the application is accessible outside the cluster.

You can use the `bookinfo` application to explore service mesh features. Using the `bookinfo` application, you can easily confirm that requests from a web browser pass through the mesh and reach the application.

The `bookinfo` application displays information about a book, similar to a single catalog entry of an online book store. The application displays a page that describes the book, lists book details (ISBN, number of pages, and other information), and book reviews.

The `bookinfo` application is exposed through the mesh, and the mesh configuration determines how the microservices comprising the application are used to serve requests. The review information comes from one of three services: `reviews-v1`, `reviews-v2` or `reviews-v3`. If you deploy the `bookinfo` application without defining the `reviews` virtual service, then the mesh uses a round robin rule to route requests to a service.

By deploying the `reviews` virtual service, you can specify a different behavior. For example, you can specify that if a user logs into the `bookinfo` application, then the mesh routes requests to the `reviews-v2` service, and the application displays reviews with black stars. If a user does not log into the `bookinfo` application, then the mesh routes requests to the `reviews-v3` service, and the application displays reviews with red stars.

For more information, see [Bookinfo Application](https://istio.io/latest/docs/examples/bookinfo/) in the upstream Istio documentation.

#### 3.6.1 Deploying the Bookinfo application

**Prerequisites**

* You have deployed a cluster on OpenShift Container Platform 4.15 or later.
* You are logged in to the OpenShift Container Platform web console as a user with the `cluster-admin` role.
* You have access to the OpenShift CLI (oc).
* You have installed the Red Hat OpenShift Service Mesh Operator, created the Istio resource, and the Operator has deployed Istio.
* You have created IstioCNI resource, and the Operator has deployed the necessary IstioCNI pods. 
* You have create Ztunnel resource, and the Operator has deployed the necessary Ztunnel pods.

**Procedure**

1. Create the `bookinfo` namespace and add a label `istio-discovery=enabled`.

```bash
$ oc create ns bookinfo
$ oc label namespace bookinfo istio-discovery=enabled
```

2. Deploy the application.

```bash
$ oc apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/release-1.25/samples/bookinfo/platform/kube/bookinfo.yaml
$ oc apply -n bookinfo -f https://raw.githubusercontent.com/istio/istio/release-1.25/samples/bookinfo/platform/kube/bookinfo-versions.yaml
```

3. Verify that the `bookinfo` pods are available.

```bash
$ oc -n bookinfo get pods
```

**Example output**
```bash
NAME                             READY   STATUS    RESTARTS   AGE
details-v1-54ffdd5947-8gk5h      1/1     Running   0          5m9s
productpage-v1-d49bb79b4-cb9sl   1/1     Running   0          5m3s
ratings-v1-856f65bcff-h6kkf      1/1     Running   0          5m7s
reviews-v1-848b8749df-wl5br      1/1     Running   0          5m6s
reviews-v2-5fdf9886c7-8xprg      1/1     Running   0          5m5s
reviews-v3-bb6b8ddc7-bvcm5       1/1     Running   0          5m5s
```

4. Verify that the `bookinfo` application is running by sending a request to the `bookinfo` page. Run the following command:

```bash
oc exec "$(oc get pod -l app=ratings -n bookinfo -o jsonpath='{.items[0].metadata.name}')" -c ratings -n bookinfo -- curl -sS productpage:9080/productpage | grep -o "<title>.*</title>"
```

5. Add `bookinfo` application to the Ambient mesh.

```bash
$ oc label namespace bookinfo istio.io/dataplane-mode=ambient
```

**Note** - You don't need to restart or redeploy any of the application pods. Unlike the sidecar mode, each pod's container count will remain the same even after adding them to the ambient mesh.

6. To confirm that `ztunnel` successfully opened listening sockets inside the pod network ns, use the following command.

```bash
$ kubectl debug -it -n bookinfo "$(kubectl get pod -n bookinfo -l app=productpage -o name)" --image quay.io/curl/curl -- netstat -tulpn
Defaulting debug container name to debugger-z6wtv.
Active Internet connections (only servers)
Proto Recv-Q Send-Q Local Address           Foreign Address         State       PID/Program name    
tcp        0      0 127.0.0.1:15053         0.0.0.0:*               LISTEN      -
tcp        0      0 :::15006                :::*                    LISTEN      -
tcp        0      0 :::15001                :::*                    LISTEN      -
tcp        0      0 :::15008                :::*                    LISTEN      -
tcp        0      0 :::9080                 :::*                    LISTEN      -
tcp        0      0 ::1:15053               :::*                    LISTEN      -
udp        0      0 127.0.0.1:15053         0.0.0.0:*                           -
udp        0      0 ::1:15053               :::*                                -
```

---

## 4. Additional Resource Links

* **Ambient mode architecture:** [https://istio.io/latest/docs/ambient/architecture/](https://istio.io/latest/docs/ambient/architecture/)
