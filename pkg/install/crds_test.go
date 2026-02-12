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

	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
					Labels: map[string]string{LabelManagedByCIO: "true"},
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
					Labels: map[string]string{LabelOLMManaged: "true"},
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
			mgr := NewCRDManager(cl)

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
