# Troubleshooting

## Migrated deployment is not able to scale up
After restarting the deployment the new ReplicaSet is failing with:
`Error creating: Internal error occurred: failed calling webhook "rev.namespace.sidecar-injector.istio.io": failed to call webhook: Post "https://istiod-istio-sample.istio-system.svc:443/inject?timeout=10s": context deadline exceeded`

This error means the workload is not able to communicate with the new control plane. It might be caused by existing `NetworkPolicies` in the control plane namespace.

1. Verify that `spec.security.manageNetworkPolicy=false` is set in your `ServiceMeshControlPlane` resource. By default the 2.6 control plane is creating `NetworkPolicies` which are blocking the traffic for 3.0 control plane. It must be disabled by `spec.security.manageNetworkPolicy=false` and re-created manually.
1. Verify there are no other  `NetworkPolicies` which would be blocking the traffic.

## Migrated deployment is not able to communicate with not yet migrated deployment
1. Verify there are no `NetworkPolicies` which would be blocking the traffic:

    For both control planes, during the migration ensure that network policies do not block traffic between the following entities:

        - The control plane and the data plane namespaces
        - The data plane namespaces and the control plane
        - The data plane namespaces themselves
1. Verify that both control planes have access to all namespaces in the mesh. During the migration, some proxies are controlled by the 3.0 control plane while other proxies remain controlled by the 2.6 control plane. To ensure that mesh communication works during the migration, both control planes must detect the same set of services. Service discovery is provided by istiod component, which runs in the control plane namespace. In 2.6 the `ClusterWide` mode is watching all namespaces by default. `MultiTenant` mode only watches namespaces which are part of the mesh. In 3.0 the control plane watches all namespaces by default unless it's limited e.g. by `discoverySelectors`.

## Newly added workloads during the migration fails to start
Although it's not recommended, it might be necessary to add new namespaces to the mesh in the middle of the migration process. The init container might be failing with:

    `MountVolume.SetUp failed for volume "istiod-ca-cert" : configmap "istio-ca-root-cert" not found`

This might be happening if 2.6 control plane is running in the `MultiTenant` mode. `MultiTenant` mode by design only interacts with namespaces which are part of the mesh. This means it will not add `istio-ca-root-cert` ConfigMap to newly created namespaces which are not part of the 2.6 mesh. As the migration procedure is installing 3.0 control plane to the namespace where 2.6 control plane is running, the leader election process will be triggered and only one of the control planes will be in charge of creating `istio-ca-root-cert` ConfigMap. In case the 2.6 control plane is elected, newly namespaces which are correctly labeled to be managed by 3.0 control plane will not contain `istio-ca-root-cert` ConfigMap and any side car injection attempts will fail. This issue is not visible in `ClusterWide` mode as the 2.6 control plane watches all namespaces by default.

To workaround this problem it's necessary to restart the 2.6 control plane so 3.0 control plane becomes a leader and creates the `istio-ca-root-cert` ConfigMap.

## Migrated workload fails to start
The init container is failing with:

    `Failed to create pod sandbox: rpc error: code = Unknown desc = failed to create pod network sandbox k8s_httpbin-5746ccddc6-tgvqz_httpbin-3-strict_db236765-6ccd-4dbe-a079-19b5f878f329_0(5c5ffe21391046b205b002cb35ee2ccb17badf5bfab3942a0cae1db1a1d5b3f4): error adding pod httpbin-3-strict_httpbin-5746ccddc6-tgvqz to CNI network "multus-cni-network": plugin type="multus-shim" name="multus-cni-network" failed (add): CmdAdd (shim): CNI request failed with status 400: 'ContainerID:"5c5ffe21391046b205b002cb35ee2ccb17badf5bfab3942a0cae1db1a1d5b3f4" Netns:"/var/run/netns/0938ea9a-401d-484b-b174-53f987811d6f" IfName:"eth0" Args:"IgnoreUnknown=1;K8S_POD_NAMESPACE=httpbin-3-strict;K8S_POD_NAME=httpbin-5746ccddc6-tgvqz;K8S_POD_INFRA_CONTAINER_ID=5c5ffe21391046b205b002cb35ee2ccb17badf5bfab3942a0cae1db1a1d5b3f4;K8S_POD_UID=db236765-6ccd-4dbe-a079-19b5f878f329" Path:"" ERRORED: error configuring pod [httpbin-3-strict/httpbin-5746ccddc6-tgvqz] networking: [httpbin-3-strict/httpbin-5746ccddc6-tgvqz/db236765-6ccd-4dbe-a079-19b5f878f329:v2-6-istio-cni]: error adding container to network "v2-6-istio-cni": exit status 1 ': StdinData: {"auxiliaryCNIChainName":"vendor-cni-chain","binDir":"/var/lib/cni/bin","clusterNetwork":"/host/run/multus/cni/net.d/10-ovn-kubernetes.conf","cniVersion":"0.3.1","daemonSocketDir":"/run/multus/`

This error is visible when both control planes are trying to inject the side car proxy. To avoid this, `maistra.io/ignore-namespace: "true"` must be added. This label disables sidecar injection for OpenShift Service Mesh 2.6 proxies in the namespace. With the label applied, OpenShift Service Mesh 2.6 stops injecting proxies in this namespace, and any new proxies are injected by OpenShift Service Mesh 3.0. Without this label, the OpenShift Service Mesh 2.6 injection webhook tries to inject the pod and the injected sidecar proxy refuses to start since it will have both the OpenShift Service Mesh 2.6 and the OpenShift Service Mesh 3.0 Container Network Interface(CNI) annotations.

## Migrated workload is not injected at all
Migrated workload is running without a side car:
```sh
oc get pods -n bookinfo
NAME                                      READY   STATUS    RESTARTS   AGE
productpage-v1-7559db9df5-xgp7j           1/1     Running   0          23m
```

1. Verify that injection labels are set correctly on the namespace and/or deployments:

    `istio-injection=enabled` label works only when the IstioRevision is called `default` or the `default` IstioRevisionTag exists. In other cases, `istio.io/rev` label must be used.
1. When using `istio.io/rev` label, make sure the namespace is not labeled also with `istio-injection=enabled` as `istio-injection` takes precedence over `istio.io/rev` label
