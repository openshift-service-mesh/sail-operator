# Authorizing requests by client certificate (XFCC header) in OpenShift Service Mesh 3

This article documents the available approaches for authorizing requests based on the
information in an external client certificate, as carried in the `x-forwarded-client-cert`
(XFCC) header, in Red Hat OpenShift Service Mesh 3. It covers the relevant
`AuthorizationPolicy` and `VirtualService` configuration, the supportability boundary, the
anti-spoofing requirements, and the validation obligations on upgrade.

## Environment

- Red Hat OpenShift Service Mesh 3 (validated on 3.4) on Red Hat OpenShift Container Platform 4.x.
- An Istio ingress gateway terminating mutual TLS (mTLS) for external clients, reached so that the
  client's TLS connection arrives at the gateway intact — either through an OpenShift **passthrough**
  route or by exposing the gateway's `LoadBalancer`/`NodePort` service directly. An **edge** or
  **reencrypt** route would terminate the client's TLS at the OpenShift Router and the gateway would
  never see the client certificate.
- `AuthorizationPolicy` and `VirtualService`, both **Generally Available (GA)**. See the
  [OpenShift Service Mesh 3.4 feature support tables](https://docs.redhat.com/en/documentation/red_hat_openshift_service_mesh/3.4/html/release_notes/ossm-release-notes-feature-support-tables).
- External client certificates whose identity is carried in the Subject Common Name (CN), with
  or without Subject Alternative Names (SANs).

## Issue

A request enters the mesh through a single ingress gateway that terminates mTLS and trusts a
shared certificate authority (CA). Multiple external clients present certificates signed by that
CA. The requirement is **per-destination-service authorization**: each backend service must
independently decide which external clients may access it, even though all clients share the same
CA and enter through the same gateway. Accepting the certificate at the gateway is therefore only
part of the solution — the client identity must be propagated to the backend so that a
per-service `AuthorizationPolicy` can act on it.

When the gateway is configured to forward client certificate details, Istio (through Envoy) populates
the `x-forwarded-client-cert` (XFCC) header on the request forwarded to the backend. Backends can
then match on this header. This article describes the supported matching approaches and their
trade-offs.

> **Note - the XFCC header is produced by the Istio gateway, not by the OpenShift Router.** XFCC is an
> Envoy header. The OpenShift Router (HAProxy) does **not** emit it: on terminating (edge/reencrypt)
> routes it adds only the standard `X-Forwarded-*` / `Forwarded` set, and on a **passthrough** route it
> runs in TCP mode, so it never terminates TLS and cannot read or add any HTTP header (and does not
> count as a trusted proxy hop). Because this pattern requires the gateway to terminate the client's
> mTLS, the client certificate is verified and the XFCC header is generated at the Istio gateway.

## Resolution

### How the XFCC header is produced

The ingress gateway forwards client certificate details in the XFCC header according to the
`forwardClientCertDetails` setting in the gateway topology. In OpenShift Service Mesh 3 this is configured on the
`Istio` resource under `spec.values.meshConfig.defaultConfig.gatewayTopology`:

```yaml
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  values:
    meshConfig:
      defaultConfig:
        gatewayTopology:
          forwardClientCertDetails: SANITIZE_SET
```

With `SANITIZE_SET` (the gateway default), the gateway strips any client-supplied XFCC and writes a
block containing the client certificate fields it is configured to emit (`Hash`, `Cert`, `Subject`,
`DNS`/`URI`). `SANITIZE_SET` is the correct mode at an edge gateway: it both sets the verified
client-certificate fields and discards any XFCC the external client may have sent, so those fields
cannot be forged. Depending on where the header is observed, it can contain more than one block: the
backend's sidecar (default `APPEND_FORWARD`) appends a second block identifying the gateway's own
SPIFFE identity, so at the backend the header is the client certificate block followed by the gateway
block (see [Where the policy is enforced](#where-the-policy-is-enforced-and-why-it-matters)). An
observed value at the backend looks like:

```text
Hash=0247d0472f91a1c6c9289c74c33fe47666a93dada8eaa380142a989bb44eb5f4;Cert="-----BEGIN%20CERTIFICATE-----...-----END%20CERTIFICATE-----%0A";Subject="CN=client1.apps.example.com,OU=Central,O=Lab,L=US,ST=Lab,C=US";URI=;DNS=client1.apps.example.com,By=spiffe://central.example.com/ns/service-a/sa/service-a;Hash=6d29545732a31b3a8f72f5609bb8059fc4091069fd35340b77bafe8c415a6a66;Subject="";URI=spiffe://central.example.com/ns/istio-ingress/sa/external-gateway
```

The first block's `Hash` is the SHA-256 fingerprint of the certificate in that block's `Cert`
field (the external client certificate); the second block identifies the gateway.

### AuthorizationPolicy header matching semantics

Matching on the XFCC header uses an `AuthorizationPolicy` `when` condition with the key
`request.headers[x-forwarded-client-cert]`. The value supports a single glob wildcard with these
semantics:

| Value form   | Match type | Example                         |
|--------------|------------|---------------------------------|
| `*`          | present    | header exists, any value        |
| `foo*`       | prefix     | value starts with `foo`         |
| `*foo`       | suffix     | value ends with `foo`           |
| `foo`        | exact      | value equals `foo`              |

The important limitation: a value supports matching anchored to **one end** only — there is no
infix match that anchors both a prefix and a suffix around arbitrary text. Because the raw `Cert=`
field embeds the full PEM-encoded certificate in the middle of the header value, a match must be
anchored either to the start (for example, the leading `Hash=`) or bounded around a field that
precedes the certificate blob. This is why the fingerprint and the Subject cannot both be matched
in a single value.

### Where the policy is enforced, and why it matters

The XFCC header is rewritten at more than one hop:

1. At the ingress gateway, `forwardClientCertDetails: SANITIZE_SET` strips any client-supplied XFCC
   and sets **one** block describing the verified external client certificate.
2. On the gateway → backend hop, the backend's sidecar (default `APPEND_FORWARD`) **appends a second
   block** describing the gateway. So the header the backend application receives is
   `[client block] , [gateway block]` — the client's fields are no longer at the end.

Consequently:

- A **prefix** match (anchored at the start, that is `Hash=…*`) still matches the client block at the
  **backend**.
- A **suffix or bounded** match (`*DNS=…`, `*;Subject="…";URI=`) needs the client's fields at
  the **end**, which holds only *before* the backend sidecar appends — for example when the policy is
  enforced **at the gateway**.

Therefore:

- Enforce **fingerprint (Approach 1)** and the **dedicated-header approach (Approach 2)** at the
  **backend** — per-service, and robust to the appended block.
- Enforce **DNS-SAN (Approach 3)** and **Subject (Approach 4)** raw-XFCC matching at the
  **gateway**, scoped per destination service by host.

**Security note for backend enforcement:** at the backend, XFCC (like any header) can be forged by
an in-mesh workload that reaches the service over mTLS. Backend policies must therefore also require
`from.source.principals` = the gateway identity, so only traffic that actually transited the gateway
is trusted. Do **not** change the backend sidecar's `forwardedClientCert` to work around the layout:
`FORWARD_ONLY` makes XFCC spoofable in-mesh, and `SANITIZE_SET` on the backend erases the client
identity (it would set the header from the gateway peer, not the external client).

> **Note - Approaches 1–2 are enforced at the backend.** The authorization decision is made at each
> destination service, and both are robust to the backend sidecar's appended block.

### Approach 1 — Client certificate fingerprint matching (backend, per-service)

Enforced at the backend. Match the leading client-certificate `Hash` field with a trailing wildcard
(prefix match), and require the gateway's authenticated principal so the header cannot be forged by
another in-mesh workload:

```yaml
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: allow-client-by-fingerprint
  namespace: service-a
spec:
  action: ALLOW
  rules:
  - from:
    - source:
        principals:
        - "central.example.com/ns/istio-ingress/sa/external-gateway"
    when:
    - key: request.headers[x-forwarded-client-cert]
      values:
      - "Hash=0247d0472f91a1c6c9289c74c33fe47666a93dada8eaa380142a989bb44eb5f4*"
```

- Requires `forwardClientCertDetails: SANITIZE_SET` on the gateway (it sets the client `Hash` and
  strips any client-supplied XFCC so the field cannot be forged).
- The prefix anchors the match to the first (client) block, whose `Hash` is the SHA-256
  fingerprint of the external client certificate. The `Hash` value is **lowercase hex with no
  colons** — not the format `openssl` prints by default (uppercase, colon-separated, with a
  `sha256 Fingerprint=` prefix). Obtain the value in the correct form with:

  ```bash
  openssl x509 -in client.crt -noout -fingerprint -sha256 \
    | sed 's/^.*=//; s/://g' | tr 'A-Z' 'a-z'
  # for example, 47:F6:78:7A:9C:...  ->  47f6787a9c...
  ```

  Use that string as `Hash=<value>*`.

- **Trade-off:** the fingerprint changes on every certificate renewal, so the policy must be
  updated whenever the client certificate is rotated. This couples service authorization to the
  certificate lifecycle.

### Approach 2 — Dedicated header through VirtualService certificate formatter operators (backend, per-service)

Instead of matching the raw XFCC value, the gateway writes a dedicated, trusted header using an
Envoy certificate formatter operator, and each backend matches that header exactly. Two operators
are relevant:

- `%DOWNSTREAM_PEER_FINGERPRINT_256%` — the SHA-256 fingerprint of the client certificate.
- `%DOWNSTREAM_PEER_SUBJECT%` — the client certificate Subject.

The fingerprint changes on renewal; the Subject is stable across renewals as long as the Subject
is unchanged. Choose the operator that matches the stability characteristics you need.

On **every** external ingress route, `remove` the dedicated header and then `set` it from the
operator, so that any value supplied by the client is discarded and the header always reflects the
certificate verified at the gateway:

```yaml
apiVersion: networking.istio.io/v1
kind: VirtualService
metadata:
  name: ingress-route
  namespace: istio-ingress
spec:
  gateways:
  - istio-ingress/external-gateway
  hosts:
  - service-a.apps.<cluster_name>.example.com
  http:
  - headers:
      request:
        remove:
        - x-client-subject
        set:
          x-client-subject: "%DOWNSTREAM_PEER_SUBJECT%"
    route:
    - destination:
        host: service-a.service-a.svc.cluster.local
```

Add the `headers.request` block to the **same HTTP route that already forwards ingress traffic to
the backend** — it is not a separate resource, and routing alone does **not** set the header. The
`headers` block and the `route` destination live together under one `http` entry. For example, if
your existing ingress route looks like this (routing only — the header is never set):

```yaml
  http:
  - route:
    - destination:
        host: service-a.service-a.svc.cluster.local
```

add the `headers.request` block to that same entry (the header is now set on every forwarded request):

```yaml
  http:
  - headers:
      request:
        remove:
        - x-client-subject
        set:
          x-client-subject: "%DOWNSTREAM_PEER_SUBJECT%"
    route:
    - destination:
        host: service-a.service-a.svc.cluster.local
```

`remove` operations are applied before `set`, and `set` overwrites any existing value, so a
client that supplies `x-client-subject` itself cannot influence the forwarded value.

Each backend then matches the dedicated header **exactly** and additionally requires the
gateway's authenticated SPIFFE principal, so the header is only trusted when it was written by the
gateway:

```yaml
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: allow-client-by-subject-header
  namespace: service-a
spec:
  action: ALLOW
  rules:
  - from:
    - source:
        principals:
        - "central.example.com/ns/istio-ingress/sa/external-gateway"
    when:
    - key: request.headers[x-client-subject]
      values:
      # The Subject is rendered in RFC 2253 form, which REVERSES the certificate's DN order.
      # A cert created as /CN=…/OU=…/O=…/L=…/ST=…/C=US renders as the value below. Do not assume
      # the order — copy the exact value from the x-client-subject header you observe at the backend.
      - "C=US,ST=Lab,L=US,O=Lab,OU=Central,CN=Special User 2"
```

- **Both** conditions are required. The gateway principal (`from.source.principals`) ensures the
  request arrived over mTLS from the gateway; the exact header value authorizes the specific
  client. Omitting the principal check would allow any workload able to set the header to
  impersonate a client.
- Because the dedicated header is written by the gateway and not parsed out of the raw XFCC value,
  this approach does not depend on the internal XFCC layout.

> **Note - Approaches 3–4 are enforced at the gateway.** The authorization decision is made on the ingress
> gateway (one rule per destination service), where the XFCC still contains only the client block, so
> suffix/bounded matching on the raw XFCC fields is reliable. Both require
> `forwardClientCertDetails: SANITIZE_SET` on the gateway (to populate the fields and strip any
> client-supplied XFCC).

### Approach 3 — Subject Alternative Name (DNS) matching (gateway-enforced)

Enforced at the ingress gateway (see [Where the policy is enforced](#where-the-policy-is-enforced-and-why-it-matters)).
For certificates that carry a DNS SAN, match the `DNS` field with a leading wildcard (suffix match),
scoped per destination service by host:

```yaml
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: allow-client-by-dns-san
  namespace: istio-ingress
spec:
  selector:
    matchLabels:
      istio: ingressgateway        # the ingress gateway workload
  action: ALLOW
  rules:
  - to:
    - operation:
        hosts: ["service-a.apps.<cluster_name>.example.com"]
    when:
    - key: request.headers[x-forwarded-client-cert]
      values:
      - "*DNS=client1.apps.example.com"
```

- Applicable only when the client certificate contains the relevant SAN; the natural transition
  approach for customers moving toward SAN-based identities. The SAN value is stable across renewals
  as long as the SAN is unchanged.
- Enforced at the gateway because the suffix match needs the client's fields at the end of the
  header, which holds only before the backend sidecar appends its block.
- For identities inside the mesh, native SPIFFE/URI principal matching (`source.principals`) is
  available; external edge certificates are not mesh principals, so at the edge the XFCC header
  remains the matching mechanism.

### Approach 4 — Subject matching for certificates without SANs (gateway-enforced)

Enforced at the ingress gateway. For the current certificate profile (identity in the Subject CN,
no SAN), bound the match around the `Subject` field using a leading wildcard and a trailing anchor
on the field that follows the Subject (here, `URI=`), scoped per destination service by host:

```yaml
apiVersion: security.istio.io/v1
kind: AuthorizationPolicy
metadata:
  name: allow-client-by-subject
  namespace: istio-ingress
spec:
  selector:
    matchLabels:
      istio: ingressgateway        # the ingress gateway workload
  action: ALLOW
  rules:
  - to:
    - operation:
        hosts: ["service-a.apps.<cluster_name>.example.com"]
    when:
    - key: request.headers[x-forwarded-client-cert]
      values:
      - "*;Subject=\"C=US,ST=Lab,L=US,O=Lab,OU=Central,CN=Special User 2\";URI="
```

- The leading `*` skips over the earlier fields (including the raw `Cert=` blob), and the trailing
  `URI=` bounds the match so that it ends immediately after the Subject. Stable across renewals as
  long as the Subject is unchanged.
- Copy the `Subject="…"` value **verbatim from the observed header**. Envoy renders the Subject in
  RFC 2253 form, which **reverses** the certificate's DN order (a cert created
  `/CN=…/…/C=US` renders as `C=US,…,CN=…`) and differs from `openssl`'s default output. Do not
  assume the RDN order — read it from the actual XFCC value.
- Enforced at the gateway for the same reason as Approach 3 (the bounded match needs the client's
  fields at the end of the header). This moves client authorization into the gateway, with one rule
  per destination service.
- The raw XFCC layout is produced by Envoy and is not a format Red Hat guarantees across releases
  (see [Supportability](#root-cause)); re-validate after each upgrade. Editing a
  gateway `AuthorizationPolicy` triggers connection draining / HTTP/2 reconnects.

### Approach summary

| Approach | Enforce at | Stable across renewals | Notes |
|----------|------------|------------------------|-------|
| 1 — fingerprint (`Hash`) | Backend (per-service) | No (hash changes on renewal) | Prefix match; pin gateway principal. |
| 2 — dedicated header (Subject) | Backend (per-service) | Yes (if Subject unchanged) | Layout-independent; pin gateway principal + `remove`/`set`. |
| 3 — DNS SAN | Gateway (per-host) | Yes | Needs a DNS SAN. |
| 4 — Subject (raw XFCC) | Gateway (per-host) | Yes | Works with no SAN; gateway policy edits cause connection draining. |

Approaches 1 and 2 place the authorization decision at each backend service; Approaches 3 and 4
place it on the shared gateway (one policy per destination service). Select based on the enforcement
point, the certificate profile, and the renewal behavior described above.

## Diagnostic steps

1. Confirm the gateway XFCC configuration:

   ```bash
   oc get istio default -o yaml | yq '.spec.values.meshConfig.defaultConfig.gatewayTopology'
   ```

2. Send a request with a client certificate and inspect the header the backend received (the
   example backend echoes request headers):

   ```bash
   # <GATEWAY_ADDR> is the gateway's external IP or hostname; --connect-to keeps the TLS SNI
   # and Host header as the request host, which the gateway and VirtualService match on.
   curl --connect-to service-a.apps.<cluster_name>.example.com:443:<GATEWAY_ADDR>:443 \
        --cacert ca.crt --cert client.crt --key client.key \
        https://service-a.apps.<cluster_name>.example.com/ -s \
     | jq -r '.headers."x-forwarded-client-cert"'
   ```

   For Approach 2, inspect the dedicated header (for example `.headers."x-client-subject"`).

3. Confirm the authorization outcome:
   - A valid, authorized client certificate is allowed (`200`).
   - The same client is denied (`403`) at a service whose policy does not list it.
   - A request that supplies the dedicated header itself is denied (the header is overwritten).
   - A request producing an empty operator value is denied, not allowed.

4. **On every OpenShift Service Mesh 3 upgrade**, re-run the checks above to detect any change in the XFCC layout
   (Approaches 1, 3, and 4) or in the formatter operator output (Approach 2).

## Related documentation

- Istio [Authorization Policy](https://istio.io/latest/docs/reference/config/security/authorization-policy/)
  and [conditions](https://istio.io/latest/docs/reference/config/security/conditions/) references.
- Envoy
  [HTTP connection manager — SetCurrentClientCertDetails](https://www.envoyproxy.io/docs/envoy/latest/api-v3/extensions/filters/network/http_connection_manager/v3/http_connection_manager.proto)
  and
  [access log / header formatters](https://www.envoyproxy.io/docs/envoy/latest/configuration/observability/access_log/usage).
