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
	"testing"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAggregateCRDState(t *testing.T) {
	tests := []struct {
		name     string
		infos    []CRDInfo
		expected CRDManagementState
	}{
		{
			name:     "empty list",
			infos:    []CRDInfo{},
			expected: CRDNoneExist,
		},
		{
			name: "all not found",
			infos: []CRDInfo{
				{Name: "a.istio.io", Found: false},
				{Name: "b.istio.io", Found: false},
			},
			expected: CRDNoneExist,
		},
		{
			name: "all CIO",
			infos: []CRDInfo{
				{Name: "a.istio.io", Found: true, State: CRDManagedByCIO},
				{Name: "b.istio.io", Found: true, State: CRDManagedByCIO},
			},
			expected: CRDManagedByCIO,
		},
		{
			name: "all OLM",
			infos: []CRDInfo{
				{Name: "a.istio.io", Found: true, State: CRDManagedByOLM},
				{Name: "b.istio.io", Found: true, State: CRDManagedByOLM},
			},
			expected: CRDManagedByOLM,
		},
		{
			name: "CIO with unknown - reclaim as CIO",
			infos: []CRDInfo{
				{Name: "a.istio.io", Found: true, State: CRDManagedByCIO},
				{Name: "b.istio.io", Found: true, State: CRDUnknownManagement},
			},
			expected: CRDManagedByCIO,
		},
		{
			name: "OLM with unknown - mixed",
			infos: []CRDInfo{
				{Name: "a.istio.io", Found: true, State: CRDManagedByOLM},
				{Name: "b.istio.io", Found: true, State: CRDUnknownManagement},
			},
			expected: CRDMixedOwnership,
		},
		{
			name: "CIO and OLM mix",
			infos: []CRDInfo{
				{Name: "a.istio.io", Found: true, State: CRDManagedByCIO},
				{Name: "b.istio.io", Found: true, State: CRDManagedByOLM},
			},
			expected: CRDMixedOwnership,
		},
		{
			name: "CIO with some missing - still CIO owned",
			infos: []CRDInfo{
				{Name: "a.istio.io", Found: true, State: CRDManagedByCIO},
				{Name: "b.istio.io", Found: false},
			},
			expected: CRDManagedByCIO,
		},
		{
			name: "OLM with some missing - mixed",
			infos: []CRDInfo{
				{Name: "a.istio.io", Found: true, State: CRDManagedByOLM},
				{Name: "b.istio.io", Found: false},
			},
			expected: CRDMixedOwnership,
		},
		{
			name: "all found but all unknown",
			infos: []CRDInfo{
				{Name: "a.istio.io", Found: true, State: CRDUnknownManagement},
				{Name: "b.istio.io", Found: true, State: CRDUnknownManagement},
			},
			expected: CRDUnknownManagement,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := aggregateCRDState(tt.infos)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMissingCRDNames(t *testing.T) {
	infos := []CRDInfo{
		{Name: "a.istio.io", Found: true},
		{Name: "b.istio.io", Found: false},
		{Name: "c.istio.io", Found: true},
		{Name: "d.istio.io", Found: false},
	}

	missing := missingCRDNames(infos)
	assert.Equal(t, []string{"b.istio.io", "d.istio.io"}, missing)
}

func TestMissingCRDNamesAllFound(t *testing.T) {
	infos := []CRDInfo{
		{Name: "a.istio.io", Found: true},
		{Name: "b.istio.io", Found: true},
	}

	missing := missingCRDNames(infos)
	assert.Empty(t, missing)
}

// crdScheme returns a runtime.Scheme with CRD types registered.
func crdScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = apiextensionsv1.AddToScheme(s)
	return s
}

func TestClassifyCRD(t *testing.T) {
	tests := []struct {
		name     string
		crd      *apiextensionsv1.CustomResourceDefinition
		expected CRDInfo
	}{
		{
			name: "CRD with CIO label",
			crd: &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "wasmplugins.extensions.istio.io",
					Labels: map[string]string{labelManagedByCIO: "true"},
				},
			},
			expected: CRDInfo{
				Name:  "wasmplugins.extensions.istio.io",
				Found: true,
				State: CRDManagedByCIO,
			},
		},
		{
			name: "CRD with OLM label",
			crd: &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "envoyfilters.networking.istio.io",
					Labels: map[string]string{labelOLMManaged: "true"},
				},
			},
			expected: CRDInfo{
				Name:  "envoyfilters.networking.istio.io",
				Found: true,
				State: CRDManagedByOLM,
			},
		},
		{
			name: "CRD with no labels",
			crd: &apiextensionsv1.CustomResourceDefinition{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gateways.networking.istio.io",
				},
			},
			expected: CRDInfo{
				Name:  "gateways.networking.istio.io",
				Found: true,
				State: CRDUnknownManagement,
			},
		},
		{
			name: "CRD not found",
			crd:  nil,
			expected: CRDInfo{
				Name:  "missing.istio.io",
				Found: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(crdScheme())
			if tt.crd != nil {
				builder = builder.WithObjects(tt.crd)
			}
			cl := builder.Build()
			mgr := newCRDManager(cl)

			crdName := tt.expected.Name
			result := mgr.classifyCRD(context.Background(), crdName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUnlabeledCRDNames(t *testing.T) {
	tests := []struct {
		name     string
		infos    []CRDInfo
		expected []string
	}{
		{
			name: "mixed states",
			infos: []CRDInfo{
				{Name: "a.istio.io", Found: true, State: CRDManagedByCIO},
				{Name: "b.istio.io", Found: true, State: CRDUnknownManagement},
				{Name: "c.istio.io", Found: false},
				{Name: "d.istio.io", Found: true, State: CRDUnknownManagement},
			},
			expected: []string{"b.istio.io", "d.istio.io"},
		},
		{
			name: "all labeled",
			infos: []CRDInfo{
				{Name: "a.istio.io", Found: true, State: CRDManagedByCIO},
				{Name: "b.istio.io", Found: true, State: CRDManagedByOLM},
			},
			expected: nil,
		},
		{
			name:     "empty",
			infos:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := unlabeledCRDNames(tt.infos)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// --- Helpers for Reconcile / WatchTargets tests ---

// The 3 CRD resource names used by Gateway API mode.
var gatewayAPICRDNames = []string{
	"wasmplugins.extensions.istio.io",
	"envoyfilters.networking.istio.io",
	"destinationrules.networking.istio.io",
}

// makeGatewayAPIValues returns Values with the Gateway API env vars that
// produce the 3 target CRDs via targetCRDsFromValues.
func makeGatewayAPIValues() *v1.Values {
	return &v1.Values{
		Pilot: &v1.PilotConfig{
			Env: map[string]string{
				envPilotIgnoreResources:  gatewayAPIIgnoreResources,
				envPilotIncludeResources: gatewayAPIIncludeResources,
			},
		},
	}
}

// makeCRDStub creates a minimal CRD object with the given name and labels.
func makeCRDStub(name string, labels map[string]string) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}

// getCRDFromClient fetches a CRD by name from the fake client.
func getCRDFromClient(t *testing.T, cl client.Client, name string) *apiextensionsv1.CustomResourceDefinition {
	t.Helper()
	crd := &apiextensionsv1.CustomResourceDefinition{}
	err := cl.Get(context.Background(), client.ObjectKey{Name: name}, crd)
	require.NoError(t, err, "expected CRD %s to exist on fake client", name)
	return crd
}

// --- TestApplyCIOLabels ---

func TestApplyCIOLabels(t *testing.T) {
	t.Run("bare CRD gets labels and annotations", func(t *testing.T) {
		crd := &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: "test.istio.io"},
		}
		applyCIOLabels(crd)
		assert.Equal(t, "true", crd.Labels[labelManagedByCIO])
		assert.Equal(t, "keep", crd.Annotations[annotationHelmKeep])
	})

	t.Run("existing labels and annotations preserved", func(t *testing.T) {
		crd := &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test.istio.io",
				Labels:      map[string]string{"existing": "label"},
				Annotations: map[string]string{"existing": "annotation"},
			},
		}
		applyCIOLabels(crd)
		assert.Equal(t, "true", crd.Labels[labelManagedByCIO])
		assert.Equal(t, "label", crd.Labels["existing"])
		assert.Equal(t, "keep", crd.Annotations[annotationHelmKeep])
		assert.Equal(t, "annotation", crd.Annotations["existing"])
	})
}

// --- TestLoadCRD ---

func TestLoadCRD(t *testing.T) {
	t.Run("valid resource", func(t *testing.T) {
		crd, err := loadCRD("wasmplugins.extensions.istio.io")
		require.NoError(t, err)
		assert.Equal(t, "wasmplugins.extensions.istio.io", crd.Name)
		assert.Equal(t, "extensions.istio.io", crd.Spec.Group)
		assert.Equal(t, "WasmPlugin", crd.Spec.Names.Kind)
	})

	t.Run("invalid resource name", func(t *testing.T) {
		_, err := loadCRD("invalid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid resource name")
	})

	t.Run("nonexistent resource", func(t *testing.T) {
		_, err := loadCRD("fake.nonexistent.istio.io")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read CRD file")
	})
}

// --- TestClassifyCRDs ---

func TestClassifyCRDs(t *testing.T) {
	t.Run("CIO with missing", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(crdScheme()).WithObjects(
			makeCRDStub("wasmplugins.extensions.istio.io", map[string]string{labelManagedByCIO: "true"}),
			makeCRDStub("envoyfilters.networking.istio.io", map[string]string{labelManagedByCIO: "true"}),
		).Build()
		mgr := newCRDManager(cl)

		state, infos := mgr.classifyCRDs(context.Background(), gatewayAPICRDNames)
		assert.Equal(t, CRDManagedByCIO, state)
		assert.Len(t, infos, 3)
		// The missing one should be Found=false
		for _, info := range infos {
			if info.Name == "destinationrules.networking.istio.io" {
				assert.False(t, info.Found)
			} else {
				assert.True(t, info.Found)
				assert.Equal(t, CRDManagedByCIO, info.State)
			}
		}
	})

	t.Run("mixed CIO and OLM", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(crdScheme()).WithObjects(
			makeCRDStub("wasmplugins.extensions.istio.io", map[string]string{labelManagedByCIO: "true"}),
			makeCRDStub("envoyfilters.networking.istio.io", map[string]string{labelOLMManaged: "true"}),
			makeCRDStub("destinationrules.networking.istio.io", nil),
		).Build()
		mgr := newCRDManager(cl)

		state, _ := mgr.classifyCRDs(context.Background(), gatewayAPICRDNames)
		assert.Equal(t, CRDMixedOwnership, state)
	})

	t.Run("empty targets", func(t *testing.T) {
		cl := fake.NewClientBuilder().WithScheme(crdScheme()).Build()
		mgr := newCRDManager(cl)

		state, infos := mgr.classifyCRDs(context.Background(), nil)
		assert.Equal(t, CRDNoneExist, state)
		assert.Nil(t, infos)
	})
}

// --- TestReconcile ---

func TestReconcile_NoneExist(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(crdScheme()).Build()
	mgr := newCRDManager(cl)

	result := mgr.Reconcile(context.Background(), makeGatewayAPIValues(), false)
	assert.Equal(t, CRDManagedByCIO, result.State)
	assert.NoError(t, result.Error)
	assert.Contains(t, result.Message, "installed by CIO")
	assert.Len(t, result.CRDs, 3)

	for _, info := range result.CRDs {
		assert.True(t, info.Found)
		assert.Equal(t, CRDManagedByCIO, info.State)
	}

	// Verify CRDs were actually created with CIO labels
	for _, name := range gatewayAPICRDNames {
		crd := getCRDFromClient(t, cl, name)
		assert.Equal(t, "true", crd.Labels[labelManagedByCIO], "CRD %s missing CIO label", name)
		assert.Equal(t, "keep", crd.Annotations[annotationHelmKeep], "CRD %s missing helm keep annotation", name)
	}
}

func TestReconcile_CIOOwned_Updates(t *testing.T) {
	// Pre-seed all 3 CRDs with CIO labels
	var objs []client.Object
	for _, name := range gatewayAPICRDNames {
		objs = append(objs, makeCRDStub(name, map[string]string{labelManagedByCIO: "true"}))
	}
	cl := fake.NewClientBuilder().WithScheme(crdScheme()).WithObjects(objs...).Build()
	mgr := newCRDManager(cl)

	result := mgr.Reconcile(context.Background(), makeGatewayAPIValues(), false)
	assert.Equal(t, CRDManagedByCIO, result.State)
	assert.NoError(t, result.Error)
	assert.Contains(t, result.Message, "updated by CIO")

	// Verify CRDs still have CIO labels
	for _, name := range gatewayAPICRDNames {
		crd := getCRDFromClient(t, cl, name)
		assert.Equal(t, "true", crd.Labels[labelManagedByCIO])
	}
}

func TestReconcile_CIOOwned_ReinstallsMissing(t *testing.T) {
	// Pre-seed 2 of 3 with CIO labels; destinationrules is missing
	cl := fake.NewClientBuilder().WithScheme(crdScheme()).WithObjects(
		makeCRDStub("wasmplugins.extensions.istio.io", map[string]string{labelManagedByCIO: "true"}),
		makeCRDStub("envoyfilters.networking.istio.io", map[string]string{labelManagedByCIO: "true"}),
	).Build()
	mgr := newCRDManager(cl)

	result := mgr.Reconcile(context.Background(), makeGatewayAPIValues(), false)
	assert.Equal(t, CRDManagedByCIO, result.State)
	assert.NoError(t, result.Error)
	assert.Contains(t, result.Message, "reinstalled")
	assert.Contains(t, result.Message, "destinationrules.networking.istio.io")

	// Verify the missing CRD was created
	crd := getCRDFromClient(t, cl, "destinationrules.networking.istio.io")
	assert.Equal(t, "true", crd.Labels[labelManagedByCIO])
}

func TestReconcile_CIOOwned_ReclaimsUnlabeled(t *testing.T) {
	// 2 CIO-labeled + 1 unlabeled
	cl := fake.NewClientBuilder().WithScheme(crdScheme()).WithObjects(
		makeCRDStub("wasmplugins.extensions.istio.io", map[string]string{labelManagedByCIO: "true"}),
		makeCRDStub("envoyfilters.networking.istio.io", map[string]string{labelManagedByCIO: "true"}),
		makeCRDStub("destinationrules.networking.istio.io", nil),
	).Build()
	mgr := newCRDManager(cl)

	result := mgr.Reconcile(context.Background(), makeGatewayAPIValues(), false)
	assert.Equal(t, CRDManagedByCIO, result.State)
	assert.NoError(t, result.Error)
	assert.Contains(t, result.Message, "reclaimed")
	assert.Contains(t, result.Message, "destinationrules.networking.istio.io")

	// Verify the previously-unlabeled CRD now has CIO label
	crd := getCRDFromClient(t, cl, "destinationrules.networking.istio.io")
	assert.Equal(t, "true", crd.Labels[labelManagedByCIO])
}

func TestReconcile_OLMOwned(t *testing.T) {
	var objs []client.Object
	for _, name := range gatewayAPICRDNames {
		objs = append(objs, makeCRDStub(name, map[string]string{labelOLMManaged: "true"}))
	}
	cl := fake.NewClientBuilder().WithScheme(crdScheme()).WithObjects(objs...).Build()
	mgr := newCRDManager(cl)

	result := mgr.Reconcile(context.Background(), makeGatewayAPIValues(), false)
	assert.Equal(t, CRDManagedByOLM, result.State)
	assert.NoError(t, result.Error)
	assert.Contains(t, result.Message, "OLM")
}

func TestReconcile_MixedOwnership(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(crdScheme()).WithObjects(
		makeCRDStub("wasmplugins.extensions.istio.io", map[string]string{labelManagedByCIO: "true"}),
		makeCRDStub("envoyfilters.networking.istio.io", map[string]string{labelOLMManaged: "true"}),
		makeCRDStub("destinationrules.networking.istio.io", nil),
	).Build()
	mgr := newCRDManager(cl)

	result := mgr.Reconcile(context.Background(), makeGatewayAPIValues(), false)
	assert.Equal(t, CRDMixedOwnership, result.State)
	assert.Error(t, result.Error)
	assert.Contains(t, result.Error.Error(), "mixed ownership")
}

func TestReconcile_NoTargets(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(crdScheme()).Build()
	mgr := newCRDManager(cl)

	result := mgr.Reconcile(context.Background(), nil, false)
	assert.Equal(t, CRDNoneExist, result.State)
	assert.NoError(t, result.Error)
	assert.Contains(t, result.Message, "no target CRDs configured")
}

// --- TestWatchTargets ---

func TestWatchTargets(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(crdScheme()).Build()
	mgr := newCRDManager(cl)

	t.Run("includeAllCRDs returns all istio.io CRDs", func(t *testing.T) {
		targets := mgr.WatchTargets(nil, true)
		assert.NotNil(t, targets)
		assert.Greater(t, len(targets), 0)
		// Should include istio.io CRDs
		_, hasWasm := targets["wasmplugins.extensions.istio.io"]
		assert.True(t, hasWasm, "should include wasmplugins")
		// Should not include sailoperator.io CRDs
		for name := range targets {
			assert.NotContains(t, name, "sailoperator.io", "should exclude sail operator CRDs")
		}
	})

	t.Run("filtered by PILOT_INCLUDE_RESOURCES", func(t *testing.T) {
		targets := mgr.WatchTargets(makeGatewayAPIValues(), false)
		assert.NotNil(t, targets)
		assert.Len(t, targets, 3)
		for _, name := range gatewayAPICRDNames {
			_, ok := targets[name]
			assert.True(t, ok, "expected %s in watch targets", name)
		}
	})

	t.Run("nil values returns nil", func(t *testing.T) {
		targets := mgr.WatchTargets(nil, false)
		assert.Nil(t, targets)
	})
}
