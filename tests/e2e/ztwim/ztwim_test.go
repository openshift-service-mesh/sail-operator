//go:build e2e

// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ztwim

import (
	"fmt"
	"os"
	"strings"
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/cleaner"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/shell"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert/yaml"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var latestVersion = istioversion.GetLatestPatchVersions()[0]

type daemonSetStatus struct {
	Status struct {
		DesiredNumberScheduled int `yaml:"desiredNumberScheduled"`
		NumberReady            int `yaml:"numberReady"`
	} `yaml:"status"`
}

var _ = Describe("ZTWIM Installation", Label("smoke", "ztwim", "slow"), Ordered, func() {
	SetDefaultEventuallyTimeout(180 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)
	debugInfoLogged := false

	Describe(fmt.Sprintf("Istio version: %s", latestVersion.Name), func() {
		clr := cleaner.New(cl)
		BeforeAll(func(ctx SpecContext) {
			clr.Record(ctx)

			By("Verifying ZTWIM operator package is available in the catalog")
			_, err := k.WithNamespace("openshift-marketplace").GetYAML("packagemanifests", ztwimOperatorName)
			if err != nil {
				Skip(fmt.Sprintf("ZTWIM operator package '%s' not found in operator catalog; skipping test suite", ztwimOperatorName))
			}
			Success(fmt.Sprintf("ZTWIM operator package '%s' found in catalog", ztwimOperatorName))

			Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Istio namespace failed to be created")
			Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed(), "IstioCNI namespace failed to be created")
			Expect(k.CreateNamespace(ztwimNamespace)).To(Succeed(), "ZTWIM Namespace failed to be created")
		})

		When("the ZTWIM Operator is deployed", func() {
			BeforeAll(func() {
				// Apply OperatorGroup YAML
				operatorGroupYaml := fmt.Sprintf(`
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: %s
  namespace: %s
spec:
  upgradeStrategy: Default`, ztwimOperatorName, ztwimNamespace)
				Expect(k.WithNamespace(ztwimNamespace).CreateFromString(operatorGroupYaml)).To(Succeed(), "OperatorGroup creation failed")

				// Apply Subscription YAML
				subscriptionYaml := fmt.Sprintf(`
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: %s
  namespace: %s
spec:
  channel: stable-v1
  name: %s
  source: redhat-operators
  sourceNamespace: openshift-marketplace
  installPlanApproval: Automatic`, ztwimOperatorName, ztwimNamespace, ztwimOperatorName)
				Expect(k.WithNamespace(ztwimNamespace).CreateFromString(subscriptionYaml)).To(Succeed(), "Subscription creation failed")
			})

			It("should have subscription created successfully", func() {
				output, err := k.WithNamespace(ztwimNamespace).GetYAML("subscription", ztwimOperatorName)
				Expect(err).NotTo(HaveOccurred(), "error getting subscription YAML")
				Expect(output).To(ContainSubstring(ztwimNamespace), "Subscription is not created")
			})

			It("verifies all ZTWIM pods are Ready", func(ctx SpecContext) {
				Eventually(common.CheckPodsReady).
					WithArguments(ctx, cl, ztwimNamespace).
					Should(Succeed(), fmt.Sprintf("Some pods in namespace %q are not ready", ztwimNamespace))

				Success("All ZTWIM pods are ready")
			})
		})

		When("ZTWIM Operator Patched", func() {
			BeforeAll(func() {
				Expect(
					k.WithNamespace(ztwimNamespace).Patch(
						"subscription",
						ztwimOperatorName,
						"merge",
						`{"spec":{"config":{"env":[{"name":"CREATE_ONLY_MODE","value":"true"}]}}}`,
					),
				).To(Succeed(), "Error patching ZTWIM")
				Success("ZTWIM subscription patched")
			})
		})

		When("ZTWIM Operands Deployed", func() {
			BeforeAll(func() {
				if jwtIssuer == "" {
					Eventually(func() error {
						var err error
						jwtIssuer, err = resolveJwtIssuer()
						return err
					}, 60*time.Second, 5*time.Second).Should(Succeed(), "Failed to resolve jwtIssuer")
				}

				zeroTrustWorkloadIdentityManager := `
apiVersion: operator.openshift.io/v1alpha1
kind: ZeroTrustWorkloadIdentityManager
metadata:
  name: cluster
  labels:
    app.kubernetes.io/name: zero-trust-workload-identity-manager
    app.kubernetes.io/managed-by: zero-trust-workload-identity-manager
spec:
  trustDomain: %s
  clusterName: "sky-computing-cluster"
  bundleConfigMap: "spire-bundle"`
				spireServer := `
apiVersion: operator.openshift.io/v1alpha1
kind: SpireServer
metadata:
  name: cluster
spec:
  logLevel: "info"
  logFormat: "text"
  jwtIssuer: %s
  caValidity: "24h"
  defaultX509Validity: "1h"
  defaultJWTValidity: "5m"
  jwtKeyType: "rsa-2048"
  caSubject:
    country: "US"
    organization: "Sky Computing Corporation"
    commonName: "SPIRE Server CA"
  persistence:
    size: "5Gi"
    accessMode: "ReadWriteOnce"
  datastore:
    databaseType: "sqlite3"
    connectionString: "/run/spire/data/datastore.sqlite3"
    tlsSecretName: ""
    maxOpenConns: 100
    maxIdleConns: 10
    connMaxLifetime: 0
    disableMigration: "false"`
				zeroTrustWorkloadIdentityManager = fmt.Sprintf(zeroTrustWorkloadIdentityManager, trustDomain)
				spireServer = fmt.Sprintf(spireServer, jwtIssuer)
				Expect(
					k.WithNamespace(ztwimNamespace).ApplyStringWithForceConflicts(zeroTrustWorkloadIdentityManager),
				).To(Succeed(), "ZTWIM custom resource creation failed")
				Expect(
					k.WithNamespace(ztwimNamespace).ApplyStringWithForceConflicts(spireServer),
				).To(Succeed(), "Spire Server custom resource creation failed")
			})

			It("restarts spire-server and waits for rollout to complete", func(ctx SpecContext) {
				By("Waiting for spire-server StatefulSet to be created")
				Eventually(func() error {
					_, err := k.WithNamespace(ztwimNamespace).GetYAML("statefulset", "spire-server")
					return err
				}, 60*time.Second, 2*time.Second).Should(Succeed(), "spire-server StatefulSet did not appear")

				By("Restarting spire-server statefulset")
				Eventually(func() error {
					_, err := k.WithNamespace(ztwimNamespace).RolloutRestart("statefulset/spire-server")
					return err
				}, 60*time.Second, 5*time.Second).Should(Succeed(), "Failed to restart spire-server")

				By("Waiting for spire-server rollout to complete")
				Eventually(func() error {
					_, err := k.WithNamespace(ztwimNamespace).RolloutStatus("statefulset/spire-server")
					return err
				}, 300*time.Second, 5*time.Second).Should(Succeed(), "spire-server rollout did not complete successfully")

				Success("spire-server rollout completed successfully")
			})
		})

		When("Spire Agent Deployed", func() {
			BeforeAll(func() {
				spireAgent := `
apiVersion: operator.openshift.io/v1alpha1
kind: SpireAgent
metadata:
  name: cluster
  annotations:
    ztwim.openshift.io/create-only: "true"
spec:
  socketPath: "/run/spire/agent-sockets"
  logLevel: "info"
  logFormat: "text"
  nodeAttestor:
    k8sPSATEnabled: "true"
  workloadAttestors:
    k8sEnabled: "true"
    workloadAttestorsVerification:
      type: "auto"
      hostCertBasePath: "/etc/kubernetes"
      hostCertFileName: "kubelet-ca.crt"
    disableContainerSelectors: "false"
    useNewContainerLocator: "true"`
				Expect(
					k.WithNamespace(ztwimNamespace).ApplyString(spireAgent),
				).To(Succeed(), "Spire Agent custom resource creation failed")
			})

			It("waits for Spire Agent daemonset and rollout completes", func() {
				By("Waiting for Spire Agent DaemonSet to become ready")
				Eventually(func() error {
					yamlStr, err := k.WithNamespace(ztwimNamespace).GetYAML("daemonset", "spire-agent")
					if err != nil {
						return err
					}

					var ds daemonSetStatus
					if err := yaml.Unmarshal([]byte(yamlStr), &ds); err != nil {
						return fmt.Errorf("failed to parse daemonset YAML: %w", err)
					}

					if ds.Status.DesiredNumberScheduled == 0 {
						return fmt.Errorf("spire-agent not scheduled yet: desired=0")
					}

					if ds.Status.DesiredNumberScheduled != ds.Status.NumberReady {
						return fmt.Errorf(
							"spire-agent not ready: desired=%d, ready=%d",
							ds.Status.DesiredNumberScheduled,
							ds.Status.NumberReady,
						)
					}

					return nil
				}, 300*time.Second, 5*time.Second).Should(Succeed(), "spire-agent DaemonSet did not become available")

				By("Waiting for spire-agent rollout to complete")
				Eventually(func() error {
					_, err := k.WithNamespace(ztwimNamespace).RolloutStatus("daemonset/spire-agent")
					return err
				}, 120*time.Second, 5*time.Second).Should(Succeed(), "spire-agent rollout did not complete successfully")

				Success("spire-agent rollout completed successfully")
			})
		})

		When("Spiffe CSI Driver Deployed", func() {
			BeforeAll(func() {
				spiffeCSIDriver := `
apiVersion: operator.openshift.io/v1alpha1
kind: SpiffeCSIDriver
metadata:
  name: cluster
spec:
  agentSocketPath: "/run/spire/agent-sockets"
  pluginName: "csi.spiffe.io"`
				Expect(
					k.WithNamespace(ztwimNamespace).ApplyString(spiffeCSIDriver),
				).To(Succeed(), "Spiffe CSI Driver custom resource creation failed")
			})

			It("waits for spire-spiffe-csi-driver daemonset and rollout completes", func() {
				By("Waiting for spire-spiffe-csi-driver DaemonSet to become ready")
				Eventually(func() error {
					yamlStr, err := k.WithNamespace(ztwimNamespace).GetYAML("daemonset", "spire-spiffe-csi-driver")
					if err != nil {
						return err
					}

					var ds daemonSetStatus
					if err := yaml.Unmarshal([]byte(yamlStr), &ds); err != nil {
						return fmt.Errorf("failed to parse daemonset YAML: %w", err)
					}

					if ds.Status.DesiredNumberScheduled == 0 {
						return fmt.Errorf("spire-spiffe-csi-driver not scheduled yet: desired=0")
					}

					if ds.Status.DesiredNumberScheduled != ds.Status.NumberReady {
						return fmt.Errorf(
							"spire-spiffe-csi-driver not ready: desired=%d, ready=%d",
							ds.Status.DesiredNumberScheduled,
							ds.Status.NumberReady,
						)
					}

					return nil
				}, 300*time.Second, 5*time.Second).Should(Succeed(), "spire-spiffe-csi-driver DaemonSet did not become available")

				By("Waiting for spire-spiffe-csi-driver rollout to complete")
				Eventually(func() error {
					_, err := k.WithNamespace(ztwimNamespace).RolloutStatus("daemonset/spire-spiffe-csi-driver")
					return err
				}, 120*time.Second, 5*time.Second).Should(Succeed(), "spire-spiffe-csi-driver rollout did not complete successfully")

				Success("spire-spiffe-csi-driver rollout completed successfully")
			})
		})

		When("Spire OIDC Discovery Provider Deployed", func() {
			BeforeAll(func() {
				spireOIDC := `
apiVersion: operator.openshift.io/v1alpha1
kind: SpireOIDCDiscoveryProvider
metadata:
  name: cluster
spec:
  logLevel: "info"
  logFormat: "text"
  csiDriverName: "csi.spiffe.io"
  jwtIssuer: ` + jwtIssuer + `
  replicaCount: 1
  managedRoute: "true"`
				Expect(
					k.WithNamespace(ztwimNamespace).ApplyString(spireOIDC),
				).To(Succeed(), "SpireOIDCDiscoveryProvider custom resource creation failed")
			})

			It("configures and restarts spire-spiffe-oidc-discovery-provider", func() {
				By("Waiting for OIDC discovery provider deployment to be available")
				Eventually(func() error {
					_, err := k.WithNamespace(ztwimNamespace).RolloutStatus("deployment/spire-spiffe-oidc-discovery-provider")
					return err
				}, 300*time.Second, 5*time.Second).Should(Succeed(), "OIDC discovery provider deployment did not become available")

				By("Patching OIDC discovery provider configmap")
				bin := os.Getenv("COMMAND")
				if bin == "" {
					bin = "kubectl"
				}
				patchCmd := fmt.Sprintf(`
			OIDC_DISCOVERY_CONFIG_MAP=spire-spiffe-oidc-discovery-provider
			PATCH_PAYLOAD=$(%[1]s get configmap ${OIDC_DISCOVERY_CONFIG_MAP} -n "%[2]s" -o json | \
			jq -r '.data["oidc-discovery-provider.conf"] | fromjson |
			.workload_api.socket_path = "/spiffe-workload-api/socket" |
			tojson | {data: {"oidc-discovery-provider.conf": .}}')
			%[1]s patch configmap ${OIDC_DISCOVERY_CONFIG_MAP} -n "%[2]s" --patch "$PATCH_PAYLOAD"
			`, bin, ztwimNamespace)
				Eventually(func() error {
					_, err := shell.ExecuteShell(patchCmd, "")
					return err
				}, 60*time.Second, 5*time.Second).Should(Succeed(), "Failed patching OIDC discovery provider configmap")

				By("Restarting OIDC discovery provider deployment")
				Eventually(func() error {
					_, err := k.WithNamespace(ztwimNamespace).RolloutRestart("deployment/spire-spiffe-oidc-discovery-provider")
					return err
				}, 60*time.Second, 5*time.Second).Should(Succeed(), "Failed to restart OIDC discovery provider")

				By("Waiting for OIDC discovery provider deployment to be available after restart")
				Eventually(func() error {
					_, err := k.WithNamespace(ztwimNamespace).RolloutStatus("deployment/spire-spiffe-oidc-discovery-provider")
					return err
				}, 300*time.Second, 5*time.Second).Should(Succeed(), "OIDC discovery provider deployment did not become available after restart")

				Success("Spire OIDC Discovery Provider deployed and configured successfully")
			})
		})

		When("the IstioCNI CR is created", func() {
			BeforeAll(func() {
				yaml := `
apiVersion: sailoperator.io/v1
kind: IstioCNI
metadata:
  name: default
spec:
  version: %s
  namespace: %s`
				yaml = fmt.Sprintf(yaml, latestVersion.Name, istioCniNamespace)
				Log("IstioCNI YAML:", common.Indent(yaml))
				Expect(k.CreateFromString(yaml)).To(Succeed(), "IstioCNI creation failed")
				Success("IstioCNI created")
			})

			It("updates the status to Ready", func(ctx SpecContext) {
				Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(istioCniName), &v1.IstioCNI{}).
					Should(HaveConditionStatus(v1.IstioCNIConditionReady, metav1.ConditionTrue), "IstioCNI is not Ready; unexpected Condition")
				Success("IstioCNI is Ready")
			})
		})

		When("the Istio CR is created", func() {
			bin := os.Getenv("COMMAND")
			if bin == "" {
				bin = "kubectl"
			}
			cmd := fmt.Sprintf(`
			%s get secret oidc-serving-cert -n "%s" -o json | \
			jq -r '.data."tls.crt"' | \
			base64 -d | \
			sed 's/^/        /'
			`, bin, ztwimNamespace)
			var extraRootCA string

			BeforeAll(func() {
				Eventually(func() error {
					var err error
					extraRootCA, err = shell.ExecuteShell(cmd, "")
					return err
				}, 60*time.Second, 5*time.Second).Should(Succeed(), "Failed to get EXTRA_ROOT_CA")
				istioYAML := `
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  namespace: %s
  updateStrategy:
    type: InPlace
  values:
    pilot:
      jwksResolverExtraRootCA: |
%s
      env:
        PILOT_JWT_ENABLE_REMOTE_JWKS: "true"
    meshConfig:
      trustDomain: %s
      defaultConfig:
        proxyMetadata:
          WORKLOAD_IDENTITY_SOCKET_FILE: "spire-agent.sock"
    sidecarInjectorWebhook:
      templates:
        spire: |
          spec:
            initContainers:
            - name: istio-proxy
              volumeMounts:
              - name: workload-socket
                mountPath: /run/secrets/workload-spiffe-uds
                readOnly: true
            volumes:
              - name: workload-socket
                csi:
                  driver: "csi.spiffe.io"
                  readOnly: true
        spireGateway: |
          spec:
            containers:
            - name: istio-proxy
              volumeMounts:
              - name: workload-socket
                mountPath: /run/secrets/workload-spiffe-uds
                readOnly: true
            volumes:
              - name: workload-socket
                csi:
                  driver: "csi.spiffe.io"
                  readOnly: true`

				istioYAML = fmt.Sprintf(istioYAML, controlPlaneNamespace, extraRootCA, trustDomain)
				Expect(k.CreateFromString(istioYAML)).To(Succeed(), "Istio CR failed to be created")

				Success("Istio CR created")
			})
		})

		When("SPIFFE-enabled curl and httpbin are deployed and mTLS is enforced", func() {
			const (
				tpj            = "test-ossm-with-ztwim"
				spiffeAudience = "sky-computing-demo"
			)

			BeforeAll(func(ctx SpecContext) {
				By("Creating and labeling the test namespace")
				Expect(k.CreateNamespace(tpj)).To(Succeed())
				Expect(k.Label("namespace", tpj, "istio-injection", "enabled")).To(Succeed())

				By("Deploying httpbin server with SPIFFE annotations")
				httpbinYAML := fmt.Sprintf(`
apiVersion: v1
kind: ServiceAccount
metadata:
  name: httpbin
  namespace: %s
---
apiVersion: v1
kind: Service
metadata:
  name: httpbin
  namespace: %s
  labels:
    app: httpbin
spec:
  ports:
  - name: http-ex-spiffe
    port: 443
    targetPort: 8080
  - name: http
    port: 80
    targetPort: 8080
  selector:
    app: httpbin
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: httpbin
  namespace: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: httpbin
      version: v1
  template:
    metadata:
      annotations:
        inject.istio.io/templates: "sidecar,spire"
        spiffe.io/audience: "%s"
      labels:
        app: httpbin
        version: v1
    spec:
      serviceAccountName: httpbin
      containers:
      - name: httpbin
        image: docker.io/mccutchen/go-httpbin:v2.15.0
        imagePullPolicy: IfNotPresent
        ports:
        - containerPort: 8080
`, tpj, tpj, tpj, spiffeAudience)

				Expect(k.WithNamespace(tpj).ApplyString(httpbinYAML)).To(Succeed())

				By("Waiting for httpbin deployment to be available")
				Eventually(common.GetObject).
					WithArguments(ctx, cl, kube.Key("httpbin", tpj), &appsv1.Deployment{}).
					Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue))

				By("Deploying curl client with SPIFFE annotations")
				curlYAML := fmt.Sprintf(`
apiVersion: v1
kind: ServiceAccount
metadata:
  name: curl
  namespace: %s
---
apiVersion: v1
kind: Service
metadata:
  name: curl
  namespace: %s
  labels:
    app: curl
    service: curl
spec:
  ports:
  - port: 80
    name: http
  selector:
    app: curl
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: curl
  namespace: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: curl
  template:
    metadata:
      annotations:
        inject.istio.io/templates: "sidecar,spire"
        spiffe.io/audience: "%s"
      labels:
        app: curl
    spec:
      terminationGracePeriodSeconds: 0
      serviceAccountName: curl
      containers:
      - name: curl
        image: quay.io/curl/curl:8.16.0
        command:
        - /bin/sh
        - -c
        - sleep inf
        imagePullPolicy: IfNotPresent
`, tpj, tpj, tpj, spiffeAudience)

				Expect(k.WithNamespace(tpj).ApplyString(curlYAML)).To(Succeed())

				By("Waiting for curl deployment to be available")
				Eventually(common.GetObject).
					WithArguments(ctx, cl, kube.Key("curl", tpj), &appsv1.Deployment{}).
					Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue))
			})

			It("allows HTTP access before mTLS is enforced", func(ctx SpecContext) {
				curlPod, err := common.GetPodNameByLabel(ctx, cl, tpj, "app", "curl")
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() error {
					out, err := k.WithNamespace(tpj).Exec(curlPod, "curl",
						"curl -s -o /dev/null -w %{http_code} http://httpbin")
					if err != nil {
						return err
					}
					if strings.TrimSpace(out) != "200" {
						return fmt.Errorf("expected HTTP 200, got %q", strings.TrimSpace(out))
					}
					return nil
				}, 60*time.Second, 5*time.Second).Should(Succeed(), "HTTP access to httpbin failed")
			})

			It("allows HTTP access after STRICT mTLS is enabled", func(ctx SpecContext) {
				By("Enforcing STRICT mTLS and ISTIO_MUTUAL destination rules")
				mtlsYAML := fmt.Sprintf(`
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
  name: default
  namespace: %s
spec:
  mtls:
    mode: STRICT
---
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: curl
  namespace: %s
spec:
  host: curl
  trafficPolicy:
    tls:
      mode: ISTIO_MUTUAL
---
apiVersion: networking.istio.io/v1
kind: DestinationRule
metadata:
  name: httpbin
  namespace: %s
spec:
  host: httpbin
  trafficPolicy:
    tls:
      mode: ISTIO_MUTUAL
`, tpj, tpj, tpj)

				Expect(k.WithNamespace(tpj).ApplyString(mtlsYAML)).To(Succeed())

				curlPod, err := common.GetPodNameByLabel(ctx, cl, tpj, "app", "curl")
				Expect(err).NotTo(HaveOccurred(), "Failed to get curl pod name")

				Eventually(func() error {
					out, err := k.WithNamespace(tpj).Exec(curlPod, "curl",
						"curl -s -o /dev/null -w %{http_code} http://httpbin")
					if err != nil {
						return err
					}
					if strings.TrimSpace(out) != "200" {
						return fmt.Errorf("expected HTTP 200, got %q", strings.TrimSpace(out))
					}
					return nil
				}, 60*time.Second, 5*time.Second).Should(Succeed(), "HTTP access to httpbin with STRICT mTLS failed")
			})
		})

		When("SPIFFE sample app namespace is deleted", func() {
			const tpj = "test-ossm-with-ztwim"

			BeforeEach(func() {
				Expect(k.Delete("namespace", tpj)).To(Succeed())
				Success("SPIFFE sample app namespace deleted")
			})

			It("removes the SPIFFE sample app namespace", func(ctx SpecContext) {
				Eventually(cl.Get).
					WithArguments(ctx, kube.Key(tpj), &corev1.Namespace{}).
					Should(ReturnNotFoundError())
			})
		})

		When("the Istio CR is deleted", func() {
			BeforeEach(func() {
				// FIX: Capture the error and ignore "NotFound".
				// This prevents the test from failing if the CR is already gone.
				err := k.Delete("istio", istioName)
				if err != nil && !strings.Contains(err.Error(), "NotFound") {
					Expect(err).ToNot(HaveOccurred(), "Istio CR failed to be deleted")
				}
				Success("Istio CR deleted")
			})

			It("removes everything from the namespace", func(ctx SpecContext) {
				Eventually(cl.Get).WithArguments(ctx, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
					Should(ReturnNotFoundError(), "Istiod should not exist anymore")
				common.CheckNamespaceEmpty(ctx, cl, controlPlaneNamespace)
				Success("Namespace is empty")
			})
		})

		When("the IstioCNI CR is deleted", func() {
			BeforeEach(func() {
				Expect(k.Delete("istiocni", istioCniName)).To(Succeed(), "IstioCNI CR failed to be deleted")
				Success("IstioCNI deleted")
			})

			It("removes everything from the CNI namespace", func(ctx SpecContext) {
				daemonset := &appsv1.DaemonSet{}
				Eventually(cl.Get).WithArguments(ctx, kube.Key("istio-cni-node", istioCniNamespace), daemonset).
					Should(ReturnNotFoundError(), "IstioCNI DaemonSet should not exist anymore")
				common.CheckNamespaceEmpty(ctx, cl, istioCniNamespace)
				Success("CNI namespace is empty")
			})
		})

		When("ZTWIM operator is deleted", func() {
			BeforeEach(func() {
				By("Deleting OLM Subscription and OperatorGroup")
				Expect(k.WithNamespace(ztwimNamespace).Delete("subscription", ztwimOperatorName)).To(Succeed())
				Expect(k.WithNamespace(ztwimNamespace).Delete("operatorgroup", ztwimOperatorName)).To(Succeed())

				By("Deleting ClusterServiceVersion (CSV) to permanently stop the Operator")
				_ = k.WithNamespace(ztwimNamespace).Delete("csv", "--all")

				Success("ZTWIM operator stopped and deleted")
			})
		})

		When("ZTWIM operands are deleted", func() {
			BeforeEach(func() {
				crTypes := "zerotrustworkloadidentitymanager,spireserver,spireagent,spiffecsidriver,spireoidcdiscoveryprovider"

				By("Removing finalizers from CRs so they can be deleted without the Operator")
				bin := os.Getenv("COMMAND")
				if bin == "" {
					bin = "kubectl"
				}
				patchCmd := fmt.Sprintf("%s patch %s --all -n %s -p '{\"metadata\":{\"finalizers\":[]}}' --type=merge || true", bin, crTypes, ztwimNamespace)
				_, _ = shell.ExecuteShell(patchCmd, "")

				By("Deleting ZTWIM Custom Resources")
				deleteCmd := fmt.Sprintf("%s delete %s --all -n %s --ignore-not-found", bin, crTypes, ztwimNamespace)
				_, _ = shell.ExecuteShell(deleteCmd, "")

				By("Forcefully deleting all remaining workload controllers")
				controllerCmd := fmt.Sprintf("%s delete daemonset,deployment,statefulset --all -n %s --ignore-not-found", bin, ztwimNamespace)
				_, _ = shell.ExecuteShell(controllerCmd, "")

				By("Forcefully deleting all pods")
				podCmd := fmt.Sprintf("%s delete pod --all -n %s --force --grace-period=0", bin, ztwimNamespace)
				_, _ = shell.ExecuteShell(podCmd, "")

				By("Force deleting ZTWIM operator deployment")
				_ = k.WithNamespace(ztwimNamespace).Delete("deployment", "zero-trust-workload-identity-manager-controller-manager")

				Success("ZTWIM operands forcefully deleted")
			})

			It("It checkes ZTWIM operator deployment to be fully removed", func(ctx SpecContext) {
				Eventually(cl.Get).WithArguments(ctx, kube.Key("zero-trust-workload-identity-manager-controller-manager", ztwimNamespace), &appsv1.Deployment{}).
					Should(ReturnNotFoundError(), "ZTWIM operator deployment should not exist anymore")
				Success("ZTWIM namespace is empty")
			})
		})

		When("ZTWIM namespace is deleted", func() {
			BeforeEach(func() {
				Expect(k.Delete("namespace", ztwimNamespace)).To(Succeed())
				Success("ZTWIM namespace deleted")
			})

			It("removes the ZTWIM namespace", func(ctx SpecContext) {
				Eventually(cl.Get).
					WithArguments(ctx, kube.Key(ztwimNamespace), &corev1.Namespace{}).
					Should(ReturnNotFoundError())
			})
		})

		AfterAll(func(ctx SpecContext) {
			if CurrentSpecReport().Failed() {
				common.LogDebugInfo(common.Ambient, k)
				debugInfoLogged = true
			}
			clr.Cleanup(ctx)
		})
	})

	AfterAll(func(ctx SpecContext) {
		if CurrentSpecReport().Failed() && !debugInfoLogged {
			common.LogDebugInfo(common.Ambient, k)
			debugInfoLogged = true
		}
	})
})

func resolveJwtIssuer() (string, error) {
	bin := os.Getenv("COMMAND")
	if bin == "" {
		bin = "kubectl"
	}
	out, err := shell.ExecuteShell(
		fmt.Sprintf(`%s get ingresses.config/cluster -o jsonpath='{.spec.domain}'`, bin),
		"",
	)
	if err != nil {
		return "", err
	}

	domain := strings.TrimSpace(out)
	if domain == "" {
		return "", fmt.Errorf("empty cluster ingress domain")
	}

	return fmt.Sprintf("https://oidc-discovery.%s", domain), nil
}
