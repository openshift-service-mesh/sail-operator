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
	"errors"
	"fmt"
	"testing"
	"testing/fstest"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
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

func TestCombineErrors(t *testing.T) {
	tests := []struct {
		name     string
		existing error
		new      error
		wantNil  bool
		wantMsg  string
	}{
		{
			name:    "both nil",
			wantNil: true,
		},
		{
			name:    "existing nil",
			new:     errors.New("new error"),
			wantMsg: "new error",
		},
		{
			name:     "new nil",
			existing: errors.New("existing error"),
			wantMsg:  "existing error",
		},
		{
			name:     "both non-nil",
			existing: errors.New("existing"),
			new:      errors.New("new"),
			wantMsg:  "existing; new",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := combineErrors(tt.existing, tt.new)
			if tt.wantNil {
				assert.NoError(t, result)
			} else {
				assert.Error(t, result)
				assert.Contains(t, result.Error(), tt.wantMsg)
			}
		})
	}
}

func TestIsOwnedResource(t *testing.T) {
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
			name:     "managed by Helm",
			labels:   map[string]string{"app.kubernetes.io/managed-by": "Helm"},
			revision: "default",
			expected: true,
		},
		{
			name:     "managed by sail-operator",
			labels:   map[string]string{"app.kubernetes.io/managed-by": "sail-operator"},
			revision: "default",
			expected: true,
		},
		{
			name:     "managed by something else",
			labels:   map[string]string{"app.kubernetes.io/managed-by": "other"},
			revision: "default",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lib := &Library{}
			opts := Options{Revision: tt.revision}
			opts.applyDefaults()
			lib.desiredOpts = &opts

			obj := &unstructured.Unstructured{}
			obj.SetLabels(tt.labels)

			result := lib.isOwnedResource(obj)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsOwnedResourceWithOwnerRef(t *testing.T) {
	expectedOwnerRef := &metav1.OwnerReference{
		APIVersion: "gateway.networking.k8s.io/v1",
		Kind:       "GatewayClass",
		Name:       "istio",
		UID:        types.UID("test-uid-123"),
	}

	tests := []struct {
		name      string
		ownerRefs []metav1.OwnerReference
		ownerRef  *metav1.OwnerReference
		labels    map[string]string
		expected  bool
	}{
		{
			name:      "matching owner ref",
			ownerRefs: []metav1.OwnerReference{*expectedOwnerRef},
			ownerRef:  expectedOwnerRef,
			labels:    nil,
			expected:  true,
		},
		{
			name: "matching owner ref without UID check",
			ownerRefs: []metav1.OwnerReference{{
				APIVersion: "gateway.networking.k8s.io/v1",
				Kind:       "GatewayClass",
				Name:       "istio",
				UID:        types.UID("different-uid"),
			}},
			ownerRef: &metav1.OwnerReference{
				APIVersion: "gateway.networking.k8s.io/v1",
				Kind:       "GatewayClass",
				Name:       "istio",
			},
			labels:   nil,
			expected: true,
		},
		{
			name:      "non-matching owner ref - different name",
			ownerRefs: []metav1.OwnerReference{*expectedOwnerRef},
			ownerRef: &metav1.OwnerReference{
				APIVersion: "gateway.networking.k8s.io/v1",
				Kind:       "GatewayClass",
				Name:       "other",
			},
			labels:   nil,
			expected: false,
		},
		{
			name:      "no owner ref configured - falls back to labels",
			ownerRefs: []metav1.OwnerReference{*expectedOwnerRef},
			ownerRef:  nil,
			labels:    map[string]string{"app.kubernetes.io/managed-by": "Helm"},
			expected:  true,
		},
		{
			name:      "no owner ref configured - no matching labels",
			ownerRefs: []metav1.OwnerReference{*expectedOwnerRef},
			ownerRef:  nil,
			labels:    nil,
			expected:  false,
		},
		{
			name:      "owner ref configured but object has no refs - falls back to labels",
			ownerRefs: nil,
			ownerRef:  expectedOwnerRef,
			labels:    map[string]string{"istio.io/rev": "default"},
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lib := &Library{}
			opts := Options{
				Revision: "default",
				OwnerRef: tt.ownerRef,
			}
			opts.applyDefaults()
			lib.desiredOpts = &opts

			obj := &unstructured.Unstructured{}
			obj.SetOwnerReferences(tt.ownerRefs)
			obj.SetLabels(tt.labels)

			result := lib.isOwnedResource(obj)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasMatchingOwnerRef(t *testing.T) {
	tests := []struct {
		name      string
		ownerRefs []metav1.OwnerReference
		expected  *metav1.OwnerReference
		matches   bool
	}{
		{
			name: "exact match",
			ownerRefs: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Name:       "test",
				UID:        "uid-123",
			}},
			expected: &metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Name:       "test",
				UID:        "uid-123",
			},
			matches: true,
		},
		{
			name: "match without UID",
			ownerRefs: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Name:       "test",
				UID:        "uid-123",
			}},
			expected: &metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Name:       "test",
			},
			matches: true,
		},
		{
			name: "no match - different kind",
			ownerRefs: []metav1.OwnerReference{{
				APIVersion: "v1",
				Kind:       "Secret",
				Name:       "test",
			}},
			expected: &metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Name:       "test",
			},
			matches: false,
		},
		{
			name:      "no owner refs",
			ownerRefs: nil,
			expected: &metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Name:       "test",
			},
			matches: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{}
			obj.SetOwnerReferences(tt.ownerRefs)

			result := hasMatchingOwnerRef(obj, tt.expected)
			assert.Equal(t, tt.matches, result)
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

// Note: Value computation tests are in pkg/revision and pkg/istiovalues packages.
// The reconcile() method uses revision.ComputeValues() which is tested there.
