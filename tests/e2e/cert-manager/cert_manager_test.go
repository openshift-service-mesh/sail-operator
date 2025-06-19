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
// WITHOUT WARRANTIES OR Condition OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package certmanager

import (
	"fmt"
	"strings"
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"istio.io/istio/pkg/ptr"
)

var _ = Describe("Control Plane Installation", Label("smoke", "cert-manager", "slow"), Ordered, func() {
	SetDefaultEventuallyTimeout(180 * time.Second)
	SetDefaultEventuallyPollingInterval(time.Second)
	debugInfoLogged := false

	Describe("given Istio version", func() {
		version := istioversion.GetLatestPatchVersions()[0]
		Context(version.Name, func() {
			BeforeAll(func() {
				Expect(k.CreateNamespace(controlPlaneNamespace)).To(Succeed(), "Istio namespace failed to be created")
				Expect(k.CreateNamespace(istioCniNamespace)).To(Succeed(), "IstioCNI namespace failed to be created")
				Expect(k.CreateNamespace(istioCSRNamespace)).To(Succeed(), "IstioCSR Namespace failed to be created")
				Expect(k.CreateNamespace(certManagerOperatorNamespace)).To(Succeed(), "Cert Manager Operator Namespace failed to be created")
				Expect(k.CreateNamespace(certManagerNamespace)).To(Succeed(), "Cert Manager Namespace failed to be created")
				Expect(deployCertManagerOperator()).To(Succeed(), "Cert Manager Operator failed to be deployed")
				Expect(patchCertManagerOperator()).To(Succeed(), "Cert Manager Operator failed to be patched")
				output, err := k.WithNamespace(certManagerOperatorNamespace).GetYAML("subscription", certManagerDeploymentName)
				Expect(output).To(ContainSubstring(certManagerDeploymentName), "Subscription is not created")
				Expect(err).NotTo(HaveOccurred(), "error in Cert Manager Operator deployment")
				Eventually(func() bool {
					output, err := k.WithNamespace(certManagerNamespace).GetPods("-o", "jsonpath={.items[*].status.containerStatuses[*].ready}")
					if err != nil {
						return false
					}
					readyFlags := strings.Fields(output)
					readyCount := 0
					for _, flag := range readyFlags {
						if flag == "true" {
							readyCount++
						}
					}
					return readyCount >= 3
				}, 90*time.Second, 5*time.Second).Should(BeTrue(), fmt.Sprintf("Not all %d pods in %s are ready", 3, certManagerNamespace))
			})

			When("root CA issuer for the IstioCSR agent is created", func() {
				BeforeAll(func() {
					Expect(
						k.WithNamespace(certManagerOperatorNamespace).Patch(
							"subscription",
							"openshift-cert-manager-operator",
							"merge",
							`{"spec":{"config":{"env":[{"name":"UNSUPPORTED_ADDON_FEATURES","value":"IstioCSR=true"}]}}}`,
						),
					).To(Succeed(), "Error patching cert manager")
					Success("Cert Manager subscription patched")
					issuerYaml := `
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: selfsigned
  namespace: %s
spec:
  selfSigned: {}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: istio-ca
  namespace: %s
spec:
  isCA: true
  duration: 87600h # 10 years
  secretName: istio-ca
  commonName: istio-ca
  privateKey:
    algorithm: ECDSA
    size: 256
  subject:
    organizations:
      - cluster.local
      - cert-manager
  issuerRef:
    name: selfsigned
    kind: Issuer
    group: cert-manager.io
---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: istio-ca
  namespace: %s
spec:
  ca:
    secretName: istio-ca`
					issuerYaml = fmt.Sprintf(issuerYaml, controlPlaneNamespace, controlPlaneNamespace, controlPlaneNamespace)

					Expect(k.WithNamespace(controlPlaneNamespace).ApplyString(issuerYaml)).To(Succeed(), "Issuer creation failed")
				})
				It("creates certificate Issuer", func() {
					Eventually(func() string {
						output, _ := k.WithNamespace(controlPlaneNamespace).GetYAML("issuer", "istio-ca")
						return output
					}, 60*time.Second, 5*time.Second).Should(ContainSubstring("True"), "Issuer is not ready")
				})
			})

			When("custom resource for the IstioCSR is created", func() {
				BeforeAll(func() {
					istioCsrYaml := `
apiVersion: operator.openshift.io/v1alpha1
kind: IstioCSR
metadata:
  name: default
  namespace: %s
spec:
  istioCSRConfig:
    certManager:
      issuerRef:
        name: istio-ca
        kind: Issuer
        group: cert-manager.io
    istiodTLSConfig:
      trustDomain: cluster.local
    istio:
      namespace: %s`
					istioCsrYaml = fmt.Sprintf(istioCsrYaml, istioCSRNamespace, controlPlaneNamespace)
					Expect(k.CreateFromString(istioCsrYaml)).To(Succeed(), "IstioCsr custom resource creation failed")
				})

				It("has IstioCSR pods running", func() {
					Eventually(func() string {
						output, _ := k.WithNamespace(istioCSRNamespace).GetPods("", "-o wide")
						return output
					}, 60*time.Second, 5*time.Second).Should(ContainSubstring("cert-manager-istio-csr"), "cert-manager-istio-csr is not created")
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
					yaml = fmt.Sprintf(yaml, version.Name, istioCniNamespace)
					Log("IstioCNI YAML:", indent(yaml))
					Expect(k.CreateFromString(yaml)).To(Succeed(), "IstioCNI creation failed")
					Success("IstioCNI created")
				})

				It("deploys the CNI DaemonSet", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						daemonset := &appsv1.DaemonSet{}
						g.Expect(cl.Get(ctx, kube.Key("istio-cni-node", istioCniNamespace), daemonset)).To(Succeed(), "Error getting IstioCNI DaemonSet")
						g.Expect(daemonset.Status.NumberAvailable).
							To(Equal(daemonset.Status.CurrentNumberScheduled), "CNI DaemonSet Pods not Available; expected numberAvailable to be equal to currentNumberScheduled")
					}).Should(Succeed(), "CNI DaemonSet Pods are not Available")
					Success("CNI DaemonSet is deployed in the namespace and Running")
				})

				It("uses the correct image", func(ctx SpecContext) {
					Expect(common.GetObject(ctx, cl, kube.Key("istio-cni-node", istioCniNamespace), &appsv1.DaemonSet{})).
						To(HaveContainersThat(HaveEach(ImageFromRegistry(expectedRegistry))))
				})

				It("updates the status to Reconciled", func(ctx SpecContext) {
					Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(istioCniName), &v1.IstioCNI{}).
						Should(HaveConditionStatus(v1.IstioCNIConditionReconciled, metav1.ConditionTrue), "IstioCNI is not Reconciled; unexpected Condition")
					Success("IstioCNI is Reconciled")
				})

				It("updates the status to Ready", func(ctx SpecContext) {
					Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(istioCniName), &v1.IstioCNI{}).
						Should(HaveConditionStatus(v1.IstioCNIConditionReady, metav1.ConditionTrue), "IstioCNI is not Ready; unexpected Condition")
					Success("IstioCNI is Ready")
				})

				It("doesn't continuously reconcile the IstioCNI CR", func() {
					Eventually(k.WithNamespace(namespace).Logs).WithArguments("deploy/"+deploymentName, ptr.Of(30*time.Second)).
						ShouldNot(ContainSubstring("Reconciliation done"), "IstioCNI is continuously reconciling")
					Success("IstioCNI stopped reconciling")
				})
			})

			When("the Istio CR is created", func() {
				BeforeAll(func() {
					istioYAML := `
apiVersion: sailoperator.io/v1
kind: Istio
metadata:
  name: default
spec:
  values:
    global:
      caAddress: cert-manager-istio-csr.istio-csr.svc:443
    pilot:
      env:
        ENABLE_CA_SERVER: "false"
  version: %s
  namespace: %s`
					istioYAML = fmt.Sprintf(istioYAML, version.Name, controlPlaneNamespace)
					Log("Istio YAML:", indent(istioYAML))
					Expect(k.CreateFromString(istioYAML)).
						To(Succeed(), "Istio CR failed to be created")
					Success("Istio CR created")
					time.Sleep(120 * time.Second)
				})

				It("updates the Istio CR status to Reconciled", func(ctx SpecContext) {
					Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(istioName), &v1.Istio{}).
						Should(HaveConditionStatus(v1.IstioConditionReconciled, metav1.ConditionTrue), "Istio is not Reconciled; unexpected Condition")
					Success("Istio CR is Reconciled")
				})

				It("updates the Istio CR status to Ready", func(ctx SpecContext) {
					Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(istioName), &v1.Istio{}).
						Should(HaveConditionStatus(v1.IstioConditionReady, metav1.ConditionTrue), "Istio is not Ready; unexpected Condition")
					Success("Istio CR is Ready")
				})

				It("deploys istiod", func(ctx SpecContext) {
					Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{}).
						Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Istiod is not Available; unexpected Condition")
					Expect(common.GetVersionFromIstiod()).To(Equal(version.Version), "Unexpected istiod version")
					Success("Istiod is deployed in the namespace and Running")
				})

				It("uses the correct image", func(ctx SpecContext) {
					Expect(common.GetObject(ctx, cl, kube.Key("istiod", controlPlaneNamespace), &appsv1.Deployment{})).
						To(HaveContainersThat(HaveEach(ImageFromRegistry(expectedRegistry))))
				})

				It("doesn't continuously reconcile the Istio CR", func() {
					Eventually(k.WithNamespace(namespace).Logs).WithArguments("deploy/"+deploymentName, ptr.Of(30*time.Second)).
						ShouldNot(ContainSubstring("Reconciliation done"), "Istio CR is continuously reconciling")
					Success("Istio CR stopped reconciling")
				})
			})

			When("sample apps are deployed in the cluster", func() {
				BeforeAll(func(ctx SpecContext) {
					Expect(k.CreateNamespace(common.SleepNamespace)).To(Succeed(), "Failed to create sleep namespace")
					Expect(k.CreateNamespace(common.HttpbinNamespace)).To(Succeed(), "Failed to create httpbin namespace")

					Expect(k.Label("namespace", common.SleepNamespace, "istio-injection", "enabled")).To(Succeed(), "Error labeling sample namespace")
					Expect(k.Label("namespace", common.HttpbinNamespace, "istio-injection", "enabled")).To(Succeed(), "Error labeling sample namespace")

					// Deploy the test pods.
					Expect(k.WithNamespace(common.SleepNamespace).Apply(common.GetSampleYAML(version, "sleep"))).To(Succeed(), "error deploying sleep pod")
					Expect(k.WithNamespace(common.HttpbinNamespace).Apply(common.GetSampleYAML(version, "httpbin"))).To(Succeed(), "error deploying httpbin pod")

					yaml := `
apiVersion: security.istio.io/v1beta1
kind: PeerAuthentication
metadata:
  name: default
  namespace: %s
spec:
  mtls:
    mode: STRICT`
					yaml = fmt.Sprintf(yaml, controlPlaneNamespace)
					Expect(k.ApplyString(yaml)).To(Succeed(), "error enable peer authentcation")

					Success("validation pods deployed and PeerAuthentication with strict mTLS mode enabled")
				})

				sleepPod := &corev1.PodList{}
				It("updates the status of pods to Running", func(ctx SpecContext) {
					sleepPod, err = common.CheckPodsReady(ctx, cl, common.SleepNamespace)
					Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Error checking status of sleep pod: %v", err))

					_, err = common.CheckPodsReady(ctx, cl, common.HttpbinNamespace)
					Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Error checking status of httpbin pod: %v", err))

					Success("Pods are ready")
				})

				It("can access the httpbin service from the sleep pod", func(ctx SpecContext) {
					checkPodConnectivity(sleepPod.Items[0].Name, common.SleepNamespace, common.HttpbinNamespace)
				})

				AfterAll(func(ctx SpecContext) {
					By("Deleting the pods")
					Expect(k.DeleteNamespace(common.HttpbinNamespace, common.SleepNamespace)).
						To(Succeed(), "Failed to delete namespaces")
					Success("validation pods deleted")
				})
			})

			When("the Istio CR is deleted", func() {
				BeforeEach(func() {
					Expect(k.Delete("istio", istioName)).To(Succeed(), "Istio CR failed to be deleted")
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

			When("the IstioCSR is deleted", func() {
				BeforeEach(func() {
					Expect(k.WithNamespace(istioCSRNamespace).Delete("istiocsrs.operator.openshift.io", "default")).To(Succeed(), "Failed to delete istio-csr")

					// Namespaced resources
					Expect(k.WithNamespace(istioCSRNamespace).Delete("deployments.apps", "cert-manager-istio-csr")).To(Succeed(), "Failed to delete deployment")
					Expect(k.WithNamespace(istioCSRNamespace).Delete("services", "cert-manager-istio-csr")).To(Succeed(), "Failed to delete service")
					Expect(k.WithNamespace(istioCSRNamespace).Delete("serviceaccounts", "cert-manager-istio-csr")).To(Succeed(), "Failed to delete service account")
				})

				It("removes cert-manager-istio-csr resources from the cluster", func() {
					Eventually(func() string {
						output, _ := k.WithNamespace(istioCSRNamespace).GetYAML("istiocsrs.operator.openshift.io", "default")
						return strings.TrimSpace(output)
					}, 60*time.Second, 5*time.Second).Should(BeEmpty(), "istio-csr is not removed")
					Success("istio-csr is removed")

					Eventually(func() string {
						out, _ := k.GetYAML("clusterrolebindings.rbac.authorization.k8s.io", "cert-manager-istio-csr")
						return strings.TrimSpace(out)
					}, 30*time.Second, 5*time.Second).Should(BeEmpty(), "clusterrolebinding not removed")

					Eventually(func() string {
						out, _ := k.GetYAML("clusterroles.rbac.authorization.k8s.io", "cert-manager-istio-csr")
						return strings.TrimSpace(out)
					}, 30*time.Second, 5*time.Second).Should(BeEmpty(), "clusterrole not removed")

					Eventually(func() string {
						out, _ := k.WithNamespace(istioCSRNamespace).GetYAML("deployments.apps", "cert-manager-istio-csr")
						return strings.TrimSpace(out)
					}, 30*time.Second, 5*time.Second).Should(BeEmpty(), "deployment not removed")

					Eventually(func() string {
						out, _ := k.WithNamespace(istioCSRNamespace).GetYAML("services", "cert-manager-istio-csr")
						return strings.TrimSpace(out)
					}, 30*time.Second, 5*time.Second).Should(BeEmpty(), "service not removed")

					Eventually(func() string {
						out, _ := k.WithNamespace(istioCSRNamespace).GetYAML("serviceaccounts", "cert-manager-istio-csr")
						return strings.TrimSpace(out)
					}, 30*time.Second, 5*time.Second).Should(BeEmpty(), "service account not removed")

					Eventually(func() string {
						out, _ := k.WithNamespace(istioCSRNamespace).GetYAML("roles.rbac.authorization.k8s.io", "cert-manager-istio-csr")
						return strings.TrimSpace(out)
					}, 30*time.Second, 5*time.Second).Should(BeEmpty(), "role not removed")

					Eventually(func() string {
						out, _ := k.WithNamespace(istioCSRNamespace).GetYAML("rolebindings.rbac.authorization.k8s.io", "cert-manager-istio-csr")
						return strings.TrimSpace(out)
					}, 30*time.Second, 5*time.Second).Should(BeEmpty(), "rolebinding not removed")

					Eventually(func() string {
						out, _ := k.WithNamespace(istioCSRNamespace).GetYAML("certificates.cert-manager.io", "cert-manager-istio-csr")
						return strings.TrimSpace(out)
					}, 30*time.Second, 5*time.Second).Should(BeEmpty(), "certificate not removed")

					Success("All cert-manager-istio-csr resources are removed")
				})
			})

			When("the cert-manager resources are deleted", func() {
				BeforeEach(func() {
					err = k.WithNamespace(certManagerNamespace).Delete("rolebinding", "cert-manager-cert-manager-tokenrequest")
					if err != nil && !strings.Contains(err.Error(), "NotFound") {
						Fail("Failed to delete rolebinding: " + err.Error())
					}

					err = k.WithNamespace(certManagerNamespace).Delete("role", "cert-manager-tokenrequest")
					if err != nil && !strings.Contains(err.Error(), "NotFound") {
						Fail("Failed to delete role: " + err.Error())
					}
				})

				It("removes rolebinding cert-manager-tokenrequest from the cluster", func() {
					Eventually(func() string {
						output, _ := k.WithNamespace(certManagerNamespace).GetYAML("rolebinding", "cert-manager-cert-manager-tokenrequest")
						return strings.TrimSpace(output)
					}, 60*time.Second, 5*time.Second).Should(BeEmpty(), "rolebinding cert-manager-tokenrequest is not removed")
					Success("rolebinding cert-manager-tokenrequest is removed")
				})

				It("removes role cert-manager-tokenrequest from the cluster", func() {
					Eventually(func() string {
						output, _ := k.WithNamespace(certManagerNamespace).GetYAML("role", "cert-manager-tokenrequest")
						return strings.TrimSpace(output)
					}, 60*time.Second, 5*time.Second).Should(BeEmpty(), "role cert-manager-tokenrequest is not removed")
					Success("role cert-manager-tokenrequest is removed")
				})
			})

			When("the cert-manager-operator resources are deleted", func() {
				BeforeEach(func() {
					err := k.WithNamespace(certManagerOperatorNamespace).Delete("subscription", "openshift-cert-manager-operator")
					if err != nil && !strings.Contains(err.Error(), "NotFound") {
						Fail("Failed to delete Subscription: " + err.Error())
					}

					err = k.WithNamespace(certManagerOperatorNamespace).Delete("operatorgroup", "openshift-cert-manager-operator")
					if err != nil && !strings.Contains(err.Error(), "NotFound") {
						Fail("Failed to delete OperatorGroup: " + err.Error())
					}
				})

				It("removes subscription from the cert-manager-operator namespace", func() {
					Eventually(func() string {
						output, _ := k.WithNamespace(certManagerOperatorNamespace).GetYAML("subscription", "openshift-cert-manager-operator")
						return strings.TrimSpace(output)
					}, 60*time.Second, 5*time.Second).Should(BeEmpty(), "subscription is not removed")
					Success("subscription is removed")
				})

				It("removes operatorgroup from the cert-manager-operator namespace", func() {
					Eventually(func() string {
						output, _ := k.WithNamespace(certManagerOperatorNamespace).GetYAML("operatorgroup", "openshift-cert-manager-operator")
						return strings.TrimSpace(output)
					}, 60*time.Second, 5*time.Second).Should(BeEmpty(), "operatorgroup is not removed")
					Success("operatorgroup is removed")
				})
			})
		})

		AfterAll(func(ctx SpecContext) {
			if CurrentSpecReport().Failed() {
				common.LogDebugInfo(common.ControlPlane, k)
				debugInfoLogged = true
			}

			certManagerCRDs := []string{
				"certificaterequests.cert-manager.io",
				"certificates.cert-manager.io",
				"challenges.acme.cert-manager.io",
				"clusterissuers.cert-manager.io",
				"issuers.cert-manager.io",
				"orders.acme.cert-manager.io",
			}

			By("Deleting the CRDs")
			Expect(k.DeleteCRDs(certManagerCRDs)).To(Succeed(), "Cert-manager CRDs failed to be deleted")
			Success("CRDs deleted")

			By("Cleaning up the Istio namespace")
			Expect(k.DeleteNamespace(controlPlaneNamespace)).To(Succeed(), "Istio Namespace failed to be deleted")

			By("Cleaning up the IstioCNI namespace")
			Expect(k.DeleteNamespace(istioCniNamespace)).To(Succeed(), "IstioCNI Namespace failed to be deleted")

			By("Cleaning up the IstioCSR namespace")
			Expect(k.DeleteNamespace(istioCSRNamespace)).To(Succeed(), "IstioCSR Namespace failed to be deleted")

			By("Cleaning up the cert-manager-operator namespace")
			Expect(k.DeleteNamespace(certManagerOperatorNamespace)).To(Succeed(), "cert-manager-operator Namespace failed to be deleted")

			By("Cleaning up the cert-manager namespace")
			Expect(k.DeleteNamespace(certManagerNamespace)).To(Succeed(), "cert-manager Namespace failed to be deleted")
			Success("Cleanup done")
		})
	})

	AfterAll(func() {
		if CurrentSpecReport().Failed() && !debugInfoLogged {
			common.LogDebugInfo(common.ControlPlane, k)
			debugInfoLogged = true
		}
	})
})

func HaveContainersThat(matcher types.GomegaMatcher) types.GomegaMatcher {
	return HaveField("Spec.Template.Spec.Containers", matcher)
}

func ImageFromRegistry(regexp string) types.GomegaMatcher {
	return HaveField("Image", MatchRegexp(regexp))
}

func indent(str string) string {
	indent := strings.Repeat(" ", 2)
	return indent + strings.ReplaceAll(str, "\n", "\n"+indent)
}

func checkPodConnectivity(podName, srcNamespace, destNamespace string) {
	command := fmt.Sprintf(`curl -o /dev/null -s -w "%%{http_code}\n" httpbin.%s.svc.cluster.local:8000/get`, destNamespace)
	response, err := k.WithNamespace(srcNamespace).Exec(podName, srcNamespace, command)
	Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("error connecting to the %q pod", podName))
	Expect(response).To(ContainSubstring("200"), fmt.Sprintf("Unexpected response from %s pod", podName))
}

func deployCertManagerOperator() error {
	yaml := `
apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: openshift-cert-manager-operator
  namespace: cert-manager-operator
spec:
  targetNamespaces: []
  spec: {}
`
	if err := k.WithNamespace(certManagerOperatorNamespace).CreateFromString(yaml); err != nil {
		return fmt.Errorf("Operator creation failed: %w", err)
	}

	yaml = `
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: openshift-cert-manager-operator
  namespace: cert-manager-operator
spec:
  channel: stable-v1
  name: openshift-cert-manager-operator
  source: redhat-operators
  sourceNamespace: openshift-marketplace
  installPlanApproval: Automatic
`
	if err := k.WithNamespace(certManagerOperatorNamespace).CreateFromString(yaml); err != nil {
		return fmt.Errorf("Subscription creation failed: %w", err)
	}

	return nil
}

// This is a workaraound for https://issues.redhat.com/browse/OCPBUGS-56758
func patchCertManagerOperator() error {
	yaml := `
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  labels:
    app: cert-manager
    app.kubernetes.io/component: controller
    app.kubernetes.io/instance: cert-manager
    app.kubernetes.io/name: cert-manager
    app.kubernetes.io/version: v1.16.4
  name: cert-manager-cert-manager-tokenrequest
  namespace: cert-manager
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: cert-manager-tokenrequest
subjects:
  - kind: ServiceAccount
    name: cert-manager
    namespace: cert-manager
`
	if err := k.WithNamespace(certManagerNamespace).CreateFromString(yaml); err != nil {
		return fmt.Errorf("tokenrequest failed: %w", err)
	}

	yaml = `
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  labels:
    app: cert-manager
    app.kubernetes.io/component: controller
    app.kubernetes.io/instance: cert-manager
    app.kubernetes.io/name: cert-manager
    app.kubernetes.io/version: v1.16.4
  name: cert-manager-tokenrequest
  namespace: cert-manager
rules:
  - apiGroups:
      - ""
    resourceNames:
      - cert-manager
    resources:
      - serviceaccounts/token
    verbs:
      - create
`

	if err := k.WithNamespace(certManagerNamespace).CreateFromString(yaml); err != nil {
		return fmt.Errorf("tokenrequest role failed: %w", err)
	}

	return nil
}
