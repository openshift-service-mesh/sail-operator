[Return to OSSM Docs](../)

# Kiali - multi-cluster

For multi-cluster Istio deployments, Kiali can show you a single unified view of your mesh across clusters.

Before proceeding with the setup, ensure you meet the requirements.

## Requirements

- Two OpenShift clusters. In this tutorial they are named `east` and `west`.
- Istio installed in a multi-cluster configuration on each cluster.
- IstioCNI installed on each cluster.
- Aggregated metrics and traces. Kiali needs a single endpoint for metrics and a single endpoint for traces where it can consume aggregated metrics/traces across all clusters. There are multiple ways to aggregate metrics/traces such as Prometheus federation or using OTEL collector pipelines.
- Cluster Admin for each cluster.
- Kiali Operator v1.89 installed on the `east` cluster.

## Setup

In this tutorial, we will deploy Kiali on the `east` cluster and then grant Kiali access to the `west` cluster. The unified multi-cluster setup requires a Kiali Service Account (SA) to have read access to each Kubernetes cluster in the mesh. This is separate from the user credentials that are required when a user logs into Kiali. Kiali uses the user's credentials to check if the user has access to a namespace and when performing any write operation such as creating/editing/deleting objects in Kubernetes. To give the Kiali Service Account access to each remote cluster, a kubeconfig with credentials needs to be created and mounted into the Kiali pod.

### Procedure

1. Install Kiali on the `east` cluster.

   Create a file named `kiali.yaml`.

   ```yaml
   apiVersion: kiali.io/v1alpha1
   kind: Kiali
   metadata:
     name: kiali
     namespace: istio-system
   spec:
     version: v1.89
   ```

   Apply the yaml file into the `east` cluster.

   ```sh
   oc --context east apply -f kiali.yaml
   ```

   Wait for the Kiali Operator to finish deploying the Kiali Server:

   ```sh
   oc --context east wait --for=condition=Successful --timeout=2m kialis/kiali -n istio-system
   ```

1. Create an `OAuthClient` on the remote cluster so that Kiali can access the OpenShift API server on behalf of users.

   Find your Kiali route's hostname.

   ```sh
   oc --context east get route kiali -n istio-system -o jsonpath='{.spec.host}'
   ```

   Create a file named `oauthclientwest.yaml`

   ```yaml
   apiVersion: oauth.openshift.io/v1
   grantMethod: auto
   kind: OAuthClient
   metadata:
     labels:
       app: kiali
       app.kubernetes.io/instance: kiali
       app.kubernetes.io/name: kiali
       app.kubernetes.io/part-of: kiali
     name: kiali-istio-system
   redirectURIs:
     - https://<your-kiali-route-hostname>/api/auth/callback/west
   ```

   Create the `OAuthClient` in the west cluster.

   ```sh
   oc --context west apply -f oauthclientwest.yaml
   ```

1. Create a remote cluster secret.

   In order to access a remote cluster, you must provide a kubeconfig to Kiali via a Kubernetes secret. You can use [this script](https://raw.githubusercontent.com/kiali/kiali/master/hack/istio/multicluster/kiali-prepare-remote-cluster.sh) to simplify this process for you. Running this script will:

   - Create a Service Account for Kiali in the remote cluster.
   - Create RBAC resources for this Service Account in the remote cluster.
   - Create a kubeconfig file and save this as a secret in the namespace where Kiali is deployed on the `east` cluster.

   1. Download the `kiali-prepare-remote-cluster.sh` script.

      ```sh
      curl -L -o kiali-prepare-remote-cluster.sh https://raw.githubusercontent.com/kiali/kiali/master/hack/istio/multicluster/kiali-prepare-remote-cluster.sh
      ```

   2. Make the script executeable.

      ```sh
      chmod +x kiali-prepare-remote-cluster.sh
      ```

   3. Run the script passing your `east` and `west` cluster contexts.

      ```sh
      ./kiali-prepare-remote-cluster.sh --kiali-cluster-context east --remote-cluster-context west --view-only false --kiali-resource-name kiali --remote-cluster-namespace istio-system --remote-cluster-name west
      ```

   **Note:** Use the option `--help` for additional details on how to use the script.

1. Restart Kiali to pickup the remote secret.

   ```sh
   oc --context east rollout restart deployments/kiali -n istio-system
   ```

   Wait for Kiali to become ready.

   ```sh
   oc --context east rollout status deployments/kiali -n istio-system
   ```

1. Login to Kiali.

   When you first visit Kiali, you will login to the cluster where Kiali is deployed. In our case it will be the `east` cluster.

   Find your Kiali route's hostname.

   ```sh
   oc --context east get route kiali -n istio-system -o jsonpath='{.spec.host}'
   ```

   Navigate to your Kiali URL in your browser.

   ```sh
   https://<your-kiali-route-hostname>
   ```

1. Login to the `west` cluster through Kiali.

   In order to see other clusters in the Kiali UI, you must first login as a user to those clusters through Kiali. Click on the user profile dropdown in the top right hand menu. Then select `Login to west`. You will again be redirected to an OpenShift login page and prompted for credentials but this will be for the `west` cluster.

1. Verify that Kiali shows information from both clusters.

   1. Navigate to the `Overview` page from the left hand nav and verify you can see namespaces from both clusters.

   1. Navigate to the `Mesh` page from the left hand nav and verify you see both clusters on the mesh graph.
