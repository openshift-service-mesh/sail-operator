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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func TestGVKToGVR(t *testing.T) {
	// Tests verify that gvkToGVR correctly uses meta.UnsafeGuessKindToResource
	// for proper K8s resource pluralization
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
			name: "NetworkPolicy",
			gvk:  schema.GroupVersionKind{Group: "networking.k8s.io", Version: "v1", Kind: "NetworkPolicy"},
			expected: schema.GroupVersionResource{
				Group:    "networking.k8s.io",
				Version:  "v1",
				Resource: "networkpolicies",
			},
		},
		{
			name: "Ingress",
			gvk:  schema.GroupVersionKind{Group: "networking.k8s.io", Version: "v1", Kind: "Ingress"},
			expected: schema.GroupVersionResource{
				Group:    "networking.k8s.io",
				Version:  "v1",
				Resource: "ingresses",
			},
		},
		{
			name: "EndpointSlice",
			gvk:  schema.GroupVersionKind{Group: "discovery.k8s.io", Version: "v1", Kind: "EndpointSlice"},
			expected: schema.GroupVersionResource{
				Group:    "discovery.k8s.io",
				Version:  "v1",
				Resource: "endpointslices",
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
			d := &DriftReconciler{
				opts: Options{Revision: tt.revision},
			}

			obj := &unstructured.Unstructured{}
			obj.SetLabels(tt.labels)

			result := d.isOwnedResource(obj)
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
				// No UID - should match any UID
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
			d := &DriftReconciler{
				opts: Options{
					Revision: "default",
					OwnerRef: tt.ownerRef,
				},
			}

			obj := &unstructured.Unstructured{}
			obj.SetOwnerReferences(tt.ownerRefs)
			obj.SetLabels(tt.labels)

			result := d.isOwnedResource(obj)
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
			gvk:      schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"},
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
