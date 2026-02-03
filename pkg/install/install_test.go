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
	"testing/fstest"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/istioversion"
	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
)

func TestNewInstaller(t *testing.T) {
	testFS := fstest.MapFS{}
	testConfig := &rest.Config{}

	t.Run("missing kubeConfig", func(t *testing.T) {
		installer, err := NewInstaller(nil, testFS)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "kubeConfig is required")
		assert.Nil(t, installer)
	})

	t.Run("missing resourceFS", func(t *testing.T) {
		installer, err := NewInstaller(testConfig, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "resourceFS is required")
		assert.Nil(t, installer)
	})

	t.Run("valid inputs", func(t *testing.T) {
		installer, err := NewInstaller(testConfig, testFS)
		assert.NoError(t, err)
		assert.NotNil(t, installer)
	})
}

func TestOptionsApplyDefaults(t *testing.T) {
	tests := []struct {
		name               string
		opts               Options
		expectedNamespace  string
		expectedVersion    string
		expectedRevision   string
		expectedManageCRDs bool
	}{
		{
			name:               "all defaults",
			opts:               Options{},
			expectedNamespace:  "istio-system",
			expectedVersion:    istioversion.Default,
			expectedRevision:   "default",
			expectedManageCRDs: true,
		},
		{
			name: "custom namespace preserved",
			opts: Options{
				Namespace: "custom-ns",
			},
			expectedNamespace:  "custom-ns",
			expectedVersion:    istioversion.Default,
			expectedRevision:   "default",
			expectedManageCRDs: true,
		},
		{
			name: "custom version preserved",
			opts: Options{
				Version: "v1.24.0",
			},
			expectedNamespace:  "istio-system",
			expectedVersion:    "v1.24.0",
			expectedRevision:   "default",
			expectedManageCRDs: true,
		},
		{
			name: "custom revision preserved",
			opts: Options{
				Revision: "canary",
			},
			expectedNamespace:  "istio-system",
			expectedVersion:    istioversion.Default,
			expectedRevision:   "canary",
			expectedManageCRDs: true,
		},
		{
			name: "all custom values preserved",
			opts: Options{
				Namespace: "my-namespace",
				Version:   "v1.23.0",
				Revision:  "my-revision",
			},
			expectedNamespace:  "my-namespace",
			expectedVersion:    "v1.23.0",
			expectedRevision:   "my-revision",
			expectedManageCRDs: true,
		},
		{
			name: "ManageCRDs false preserved",
			opts: Options{
				ManageCRDs: ptr.To(false),
			},
			expectedNamespace:  "istio-system",
			expectedVersion:    istioversion.Default,
			expectedRevision:   "default",
			expectedManageCRDs: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.opts.applyDefaults()
			assert.Equal(t, tt.expectedNamespace, tt.opts.Namespace)
			assert.Equal(t, tt.expectedVersion, tt.opts.Version)
			assert.Equal(t, tt.expectedRevision, tt.opts.Revision)
			assert.Equal(t, tt.expectedManageCRDs, *tt.opts.ManageCRDs)
		})
	}
}

func TestPrepareValues(t *testing.T) {
	tests := []struct {
		name             string
		userValues       *v1.Values
		namespace        string
		revision         string
		expectedRevision string
	}{
		{
			name:             "nil values with default revision",
			userValues:       nil,
			namespace:        "istio-system",
			revision:         "default",
			expectedRevision: "", // default revision maps to empty string
		},
		{
			name:             "empty values with custom revision",
			userValues:       &v1.Values{},
			namespace:        "custom-ns",
			revision:         "canary",
			expectedRevision: "canary",
		},
		{
			name: "values with existing global",
			userValues: &v1.Values{
				Global: &v1.GlobalConfig{
					Hub: ptr.To("custom-hub"),
				},
			},
			namespace:        "istio-system",
			revision:         "default",
			expectedRevision: "",
		},
		{
			name: "values with pilot config and non-default revision",
			userValues: &v1.Values{
				Pilot: &v1.PilotConfig{
					Enabled: ptr.To(true),
				},
			},
			namespace:        "test-ns",
			revision:         "stable",
			expectedRevision: "stable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := prepareValues(tt.userValues, tt.namespace, tt.revision)

			// Check required fields are set
			assert.NotNil(t, result)
			assert.NotNil(t, result.Global)
			assert.NotNil(t, result.Global.IstioNamespace)
			assert.Equal(t, tt.namespace, *result.Global.IstioNamespace)
			assert.NotNil(t, result.Revision)
			assert.Equal(t, tt.expectedRevision, *result.Revision)

			// Check that user values are preserved
			if tt.userValues != nil && tt.userValues.Global != nil && tt.userValues.Global.Hub != nil {
				assert.Equal(t, *tt.userValues.Global.Hub, *result.Global.Hub)
			}
			if tt.userValues != nil && tt.userValues.Pilot != nil {
				assert.NotNil(t, result.Pilot)
			}
		})
	}
}

func TestPrepareValuesDoesNotMutateOriginal(t *testing.T) {
	original := &v1.Values{
		Global: &v1.GlobalConfig{
			Hub: ptr.To("original-hub"),
		},
	}

	result := prepareValues(original, "test-ns", "default")

	// Original should not be modified
	assert.Nil(t, original.Revision)
	assert.Nil(t, original.Global.IstioNamespace)

	// Result should have the new values
	assert.NotNil(t, result.Revision)
	assert.NotNil(t, result.Global.IstioNamespace)
}
