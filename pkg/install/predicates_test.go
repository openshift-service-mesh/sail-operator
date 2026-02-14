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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestShouldReconcileOnCreate(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetName("test")

	// Create events should always trigger reconciliation
	assert.True(t, shouldReconcileOnCreate(obj))
}

func TestShouldReconcileOnDelete(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetName("test")

	// Delete events should always trigger reconciliation
	assert.True(t, shouldReconcileOnDelete(obj))
}

func TestHasIgnoreAnnotation(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		expected    bool
	}{
		{
			name:        "no annotations",
			annotations: nil,
			expected:    false,
		},
		{
			name:        "empty annotations",
			annotations: map[string]string{},
			expected:    false,
		},
		{
			name:        "ignore annotation set to true",
			annotations: map[string]string{ignoreAnnotation: "true"},
			expected:    true,
		},
		{
			name:        "ignore annotation set to false",
			annotations: map[string]string{ignoreAnnotation: "false"},
			expected:    false,
		},
		{
			name:        "ignore annotation set to other value",
			annotations: map[string]string{ignoreAnnotation: "yes"},
			expected:    false,
		},
		{
			name:        "other annotations present",
			annotations: map[string]string{"other": "value"},
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{}
			obj.SetAnnotations(tt.annotations)

			result := hasIgnoreAnnotation(obj)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShouldReconcileOnUpdate_IgnoreAnnotation(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}

	oldObj := &unstructured.Unstructured{}
	oldObj.SetName("test")

	newObj := &unstructured.Unstructured{}
	newObj.SetName("test")
	newObj.SetAnnotations(map[string]string{ignoreAnnotation: "true"})

	// Should not reconcile when ignore annotation is set
	assert.False(t, shouldReconcileOnUpdate(gvk, oldObj, newObj))
}

func TestShouldReconcileOnUpdate_ServiceAccount(t *testing.T) {
	gvk := serviceAccountGVK

	oldObj := &unstructured.Unstructured{}
	oldObj.SetName("test")
	oldObj.SetGeneration(1)

	newObj := &unstructured.Unstructured{}
	newObj.SetName("test")
	newObj.SetGeneration(2)

	// ServiceAccount updates should always be ignored
	assert.False(t, shouldReconcileOnUpdate(gvk, oldObj, newObj))
}

func TestIsStatusOnlyChange(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Service"}

	tests := []struct {
		name         string
		setupOld     func(*unstructured.Unstructured)
		setupNew     func(*unstructured.Unstructured)
		isStatusOnly bool
	}{
		{
			name: "same objects - status only",
			setupOld: func(obj *unstructured.Unstructured) {
				obj.SetGeneration(1)
			},
			setupNew: func(obj *unstructured.Unstructured) {
				obj.SetGeneration(1)
			},
			isStatusOnly: true,
		},
		{
			name: "generation changed - not status only",
			setupOld: func(obj *unstructured.Unstructured) {
				obj.SetGeneration(1)
			},
			setupNew: func(obj *unstructured.Unstructured) {
				obj.SetGeneration(2)
			},
			isStatusOnly: false,
		},
		{
			name: "labels changed - not status only",
			setupOld: func(obj *unstructured.Unstructured) {
				obj.SetGeneration(1)
			},
			setupNew: func(obj *unstructured.Unstructured) {
				obj.SetGeneration(1)
				obj.SetLabels(map[string]string{"new": "label"})
			},
			isStatusOnly: false,
		},
		{
			name: "annotations changed - not status only",
			setupOld: func(obj *unstructured.Unstructured) {
				obj.SetGeneration(1)
			},
			setupNew: func(obj *unstructured.Unstructured) {
				obj.SetGeneration(1)
				obj.SetAnnotations(map[string]string{"new": "annotation"})
			},
			isStatusOnly: false,
		},
		{
			name: "finalizers changed - not status only",
			setupOld: func(obj *unstructured.Unstructured) {
				obj.SetGeneration(1)
			},
			setupNew: func(obj *unstructured.Unstructured) {
				obj.SetGeneration(1)
				obj.SetFinalizers([]string{"new-finalizer"})
			},
			isStatusOnly: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldObj := &unstructured.Unstructured{}
			newObj := &unstructured.Unstructured{}

			tt.setupOld(oldObj)
			tt.setupNew(newObj)

			result := isStatusOnlyChange(gvk, oldObj, newObj)
			assert.Equal(t, tt.isStatusOnly, result)
		})
	}
}

func TestSpecWasUpdated_HPA(t *testing.T) {
	gvk := hpaGVK

	tests := []struct {
		name     string
		oldSpec  map[string]interface{}
		newSpec  map[string]interface{}
		expected bool
	}{
		{
			name:     "same spec",
			oldSpec:  map[string]interface{}{"minReplicas": int64(1)},
			newSpec:  map[string]interface{}{"minReplicas": int64(1)},
			expected: false,
		},
		{
			name:     "different spec",
			oldSpec:  map[string]interface{}{"minReplicas": int64(1)},
			newSpec:  map[string]interface{}{"minReplicas": int64(2)},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldObj := &unstructured.Unstructured{Object: map[string]interface{}{}}
			newObj := &unstructured.Unstructured{Object: map[string]interface{}{}}

			_ = unstructured.SetNestedMap(oldObj.Object, tt.oldSpec, "spec")
			_ = unstructured.SetNestedMap(newObj.Object, tt.newSpec, "spec")

			result := specWasUpdated(gvk, oldObj, newObj)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShouldFilterStatusChanges(t *testing.T) {
	tests := []struct {
		gvk      schema.GroupVersionKind
		expected bool
	}{
		{serviceGVK, true},
		{networkPolicyGVK, true},
		{pdbGVK, true},
		{hpaGVK, true},
		{namespaceGVK, true},
		{serviceAccountGVK, false},
		{validatingWebhookConfigurationGVK, false},
		{schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.gvk.Kind, func(t *testing.T) {
			result := shouldFilterStatusChanges(tt.gvk)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShouldReconcileCRDOnUpdate(t *testing.T) {
	tests := []struct {
		name     string
		setupOld func(*unstructured.Unstructured)
		setupNew func(*unstructured.Unstructured)
		expected bool
	}{
		{
			name: "no changes",
			setupOld: func(obj *unstructured.Unstructured) {
				obj.SetGeneration(1)
				obj.SetLabels(map[string]string{"foo": "bar"})
			},
			setupNew: func(obj *unstructured.Unstructured) {
				obj.SetGeneration(1)
				obj.SetLabels(map[string]string{"foo": "bar"})
			},
			expected: false,
		},
		{
			name: "label added",
			setupOld: func(obj *unstructured.Unstructured) {
				obj.SetGeneration(1)
			},
			setupNew: func(obj *unstructured.Unstructured) {
				obj.SetGeneration(1)
				obj.SetLabels(map[string]string{"ingress.operator.openshift.io/owned": "true"})
			},
			expected: true,
		},
		{
			name: "label removed",
			setupOld: func(obj *unstructured.Unstructured) {
				obj.SetGeneration(1)
				obj.SetLabels(map[string]string{"ingress.operator.openshift.io/owned": "true"})
			},
			setupNew: func(obj *unstructured.Unstructured) {
				obj.SetGeneration(1)
			},
			expected: true,
		},
		{
			name: "annotation changed",
			setupOld: func(obj *unstructured.Unstructured) {
				obj.SetGeneration(1)
			},
			setupNew: func(obj *unstructured.Unstructured) {
				obj.SetGeneration(1)
				obj.SetAnnotations(map[string]string{"helm.sh/resource-policy": "keep"})
			},
			expected: true,
		},
		{
			name: "generation changed (spec update)",
			setupOld: func(obj *unstructured.Unstructured) {
				obj.SetGeneration(1)
			},
			setupNew: func(obj *unstructured.Unstructured) {
				obj.SetGeneration(2)
			},
			expected: true,
		},
		{
			name: "only resourceVersion changed (status-like)",
			setupOld: func(obj *unstructured.Unstructured) {
				obj.SetGeneration(1)
				obj.SetResourceVersion("100")
			},
			setupNew: func(obj *unstructured.Unstructured) {
				obj.SetGeneration(1)
				obj.SetResourceVersion("101")
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldObj := &unstructured.Unstructured{Object: map[string]interface{}{}}
			newObj := &unstructured.Unstructured{Object: map[string]interface{}{}}
			tt.setupOld(oldObj)
			tt.setupNew(newObj)
			assert.Equal(t, tt.expected, shouldReconcileCRDOnUpdate(oldObj, newObj))
		})
	}
}

func TestIsTargetCRD(t *testing.T) {
	targets := map[string]struct{}{
		"wasmplugins.extensions.istio.io":      {},
		"destinationrules.networking.istio.io": {},
		"envoyfilters.networking.istio.io":     {},
	}

	tests := []struct {
		name     string
		crdName  string
		targets  map[string]struct{}
		expected bool
	}{
		{
			name:     "matching target",
			crdName:  "wasmplugins.extensions.istio.io",
			targets:  targets,
			expected: true,
		},
		{
			name:     "not a target",
			crdName:  "gateways.gateway.networking.k8s.io",
			targets:  targets,
			expected: false,
		},
		{
			name:     "empty targets",
			crdName:  "wasmplugins.extensions.istio.io",
			targets:  nil,
			expected: false,
		},
		{
			name:     "empty name",
			crdName:  "",
			targets:  targets,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obj := &unstructured.Unstructured{}
			obj.SetName(tt.crdName)
			assert.Equal(t, tt.expected, isTargetCRD(obj, tt.targets))
		})
	}
}

func TestShouldReconcileValidatingWebhook(t *testing.T) {
	tests := []struct {
		name     string
		objName  string
		oldObj   func() *unstructured.Unstructured
		newObj   func() *unstructured.Unstructured
		expected bool
	}{
		{
			name:    "non-istiod webhook - always reconcile",
			objName: "some-other-webhook",
			oldObj: func() *unstructured.Unstructured {
				obj := &unstructured.Unstructured{}
				obj.SetGeneration(1)
				return obj
			},
			newObj: func() *unstructured.Unstructured {
				obj := &unstructured.Unstructured{}
				obj.SetGeneration(2)
				return obj
			},
			expected: true,
		},
		{
			name:    "istiod validator - same content",
			objName: "istiod-istio-system-validator",
			oldObj: func() *unstructured.Unstructured {
				obj := &unstructured.Unstructured{}
				obj.SetName("istiod-istio-system-validator")
				obj.SetResourceVersion("123")
				return obj
			},
			newObj: func() *unstructured.Unstructured {
				obj := &unstructured.Unstructured{}
				obj.SetName("istiod-istio-system-validator")
				obj.SetResourceVersion("456") // Different resource version
				return obj
			},
			expected: false, // Resource version is cleared, so they're equal
		},
		{
			name:    "istio-validator - same content",
			objName: "istio-validator-istio-system",
			oldObj: func() *unstructured.Unstructured {
				obj := &unstructured.Unstructured{}
				obj.SetName("istio-validator-istio-system")
				return obj
			},
			newObj: func() *unstructured.Unstructured {
				obj := &unstructured.Unstructured{}
				obj.SetName("istio-validator-istio-system")
				return obj
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oldObj := tt.oldObj()
			oldObj.SetName(tt.objName)
			newObj := tt.newObj()
			newObj.SetName(tt.objName)

			result := shouldReconcileValidatingWebhook(oldObj, newObj)
			assert.Equal(t, tt.expected, result)
		})
	}
}
