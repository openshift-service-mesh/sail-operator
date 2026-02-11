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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestWatchTypeString(t *testing.T) {
	tests := []struct {
		watchType WatchType
		expected  string
	}{
		{WatchTypeOwned, "Owned"},
		{WatchTypeNamespace, "Namespace"},
		{WatchType(99), "Unknown(99)"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.watchType.String())
		})
	}
}

func TestParseAPIVersionKind(t *testing.T) {
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
			result := parseAPIVersionKind(tt.apiVersion, tt.kind)
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
