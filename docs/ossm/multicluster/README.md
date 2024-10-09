# About OpenShift Service Mesh deployment models
Red Hat OpenShift Service Mesh supports several different deployment models that can be combined in different ways to best suit your business requirements.

## Multicluster deployments models
The multicluster deployment model is a way to deploy OpenShift Service Mesh across multiple OpenShift Container Platform clusters. This model is useful when you have multiple clusters that need to communicate with each other, but you want to keep them separate for security or compliance reasons.

Prerequisites:
* `istioctl` installed in your local machine.
* `oc` installed in your local machine.
* Two OpenShift Container Platform clusters with version 4.16 or higher.
* OpenShift Service Mesh 3 operator installed (See installation instructions[Add here the link to the docs section when is done]) on each cluster.
* kubeconfig file with a context for each cluster.

**Important:** Before configure OpenShift Service Mesh in a multicluster environment, you need to complete a few common steps:

These steps are common to every multicluster deployment and should be completed after meeting the prerequisites but before starting on a specific deployment model.

* Setup env vars. This to avoid to set the `--kubeconfig` flag in every command.
```bash
export KUBECONFIG_1=<path-to-kubeconfig-cluster-1>
export KUBECONFIG_2=<path-to-kubeconfig-cluster-2>
export ISTIO_VERSION=1.23.0
```
Set the `ISTIO_VERSION` to the version of Istio that you want to install using the OpenShift Service Mesh 3 operator (Check current available Istio version in the OpenShift Service Mesh 3).

* Create `istio-system` namespace on each cluster.
```bash
oc --kubeconfig "${KUBECONFIG_1}" create namespace istio-system
oc --kubeconfig "${KUBECONFIG_2}" create namespace istio-system
```

* Create shared trust and add intermediate CAs to each cluster.
If you already have a [shared trust](https://istio.io/latest/docs/setup/install/multicluster/before-you-begin/#configure-trust) for each cluster you can skip this. Otherwise, you can use the instructions below to create a shared trust and push the intermediate CAs into your clusters.

    * Generate certificates:
```bash
# Create Root Certificates:
root_ca_dir=cacerts/root
mkdir -p $root_ca_dir

openssl genrsa -out ${root_ca_dir}/root-key.pem 4096
cat <<EOF > ${root_ca_dir}/root-ca.conf
[ req ]
encrypt_key = no
prompt = no
utf8 = yes
default_md = sha256
default_bits = 4096
req_extensions = req_ext
x509_extensions = req_ext
distinguished_name = req_dn
[ req_ext ]
subjectKeyIdentifier = hash
basicConstraints = critical, CA:true
keyUsage = critical, digitalSignature, nonRepudiation, keyEncipherment, keyCertSign
[ req_dn ]
O = Istio
CN = Root CA
EOF

openssl req -sha256 -new -key ${root_ca_dir}/root-key.pem \
  -config ${root_ca_dir}/root-ca.conf \
  -out ${root_ca_dir}/root-cert.csr

openssl x509 -req -sha256 -days 3650 \
  -signkey ${root_ca_dir}/root-key.pem \
  -extensions req_ext -extfile ${root_ca_dir}/root-ca.conf \
  -in ${root_ca_dir}/root-cert.csr \
  -out ${root_ca_dir}/root-cert.pem

# Create Intermediate Certificates:
for cluster in west east; do
  int_ca_dir=cacerts/${cluster}
  mkdir $int_ca_dir

  openssl genrsa -out ${int_ca_dir}/ca-key.pem 4096
  cat <<EOF > ${int_ca_dir}/intermediate.conf
[ req ]
encrypt_key = no
prompt = no
utf8 = yes
default_md = sha256
default_bits = 4096
req_extensions = req_ext
x509_extensions = req_ext
distinguished_name = req_dn
[ req_ext ]
subjectKeyIdentifier = hash
basicConstraints = critical, CA:true, pathlen:0
keyUsage = critical, digitalSignature, nonRepudiation, keyEncipherment, keyCertSign
subjectAltName=@san
[ san ]
DNS.1 = istiod.istio-system.svc
[ req_dn ]
O = Istio
CN = Intermediate CA
L = $cluster
EOF

  openssl req -new -config ${int_ca_dir}/intermediate.conf \
    -key ${int_ca_dir}/ca-key.pem \
    -out ${int_ca_dir}/cluster-ca.csr

  openssl x509 -req -sha256 -days 3650 \
    -CA ${root_ca_dir}/root-cert.pem \
    -CAkey ${root_ca_dir}/root-key.pem -CAcreateserial \
    -extensions req_ext -extfile ${int_ca_dir}/intermediate.conf \
    -in ${int_ca_dir}/cluster-ca.csr \
    -out ${int_ca_dir}/ca-cert.pem

  cat ${int_ca_dir}/ca-cert.pem ${root_ca_dir}/root-cert.pem \
    > ${int_ca_dir}/cert-chain.pem
  cp ${root_ca_dir}/root-cert.pem ${int_ca_dir}
done
```

    * Push the intermediate CAs to the clusters:
```bash
oc --kubeconfig "${KUBECONFIG_1}" label namespace istio-system topology.istio.io/network=network1
oc --kubeconfig "${KUBECONFIG_2}" label namespace istio-system topology.istio.io/network=network2

oc get secret -n istio-system --kubeconfig "${KUBECONFIG_1}" cacerts || oc --kubeconfig "${KUBECONFIG_1}" create secret generic cacerts -n istio-system \
  --from-file=cacerts/east/ca-cert.pem \
  --from-file=cacerts/east/ca-key.pem \
  --from-file=cacerts/east/root-cert.pem \
  --from-file=cacerts/east/cert-chain.pem

oc get secret -n istio-system --kubeconfig "${KUBECONFIG_2}" cacerts || oc --kubeconfig "${KUBECONFIG_2}" create secret generic cacerts -n istio-system \
  --from-file=cacerts/west/ca-cert.pem \
  --from-file=cacerts/west/ca-key.pem \
  --from-file=cacerts/west/root-cert.pem \
  --from-file=cacerts/west/cert-chain.pem
```

### Multicluster: Multi-Primary - Multi-Network
The Multi-Primary - Multi-Network deployment model is a way to deploy OpenShift Service Mesh across multiple OpenShift Container Platform clusters with multiple control planes, each cluster has its own control plane making each a primary cluster, also each cluster has its own network. This causes the clusters to be isolated from each other, and the services in one cluster cannot communicate with the services in another cluster. More information about this deployment model can be found [here](https://istio.io/latest/docs/setup/install/multicluster/multi-primary_multi-network/).

#### Procedure
1. Create an Istio and IstioCNI resources on cluster1.
```bash
oc apply --kubeconfig "${KUBECONFIG_1}" -f - <<EOF
apiVersion: sailoperator.io/v1alpha1
kind: Istio
metadata:
  name: default
spec:
  version: v${ISTIO_VERSION}
  namespace: istio-system
  values:
    global:
      meshID: mesh1
      multiCluster:
        clusterName: cluster1
      network: network1
    pilot:
      env:
        ROOT_CA_DIR: "/etc/cacerts"
EOF
oc wait --kubeconfig "${KUBECONFIG_1}" --for=condition=Ready istios/default --timeout=3m
```

```bash
oc create namespace istio-cni --kubeconfig "${KUBECONFIG_1}" 
oc apply --kubeconfig "${KUBECONFIG_1}" -f - <<EOF
apiVersion: sailoperator.io/v1alpha1
kind: IstioCNI
metadata:
  name: default
spec:
  version: v${ISTIO_VERSION}
  namespace: istio-cni
EOF
```

2. Create east-west gateway on cluster1.
```bash
oc apply --kubeconfig "${KUBECONFIG_1}" -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    app: istio-eastwestgateway
    install.operator.istio.io/owning-resource: unknown
    istio: eastwestgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
    topology.istio.io/network: network1
  name: istio-eastwestgateway-service-account
  namespace: istio-system

---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: istio-eastwestgateway
    install.operator.istio.io/owning-resource: unknown
    istio: eastwestgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
    topology.istio.io/network: network1
  name: istio-eastwestgateway
  namespace: istio-system
spec:
  selector:
    matchLabels:
      app: istio-eastwestgateway
      istio: eastwestgateway
      topology.istio.io/network: network1
  strategy:
    rollingUpdate:
      maxSurge: 100%
      maxUnavailable: 25%
  template:
    metadata:
      annotations:
        inject.istio.io/templates: gateway
        prometheus.io/path: /stats/prometheus
        prometheus.io/port: "15020"
        prometheus.io/scrape: "true"
        sidecar.istio.io/inject: "true"
      labels:
        app: istio-eastwestgateway
        chart: gateways
        heritage: Tiller
        install.operator.istio.io/owning-resource: unknown
        istio: eastwestgateway
        operator.istio.io/component: IngressGateways
        release: istio
        sidecar.istio.io/inject: "true"
        topology.istio.io/network: network1
    spec:
      affinity:
        nodeAffinity:
          preferredDuringSchedulingIgnoredDuringExecution: null
          requiredDuringSchedulingIgnoredDuringExecution: null
      containers:
        - env:
            - name: ISTIO_META_REQUESTED_NETWORK_VIEW
              value: network1
            - name: ISTIO_META_UNPRIVILEGED_POD
              value: "true"
          image: auto
          name: istio-proxy
          ports:
            - containerPort: 15021
              protocol: TCP
            - containerPort: 15443
              protocol: TCP
            - containerPort: 15012
              protocol: TCP
            - containerPort: 15017
              protocol: TCP
            - containerPort: 15090
              name: http-envoy-prom
              protocol: TCP
          resources:
            limits:
              cpu: 2000m
              memory: 1024Mi
            requests:
              cpu: 100m
              memory: 128Mi
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
            privileged: false
            readOnlyRootFilesystem: true
          volumeMounts:
            - mountPath: /etc/istio/ingressgateway-certs
              name: ingressgateway-certs
              readOnly: true
            - mountPath: /etc/istio/ingressgateway-ca-certs
              name: ingressgateway-ca-certs
              readOnly: true
      securityContext:
        runAsNonRoot: true
      serviceAccountName: istio-eastwestgateway-service-account
      volumes:
        - name: ingressgateway-certs
          secret:
            optional: true
            secretName: istio-ingressgateway-certs
        - name: ingressgateway-ca-certs
          secret:
            optional: true
            secretName: istio-ingressgateway-ca-certs

---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  labels:
    app: istio-eastwestgateway
    install.operator.istio.io/owning-resource: unknown
    istio: eastwestgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
    topology.istio.io/network: network1
  name: istio-eastwestgateway
  namespace: istio-system
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: istio-eastwestgateway
      istio: eastwestgateway
      topology.istio.io/network: network1

---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  labels:
    install.operator.istio.io/owning-resource: unknown
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
  name: istio-eastwestgateway-sds
  namespace: istio-system
rules:
  - apiGroups:
      - ""
    resources:
      - secrets
    verbs:
      - get
      - watch
      - list

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  labels:
    install.operator.istio.io/owning-resource: unknown
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
  name: istio-eastwestgateway-sds
  namespace: istio-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: istio-eastwestgateway-sds
subjects:
  - kind: ServiceAccount
    name: istio-eastwestgateway-service-account

---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  labels:
    app: istio-eastwestgateway
    install.operator.istio.io/owning-resource: unknown
    istio: eastwestgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
    topology.istio.io/network: network1
  name: istio-eastwestgateway
  namespace: istio-system
spec:
  maxReplicas: 5
  metrics:
    - resource:
        name: cpu
        target:
          averageUtilization: 80
          type: Utilization
      type: Resource
  minReplicas: 1
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: istio-eastwestgateway

---
apiVersion: v1
kind: Service
metadata:
  annotations: null
  labels:
    app: istio-eastwestgateway
    install.operator.istio.io/owning-resource: unknown
    istio: eastwestgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
    topology.istio.io/network: network1
  name: istio-eastwestgateway
  namespace: istio-system
spec:
  ports:
    - name: status-port
      port: 15021
      protocol: TCP
      targetPort: 15021
    - name: tls
      port: 15443
      protocol: TCP
      targetPort: 15443
    - name: tls-istiod
      port: 15012
      protocol: TCP
      targetPort: 15012
    - name: tls-webhook
      port: 15017
      protocol: TCP
      targetPort: 15017
  selector:
    app: istio-eastwestgateway
    istio: eastwestgateway
    topology.istio.io/network: network1
  type: LoadBalancer
---
EOF
```

3. Expose services on cluster1.
```bash
oc --kubeconfig "${KUBECONFIG_1}" apply -n istio-system -f - <<EOF
apiVersion: networking.istio.io/v1alpha3
kind: Gateway
metadata:
  name: cross-network-gateway
spec:
  selector:
    istio: eastwestgateway
  servers:
    - port:
        number: 15443
        name: tls
        protocol: TLS
      tls:
        mode: AUTO_PASSTHROUGH
      hosts:
        - "*.local"
EOF
```

4. Create Istio and IstioCNI resources on cluster2.
```bash
oc apply --kubeconfig "${KUBECONFIG_2}" -f - <<EOF
apiVersion: sailoperator.io/v1alpha1
kind: Istio
metadata:
  name: default
spec:
  version: v${ISTIO_VERSION}
  namespace: istio-system
  values:
    global:
      meshID: mesh1
      multiCluster:
        clusterName: cluster2
      network: network2
    pilot:
      env:
        ROOT_CA_DIR: "/etc/cacerts"
EOF
oc wait --kubeconfig "${KUBECONFIG_2}" --for=jsonpath='{.status.revisions.ready}'=1 istios/default --timeout=3m
```

```bash
oc create namespace istio-cni --kubeconfig "${KUBECONFIG_2}"
oc apply --kubeconfig "${KUBECONFIG_2}" -f - <<EOF
apiVersion: sailoperator.io/v1alpha1
kind: IstioCNI
metadata:
  name: default
spec:
  version: v${ISTIO_VERSION}
  namespace: istio-cni
EOF
```

5. Create east-west gateway on cluster2.
```bash
oc apply --kubeconfig "${KUBECONFIG_2}" -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    app: istio-eastwestgateway
    install.operator.istio.io/owning-resource: unknown
    istio: eastwestgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
    topology.istio.io/network: network2
  name: istio-eastwestgateway-service-account
  namespace: istio-system

---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: istio-eastwestgateway
    install.operator.istio.io/owning-resource: unknown
    istio: eastwestgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
    topology.istio.io/network: network2
  name: istio-eastwestgateway
  namespace: istio-system
spec:
  selector:
    matchLabels:
      app: istio-eastwestgateway
      istio: eastwestgateway
      topology.istio.io/network: network2
  strategy:
    rollingUpdate:
      maxSurge: 100%
      maxUnavailable: 25%
  template:
    metadata:
      annotations:
        inject.istio.io/templates: gateway
        prometheus.io/path: /stats/prometheus
        prometheus.io/port: "15020"
        prometheus.io/scrape: "true"
        sidecar.istio.io/inject: "true"
      labels:
        app: istio-eastwestgateway
        chart: gateways
        heritage: Tiller
        install.operator.istio.io/owning-resource: unknown
        istio: eastwestgateway
        operator.istio.io/component: IngressGateways
        release: istio
        sidecar.istio.io/inject: "true"
        topology.istio.io/network: network2
    spec:
      affinity:
        nodeAffinity:
          preferredDuringSchedulingIgnoredDuringExecution: null
          requiredDuringSchedulingIgnoredDuringExecution: null
      containers:
        - env:
            - name: ISTIO_META_REQUESTED_NETWORK_VIEW
              value: network2
            - name: ISTIO_META_UNPRIVILEGED_POD
              value: "true"
          image: auto
          name: istio-proxy
          ports:
            - containerPort: 15021
              protocol: TCP
            - containerPort: 15443
              protocol: TCP
            - containerPort: 15012
              protocol: TCP
            - containerPort: 15017
              protocol: TCP
            - containerPort: 15090
              name: http-envoy-prom
              protocol: TCP
          resources:
            limits:
              cpu: 2000m
              memory: 1024Mi
            requests:
              cpu: 100m
              memory: 128Mi
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
            privileged: false
            readOnlyRootFilesystem: true
          volumeMounts:
            - mountPath: /etc/istio/ingressgateway-certs
              name: ingressgateway-certs
              readOnly: true
            - mountPath: /etc/istio/ingressgateway-ca-certs
              name: ingressgateway-ca-certs
              readOnly: true
      securityContext:
        runAsNonRoot: true
      serviceAccountName: istio-eastwestgateway-service-account
      volumes:
        - name: ingressgateway-certs
          secret:
            optional: true
            secretName: istio-ingressgateway-certs
        - name: ingressgateway-ca-certs
          secret:
            optional: true
            secretName: istio-ingressgateway-ca-certs

---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  labels:
    app: istio-eastwestgateway
    install.operator.istio.io/owning-resource: unknown
    istio: eastwestgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
    topology.istio.io/network: network2
  name: istio-eastwestgateway
  namespace: istio-system
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: istio-eastwestgateway
      istio: eastwestgateway
      topology.istio.io/network: network2

---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  labels:
    install.operator.istio.io/owning-resource: unknown
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
  name: istio-eastwestgateway-sds
  namespace: istio-system
rules:
  - apiGroups:
      - ""
    resources:
      - secrets
    verbs:
      - get
      - watch
      - list

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  labels:
    install.operator.istio.io/owning-resource: unknown
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
  name: istio-eastwestgateway-sds
  namespace: istio-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: istio-eastwestgateway-sds
subjects:
  - kind: ServiceAccount
    name: istio-eastwestgateway-service-account

---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  labels:
    app: istio-eastwestgateway
    install.operator.istio.io/owning-resource: unknown
    istio: eastwestgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
    topology.istio.io/network: network2
  name: istio-eastwestgateway
  namespace: istio-system
spec:
  maxReplicas: 5
  metrics:
    - resource:
        name: cpu
        target:
          averageUtilization: 80
          type: Utilization
      type: Resource
  minReplicas: 1
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: istio-eastwestgateway

---
apiVersion: v1
kind: Service
metadata:
  annotations: null
  labels:
    app: istio-eastwestgateway
    install.operator.istio.io/owning-resource: unknown
    istio: eastwestgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
    topology.istio.io/network: network2
  name: istio-eastwestgateway
  namespace: istio-system
spec:
  ports:
    - name: status-port
      port: 15021
      protocol: TCP
      targetPort: 15021
    - name: tls
      port: 15443
      protocol: TCP
      targetPort: 15443
    - name: tls-istiod
      port: 15012
      protocol: TCP
      targetPort: 15012
    - name: tls-webhook
      port: 15017
      protocol: TCP
      targetPort: 15017
  selector:
    app: istio-eastwestgateway
    istio: eastwestgateway
    topology.istio.io/network: network2
  type: LoadBalancer

---
EOF
```

6. Expose services on cluster2.
```bash
oc --kubeconfig "${KUBECONFIG_2}" apply -n istio-system -f - <<EOF
apiVersion: networking.istio.io/v1alpha3
kind: Gateway
metadata:
  name: cross-network-gateway
spec:
  selector:
    istio: eastwestgateway
  servers:
    - port:
        number: 15443
        name: tls
        protocol: TLS
      tls:
        mode: AUTO_PASSTHROUGH
      hosts:
        - "*.local"
EOF
```

7. Install a remote secret in cluster2 that provides access to the cluster1 API server.
```bash
istioctl create-remote-secret \
  --kubeconfig "${KUBECONFIG_1}" \
  --name=cluster1 |\
  oc apply -f - --kubeconfig "${KUBECONFIG_2}"
```

8. Install a remote secret in cluster1 that provides access to the cluster2 API server.
```bash
istioctl create-remote-secret \
  --kubeconfig "${KUBECONFIG_2}" \
  --name=cluster2 |\
  oc apply -f - --kubeconfig "${KUBECONFIG_1}"
```

#### Verification
1. Deploy sample applications to cluster1.
We will be deploying the `helloworld` and `sleep` applications to cluster1.
```bash
oc get ns sample --kubeconfig "${KUBECONFIG_1}" || oc create --kubeconfig "${KUBECONFIG_1}" namespace sample
oc label --kubeconfig "${KUBECONFIG_1}" namespace sample istio-injection=enabled
oc apply --kubeconfig "${KUBECONFIG_1}" \
  -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml \
  -l service=helloworld -n sample
oc apply --kubeconfig "${KUBECONFIG_1}" \
  -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml \
  -l version=v1 -n sample
oc apply --kubeconfig "${KUBECONFIG_1}" \
  -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/sleep/sleep.yaml -n sample
```

2. Deploy sample applications to cluster2.
```bash
oc get ns sample --kubeconfig "${KUBECONFIG_2}" || oc create --kubeconfig "${KUBECONFIG_2}" namespace sample
oc label --kubeconfig "${KUBECONFIG_2}" namespace sample istio-injection=enabled
oc apply --kubeconfig "${KUBECONFIG_2}" \
  -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml \
  -l service=helloworld -n sample
oc apply --kubeconfig "${KUBECONFIG_2}" \
  -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml \
  -l version=v2 -n sample
oc apply --kubeconfig "${KUBECONFIG_2}" \
  -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/sleep/sleep.yaml -n sample
```

3. Verify that you see a response from both v1 and v2.
```bash
for i in $(seq 10); do oc exec -n sample "$(oc get pod --kubeconfig "${KUBECONFIG_1}" -n sample -l \
    app=sleep -o jsonpath='{.items[0].metadata.name}')" --kubeconfig "${KUBECONFIG_1}" \
    -c sleep -- curl -s helloworld:5000/hello; done
```
You should see a response from v1 and v2 `helloworld` app. For example:
```
Hello version: v1, instance: helloworld-v1-69ff8fc747-w6h59
Hello version: v1, instance: helloworld-v1-69ff8fc747-w6h59
Hello version: v1, instance: helloworld-v1-69ff8fc747-w6h59
Hello version: v1, instance: helloworld-v1-69ff8fc747-w6h59
Hello version: v1, instance: helloworld-v1-69ff8fc747-w6h59
Hello version: v2, instance: helloworld-v2-779454bb5f-psfhk
Hello version: v1, instance: helloworld-v1-69ff8fc747-w6h59
Hello version: v2, instance: helloworld-v2-779454bb5f-psfhk
Hello version: v1, instance: helloworld-v1-69ff8fc747-w6h59
Hello version: v2, instance: helloworld-v2-779454bb5f-psfhk
```

```bash
for i in $(seq 10); do oc exec -n sample "$(oc get pod --kubeconfig "${KUBECONFIG_2}" -n sample -l \
    app=sleep -o jsonpath='{.items[0].metadata.name}')" --kubeconfig "${KUBECONFIG_2}" \
    -c sleep -- curl -s helloworld:5000/hello; done
```
You should see a response from v1 and v2 `helloworld` app. For example:
```
Hello version: v1, instance: helloworld-v1-69ff8fc747-w6h59
Hello version: v2, instance: helloworld-v2-779454bb5f-psfhk
Hello version: v1, instance: helloworld-v1-69ff8fc747-w6h59
Hello version: v2, instance: helloworld-v2-779454bb5f-psfhk
Hello version: v1, instance: helloworld-v1-69ff8fc747-w6h59
Hello version: v1, instance: helloworld-v1-69ff8fc747-w6h59
Hello version: v2, instance: helloworld-v2-779454bb5f-psfhk
Hello version: v1, instance: helloworld-v1-69ff8fc747-w6h59
Hello version: v2, instance: helloworld-v2-779454bb5f-psfhk
Hello version: v1, instance: helloworld-v1-69ff8fc747-w6h59
```

**Note:** If you have issues with the validation and configuration you can check the [debugging section](#debugging-multicluster-deployments).

#### Clean up.
```bash
oc delete istios default --kubeconfig "${KUBECONFIG_1}"
oc delete istiocni default --kubeconfig "${KUBECONFIG_1}"
oc delete ns istio-system --kubeconfig "${KUBECONFIG_1}"
oc delete ns istio-cni --kubeconfig "${KUBECONFIG_1}" 
oc delete ns sample --kubeconfig "${KUBECONFIG_1}"
oc delete istios default --kubeconfig "${KUBECONFIG_2}"
oc delete istiocni default --kubeconfig "${KUBECONFIG_2}"
oc delete ns istio-system --kubeconfig "${KUBECONFIG_2}" 
oc delete ns istio-cni --kubeconfig "${KUBECONFIG_2}"
oc delete ns sample --kubeconfig "${KUBECONFIG_2}"
```

### Multicluster: Multi-Primary - Single-Network
[TBD]

### Multicluster: Primary-Remote
These instructions install a [primary-remote/multi-network](https://istio.io/latest/docs/setup/install/multicluster/primary-remote_multi-network/) Istio deployment using the Sail Operator and Sail CRDs. Before you begin, ensure you complete the common setup.

#### Procedure
1. Create an Istio and IstioCNI resources on cluster1.
```bash
oc apply --kubeconfig "${KUBECONFIG_1}" -f - <<EOF
apiVersion: sailoperator.io/v1alpha1
kind: Istio
metadata:
  name: default
spec:
  version: v${ISTIO_VERSION}
  namespace: istio-system
  values:
    pilot:
      env:
        EXTERNAL_ISTIOD: "true"
        ROOT_CA_DIR: "/etc/cacerts"
    global:
      meshID: mesh1
      multiCluster:
        clusterName: cluster1
      network: network1        
EOF
oc wait --kubeconfig "${KUBECONFIG_1}" --for=jsonpath='{.status.revisions.ready}'=1 istios/default --timeout=3m
```

```bash
oc create namespace istio-cni --kubeconfig "${KUBECONFIG_1}" 
oc apply --kubeconfig "${KUBECONFIG_1}" -f - <<EOF
apiVersion: sailoperator.io/v1alpha1
kind: IstioCNI
metadata:
  name: default
spec:
  version: v${ISTIO_VERSION}
  namespace: istio-cni
EOF
```

2. Create east-west gateway on cluster1.
```bash
oc apply --kubeconfig "${KUBECONFIG_1}" -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    app: istio-eastwestgateway
    install.operator.istio.io/owning-resource: unknown
    istio: eastwestgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
    topology.istio.io/network: network1
  name: istio-eastwestgateway-service-account
  namespace: istio-system

---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: istio-eastwestgateway
    install.operator.istio.io/owning-resource: unknown
    istio: eastwestgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
    topology.istio.io/network: network1
  name: istio-eastwestgateway
  namespace: istio-system
spec:
  selector:
    matchLabels:
      app: istio-eastwestgateway
      istio: eastwestgateway
      topology.istio.io/network: network1
  strategy:
    rollingUpdate:
      maxSurge: 100%
      maxUnavailable: 25%
  template:
    metadata:
      annotations:
        inject.istio.io/templates: gateway
        prometheus.io/path: /stats/prometheus
        prometheus.io/port: "15020"
        prometheus.io/scrape: "true"
        sidecar.istio.io/inject: "true"
      labels:
        app: istio-eastwestgateway
        chart: gateways
        heritage: Tiller
        install.operator.istio.io/owning-resource: unknown
        istio: eastwestgateway
        operator.istio.io/component: IngressGateways
        release: istio
        sidecar.istio.io/inject: "true"
        topology.istio.io/network: network1
    spec:
      affinity:
        nodeAffinity:
          preferredDuringSchedulingIgnoredDuringExecution: null
          requiredDuringSchedulingIgnoredDuringExecution: null
      containers:
        - env:
            - name: ISTIO_META_REQUESTED_NETWORK_VIEW
              value: network1
            - name: ISTIO_META_UNPRIVILEGED_POD
              value: "true"
          image: auto
          name: istio-proxy
          ports:
            - containerPort: 15021
              protocol: TCP
            - containerPort: 15443
              protocol: TCP
            - containerPort: 15012
              protocol: TCP
            - containerPort: 15017
              protocol: TCP
            - containerPort: 15090
              name: http-envoy-prom
              protocol: TCP
          resources:
            limits:
              cpu: 2000m
              memory: 1024Mi
            requests:
              cpu: 100m
              memory: 128Mi
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
            privileged: false
            readOnlyRootFilesystem: true
          volumeMounts:
            - mountPath: /etc/istio/ingressgateway-certs
              name: ingressgateway-certs
              readOnly: true
            - mountPath: /etc/istio/ingressgateway-ca-certs
              name: ingressgateway-ca-certs
              readOnly: true
      securityContext:
        runAsNonRoot: true
      serviceAccountName: istio-eastwestgateway-service-account
      volumes:
        - name: ingressgateway-certs
          secret:
            optional: true
            secretName: istio-ingressgateway-certs
        - name: ingressgateway-ca-certs
          secret:
            optional: true
            secretName: istio-ingressgateway-ca-certs

---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  labels:
    app: istio-eastwestgateway
    install.operator.istio.io/owning-resource: unknown
    istio: eastwestgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
    topology.istio.io/network: network1
  name: istio-eastwestgateway
  namespace: istio-system
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: istio-eastwestgateway
      istio: eastwestgateway
      topology.istio.io/network: network1

---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  labels:
    install.operator.istio.io/owning-resource: unknown
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
  name: istio-eastwestgateway-sds
  namespace: istio-system
rules:
  - apiGroups:
      - ""
    resources:
      - secrets
    verbs:
      - get
      - watch
      - list

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  labels:
    install.operator.istio.io/owning-resource: unknown
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
  name: istio-eastwestgateway-sds
  namespace: istio-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: istio-eastwestgateway-sds
subjects:
  - kind: ServiceAccount
    name: istio-eastwestgateway-service-account

---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  labels:
    app: istio-eastwestgateway
    install.operator.istio.io/owning-resource: unknown
    istio: eastwestgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
    topology.istio.io/network: network1
  name: istio-eastwestgateway
  namespace: istio-system
spec:
  maxReplicas: 5
  metrics:
    - resource:
        name: cpu
        target:
          averageUtilization: 80
          type: Utilization
      type: Resource
  minReplicas: 1
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: istio-eastwestgateway

---
apiVersion: v1
kind: Service
metadata:
  annotations: null
  labels:
    app: istio-eastwestgateway
    install.operator.istio.io/owning-resource: unknown
    istio: eastwestgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
    topology.istio.io/network: network1
  name: istio-eastwestgateway
  namespace: istio-system
spec:
  ports:
    - name: status-port
      port: 15021
      protocol: TCP
      targetPort: 15021
    - name: tls
      port: 15443
      protocol: TCP
      targetPort: 15443
    - name: tls-istiod
      port: 15012
      protocol: TCP
      targetPort: 15012
    - name: tls-webhook
      port: 15017
      protocol: TCP
      targetPort: 15017
  selector:
    app: istio-eastwestgateway
    istio: eastwestgateway
    topology.istio.io/network: network1
  type: LoadBalancer

---
EOF
```

3. Expose istiod on cluster1.
```bash
oc apply --kubeconfig "${KUBECONFIG_1}" -n istio-system -f - <<EOF
apiVersion: networking.istio.io/v1alpha3
kind: Gateway
metadata:
  name: istiod-gateway
spec:
  selector:
    istio: eastwestgateway
  servers:
    - port:
        name: tls-istiod
        number: 15012
        protocol: tls
      tls:
        mode: PASSTHROUGH        
      hosts:
        - "*"
    - port:
        name: tls-istiodwebhook
        number: 15017
        protocol: tls
      tls:
        mode: PASSTHROUGH          
      hosts:
        - "*"
---
apiVersion: networking.istio.io/v1alpha3
kind: VirtualService
metadata:
  name: istiod-vs
spec:
  hosts:
  - "*"
  gateways:
  - istiod-gateway
  tls:
  - match:
    - port: 15012
      sniHosts:
      - "*"
    route:
    - destination:
        host: istiod.istio-system.svc.cluster.local
        port:
          number: 15012
  - match:
    - port: 15017
      sniHosts:
      - "*"
    route:
    - destination:
        host: istiod.istio-system.svc.cluster.local
        port:
          number: 443
---
EOF
```
4. Expose services on cluster1.
```bash
oc --kubeconfig "${KUBECONFIG_1}" apply -n istio-system -f - <<EOF
apiVersion: networking.istio.io/v1alpha3
kind: Gateway
metadata:
  name: cross-network-gateway
spec:
  selector:
    istio: eastwestgateway
  servers:
    - port:
        number: 15443
        name: tls
        protocol: TLS
      tls:
        mode: AUTO_PASSTHROUGH
      hosts:
        - "*.local"
EOF
```

5. Create RemoteIstio and IstionCNI resources on cluster2.
```bash
export EXTERNAL_ISTIOD_ADDR=<hostname-from-loadbalancer-istio-eastwestgateway-service>
oc apply --kubeconfig "${KUBECONFIG_2}" -f - <<EOF
apiVersion: sailoperator.io/v1alpha1
kind: RemoteIstio
metadata:
  name: default
spec:
  version: v${ISTIO_VERSION}
  namespace: istio-system
  values:
    istiodRemote:
      injectionPath: /inject/cluster/remote/net/network2
      injectionURL: https://${EXTERNAL_ISTIOD_ADDR}:15017/inject/cluster/cluster1/net/network1
    base:
      validationURL: https://${EXTERNAL_ISTIOD_ADDR}:15017/validate
EOF
```
**Note**: `injectionURL` should be used if the loadbalancer configured use hostname instead of ExternalIP. In case you have ExternalIP available you can set `remotePilotAddress` to the `istio-eastwestgateway` service in cluster1 by deleting `spec.values.istiodRemote.injectionURL` and `spec.values.base.validationURL` and adding `spec.values.global.remotePilotAddress` under `spec.values.`:
```bash
    global:
      remotePilotAddress: $(oc --kubeconfig "${KUBECONFIG_1}" -n istio-system get svc istio-eastwestgateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
```

5. Set the controlplane cluster and network for cluster2.
```bash
oc --kubeconfig "${KUBECONFIG_2}" annotate namespace istio-system topology.istio.io/controlPlaneClusters=cluster1
oc --kubeconfig "${KUBECONFIG_2}" label namespace istio-system topology.istio.io/network=network2
```

6. Install a remote secret on cluster1 that provides access to the cluster2 API server.
```bash
istioctl create-remote-secret \
  --kubeconfig "${KUBECONFIG_2}" \
  --name=remote | \
  oc apply -f - --kubeconfig "${KUBECONFIG_1}"
```

7. Install east-west gateway in cluster2.
```bash
oc apply --kubeconfig "${KUBECONFIG_2}" -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    app: istio-eastwestgateway
    install.operator.istio.io/owning-resource: unknown
    istio: eastwestgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
    topology.istio.io/network: network2
  name: istio-eastwestgateway-service-account
  namespace: istio-system

---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: istio-eastwestgateway
    install.operator.istio.io/owning-resource: unknown
    istio: eastwestgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
    topology.istio.io/network: network2
  name: istio-eastwestgateway
  namespace: istio-system
spec:
  selector:
    matchLabels:
      app: istio-eastwestgateway
      istio: eastwestgateway
      topology.istio.io/network: network2
  strategy:
    rollingUpdate:
      maxSurge: 100%
      maxUnavailable: 25%
  template:
    metadata:
      annotations:
        inject.istio.io/templates: gateway
        prometheus.io/path: /stats/prometheus
        prometheus.io/port: "15020"
        prometheus.io/scrape: "true"
        sidecar.istio.io/inject: "true"
      labels:
        app: istio-eastwestgateway
        chart: gateways
        heritage: Tiller
        install.operator.istio.io/owning-resource: unknown
        istio: eastwestgateway
        operator.istio.io/component: IngressGateways
        release: istio
        sidecar.istio.io/inject: "true"
        topology.istio.io/network: network2
    spec:
      affinity:
        nodeAffinity:
          preferredDuringSchedulingIgnoredDuringExecution: null
          requiredDuringSchedulingIgnoredDuringExecution: null
      containers:
        - env:
            - name: ISTIO_META_REQUESTED_NETWORK_VIEW
              value: network2
            - name: ISTIO_META_UNPRIVILEGED_POD
              value: "true"
          image: auto
          name: istio-proxy
          ports:
            - containerPort: 15021
              protocol: TCP
            - containerPort: 15443
              protocol: TCP
            - containerPort: 15012
              protocol: TCP
            - containerPort: 15017
              protocol: TCP
            - containerPort: 15090
              name: http-envoy-prom
              protocol: TCP
          resources:
            limits:
              cpu: 2000m
              memory: 1024Mi
            requests:
              cpu: 100m
              memory: 128Mi
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
            privileged: false
            readOnlyRootFilesystem: true
          volumeMounts:
            - mountPath: /etc/istio/ingressgateway-certs
              name: ingressgateway-certs
              readOnly: true
            - mountPath: /etc/istio/ingressgateway-ca-certs
              name: ingressgateway-ca-certs
              readOnly: true
      securityContext:
        runAsNonRoot: true
      serviceAccountName: istio-eastwestgateway-service-account
      volumes:
        - name: ingressgateway-certs
          secret:
            optional: true
            secretName: istio-ingressgateway-certs
        - name: ingressgateway-ca-certs
          secret:
            optional: true
            secretName: istio-ingressgateway-ca-certs

---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  labels:
    app: istio-eastwestgateway
    install.operator.istio.io/owning-resource: unknown
    istio: eastwestgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
    topology.istio.io/network: network2
  name: istio-eastwestgateway
  namespace: istio-system
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: istio-eastwestgateway
      istio: eastwestgateway
      topology.istio.io/network: network2

---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  labels:
    install.operator.istio.io/owning-resource: unknown
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
  name: istio-eastwestgateway-sds
  namespace: istio-system
rules:
  - apiGroups:
      - ""
    resources:
      - secrets
    verbs:
      - get
      - watch
      - list

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  labels:
    install.operator.istio.io/owning-resource: unknown
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
  name: istio-eastwestgateway-sds
  namespace: istio-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: istio-eastwestgateway-sds
subjects:
  - kind: ServiceAccount
    name: istio-eastwestgateway-service-account

---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  labels:
    app: istio-eastwestgateway
    install.operator.istio.io/owning-resource: unknown
    istio: eastwestgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
    topology.istio.io/network: network2
  name: istio-eastwestgateway
  namespace: istio-system
spec:
  maxReplicas: 5
  metrics:
    - resource:
        name: cpu
        target:
          averageUtilization: 80
          type: Utilization
      type: Resource
  minReplicas: 1
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: istio-eastwestgateway

---
apiVersion: v1
kind: Service
metadata:
  annotations: null
  labels:
    app: istio-eastwestgateway
    install.operator.istio.io/owning-resource: unknown
    istio: eastwestgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
    topology.istio.io/network: network2
  name: istio-eastwestgateway
  namespace: istio-system
spec:
  ports:
    - name: status-port
      port: 15021
      protocol: TCP
      targetPort: 15021
    - name: tls
      port: 15443
      protocol: TCP
      targetPort: 15443
    - name: tls-istiod
      port: 15012
      protocol: TCP
      targetPort: 15012
    - name: tls-webhook
      port: 15017
      protocol: TCP
      targetPort: 15017
  selector:
    app: istio-eastwestgateway
    istio: eastwestgateway
    topology.istio.io/network: network2
  type: LoadBalancer

---
EOF
```

#### Verification
1. Deploy sample applications to cluster1.
We will be deploying the `helloworld` and `sleep` applications to cluster1.
```bash
oc get ns sample --kubeconfig "${KUBECONFIG_1}" || oc create --kubeconfig "${KUBECONFIG_1}" namespace sample
oc label --kubeconfig "${KUBECONFIG_1}" namespace sample istio-injection=enabled
oc apply --kubeconfig "${KUBECONFIG_1}" \
  -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml \
  -l service=helloworld -n sample
oc apply --kubeconfig "${KUBECONFIG_1}" \
  -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml \
  -l version=v1 -n sample
oc apply --kubeconfig "${KUBECONFIG_1}" \
  -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/sleep/sleep.yaml -n sample
```

2. Deploy sample applications to cluster2.
```bash
oc get ns sample --kubeconfig "${KUBECONFIG_2}" || oc create --kubeconfig "${KUBECONFIG_2}" namespace sample
oc label --kubeconfig "${KUBECONFIG_2}" namespace sample istio-injection=enabled
oc apply --kubeconfig "${KUBECONFIG_2}" \
  -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml \
  -l service=helloworld -n sample
oc apply --kubeconfig "${KUBECONFIG_2}" \
  -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml \
  -l version=v2 -n sample
oc apply --kubeconfig "${KUBECONFIG_2}" \
  -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/sleep/sleep.yaml -n sample
```

3. Verify that you see a response from both v1 and v2.
```bash
for i in $(seq 10); do oc exec -n sample "$(oc get pod --kubeconfig "${KUBECONFIG_1}" -n sample -l \
    app=sleep -o jsonpath='{.items[0].metadata.name}')" --kubeconfig "${KUBECONFIG_1}" \
    -c sleep -- curl -s helloworld:5000/hello; done
```
You should see a response from v1 and v2 `helloworld` app on cluster1.

```bash
for i in $(seq 10); do oc exec -n sample "$(oc get pod --kubeconfig "${KUBECONFIG_2}" -n sample -l \
    app=sleep -o jsonpath='{.items[0].metadata.name}')" --kubeconfig "${KUBECONFIG_2}" \
    -c sleep -- curl -s helloworld:5000/hello; done
```
You should see a response from v1 and v2 `helloworld` app on cluster2.

**Note:** If you have issues with the validation you can check the [debugging section](#debugging-multicluster-deployments).

#### Clean up.
```bash
oc delete istios default --kubeconfig "${KUBECONFIG_1}"
oc delete istiocni default --kubeconfig "${KUBECONFIG_1}"
oc delete ns istio-system --kubeconfig "${KUBECONFIG_1}"
oc delete ns istio-cni --kubeconfig "${KUBECONFIG_1}" 
oc delete ns sample --kubeconfig "${KUBECONFIG_1}"
oc delete remoteistio default --kubeconfig "${KUBECONFIG_2}"
oc delete ns istio-system --kubeconfig "${KUBECONFIG_2}" 
oc delete ns sample --kubeconfig "${KUBECONFIG_2}"
```

### Multicluster: External Control Plane
These instructions install an [external control plane](https://istio.io/latest/docs/setup/install/external-controlplane/) Istio deployment using the Sail Operator and Sail CRDs. In this case, you will not follow the common setup.

In this setup there is an external control plane cluster (cluster1) and a remote cluster (cluster2) which are on separate networks. The control plane cluster is responsible for managing the remote cluster.

#### Procedure
1. Export the env vars.
```bash
export KUBECONFIG_1=</path/to/kubeconfig1>
export KUBECONFIG_2=</path/to/kubeconfig2>
export ISTIO_VERSION=1.23.0
```

2. Create an Istio and IstioCNI resources on cluster1 to manage the ingress gateways for the external control plane.
```bash
oc create namespace istio-system --kubeconfig "${KUBECONFIG_1}""
oc apply --kubeconfig "${KUBECONFIG_1}"" -f - <<EOF
apiVersion: sailoperator.io/v1alpha1
kind: Istio
metadata:
  name: default
spec:
  version: v${ISTIO_VERSION}
  namespace: istio-system
  global:
    network: network1
    pilot:
      env:
        ROOT_CA_DIR: "/etc/cacerts"
EOF
oc wait --kubeconfig "${KUBECONFIG_1}"" --for=condition=Ready istios/default --timeout=3m
```

```bash
oc create namespace istio-cni --kubeconfig "${KUBECONFIG_1}" 
oc apply --kubeconfig "${KUBECONFIG_1}" -f - <<EOF
apiVersion: sailoperator.io/v1alpha1
kind: IstioCNI
metadata:
  name: default
spec:
  version: v${ISTIO_VERSION}
  namespace: istio-cni
EOF
```

3. Create the ingress gateway for the external control plane.
```bash
oc --kubeconfig "${KUBECONFIG_1}"" apply -f - <<EOF
apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    app: istio-ingressgateway
    install.operator.istio.io/owning-resource: unknown
    istio: ingressgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
  name: istio-ingressgateway-service-account
  namespace: istio-system

---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: istio-ingressgateway
    install.operator.istio.io/owning-resource: unknown
    istio: ingressgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
  name: istio-ingressgateway
  namespace: istio-system
spec:
  selector:
    matchLabels:
      app: istio-ingressgateway
      istio: ingressgateway
  strategy:
    rollingUpdate:
      maxSurge: 100%
      maxUnavailable: 25%
  template:
    metadata:
      annotations:
        istio.io/rev: default
        prometheus.io/path: /stats/prometheus
        prometheus.io/port: "15020"
        prometheus.io/scrape: "true"
        sidecar.istio.io/inject: "false"
      labels:
        app: istio-ingressgateway
        chart: gateways
        heritage: Tiller
        install.operator.istio.io/owning-resource: unknown
        istio: ingressgateway
        istio.io/rev: default
        operator.istio.io/component: IngressGateways
        release: istio
        service.istio.io/canonical-name: istio-ingressgateway
        service.istio.io/canonical-revision: latest
        sidecar.istio.io/inject: "false"
    spec:
      affinity:
        nodeAffinity:
          preferredDuringSchedulingIgnoredDuringExecution: null
          requiredDuringSchedulingIgnoredDuringExecution: null
      containers:
      - args:
        - proxy
        - router
        - --domain
        - $(POD_NAMESPACE).svc.cluster.local
        - --proxyLogLevel=warning
        - --proxyComponentLogLevel=misc:error
        - --log_output_level=default:info
        env:
        - name: PILOT_CERT_PROVIDER
          value: istiod
        - name: CA_ADDR
          value: istiod.istio-system.svc:15012
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        - name: POD_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
        - name: INSTANCE_IP
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: status.podIP
        - name: HOST_IP
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: status.hostIP
        - name: ISTIO_CPU_LIMIT
          valueFrom:
            resourceFieldRef:
              resource: limits.cpu
        - name: SERVICE_ACCOUNT
          valueFrom:
            fieldRef:
              fieldPath: spec.serviceAccountName
        - name: ISTIO_META_WORKLOAD_NAME
          value: istio-ingressgateway
        - name: ISTIO_META_OWNER
          value: kubernetes://apis/apps/v1/namespaces/istio-system/deployments/istio-ingressgateway
        - name: ISTIO_META_MESH_ID
          value: cluster.local
        - name: TRUST_DOMAIN
          value: cluster.local
        - name: ISTIO_META_UNPRIVILEGED_POD
          value: "true"
        - name: ISTIO_META_CLUSTER_ID
          value: Kubernetes
        - name: ISTIO_META_NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        image: docker.io/istio/proxyv2:1.22.1
        name: istio-proxy
        ports:
        - containerPort: 15021
          protocol: TCP
        - containerPort: 15012
          protocol: TCP
        - containerPort: 15017
          protocol: TCP
        - containerPort: 15090
          name: http-envoy-prom
          protocol: TCP
        readinessProbe:
          failureThreshold: 30
          httpGet:
            path: /healthz/ready
            port: 15021
            scheme: HTTP
          initialDelaySeconds: 1
          periodSeconds: 2
          successThreshold: 1
          timeoutSeconds: 1
        resources:
          limits:
            cpu: 2000m
            memory: 1024Mi
          requests:
            cpu: 100m
            memory: 128Mi
        securityContext:
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
          privileged: false
          readOnlyRootFilesystem: true
        volumeMounts:
        - mountPath: /var/run/secrets/workload-spiffe-uds
          name: workload-socket
        - mountPath: /var/run/secrets/credential-uds
          name: credential-socket
        - mountPath: /var/run/secrets/workload-spiffe-credentials
          name: workload-certs
        - mountPath: /etc/istio/proxy
          name: istio-envoy
        - mountPath: /etc/istio/config
          name: config-volume
        - mountPath: /var/run/secrets/istio
          name: istiod-ca-cert
        - mountPath: /var/run/secrets/tokens
          name: istio-token
          readOnly: true
        - mountPath: /var/lib/istio/data
          name: istio-data
        - mountPath: /etc/istio/pod
          name: podinfo
        - mountPath: /etc/istio/ingressgateway-certs
          name: ingressgateway-certs
          readOnly: true
        - mountPath: /etc/istio/ingressgateway-ca-certs
          name: ingressgateway-ca-certs
          readOnly: true
      securityContext:
        runAsNonRoot: true
      serviceAccountName: istio-ingressgateway-service-account
      volumes:
      - emptyDir: {}
        name: workload-socket
      - emptyDir: {}
        name: credential-socket
      - emptyDir: {}
        name: workload-certs
      - configMap:
          name: istio-ca-root-cert
        name: istiod-ca-cert
      - downwardAPI:
          items:
          - fieldRef:
              fieldPath: metadata.labels
            path: labels
          - fieldRef:
              fieldPath: metadata.annotations
            path: annotations
        name: podinfo
      - emptyDir: {}
        name: istio-envoy
      - emptyDir: {}
        name: istio-data
      - name: istio-token
        projected:
          sources:
          - serviceAccountToken:
              audience: istio-ca
              expirationSeconds: 43200
              path: istio-token
      - configMap:
          name: istio
          optional: true
        name: config-volume
      - name: ingressgateway-certs
        secret:
          optional: true
          secretName: istio-ingressgateway-certs
      - name: ingressgateway-ca-certs
        secret:
          optional: true
          secretName: istio-ingressgateway-ca-certs

---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  labels:
    app: istio-ingressgateway
    install.operator.istio.io/owning-resource: unknown
    istio: ingressgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
  name: istio-ingressgateway
  namespace: istio-system
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: istio-ingressgateway
      istio: ingressgateway

---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  labels:
    install.operator.istio.io/owning-resource: unknown
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
  name: istio-ingressgateway-sds
  namespace: istio-system
rules:
- apiGroups:
  - ""
  resources:
  - secrets
  verbs:
  - get
  - watch
  - list

---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  labels:
    install.operator.istio.io/owning-resource: unknown
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
  name: istio-ingressgateway-sds
  namespace: istio-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: istio-ingressgateway-sds
subjects:
- kind: ServiceAccount
  name: istio-ingressgateway-service-account

---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  labels:
    app: istio-ingressgateway
    install.operator.istio.io/owning-resource: unknown
    istio: ingressgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
  name: istio-ingressgateway
  namespace: istio-system
spec:
  maxReplicas: 5
  metrics:
  - resource:
      name: cpu
      target:
        averageUtilization: 80
        type: Utilization
    type: Resource
  minReplicas: 1
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: istio-ingressgateway

---
apiVersion: v1
kind: Service
metadata:
  annotations: null
  labels:
    app: istio-ingressgateway
    install.operator.istio.io/owning-resource: unknown
    istio: ingressgateway
    istio.io/rev: default
    operator.istio.io/component: IngressGateways
    release: istio
  name: istio-ingressgateway
  namespace: istio-system
spec:
  ports:
  - name: status-port
    port: 15021
    protocol: TCP
    targetPort: 15021
  - name: tls-xds
    port: 15012
    protocol: TCP
    targetPort: 15012
  - name: tls-webhook
    port: 15017
    protocol: TCP
    targetPort: 15017
  selector:
    app: istio-ingressgateway
    istio: ingressgateway
  type: LoadBalancer

---
EOF

# Wait for the ingress gateway to be ready.
oc --kubeconfig "${KUBECONFIG_1}"" wait '--for=jsonpath={.status.loadBalancer.ingress[].ip}' --timeout=30s svc istio-ingressgateway -n istio-system
```

4. Get external Istiod address to expose the ingress gateway.
Note: these instructions are intended to be executed in a test environment. For production environments, please refer to: https://istio.io/latest/docs/setup/install/external-controlplane/#set-up-a-gateway-in-the-external-cluster and https://istio.io/latest/docs/tasks/traffic-management/ingress/secure-ingress/#configure-a-tls-ingress-gateway-for-a-single-host for setting up a secure ingress gateway.

```bash
export EXTERNAL_ISTIOD_ADDR=$(oc -n istio-system --kubeconfig "${KUBECONFIG_1}" get svc istio-ingressgateway -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
```

5. Create the external-istiod namespace and RemoteIstio resource in cluster2.
```bash
oc create namespace external-istiod --kubeconfig "${KUBECONFIG_2}"
oc apply --kubeconfig "${KUBECONFIG_2}" -f - <<EOF
apiVersion: sailoperator.io/v1alpha1
kind: RemoteIstio
metadata:
  name: external-istiod
spec:
  version: v${ISTIO_VERSION}
  namespace: external-istiod
  values:
    defaultRevision: external-istiod
    global:
      istioNamespace: external-istiod
      remotePilotAddress: ${EXTERNAL_ISTIOD_ADDR}
      configCluster: true
    pilot:
      configMap: true
    istiodRemote:
      injectionPath: /inject/cluster/cluster2/net/network1
EOF
```

6. Create the external-istiod namespace on cluster1.
```bash
oc create namespace external-istiod --kubeconfig "${KUBECONFIG_1}"
```

7. Create the remote-cluster-secret on cluster1 so that the external-istiod can access the remote cluster.
```bash
oc create sa istiod-service-account -n external-istiod --kubeconfig "${KUBECONFIG_1}"
REMOTE_NODE_IP=$(oc get nodes -l node-role.kubernetes.io/control-plane --kubeconfig "${KUBECONFIG_2}" -o jsonpath='{.items[0].status.addresses[?(@.type == "InternalIP")].address}')
istioctl create-remote-secret \
  --kubeconfig "${KUBECONFIG_2}" \
  --type=config \
  --namespace=external-istiod \
  --service-account=istiod-external-istiod \
  --create-service-account=false \
  --server="https://${REMOTE_NODE_IP}:6443" | \
  oc apply -f - --kubeconfig "${KUBECONFIG_1}"
```

8. Create the Istio resource on the external control plane cluster. This will manage both Istio configuration and proxies on the remote cluster.
```bash
oc apply --kubeconfig "${KUBECONFIG_1}" -f - <<EOF
apiVersion: sailoperator.io/v1alpha1
kind: Istio
metadata:
  name: external-istiod
spec:
  namespace: external-istiod
  profile: empty
  values:
    meshConfig:
      rootNamespace: external-istiod
      defaultConfig:
        discoveryAddress: $EXTERNAL_ISTIOD_ADDR:15012
    pilot:
      enabled: true
      volumes:
        - name: config-volume
          configMap:
            name: istio-external-istiod
        - name: inject-volume
          configMap:
            name: istio-sidecar-injector-external-istiod
      volumeMounts:
        - name: config-volume
          mountPath: /etc/istio/config
        - name: inject-volume
          mountPath: /var/lib/istio/inject
      env:
        INJECTION_WEBHOOK_CONFIG_NAME: "istio-sidecar-injector-external-istiod-external-istiod"
        VALIDATION_WEBHOOK_CONFIG_NAME: "istio-validator-external-istiod-external-istiod"
        EXTERNAL_ISTIOD: "true"
        LOCAL_CLUSTER_SECRET_WATCHER: "true"
        CLUSTER_ID: cluster2
        SHARED_MESH_CONFIG: istio
        ROOT_CA_DIR: "/etc/cacerts"
    global:
      caAddress: $EXTERNAL_ISTIOD_ADDR:15012
      istioNamespace: external-istiod
      operatorManageWebhooks: true
      configValidation: false
      meshID: mesh1
      multiCluster:
        clusterName: cluster2
      network: network1
EOF
oc wait --kubeconfig "${KUBECONFIG_1}" --for=condition=Ready istios/external-istiod --timeout=3m
```

9. Create the Istio resources to route traffic from the ingress gateway to the external control plane.
```bash
oc apply --kubeconfig "${KUBECONFIG_1}" -f - <<EOF
apiVersion: networking.istio.io/v1
kind: Gateway
metadata:
  name: external-istiod-gw
  namespace: external-istiod
spec:
  selector:
    istio: ingressgateway
  servers:
    - port:
        number: 15012
        protocol: tls
        name: tls-XDS
      tls:
        mode: PASSTHROUGH
      hosts:
      - "*"
    - port:
        number: 15017
        protocol: tls
        name: tls-WEBHOOK
      tls:
        mode: PASSTHROUGH
      hosts:
      - "*"
---
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata:
  name: external-istiod-vs
  namespace: external-istiod
spec:
    hosts:
    - "*"
    gateways:
    - external-istiod-gw
    tls:
    - match:
      - port: 15012
        sniHosts:
        - "*"
      route:
      - destination:
          host: istiod-external-istiod.external-istiod.svc.cluster.local
          port:
            number: 15012
    - match:
      - port: 15017
        sniHosts:
        - "*"
      route:
      - destination:
          host: istiod-external-istiod.external-istiod.svc.cluster.local
          port:
            number: 443
EOF

oc wait --kubeconfig "${KUBECONFIG_2}" --for=condition=Ready remoteistios/external-istiod --timeout=3m
```

#### Verification
1. Create the sample namespace on the remote cluster and label it to enable injection.
```bash
oc create --kubeconfig "${KUBECONFIG_2}" namespace sample
oc label --kubeconfig "${KUBECONFIG_2}" namespace sample istio.io/rev=external-istiod
```

2. Deploy the sleep and helloworld applications to the sample namespace.
```bash
oc get ns sample --kubeconfig "${KUBECONFIG_2}" || oc create --kubeconfig "${KUBECONFIG_2}" namespace sample
oc label --kubeconfig "${KUBECONFIG_2}" namespace sample istio-injection=enabled
oc apply --kubeconfig "${KUBECONFIG_2}" \
  -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml \
  -l service=helloworld -n sample
oc apply --kubeconfig "${KUBECONFIG_2}" \
  -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml \
  -l version=v1 -n sample
oc apply --kubeconfig "${KUBECONFIG_2}" \
  -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/sleep/sleep.yaml -n sample
```

3. Verify the pods in the sample namespace have a sidecar injected.
```bash
oc get pods --kubeconfig "${KUBECONFIG_2}" -n sample
```
You should see the `helloworld` and `sleep` pods with a sidecar injected. For example:
```
NAME                             READY   STATUS    RESTARTS   AGE
helloworld-v1-6d65866976-jb6qc   2/2     Running   0          49m
sleep-5fcd8fd6c8-mg8n2           2/2     Running   0          49m
```

4. Verify you can send a request to helloworld through the sleep app on the Remote cluster.
```bash
oc exec --kubeconfig "${KUBECONFIG_2}" -n sample -c sleep "$(oc get pod --kubeconfig "${KUBECONFIG_2}" -n sample -l app=sleep -o jsonpath='{.items[0].metadata.name}')" -- curl -sS helloworld.sample:5000/hello
```
You should see a response from the `helloworld` app. For example:
```
Hello version: v1, instance: helloworld-v1-6d65866976-jb6qc
```

5. Deploy an ingress gateway to the Remote cluster and verify you can reach helloworld externally.
```bash
oc get crd gateways.gateway.networking.k8s.io --kubeconfig "${KUBECONFIG_2}" &> /dev/null || \
{ oc kustomize "github.com/kubernetes-sigs/gateway-api/config/crd?ref=v1.1.0" | oc apply -f - --kubeconfig "${KUBECONFIG_2}"; }
```

Expose helloworld through the ingress gateway.
```bash
oc apply -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/gateway-api/helloworld-gateway.yaml -n sample --kubeconfig "${KUBECONFIG_2}"
oc -n sample --kubeconfig "${KUBECONFIG_2}" wait --for=condition=programmed gtw helloworld-gateway
```

Confirm you can access the helloworld application through the ingress gateway created in the Remote cluster.
```bash
curl -s "http://$(oc -n sample --kubeconfig "${KUBECONFIG_2}" get gtw helloworld-gateway -o jsonpath='{.status.addresses[0].value}'):80/hello"
```
You should see a response from the `helloworld` app. For example:
```
Hello version: v1, instance: helloworld-v1-6d65866976-jb6qc
```

#### Clean up.
```bash
oc delete istios default --kubeconfig "${KUBECONFIG_1}"
oc delete istiocni default --kubeconfig "${KUBECONFIG_1}"
oc delete ns istio-system --kubeconfig "${KUBECONFIG_1}"
oc delete istios external-istiod --kubeconfig "${KUBECONFIG_1}"
oc delete istiocni external-istiod --kubeconfig "${KUBECONFIG_1}"
oc delete ns external-istiod --kubeconfig "${KUBECONFIG_1}"
oc delete remoteistios external-istiod --kubeconfig "${KUBECONFIG_2}"
oc delete ns external-istiod --kubeconfig "${KUBECONFIG_2}"
oc delete ns sample --kubeconfig "${KUBECONFIG_2}"
```

### Debugging multicluster deployments
There are some steps that you can follow to debug the multicluster deployments. Istio provides documentation related to [troubleshooting](https://istio.io/latest/docs/ops/diagnostic-tools/multicluster) multicluster deployments. Some of the steps that you can follow are:

#### Verify the endpoints
To verify that the endpoints are correctly configured, you can run the following commands:
```bash
istioctl proxy-config endpoint "$(oc get pod --kubeconfig "${KUBECONFIG_2}" -n sample -l \
    app=sleep -o jsonpath='{.items[0].metadata.name}')" -n sample --kubeconfig "${KUBECONFIG_2}" |grep helloworld

istioctl proxy-config endpoint "$(oc get pod --kubeconfig "${KUBECONFIG_1}" -n sample -l \
    app=helloworld -o jsonpath='{.items[0].metadata.name}')" -n sample --kubeconfig "${KUBECONFIG_1}" |grep helloworld

```

This will show you the endpoints that the `sleep` application is trying to reach and the endpoints that the `helloworld` application is exposing. For example:
```
istioctl proxy-config endpoint "$(oc get pod --kubeconfig "${KUBECONFIG_1}" -n sample -l \
    app=helloworld -o jsonpath='{.items[0].metadata.name}')" -n sample --kubeconfig "${KUBECONFIG_1}" |grep helloworld
10.128.2.53:5000                                        HEALTHY     OK                outbound|5000||helloworld.sample.svc.cluster.local
18.144.93.72:15443                                      HEALTHY     OK                outbound|5000||helloworld.sample.svc.cluster.local
```
If you see the endpoints that you are expecting (local and remote address), then the services are correctly configured.

#### Enable access-logs
To enable access-logs for debugging purposes, you can follow the Istio documentation [here](https://istio.io/latest/docs/tasks/observability/logs/access-log/). Enabeling the access-logs will help you to see the requests that are being sent between the services.

The Telemetry API can be used to enable or disable access logs:
```bash
oc apply --kubeconfig "${KUBECONFIG_1}" -f - <<EOF
apiVersion: telemetry.istio.io/v1
kind: Telemetry
metadata:
  name: mesh-default
  namespace: istio-system
spec:
  accessLogging:
    - providers:
      - name: envoy
EOF

oc apply --kubeconfig "${KUBECONFIG_2}" -f - <<EOF
apiVersion: telemetry.istio.io/v1
kind: Telemetry
metadata:
  name: mesh-default
  namespace: istio-system
spec:
  accessLogging:
    - providers:
      - name: envoy
EOF
```
Now, if you make a new request you will find in the logs the request that was made. To understand the logs you can follow the [Istio documentation](https://istio.io/latest/docs/tasks/observability/logs/access-log/#default-access-log-format).
