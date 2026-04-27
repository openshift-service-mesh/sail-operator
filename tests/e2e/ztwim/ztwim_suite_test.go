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
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/istio-ecosystem/sail-operator/pkg/env"
	k8sclient "github.com/istio-ecosystem/sail-operator/tests/e2e/util/client"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/common"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/kubectl"
	"github.com/istio-ecosystem/sail-operator/tests/e2e/util/shell"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	cl                    client.Client
	err                   error
	controlPlaneNamespace = common.ControlPlaneNamespace
	istioName             = env.Get("ISTIO_NAME", "default")
	istioCniNamespace     = common.IstioCniNamespace
	istioCniName          = env.Get("ISTIOCNI_NAME", "default")
	ztwimNamespace        = env.Get("ZTWIM_NAMESPACE", "zero-trust-workload-identity-manager")
	trustDomain           = env.Get("TRUST_DOMAIN", "ocp.one")
	jwtIssuer             = env.Get("JWT_ISSUER", "")
	multicluster          = env.GetBool("MULTICLUSTER", false)

	k kubectl.Kubectl
)

// isOpenshift dynamically checks if the cluster is OCP by looking for a core OpenShift API resource
func isOpenshift() bool {
	_, err := shell.ExecuteShell("kubectl get clusterversion", "")
	return err == nil
}

func TestZTWIM(t *testing.T) {
	if multicluster {
		t.Skip("Skipping test for multicluster")
	}

	if !isOpenshift() {
		t.Skip("Skipping test: ZTWIM is only supported on OpenShift (OCP)")
	}

	RegisterFailHandler(Fail)
	setup()
	RunSpecs(t, "Zero Trust Workload Identity Manager Test Suite")
}

func logMemoryStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	GinkgoWriter.Printf("Memory Stats: Alloc=%dMB, Sys=%dMB, HeapAlloc=%dMB, HeapSys=%dMB, NumGC=%d\n",
		m.Alloc/1024/1024, m.Sys/1024/1024, m.HeapAlloc/1024/1024, m.HeapSys/1024/1024, m.NumGC)

	// Try to read container memory limit from cgroups
	if data, err := os.ReadFile("/sys/fs/cgroup/memory.max"); err == nil {
		GinkgoWriter.Printf("Container memory limit (cgroups v2): %s", string(data))
	} else if data, err := os.ReadFile("/sys/fs/cgroup/memory/memory.limit_in_bytes"); err == nil {
		GinkgoWriter.Printf("Container memory limit (cgroups v1): %s", string(data))
	} else {
		GinkgoWriter.Println("Could not read container memory limit from cgroups")
	}

	// Log GOMEMLIMIT if set
	if gomemlimit := os.Getenv("GOMEMLIMIT"); gomemlimit != "" {
		GinkgoWriter.Printf("GOMEMLIMIT: %s\n", gomemlimit)
	}
}

func setup() {
	GinkgoWriter.Println("************ Running Setup ************")

	GinkgoWriter.Println("Memory diagnostics at test start:")
	logMemoryStats()

	GinkgoWriter.Println("Initializing k8s client")
	cl, err = k8sclient.InitK8sClient("")
	Expect(err).NotTo(HaveOccurred())

	GinkgoWriter.Println("Memory after k8s client init:")
	logMemoryStats()

	k = kubectl.New()
}
