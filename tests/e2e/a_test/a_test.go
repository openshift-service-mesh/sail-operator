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

package a_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("MachineConfig Monitor", Label("machineconfig-monitor"), Ordered, func() {
	When("waiting for MachineConfig changes", func() {
		It("sleeps for 1 hour to observe MachineConfig updates via background monitor", func() {
			GinkgoWriter.Println("Starting 1 hour sleep to monitor MachineConfig changes...")
			GinkgoWriter.Println("The MachineConfig monitor runs in the background and will log any changes to internalRegistryPullSecret")
			GinkgoWriter.Println("Look for [CC-DIFF-START]...[CC-DIFF-END] in the test output")

			time.Sleep(1 * time.Hour)

			GinkgoWriter.Println("1 hour sleep completed")
		})
	})
})
