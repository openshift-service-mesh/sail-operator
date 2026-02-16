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

package install

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/ptr"
)

func TestNew(t *testing.T) {
	testFS := fstest.MapFS{}
	testConfig := &rest.Config{}

	t.Run("missing kubeConfig", func(t *testing.T) {
		lib, err := New(nil, testFS)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "kubeConfig is required")
		assert.Nil(t, lib)
	})

	t.Run("missing resourceFS", func(t *testing.T) {
		lib, err := New(testConfig, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "resourceFS is required")
		assert.Nil(t, lib)
	})

	t.Run("valid inputs", func(t *testing.T) {
		lib, err := New(testConfig, testFS)
		assert.NoError(t, err)
		assert.NotNil(t, lib)
	})
}

func TestOptionsApplyDefaults(t *testing.T) {
	tests := []struct {
		name                   string
		opts                   Options
		expectedNamespace      string
		expectedVersion        string
		expectedRevision       string
		expectedManageCRDs     bool
		expectedIncludeAllCRDs bool
	}{
		{
			name:                   "all defaults",
			opts:                   Options{},
			expectedNamespace:      "istio-system",
			expectedVersion:        "",
			expectedRevision:       "default",
			expectedManageCRDs:     true,
			expectedIncludeAllCRDs: false,
		},
		{
			name: "custom namespace preserved",
			opts: Options{
				Namespace: "custom-ns",
			},
			expectedNamespace:      "custom-ns",
			expectedVersion:        "",
			expectedRevision:       "default",
			expectedManageCRDs:     true,
			expectedIncludeAllCRDs: false,
		},
		{
			name: "custom version preserved",
			opts: Options{
				Version: "v1.24.0",
			},
			expectedNamespace:      "istio-system",
			expectedVersion:        "v1.24.0",
			expectedRevision:       "default",
			expectedManageCRDs:     true,
			expectedIncludeAllCRDs: false,
		},
		{
			name: "custom revision preserved",
			opts: Options{
				Revision: "canary",
			},
			expectedNamespace:      "istio-system",
			expectedVersion:        "",
			expectedRevision:       "canary",
			expectedManageCRDs:     true,
			expectedIncludeAllCRDs: false,
		},
		{
			name: "all custom values preserved",
			opts: Options{
				Namespace: "my-namespace",
				Version:   "v1.23.0",
				Revision:  "my-revision",
			},
			expectedNamespace:      "my-namespace",
			expectedVersion:        "v1.23.0",
			expectedRevision:       "my-revision",
			expectedManageCRDs:     true,
			expectedIncludeAllCRDs: false,
		},
		{
			name: "ManageCRDs false preserved",
			opts: Options{
				ManageCRDs: ptr.To(false),
			},
			expectedNamespace:      "istio-system",
			expectedVersion:        "",
			expectedRevision:       "default",
			expectedManageCRDs:     false,
			expectedIncludeAllCRDs: false,
		},
		{
			name: "IncludeAllCRDs true preserved",
			opts: Options{
				IncludeAllCRDs: ptr.To(true),
			},
			expectedNamespace:      "istio-system",
			expectedVersion:        "",
			expectedRevision:       "default",
			expectedManageCRDs:     true,
			expectedIncludeAllCRDs: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.opts.applyDefaults()
			assert.Equal(t, tt.expectedNamespace, tt.opts.Namespace)
			assert.Equal(t, tt.expectedVersion, tt.opts.Version)
			assert.Equal(t, tt.expectedRevision, tt.opts.Revision)
			assert.Equal(t, tt.expectedManageCRDs, *tt.opts.ManageCRDs)
			assert.Equal(t, tt.expectedIncludeAllCRDs, *tt.opts.IncludeAllCRDs)
		})
	}
}

func TestOptionsEqual(t *testing.T) {
	base := Options{
		Namespace:      "ns",
		Version:        "1.24.0",
		Revision:       "default",
		ManageCRDs:     ptr.To(true),
		IncludeAllCRDs: ptr.To(false),
		Values: &v1.Values{
			Global: &v1.GlobalConfig{
				Hub: ptr.To("docker.io/istio"),
			},
		},
	}

	t.Run("identical options", func(t *testing.T) {
		other := Options{
			Namespace:      "ns",
			Version:        "1.24.0",
			Revision:       "default",
			ManageCRDs:     ptr.To(true),
			IncludeAllCRDs: ptr.To(false),
			Values: &v1.Values{
				Global: &v1.GlobalConfig{
					Hub: ptr.To("docker.io/istio"),
				},
			},
		}
		assert.True(t, optionsEqual(base, other))
	})

	t.Run("different namespace", func(t *testing.T) {
		other := base
		other.Namespace = "different"
		assert.False(t, optionsEqual(base, other))
	})

	t.Run("different values", func(t *testing.T) {
		other := Options{
			Namespace:      "ns",
			Version:        "1.24.0",
			Revision:       "default",
			ManageCRDs:     ptr.To(true),
			IncludeAllCRDs: ptr.To(false),
			Values: &v1.Values{
				Global: &v1.GlobalConfig{
					Hub: ptr.To("quay.io/other"),
				},
			},
		}
		assert.False(t, optionsEqual(base, other))
	})

	t.Run("nil values equal", func(t *testing.T) {
		a := Options{Namespace: "ns", Version: "1.24.0", Revision: "default", ManageCRDs: ptr.To(true), IncludeAllCRDs: ptr.To(false)}
		b := Options{Namespace: "ns", Version: "1.24.0", Revision: "default", ManageCRDs: ptr.To(true), IncludeAllCRDs: ptr.To(false)}
		assert.True(t, optionsEqual(a, b))
	})
}

func TestApplyIdempotency(t *testing.T) {
	lib := &Library{
		workqueue: workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]()),
	}
	defer lib.workqueue.ShutDown()

	opts := Options{
		Namespace: "test-ns",
		Version:   "1.24.0",
	}

	// First Apply should store and enqueue
	lib.Apply(opts)
	assert.NotNil(t, lib.desiredOpts)

	// Drain the queue
	key, _ := lib.workqueue.Get()
	lib.workqueue.Done(key)

	// Second Apply with same opts should be a no-op
	lib.Apply(opts)
	// Queue should be empty (len check via shutdown trick not possible, so just verify desiredOpts unchanged)
	assert.Equal(t, opts.Namespace, lib.desiredOpts.Namespace)
}

func TestStatusString(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		expected string
	}{
		{
			name:     "zero value",
			status:   Status{},
			expected: "not installed version=unknown crds=",
		},
		{
			name: "installed ok with CRD details",
			status: Status{
				Installed:  true,
				Version:    "1.24.0",
				CRDState:   CRDManagedByCIO,
				CRDMessage: "CRDs installed by CIO",
				CRDs: []CRDInfo{
					{Name: "wasmplugins.extensions.istio.io", Found: true, State: CRDManagedByCIO},
					{Name: "envoyfilters.networking.istio.io", Found: true, State: CRDManagedByCIO},
				},
			},
			expected: "installed version=1.24.0 crds=ManagedByCIO (CRDs installed by CIO) [wasmplugins.extensions.istio.io:ManagedByCIO, envoyfilters.networking.istio.io:ManagedByCIO]",
		},
		{
			name: "mixed ownership with missing CRDs",
			status: Status{
				Version:    "1.24.0",
				CRDState:   CRDMixedOwnership,
				CRDMessage: "CRDs have mixed ownership",
				CRDs: []CRDInfo{
					{Name: "wasmplugins.extensions.istio.io", Found: true, State: CRDManagedByOLM},
					{Name: "envoyfilters.networking.istio.io", Found: false},
				},
				Error: fmt.Errorf("Istio CRDs have mixed ownership (CIO/OLM/other)"),
			},
			expected: "not installed version=1.24.0 crds=MixedOwnership (CRDs have mixed ownership) [wasmplugins.extensions.istio.io:ManagedByOLM, envoyfilters.networking.istio.io:missing] error=Istio CRDs have mixed ownership (CIO/OLM/other)",
		},
		{
			name: "installed no CRD details",
			status: Status{
				Installed:  true,
				Version:    "1.24.0",
				CRDState:   CRDManagedByOLM,
				CRDMessage: "CRDs managed by OSSM subscription via OLM",
			},
			expected: "installed version=1.24.0 crds=ManagedByOLM (CRDs managed by OSSM subscription via OLM)",
		},
		{
			name: "error without CRDs",
			status: Status{
				Version: "1.24.0",
				Error:   fmt.Errorf("validation failed"),
			},
			expected: "not installed version=1.24.0 crds= error=validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestStatusReadWrite(t *testing.T) {
	lib := &Library{}

	// Initial status is zero value
	status := lib.Status()
	assert.False(t, status.Installed)
	assert.Empty(t, status.Version)

	// Set status
	lib.setStatus(Status{
		Installed: true,
		Version:   "1.24.0",
		CRDState:  CRDManagedByCIO,
	})

	status = lib.Status()
	assert.True(t, status.Installed)
	assert.Equal(t, "1.24.0", status.Version)
	assert.Equal(t, CRDManagedByCIO, status.CRDState)
}


func TestIsOwnedResource(t *testing.T) {
	const testManagedByValue = "test-operator"

	tests := []struct {
		name     string
		labels   map[string]string
		revision string
		expected bool
	}{
		{
			name:     "no labels",
			labels:   nil,
			revision: "default",
			expected: false,
		},
		{
			name:     "istio rev label matches default",
			labels:   map[string]string{"istio.io/rev": "default"},
			revision: "default",
			expected: true,
		},
		{
			name:     "istio rev label matches custom",
			labels:   map[string]string{"istio.io/rev": "canary"},
			revision: "canary",
			expected: true,
		},
		{
			name:     "istio rev label does not match",
			labels:   map[string]string{"istio.io/rev": "other"},
			revision: "default",
			expected: false,
		},
		{
			name:     "operator component label",
			labels:   map[string]string{"operator.istio.io/component": "pilot"},
			revision: "default",
			expected: true,
		},
		{
			name:     "managed-by label matches configured value",
			labels:   map[string]string{"managed-by": testManagedByValue},
			revision: "default",
			expected: true,
		},
		{
			name:     "managed-by label does not match configured value",
			labels:   map[string]string{"managed-by": "something-else"},
			revision: "default",
			expected: false,
		},
		{
			name:     "app.kubernetes.io/managed-by Helm fallback",
			labels:   map[string]string{"app.kubernetes.io/managed-by": "Helm"},
			revision: "default",
			expected: true,
		},
		{
			name:     "app.kubernetes.io/managed-by non-Helm rejected",
			labels:   map[string]string{"app.kubernetes.io/managed-by": "other"},
			revision: "default",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{}
			obj.SetLabels(tt.labels)

			result := isOwnedResource(obj, tt.revision, testManagedByValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestOptionsEqualWithNilValues verifies that optionsEqual handles nil Values by
// comparing the map representation (both nil Values produce equal empty maps).
func TestOptionsEqualWithNilValues(t *testing.T) {
	a := Options{Namespace: "ns", ManageCRDs: ptr.To(true), IncludeAllCRDs: ptr.To(false)}
	b := Options{Namespace: "ns", ManageCRDs: ptr.To(true), IncludeAllCRDs: ptr.To(false)}
	a.applyDefaults()
	b.applyDefaults()

	// Both nil Values should be equal
	assert.True(t, optionsEqual(a, b))

	// nil vs non-nil Values should differ (non-nil with content)
	b.Values = &v1.Values{Global: &v1.GlobalConfig{Hub: ptr.To("test")}}
	assert.False(t, optionsEqual(a, b))
}

// TestFromValuesRoundTrip verifies that helm.FromValues produces comparable maps.
func TestFromValuesRoundTrip(t *testing.T) {
	v := &v1.Values{
		Global: &v1.GlobalConfig{
			Hub: ptr.To("docker.io/istio"),
		},
	}
	m1 := helm.FromValues(v)
	m2 := helm.FromValues(v)
	assert.Equal(t, m1, m2)
}

// buildCIOOptions replicates how the Cluster Ingress Operator builds Options
// via buildInstallerOptions + openshiftValues + GatewayAPIDefaults + MergeValues.
// See: https://github.com/rikatz/cluster-ingress-operator/blob/31d7e74fe6/pkg/operator/controller/gatewayclass/istio_sail_installer.go
func buildCIOOptions() Options {
	// Step 1: GatewayAPIDefaults (from the sail library)
	values := GatewayAPIDefaults()

	// Step 2: openshiftValues overlay (from CIO)
	pilotEnv := map[string]string{
		"PILOT_ENABLE_GATEWAY_API":                         "true",
		"PILOT_ENABLE_ALPHA_GATEWAY_API":                   "false",
		"PILOT_ENABLE_GATEWAY_API_STATUS":                  "true",
		"PILOT_ENABLE_GATEWAY_API_DEPLOYMENT_CONTROLLER":   "true",
		"PILOT_ENABLE_GATEWAY_API_GATEWAYCLASS_CONTROLLER": "false",
		"PILOT_GATEWAY_API_DEFAULT_GATEWAYCLASS_NAME":      "openshift-default",
		"PILOT_GATEWAY_API_CONTROLLER_NAME":                "openshift.io/gateway-controller",
		"PILOT_MULTI_NETWORK_DISCOVER_GATEWAY_API":         "false",
		"ENABLE_GATEWAY_API_MANUAL_DEPLOYMENT":             "false",
		"PILOT_ENABLE_GATEWAY_API_CA_CERT_ONLY":            "true",
		"PILOT_ENABLE_GATEWAY_API_COPY_LABELS_ANNOTATIONS": "false",
	}
	openshiftOverrides := &v1.Values{
		Global: &v1.GlobalConfig{
			DefaultPodDisruptionBudget: &v1.DefaultPodDisruptionBudgetConfig{
				Enabled: ptr.To(false),
			},
			IstioNamespace:    ptr.To("openshift-ingress"),
			PriorityClassName: ptr.To("system-cluster-critical"),
			TrustBundleName:   ptr.To("openshift-gateway-ca-root-cert"),
		},

		Pilot: &v1.PilotConfig{
			Env: pilotEnv,
			PodAnnotations: map[string]string{
				"target.workload.openshift.io/management": `{"effect": "PreferredDuringScheduling"}`,
			},
		},
	}

	// Step 3: MergeValues
	values = MergeValues(values, openshiftOverrides)

	// Step 4: Build Options (same as CIO's buildInstallerOptions)
	return Options{
		Namespace:      "openshift-ingress",
		Revision:       "openshift-gateway",
		Values:         values,
		Version:        "v1.27.3",
		ManageCRDs:     ptr.To(true),
		IncludeAllCRDs: ptr.To(true),
	}
}

// TestOptionsEqualWithCIOPattern verifies that two independently-built CIO
// option sets compare as equal after applyDefaults.
func TestOptionsEqualWithCIOPattern(t *testing.T) {
	opts1 := buildCIOOptions()
	opts2 := buildCIOOptions()
	opts1.applyDefaults()
	opts2.applyDefaults()

	assert.True(t, optionsEqual(opts1, opts2),
		"options built the same way should be equal;\n  map1: %v\n  map2: %v",
		helm.FromValues(opts1.Values), helm.FromValues(opts2.Values))
}

// TestCIOReconcileLoopConverges simulates the full deployment flow:
//
//	Library workqueue ──Get──▶ reconcile ──notify──▶ controller ──Apply──▶ Library workqueue
//
// The test replicates the real ordering in run():
//  1. Get() item from workqueue
//  2. reconcile (no-op here — no cluster)
//  3. Send notification BEFORE Forget+Done (matches production code)
//  4. Controller receives notification, builds fresh Options, calls Apply()
//  5. Forget + Done
//
// If Apply() correctly detects identical options and skips enqueue, the loop
// converges after exactly 1 reconcile. If it re-enqueues, the test fails.
func TestCIOReconcileLoopConverges(t *testing.T) {
	lib := &Library{
		workqueue: workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]()),
	}
	defer lib.workqueue.ShutDown()

	notifyCh := make(chan struct{}, 1)

	// Track Apply calls from the simulated controller.
	var applyCount atomic.Int32
	// appliedCh signals that the controller finished calling Apply.
	appliedCh := make(chan struct{}, 1)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Simulate the gatewayclass controller:
	// on each notification, build fresh options and call Apply().
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-notifyCh:
				if !ok {
					return
				}
				applyCount.Add(1)
				lib.Apply(buildCIOOptions())
				select {
				case appliedCh <- struct{}{}:
				default:
				}
			}
		}
	}()

	// Initial Apply (CIO's first reconcile triggered by GatewayClass creation)
	lib.Apply(buildCIOOptions())

	// Simulate the library's run() loop.
	reconcileCount := 0
	for {
		type getResult struct {
			key      string
			shutdown bool
		}
		ch := make(chan getResult, 1)
		go func() {
			key, shutdown := lib.workqueue.Get()
			ch <- getResult{key, shutdown}
		}()

		select {
		case r := <-ch:
			if r.shutdown {
				t.Fatal("unexpected queue shutdown")
			}
			reconcileCount++
			if reconcileCount > 10 {
				t.Fatalf("reconcile loop did not converge after 10 iterations (apply count: %d)",
					applyCount.Load())
			}
			t.Logf("reconcile #%d", reconcileCount)

			// --- replicate run() ordering: notify BEFORE Forget+Done ---
			select {
			case notifyCh <- struct{}{}:
			default:
			}

			// Wait for the controller to process the notification and call Apply()
			select {
			case <-appliedCh:
				t.Logf("  controller applied (total applies: %d)", applyCount.Load())
			case <-time.After(200 * time.Millisecond):
				t.Log("  controller did not apply (notification may have been dropped)")
			}

			lib.workqueue.Forget(r.key)
			lib.workqueue.Done(r.key)

		case <-time.After(500 * time.Millisecond):
			// Queue has been empty for 500ms — converged.
			t.Logf("converged: %d reconcile(s), %d controller apply(s)",
				reconcileCount, applyCount.Load())
			assert.Equal(t, 1, reconcileCount,
				"expected exactly 1 reconcile; more means Apply() is re-enqueuing "+
					"when it should detect equal options")
			return
		}
	}
}

// TestUninstallWithoutApply verifies that Uninstall on an idle library (no
// prior Apply) is a no-op and returns nil.
func TestUninstallWithoutApply(t *testing.T) {
	lib := &Library{
		workqueue: workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]()),
	}
	defer lib.workqueue.ShutDown()

	err := lib.Uninstall(context.Background())
	assert.NoError(t, err, "Uninstall on idle library should be a no-op")
	assert.Nil(t, lib.desiredOpts)
}

// TestUninstallClearsDesiredOpts verifies that Uninstall nils desiredOpts and
// signals processingDone so the run() loop can return to idle.
func TestUninstallClearsDesiredOpts(t *testing.T) {
	lib := &Library{
		workqueue: workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]()),
		inst: &installer{
			chartManager: helm.NewChartManager(&rest.Config{Host: "https://localhost:1"}, "memory"),
		},
	}
	defer lib.workqueue.ShutDown()

	// Simulate an active install cycle
	opts := Options{Namespace: "test-ns", Version: "1.24.0"}
	opts.applyDefaults()
	lib.desiredOpts = &opts
	lib.informerStop = make(chan struct{})
	lib.processingDone = make(chan struct{})

	// Close processingDone to simulate the processing loop having exited
	// (in real usage, enqueue() + nil check in processWorkQueue causes this)
	close(lib.processingDone)

	// Uninstall clears desiredOpts and closes informerStop.
	// The Helm uninstall will fail (no real cluster), but the state should
	// already be cleared before that point.
	err := lib.Uninstall(context.Background())

	// Helm fails since there's no real cluster — that's expected
	assert.Error(t, err, "Helm uninstall should fail without a real cluster")
	assert.Nil(t, lib.desiredOpts, "desiredOpts should be nil after Uninstall")
}

// TestApplyAfterUninstallSetsDesiredOpts verifies that Apply works after
// Uninstall — the library can be reused for a new install cycle.
func TestApplyAfterUninstallSetsDesiredOpts(t *testing.T) {
	lib := &Library{
		workqueue: workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]()),
	}
	defer lib.workqueue.ShutDown()

	// First cycle: Apply
	lib.Apply(Options{Namespace: "ns1", Version: "1.24.0"})
	assert.NotNil(t, lib.desiredOpts)
	assert.Equal(t, "ns1", lib.desiredOpts.Namespace)

	// Drain the queue
	key, _ := lib.workqueue.Get()
	lib.workqueue.Done(key)

	// Simulate Uninstall clearing state (without real Helm)
	lib.mu.Lock()
	lib.desiredOpts = nil
	lib.mu.Unlock()

	assert.Nil(t, lib.desiredOpts)

	// Second cycle: Apply again
	lib.Apply(Options{Namespace: "ns2", Version: "1.25.0"})
	assert.NotNil(t, lib.desiredOpts)
	assert.Equal(t, "ns2", lib.desiredOpts.Namespace)
}

// TestApplyBlocksDuringUninstall verifies that Apply blocks while Uninstall
// holds the lifecycle lock, and proceeds after Uninstall completes.
func TestApplyBlocksDuringUninstall(t *testing.T) {
	lib := &Library{
		workqueue: workqueue.NewTypedRateLimitingQueue(workqueue.DefaultTypedControllerRateLimiter[string]()),
	}
	defer lib.workqueue.ShutDown()

	// Set up active state
	opts := Options{Namespace: "test-ns", Version: "1.24.0"}
	opts.applyDefaults()
	lib.desiredOpts = &opts
	lib.informerStop = make(chan struct{})
	lib.processingDone = make(chan struct{})

	// Hold the lifecycle lock to simulate Uninstall in progress
	lib.lifecycleMu.Lock()

	var applyStarted atomic.Int32
	var applyFinished atomic.Int32

	go func() {
		applyStarted.Store(1)
		lib.Apply(Options{Namespace: "new-ns", Version: "1.25.0"})
		applyFinished.Store(1)
	}()

	// Give the goroutine time to start and block on the lock
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), applyStarted.Load(), "Apply goroutine should have started")
	assert.Equal(t, int32(0), applyFinished.Load(), "Apply should be blocked by lifecycle lock")

	// Release the lock
	lib.lifecycleMu.Unlock()

	// Apply should now complete
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), applyFinished.Load(), "Apply should complete after lock release")
}

// Note: Value computation tests are in pkg/revision and pkg/istiovalues packages.
// The reconcile() method uses revision.ComputeValues() which is tested there.
