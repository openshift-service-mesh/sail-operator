# OpenShift Service Mesh 2.6 migration to 3.0

In OpenShift Service Mesh 2.6, Network Policies are created by default when `spec.security.manageNetworkPolicy=true` in the ServiceMeshControlPlane config. During migration to Service Mesh 3.0, these Network Policies will be removed and will need to be recreated manually if you wish to maintain identical NetworkPolicies.

## Network Policies created by 2.6:

When `spec.security.manageNetworkPolicy=true`, the following Network Policies are created:

### 1. Istiod Network Policy
- **Purpose**: Controls incoming traffic to the webhook port of istiod pod(s)
- **Location**: Created in SMCP namespace
- **Sample YAML**:
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: istio-istiod-default    # Name format: istio-istiod-<revision>
  namespace: istio-system       # Your SMCP namespace
  labels:
    maistra-version: "2.6.5"    # Version label
    app: istiod                 # Identifies istiod component
    istio: pilot                # Identifies as Istio pilot component
    istio.io/rev: default       # Revision identifier
    release: istio              
  annotations:
    "maistra.io/internal": "true"
spec:
  podSelector:
    matchLabels:
      app: istiod
      istio.io/rev: default
  ingress:
    - ports:
      port: webhook
 ```

### 2. Expose Route Network Policy
- **Purpose**: Allows traffic from OpenShift ingress namespaces to pods labeled with `maistra.io/expose-route: "true"`
- **Location**: Created in both:
    - SMCP namespace
    - Any namespace where ServiceMeshMember (SMM) is created
- **Sample YAML**:
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: istio-expose-route-default   # Name format: istio-expose-route-<revision>
  namespace: istio-system            # Your SMCP or member namespace
  labels:
    maistra-version: "2.6.5"
    app: istio
    release: istio
spec: 
  podSelector:
    matchLabels:
      maistra.io/expose-route: "true"
  ingress:
  - from:
    - namespaceSelector:    # Allows traffic from OpenShift ingress
        matchLabels:
          network.openshift.io/policy-group: ingress
```

### 3. Default Mesh Network Policy
- **Purpose**: Restricts traffic to pods only from namespaces explicitly labeled as part of the mesh (using label `maistra.io/member-of: <mesh-namespace>`)
- **Location**: Created in both:
    - SMCP namespace
    - Any namespace where ServiceMeshMember is created
- **Sample YAML**:
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: istio-mesh-default        # Name format: istio-mesh-<revision>
  namespace: istio-system         # Your SMCP or member namespace
  labels:
    maistra-version: "2.6.5"
    app: istio
    release: istio
spec:
  ingress:
  - from:
    - namespaceSelector:                        # Only allows traffic from mesh members
      matchLabels:
        maistra.io/member-of: istio-system      # Replace with your SMCP namespace
```

### 4. Ingress Gateway Network Policy
- **Purpose**: Allows inbound traffic from any source to ingress gateway pods
- **Note**: This is only created when the ingress gateway is created through SMCP spec (not through gateway injection).
    So if you have followed the other steps in the checklist, this will not exist.
- **Location**: Created in SMCP namespace
- **Sample YAML**:
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: istio-ingressgateway             # Gateway name
  namespace: istio-system               
  labels:
    maistra-version: "2.6.5"
    app: istio-ingressgateway            # Gateway app label
    istio: ingressgateway                # Gateway component label
  annotations:
    "maistra.io/internal": "true"
spec:
  podSelector:                           # Targets ingress gateway pods
    matchLabels:
      app: istio-ingressgateway
      istio: ingressgateway
  ingress:
    - {}                                 # Empty rule allows all ingress traffic
```

## How to Migrate Network Policies to 3.0

### Migration Steps

#### For SMCP Namespace
Recreate necessary Network Policies in the new Service Mesh 3.0 control plane namespace:
- Istiod Network Policy
- Default Mesh Network Policy
- Expose Route Network Policy
- Ingress Gateway Network Policy (if you were previously using SMCP-created gateways)

#### For Workload Namespaces
For each namespace that was part of the 2.6 mesh:

1. Recreate the following Network Policies:
    - Default Mesh Network Policy
    - Expose Route Network Policy

2. Update labels:
    - Consider replacing the `maistra.io/expose-route: "true"` label with a new label scheme
    - Update corresponding Network Policy selectors to match new labels

### Best Practices
1. Test Network Policies in a non-production environment first
2. Create Network Policies before migrating workloads to ensure continuous protection

### Important Notes
- Simply removing ownerReferences from existing Network Policies won't prevent their deletion during migration.
- When ServiceMeshMember is removed from a namespace, the `maistra.io/member-of` label is automatically removed from the namespace.
- Duplicate NetworkPolicies in the same namespace should not cause issues.