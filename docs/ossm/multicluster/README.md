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

* Setup env vars. This to avoid to set the `--context` flag in every command.
```bash
export CTX_CLUSTER1=<cluster1-ctx>
export CTX_CLUSTER2=<cluster2-ctx>
export ISTIO_VERSION=1.23.0
```
Set the `ISTIO_VERSION` to the version of Istio that you want to install using the OpenShift Service Mesh 3 operator (Check current available Istio version in the OpenShift Service Mesh 3).

* Create `istio-system` namespace on each cluster.
```bash
oc --context $CTX_CLUSTER1 create namespace istio-system
oc --context $CTX_CLUSTER2 create namespace istio-system
```

* Create shared trust and add intermediate CAs to each cluster.
If you already have a [shared trust](https://istio.io/latest/docs/setup/install/multicluster/before-you-begin/#configure-trust) for each cluster you can skip this. Otherwise, you can use the instructions below to create a shared trust and push the intermediate CAs into your clusters.

    * Generate certificates:
```bash
# Create a shared trust
mkdir -p certs

# Create a root CA
cat <<EOF > certs/root-ca.conf
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

# Generate Root Key
openssl genrsa -out certs/root-key.pem 4096

# Generate Root Certificate Signing Request (CSR)
openssl req -sha256 -new -key certs/root-key.pem -config certs/root-ca.conf -out certs/root-cert.csr

# Generate Root Certificate
openssl x509 -req -sha256 -days 3650 -signkey certs/root-key.pem -extensions req_ext -extfile certs/root-ca.conf -in certs/root-cert.csr -out certs/root-cert.pem

# Create Intermediate CA Configuration for East and West clusters
mkdir -p certs/east
cat <<EOF > certs/east/ca.conf
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
L = East
EOF

mkdir -p certs/west
cat <<EOF > certs/west/ca.conf
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
L = West
EOF

# Generate Intermediate CA Certificates
# East
openssl genrsa -out certs/east/ca-key.pem 4096
openssl req -sha256 -new -config certs/east/ca.conf -key certs/east/ca-key.pem -out certs/east/ca-cert.csr
openssl x509 -req -sha256 -days 3650 -CA certs/root-cert.pem -CAkey certs/root-key.pem -CAcreateserial -extensions req_ext -extfile certs/east/ca.conf -in certs/east/ca-cert.csr -out certs/east/ca-cert.pem
cat certs/east/ca-cert.pem certs/root-cert.pem > certs/east/cert-chain.pem
# West
openssl genrsa -out certs/west/ca-key.pem 4096
openssl req -sha256 -new -config certs/west/ca.conf -key certs/west/ca-key.pem -out certs/west/ca-cert.csr
openssl x509 -req -sha256 -days 3650 -CA certs/root-cert.pem -CAkey certs/root-key.pem -CAcreateserial -extensions req_ext -extfile certs/west/ca.conf -in certs/west/ca-cert.csr -out certs/west/ca-cert.pem
cat certs/west/ca-cert.pem certs/root-cert.pem > certs/west/cert-chain.pem
```

    * Push the intermediate CAs to the clusters:
```bash
oc --context "${CTX_CLUSTER1}" label namespace istio-system topology.istio.io/network=network1
oc --context "${CTX_CLUSTER2}" label namespace istio-system topology.istio.io/network=network2

oc get secret -n istio-system --context "${CTX_CLUSTER1}" cacerts || oc --context "${CTX_CLUSTER1}" create secret generic cacerts -n istio-system \
    --from-file=certs/east/ca-cert.pem \
    --from-file=certs/east/ca-key.pem \
    --from-file=certs/root-cert.pem \
    --from-file=certs/east/cert-chain.pem

oc get secret -n istio-system --context "${CTX_CLUSTER2}" cacerts || oc --context "${CTX_CLUSTER2}" create secret generic cacerts -n istio-system \
    --from-file=certs/west/ca-cert.pem \
    --from-file=certs/west/ca-key.pem \
    --from-file=certs/root-cert.pem \
    --from-file=certs/west/cert-chain.pem
```

### Multicluster: Multi-Primary - Multi-Network
The Multi-Primary - Multi-Network deployment model is a way to deploy OpenShift Service Mesh across multiple OpenShift Container Platform clusters with multiple control planes, each cluster has its own control plane making each a primary cluster, also each cluster has its own network. This causes the clusters to be isolated from each other, and the services in one cluster cannot communicate with the services in another cluster. More information about this deployment model can be found [here](https://istio.io/latest/docs/setup/install/multicluster/multi-primary_multi-network/).

#### Procedure
1. Create an Istio and IstioCNI resources on cluster1.
```bash
oc apply --context "${CTX_CLUSTER1}" -f - <<EOF
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
EOF
oc wait --context "${CTX_CLUSTER1}" --for=condition=Ready istios/default --timeout=3m
```

```bash
oc create namespace istio-cni --context="${CTX_CLUSTER1}"
oc apply --context "${CTX_CLUSTER1}" -f - <<EOF
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
oc apply --context "${CTX_CLUSTER1}" -f - <<EOF
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
oc --context "${CTX_CLUSTER1}" apply -n istio-system -f - <<EOF
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
oc apply --context "${CTX_CLUSTER2}" -f - <<EOF
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
EOF
oc wait --context "${CTX_CLUSTER2}" --for=jsonpath='{.status.revisions.ready}'=1 istios/default --timeout=3m
```

```bash
oc create namespace istio-cni --context="${CTX_CLUSTER2}"
oc apply --context "${CTX_CLUSTER2}" -f - <<EOF
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
oc apply --context "${CTX_CLUSTER2}" -f - <<EOF
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
oc --context "${CTX_CLUSTER2}" apply -n istio-system -f - <<EOF
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
  --context="${CTX_CLUSTER1}" \
  --name=cluster1 | \
  oc apply -f - --context="${CTX_CLUSTER2}"
```

8. Install a remote secret in cluster1 that provides access to the cluster2 API server.
```bash
istioctl create-remote-secret \
  --context="${CTX_CLUSTER2}" \
  --name=cluster2 | \
  oc apply -f - --context="${CTX_CLUSTER1}"
```

#### Verification
1. Deploy sample applications to cluster1.
We will be deploying the `helloworld` and `sleep` applications to cluster1.
```bash
oc get ns sample --context "${CTX_CLUSTER1}" || oc create --context="${CTX_CLUSTER1}" namespace sample
oc label --context="${CTX_CLUSTER1}" namespace sample istio-injection=enabled
oc apply --context="${CTX_CLUSTER1}" \
  -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml \
  -l service=helloworld -n sample
oc apply --context="${CTX_CLUSTER1}" \
  -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml \
  -l version=v1 -n sample
oc apply --context="${CTX_CLUSTER1}" \
  -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/sleep/sleep.yaml -n sample
```

2. Deploy sample applications to cluster2.
```bash
oc get ns sample --context "${CTX_CLUSTER2}" || oc create --context="${CTX_CLUSTER2}" namespace sample
oc label --context="${CTX_CLUSTER2}" namespace sample istio-injection=enabled
oc apply --context="${CTX_CLUSTER2}" \
  -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml \
  -l service=helloworld -n sample
oc apply --context="${CTX_CLUSTER2}" \
  -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/helloworld/helloworld.yaml \
  -l version=v2 -n sample
oc apply --context="${CTX_CLUSTER2}" \
  -f https://raw.githubusercontent.com/istio/istio/${ISTIO_VERSION}/samples/sleep/sleep.yaml -n sample
```

3. Verify that you see a response from both v1 and v2.
```bash
oc exec --context="${CTX_CLUSTER1}" -n sample -c sleep \
    "$(oc get pod --context="${CTX_CLUSTER1}" -n sample -l \
    app=sleep -o jsonpath='{.items[0].metadata.name}')" \
    -- curl -sS helloworld.sample:5000/hello
```
You should see a response from v1 and v2 `helloworld` app.

```bash
    oc exec --context="${CTX_CLUSTER2}" -n sample -c sleep \
        "$(oc get pod --context="${CTX_CLUSTER2}" -n sample -l \
        app=sleep -o jsonpath='{.items[0].metadata.name}')" \
        -- curl -sS helloworld.sample:5000/hello
```
You should see a response from v1 and v2 `helloworld` app.

**Note:** If you have issues with the validation you can follow the [troubleshooting](https://istio.io/latest/docs/ops/diagnostic-tools/multicluster/#step-by-step-diagnosis) section.

#### Clean up.
```bash
oc delete istios default --context="${CTX_CLUSTER1}"
oc delete ns istio-system --context="${CTX_CLUSTER1}" 
oc delete ns sample --context="${CTX_CLUSTER1}"
oc delete istios default --context="${CTX_CLUSTER2}"
oc delete ns istio-system --context="${CTX_CLUSTER2}" 
oc delete ns sample --context="${CTX_CLUSTER2}"
```

### Multicluster: Multi-Primary - Single-Network
[TBD]


