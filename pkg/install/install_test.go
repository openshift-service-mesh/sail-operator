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

// Note: Value computation tests are in pkg/revision and pkg/istiovalues packages.
// The Install() method uses revision.ComputeValues() which is tested there.
