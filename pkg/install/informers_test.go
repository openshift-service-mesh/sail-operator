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
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
)

// enqueueCounter returns an enqueue func and a counter to check how many times it was called.
func enqueueCounter() (func(), *atomic.Int32) {
	var count atomic.Int32
	return func() { count.Add(1) }, &count
}

// makeObj creates an unstructured object with the given name and labels.
func makeObj(name string, labels map[string]string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetName(name)
	if labels != nil {
		obj.SetLabels(labels)
	}
	return obj
}

// --- makeOwnedEventHandler tests ---

func TestOwnedHandler_AddIsNoOp(t *testing.T) {
	enqueue, count := enqueueCounter()
	handler := makeOwnedEventHandler(
		schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
		watchTypeOwned,
		"default",
		defaultManagedByValue,
		enqueue,
	)

	handler.(cache.ResourceEventHandlerFuncs).AddFunc(makeObj("test", map[string]string{"istio.io/rev": "default"}))
	assert.Equal(t, int32(0), count.Load(), "Add events should be ignored (initial cache sync)")
}

func TestOwnedHandler_UpdateOwnedEnqueues(t *testing.T) {
	enqueue, count := enqueueCounter()
	gvk := schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
	handler := makeOwnedEventHandler(gvk, watchTypeOwned, "default", defaultManagedByValue, enqueue)

	oldObj := makeObj("test", map[string]string{"istio.io/rev": "default"})
	oldObj.SetGeneration(1)
	newObj := makeObj("test", map[string]string{"istio.io/rev": "default"})
	newObj.SetGeneration(2)

	handler.(cache.ResourceEventHandlerFuncs).UpdateFunc(oldObj, newObj)
	assert.Equal(t, int32(1), count.Load(), "Owned resource update should enqueue")
}

func TestOwnedHandler_UpdateNotOwnedSkips(t *testing.T) {
	enqueue, count := enqueueCounter()
	gvk := schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
	handler := makeOwnedEventHandler(gvk, watchTypeOwned, "default", defaultManagedByValue, enqueue)

	oldObj := makeObj("test", map[string]string{"istio.io/rev": "other-revision"})
	oldObj.SetGeneration(1)
	newObj := makeObj("test", map[string]string{"istio.io/rev": "other-revision"})
	newObj.SetGeneration(2)

	handler.(cache.ResourceEventHandlerFuncs).UpdateFunc(oldObj, newObj)
	assert.Equal(t, int32(0), count.Load(), "Non-owned resource update should be skipped")
}

func TestOwnedHandler_UpdateIgnoreAnnotationSkips(t *testing.T) {
	enqueue, count := enqueueCounter()
	gvk := schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
	handler := makeOwnedEventHandler(gvk, watchTypeOwned, "default", defaultManagedByValue, enqueue)

	oldObj := makeObj("test", map[string]string{"istio.io/rev": "default"})
	oldObj.SetGeneration(1)
	newObj := makeObj("test", map[string]string{"istio.io/rev": "default"})
	newObj.SetGeneration(2)
	newObj.SetAnnotations(map[string]string{ignoreAnnotation: "true"})

	handler.(cache.ResourceEventHandlerFuncs).UpdateFunc(oldObj, newObj)
	assert.Equal(t, int32(0), count.Load(), "Resource with ignore annotation should be skipped")
}

func TestOwnedHandler_UpdateNamespaceTypeSkipsOwnerCheck(t *testing.T) {
	enqueue, count := enqueueCounter()
	gvk := schema.GroupVersionKind{Version: "v1", Kind: "Namespace"}
	// watchTypeNamespace does not check isOwnedResource
	handler := makeOwnedEventHandler(gvk, watchTypeNamespace, "default", defaultManagedByValue, enqueue)

	oldObj := makeObj("istio-system", nil) // no ownership labels
	oldObj.SetGeneration(1)
	newObj := makeObj("istio-system", nil)
	newObj.SetGeneration(2)

	handler.(cache.ResourceEventHandlerFuncs).UpdateFunc(oldObj, newObj)
	assert.Equal(t, int32(1), count.Load(), "Namespace watch should not filter by ownership")
}

func TestOwnedHandler_DeleteOwnedEnqueues(t *testing.T) {
	enqueue, count := enqueueCounter()
	gvk := schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
	handler := makeOwnedEventHandler(gvk, watchTypeOwned, "default", defaultManagedByValue, enqueue)

	obj := makeObj("test", map[string]string{"istio.io/rev": "default"})
	handler.(cache.ResourceEventHandlerFuncs).DeleteFunc(obj)
	assert.Equal(t, int32(1), count.Load(), "Owned resource delete should enqueue")
}

func TestOwnedHandler_DeleteNotOwnedSkips(t *testing.T) {
	enqueue, count := enqueueCounter()
	gvk := schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
	handler := makeOwnedEventHandler(gvk, watchTypeOwned, "default", defaultManagedByValue, enqueue)

	obj := makeObj("test", map[string]string{"istio.io/rev": "other"})
	handler.(cache.ResourceEventHandlerFuncs).DeleteFunc(obj)
	assert.Equal(t, int32(0), count.Load(), "Non-owned resource delete should be skipped")
}

func TestOwnedHandler_DeleteTombstoneEnqueues(t *testing.T) {
	enqueue, count := enqueueCounter()
	gvk := schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}
	handler := makeOwnedEventHandler(gvk, watchTypeOwned, "default", defaultManagedByValue, enqueue)

	obj := makeObj("test", map[string]string{"istio.io/rev": "default"})
	tombstone := cache.DeletedFinalStateUnknown{Key: "test", Obj: obj}

	handler.(cache.ResourceEventHandlerFuncs).DeleteFunc(tombstone)
	assert.Equal(t, int32(1), count.Load(), "Tombstone delete of owned resource should enqueue")
}

// --- makeCRDEventHandler tests ---

func TestCRDHandler_AddTargetEnqueues(t *testing.T) {
	enqueue, count := enqueueCounter()
	targets := map[string]struct{}{"wasmplugins.extensions.istio.io": {}}
	handler := makeCRDEventHandler(targets, enqueue)

	obj := makeObj("wasmplugins.extensions.istio.io", nil)
	handler.(cache.ResourceEventHandlerFuncs).AddFunc(obj)
	assert.Equal(t, int32(1), count.Load(), "Target CRD add should enqueue")
}

func TestCRDHandler_AddNonTargetSkips(t *testing.T) {
	enqueue, count := enqueueCounter()
	targets := map[string]struct{}{"wasmplugins.extensions.istio.io": {}}
	handler := makeCRDEventHandler(targets, enqueue)

	obj := makeObj("gateways.gateway.networking.k8s.io", nil)
	handler.(cache.ResourceEventHandlerFuncs).AddFunc(obj)
	assert.Equal(t, int32(0), count.Load(), "Non-target CRD add should be skipped")
}

func TestCRDHandler_UpdateTargetWithLabelChangeEnqueues(t *testing.T) {
	enqueue, count := enqueueCounter()
	targets := map[string]struct{}{"wasmplugins.extensions.istio.io": {}}
	handler := makeCRDEventHandler(targets, enqueue)

	oldObj := makeObj("wasmplugins.extensions.istio.io", nil)
	oldObj.SetGeneration(1)
	newObj := makeObj("wasmplugins.extensions.istio.io", map[string]string{"ingress.operator.openshift.io/owned": "true"})
	newObj.SetGeneration(1)

	handler.(cache.ResourceEventHandlerFuncs).UpdateFunc(oldObj, newObj)
	assert.Equal(t, int32(1), count.Load(), "Target CRD label change should enqueue")
}

func TestCRDHandler_UpdateTargetNoChangeSkips(t *testing.T) {
	enqueue, count := enqueueCounter()
	targets := map[string]struct{}{"wasmplugins.extensions.istio.io": {}}
	handler := makeCRDEventHandler(targets, enqueue)

	oldObj := makeObj("wasmplugins.extensions.istio.io", map[string]string{"foo": "bar"})
	oldObj.SetGeneration(1)
	oldObj.SetResourceVersion("100")
	newObj := makeObj("wasmplugins.extensions.istio.io", map[string]string{"foo": "bar"})
	newObj.SetGeneration(1)
	newObj.SetResourceVersion("101") // only RV changed

	handler.(cache.ResourceEventHandlerFuncs).UpdateFunc(oldObj, newObj)
	assert.Equal(t, int32(0), count.Load(), "Target CRD with no meaningful change should be skipped")
}

func TestCRDHandler_UpdateNonTargetSkips(t *testing.T) {
	enqueue, count := enqueueCounter()
	targets := map[string]struct{}{"wasmplugins.extensions.istio.io": {}}
	handler := makeCRDEventHandler(targets, enqueue)

	oldObj := makeObj("gateways.gateway.networking.k8s.io", nil)
	oldObj.SetGeneration(1)
	newObj := makeObj("gateways.gateway.networking.k8s.io", map[string]string{"new": "label"})
	newObj.SetGeneration(1)

	handler.(cache.ResourceEventHandlerFuncs).UpdateFunc(oldObj, newObj)
	assert.Equal(t, int32(0), count.Load(), "Non-target CRD update should be skipped")
}

func TestCRDHandler_DeleteTargetEnqueues(t *testing.T) {
	enqueue, count := enqueueCounter()
	targets := map[string]struct{}{"wasmplugins.extensions.istio.io": {}}
	handler := makeCRDEventHandler(targets, enqueue)

	obj := makeObj("wasmplugins.extensions.istio.io", nil)
	handler.(cache.ResourceEventHandlerFuncs).DeleteFunc(obj)
	assert.Equal(t, int32(1), count.Load(), "Target CRD delete should enqueue")
}

func TestCRDHandler_DeleteNonTargetSkips(t *testing.T) {
	enqueue, count := enqueueCounter()
	targets := map[string]struct{}{"wasmplugins.extensions.istio.io": {}}
	handler := makeCRDEventHandler(targets, enqueue)

	obj := makeObj("gateways.gateway.networking.k8s.io", nil)
	handler.(cache.ResourceEventHandlerFuncs).DeleteFunc(obj)
	assert.Equal(t, int32(0), count.Load(), "Non-target CRD delete should be skipped")
}

func TestCRDHandler_DeleteTombstoneTargetEnqueues(t *testing.T) {
	enqueue, count := enqueueCounter()
	targets := map[string]struct{}{"wasmplugins.extensions.istio.io": {}}
	handler := makeCRDEventHandler(targets, enqueue)

	obj := makeObj("wasmplugins.extensions.istio.io", nil)
	tombstone := cache.DeletedFinalStateUnknown{Key: "wasmplugins.extensions.istio.io", Obj: obj}

	handler.(cache.ResourceEventHandlerFuncs).DeleteFunc(tombstone)
	assert.Equal(t, int32(1), count.Load(), "Tombstone delete of target CRD should enqueue")
}
