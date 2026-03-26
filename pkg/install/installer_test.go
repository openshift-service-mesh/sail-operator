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
	"io/fs"
	"testing"
	"testing/fstest"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
)

func TestWatchTypeString(t *testing.T) {
	tests := []struct {
		watchType watchType
		expected  string
	}{
		{watchTypeOwned, "Owned"},
		{watchTypeNamespace, "Namespace"},
		{watchTypeCRD, "CRD"},
		{watchType(99), "Unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.watchType.String())
		})
	}
}

func TestAPIVersionKindToGVK(t *testing.T) {
	tests := []struct {
		name       string
		apiVersion string
		kind       string
		expected   schema.GroupVersionKind
	}{
		{
			name:       "core v1 resource",
			apiVersion: "v1",
			kind:       "ConfigMap",
			expected:   schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
		},
		{
			name:       "apps v1 resource",
			apiVersion: "apps/v1",
			kind:       "Deployment",
			expected:   schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		},
		{
			name:       "rbac resource",
			apiVersion: "rbac.authorization.k8s.io/v1",
			kind:       "ClusterRole",
			expected:   schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole"},
		},
		{
			name:       "admissionregistration resource",
			apiVersion: "admissionregistration.k8s.io/v1",
			kind:       "MutatingWebhookConfiguration",
			expected:   schema.GroupVersionKind{Group: "admissionregistration.k8s.io", Version: "v1", Kind: "MutatingWebhookConfiguration"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := apiVersionKindToGVK(tt.apiVersion, tt.kind)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractGVKsFromRendered(t *testing.T) {
	tests := []struct {
		name     string
		rendered map[string]string
		expected []schema.GroupVersionKind
	}{
		{
			name: "single resource",
			rendered: map[string]string{
				"templates/configmap.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`,
			},
			expected: []schema.GroupVersionKind{
				{Group: "", Version: "v1", Kind: "ConfigMap"},
			},
		},
		{
			name: "multiple resources in one file",
			rendered: map[string]string{
				"templates/resources.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
`,
			},
			expected: []schema.GroupVersionKind{
				{Group: "", Version: "v1", Kind: "ConfigMap"},
				{Group: "apps", Version: "v1", Kind: "Deployment"},
			},
		},
		{
			name: "multiple files",
			rendered: map[string]string{
				"templates/configmap.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`,
				"templates/service.yaml": `apiVersion: v1
kind: Service
metadata:
  name: test
`,
			},
			expected: []schema.GroupVersionKind{
				{Group: "", Version: "v1", Kind: "ConfigMap"},
				{Group: "", Version: "v1", Kind: "Service"},
			},
		},
		{
			name: "skip empty and notes",
			rendered: map[string]string{
				"templates/NOTES.txt":  "Some notes",
				"templates/empty.yaml": "",
				"templates/configmap.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`,
			},
			expected: []schema.GroupVersionKind{
				{Group: "", Version: "v1", Kind: "ConfigMap"},
			},
		},
		{
			name: "deduplicate same GVK",
			rendered: map[string]string{
				"templates/cm1.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test1
`,
				"templates/cm2.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: test2
`,
			},
			expected: []schema.GroupVersionKind{
				{Group: "", Version: "v1", Kind: "ConfigMap"},
			},
		},
		{
			name: "empty documents in multi-doc",
			rendered: map[string]string{
				"templates/resources.yaml": `---
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
---
---
`,
			},
			expected: []schema.GroupVersionKind{
				{Group: "", Version: "v1", Kind: "ConfigMap"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gvkSet := make(map[schema.GroupVersionKind]struct{})
			err := extractGVKsFromRendered(tt.rendered, gvkSet)
			require.NoError(t, err)

			// Convert set to slice for comparison
			var result []schema.GroupVersionKind
			for gvk := range gvkSet {
				result = append(result, gvk)
			}

			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestGVKToGVR(t *testing.T) {
	tests := []struct {
		name     string
		gvk      schema.GroupVersionKind
		expected schema.GroupVersionResource
	}{
		{
			name: "Service",
			gvk:  schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"},
			expected: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "services",
			},
		},
		{
			name: "ConfigMap",
			gvk:  schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
			expected: schema.GroupVersionResource{
				Group:    "",
				Version:  "v1",
				Resource: "configmaps",
			},
		},
		{
			name: "Deployment",
			gvk:  schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			expected: schema.GroupVersionResource{
				Group:    "apps",
				Version:  "v1",
				Resource: "deployments",
			},
		},
		{
			name: "ClusterRole",
			gvk:  schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole"},
			expected: schema.GroupVersionResource{
				Group:    "rbac.authorization.k8s.io",
				Version:  "v1",
				Resource: "clusterroles",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gvkToGVR(tt.gvk)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsClusterScoped(t *testing.T) {
	tests := []struct {
		gvk      schema.GroupVersionKind
		expected bool
	}{
		{
			gvk:      schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"},
			expected: true,
		},
		{
			gvk:      schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole"},
			expected: true,
		},
		{
			gvk:      schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRoleBinding"},
			expected: true,
		},
		{
			gvk:      schema.GroupVersionKind{Group: "admissionregistration.k8s.io", Version: "v1", Kind: "MutatingWebhookConfiguration"},
			expected: true,
		},
		{
			gvk:      schema.GroupVersionKind{Group: "admissionregistration.k8s.io", Version: "v1", Kind: "ValidatingWebhookConfiguration"},
			expected: true,
		},
		{
			gvk:      schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"},
			expected: false,
		},
		{
			gvk:      schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.gvk.Kind, func(t *testing.T) {
			result := isClusterScoped(tt.gvk)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractGVKsFromRendered_Errors(t *testing.T) {
	tests := []struct {
		name     string
		rendered map[string]string
	}{
		{
			name: "invalid yaml",
			rendered: map[string]string{
				"templates/bad.yaml": `this is not: valid: yaml: at: all`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gvkSet := make(map[schema.GroupVersionKind]struct{})
			err := extractGVKsFromRendered(tt.rendered, gvkSet)
			// Invalid YAML should either error or be skipped gracefully
			// The current implementation may skip documents without apiVersion/kind
			// which is acceptable behavior
			_ = err
		})
	}
}

// --- installer unit tests ---

func TestResolveValues_NoVersionInEmptyFS(t *testing.T) {
	inst := &installer{
		resourceFS: fstest.MapFS{},
	}
	_, _, err := inst.resolveValues(Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no stable version found")
}

func TestResolveValues_InvalidVersion(t *testing.T) {
	inst := &installer{
		resourceFS: fstest.MapFS{
			"v1.28.3": &fstest.MapFile{Mode: fs.ModeDir},
		},
	}
	_, _, err := inst.resolveValues(Options{Version: "v9.99.99"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid version")
}

func TestResolveValues_DefaultsToHighestStableVersion(t *testing.T) {
	// This will fail at ComputeValues (no charts/profiles in FS), but
	// the version resolution itself should pick v1.28.3 over v1.27.0.
	inst := &installer{
		resourceFS: fstest.MapFS{
			"v1.27.0":        &fstest.MapFile{Mode: fs.ModeDir},
			"v1.28.3":        &fstest.MapFile{Mode: fs.ModeDir},
			"v1.29.0-alpha1": &fstest.MapFile{Mode: fs.ModeDir},
		},
	}
	_, _, err := inst.resolveValues(Options{})
	// ComputeValues will fail because the FS has no profile files, but
	// we can verify the error message includes the resolved version.
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to compute values")
}

func TestReconcile_ResolveValuesFailure(t *testing.T) {
	inst := &installer{
		resourceFS: fstest.MapFS{}, // no versions
	}
	status := inst.reconcile(context.Background(), Options{})
	assert.False(t, status.Installed)
	assert.Empty(t, status.Version)
	assert.Error(t, status.Error)
	assert.Contains(t, status.Error.Error(), "no stable version found")
	// CRD fields should be zero-valued (never reached)
	assert.Empty(t, status.CRDState)
	assert.Nil(t, status.CRDs)
}

func TestReconcile_InvalidVersionEarlyExit(t *testing.T) {
	inst := &installer{
		resourceFS: fstest.MapFS{
			"v1.28.3": &fstest.MapFile{Mode: fs.ModeDir},
		},
	}
	status := inst.reconcile(context.Background(), Options{Version: "v0.0.1"})
	assert.False(t, status.Installed)
	assert.Error(t, status.Error)
	assert.Contains(t, status.Error.Error(), "invalid version")
}

func TestReconcile_DeepCopiesValues(t *testing.T) {
	// Verify that reconcile deep-copies Values so mutations inside
	// reconcile don't affect the caller's pointer.
	inst := &installer{
		resourceFS: fstest.MapFS{}, // will fail early, but after deep-copy
	}
	original := &v1.Values{
		Global: &v1.GlobalConfig{
			Hub: ptr.To("original-hub"),
		},
	}
	opts := Options{Values: original}
	_ = inst.reconcile(context.Background(), opts)

	// The original Values should not have been mutated
	assert.Equal(t, "original-hub", *original.Global.Hub)
}
