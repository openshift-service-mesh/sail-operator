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

package certmanager

import (
	"testing"

	"github.com/istio-ecosystem/sail-operator/pkg/env"
	"github.com/istio-ecosystem/sail-operator/pkg/kube"
	. "github.com/istio-ecosystem/sail-operator/pkg/test/util/ginkgo"
	k8sclient "github.com/istio-ecosystem/sail-operator/tests/e2e/util/client"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	. "github.com/istio-ecosystem/sail-operator/tests/e2e/util/gomega"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	cl                           client.Client
	err                          error
	namespace                    = common.OperatorNamespace
	certManagerOperatorNamespace = env.Get("CERT_MANAGER_OPERATOR_NAMESPACE", "cert-manager-operator")
	certManagerNamespace         = env.Get("CERT_MANAGER_NAMESPACE", "cert-manager")
	deploymentName               = env.Get("DEPLOYMENT_NAME", "sail-operator")
	certManagerDeploymentName    = env.Get("CERT_MANAGER_DEPLOYMENT_NAME", "openshift-cert-manager-operator")
	controlPlaneNamespace        = env.Get("CONTROL_PLANE_NS", "istio-system")
	istioCSRNamespace            = env.Get("ISTIO_CSR_NS", "istio-csr")
	istioName                    = env.Get("ISTIO_NAME", "default")
	istioCniNamespace            = env.Get("ISTIOCNI_NAMESPACE", "istio-cni")
	istioCniName                 = env.Get("ISTIOCNI_NAME", "default")
	skipDeploy                   = env.GetBool("SKIP_DEPLOY", false)
	expectedRegistry             = env.Get("EXPECTED_REGISTRY", "^docker\\.io|^gcr\\.io")

	k kubectl.Kubectl
)

func TestCertManager(t *testing.T) {
	RegisterFailHandler(Fail)
	setup()
	RunSpecs(t, "Cert Manager Test Suite")
}

func setup() {
	GinkgoWriter.Println("************ Running Setup ************")
	GinkgoWriter.Println("Initializing k8s client")
	cl, err = k8sclient.InitK8sClient("")
	Expect(err).NotTo(HaveOccurred())
	k = kubectl.New()
}

var _ = BeforeSuite(func(ctx SpecContext) {
	Expect(k.CreateNamespace(namespace)).To(Succeed(), "Namespace failed to be created")
	GinkgoWriter.Println("Created: namespace cert manager")
	if skipDeploy {
		Success("Skipping operator installation because it was deployed externally")
	} else {
		Expect(common.InstallOperatorViaHelm()).
			To(Succeed(), "Operator failed to be deployed")
	}

	Eventually(common.GetObject).WithArguments(ctx, cl, kube.Key(deploymentName, namespace), &appsv1.Deployment{}).
		Should(HaveConditionStatus(appsv1.DeploymentAvailable, metav1.ConditionTrue), "Error")
	Success("Cert Manager Operator is deployed in the namespace and Running")
})

var _ = AfterSuite(func(ctx SpecContext) {
	if skipDeploy {
		Success("Skipping operator undeploy because it was deployed externally")
		return
	}

	By("Deleting operator deployment")
	Expect(common.UninstallOperator()).
		To(Succeed(), "Operator failed to be deleted")
	GinkgoWriter.Println("Operator uninstalled")
})
