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

	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetImageDefaults(t *testing.T) {
	registry := "registry.example.com/istio"
	images := ImageNames{
		Istiod:  "pilot-rhel9",
		Proxy:   "proxyv2-rhel9",
		CNI:     "cni-rhel9",
		ZTunnel: "ztunnel-rhel9",
	}

	t.Run("populates from FS with multiple version dirs", func(t *testing.T) {
		config.Config = config.OperatorConfig{}
		fs := fstest.MapFS{
			"v1.27.1/charts/istiod/Chart.yaml": &fstest.MapFile{},
			"v1.28.0/charts/istiod/Chart.yaml": &fstest.MapFile{},
		}

		err := SetImageDefaults(fs, registry, images)
		require.NoError(t, err)

		assert.Len(t, config.Config.ImageDigests, 2)

		v1271 := config.Config.ImageDigests["v1.27.1"]
		assert.Equal(t, "registry.example.com/istio/pilot-rhel9:1.27.1", v1271.IstiodImage)
		assert.Equal(t, "registry.example.com/istio/proxyv2-rhel9:1.27.1", v1271.ProxyImage)
		assert.Equal(t, "registry.example.com/istio/cni-rhel9:1.27.1", v1271.CNIImage)
		assert.Equal(t, "registry.example.com/istio/ztunnel-rhel9:1.27.1", v1271.ZTunnelImage)

		v1280 := config.Config.ImageDigests["v1.28.0"]
		assert.Equal(t, "registry.example.com/istio/pilot-rhel9:1.28.0", v1280.IstiodImage)
		assert.Equal(t, "registry.example.com/istio/proxyv2-rhel9:1.28.0", v1280.ProxyImage)
		assert.Equal(t, "registry.example.com/istio/cni-rhel9:1.28.0", v1280.CNIImage)
		assert.Equal(t, "registry.example.com/istio/ztunnel-rhel9:1.28.0", v1280.ZTunnelImage)
	})

	t.Run("no-op when ImageDigests already set", func(t *testing.T) {
		config.Config = config.OperatorConfig{
			ImageDigests: map[string]config.IstioImageConfig{
				"v1.27.1": {IstiodImage: "already-set"},
			},
		}

		fs := fstest.MapFS{
			"v1.28.0/charts/istiod/Chart.yaml": &fstest.MapFile{},
		}

		err := SetImageDefaults(fs, registry, images)
		require.NoError(t, err)

		assert.Len(t, config.Config.ImageDigests, 1)
		assert.Equal(t, "already-set", config.Config.ImageDigests["v1.27.1"].IstiodImage)
	})

	t.Run("skips non-directory entries", func(t *testing.T) {
		config.Config = config.OperatorConfig{}
		fs := fstest.MapFS{
			"v1.27.1/charts/istiod/Chart.yaml": &fstest.MapFile{},
			"resources.go":                      &fstest.MapFile{},
			"README.md":                         &fstest.MapFile{},
		}

		err := SetImageDefaults(fs, registry, images)
		require.NoError(t, err)

		assert.Len(t, config.Config.ImageDigests, 1)
		assert.Contains(t, config.Config.ImageDigests, "v1.27.1")
	})
}

const minimalCSV = `apiVersion: operators.coreos.com/v1alpha1
kind: ClusterServiceVersion
metadata:
  name: test-operator.v1.0.0
spec:
  install:
    spec:
      deployments:
        - name: test-operator
          spec:
            template:
              metadata:
                annotations:
                  images.v1_27_0.istiod: gcr.io/istio-release/pilot:1.27.0
                  images.v1_27_0.proxy: gcr.io/istio-release/proxyv2:1.27.0
                  images.v1_27_0.cni: gcr.io/istio-release/install-cni:1.27.0
                  images.v1_27_0.ztunnel: gcr.io/istio-release/ztunnel:1.27.0
                  images.v1_28_1.istiod: gcr.io/istio-release/pilot:1.28.1
                  images.v1_28_1.proxy: gcr.io/istio-release/proxyv2:1.28.1
                  images.v1_28_1.cni: gcr.io/istio-release/install-cni:1.28.1
                  images.v1_28_1.ztunnel: gcr.io/istio-release/ztunnel:1.28.1
                  unrelated-annotation: something-else
`

func TestLoadImageDigestsFromCSV(t *testing.T) {
	t.Run("parses image annotations by version", func(t *testing.T) {
		config.Config = config.OperatorConfig{}
		fs := fstest.MapFS{
			"test.clusterserviceversion.yaml": &fstest.MapFile{
				Data: []byte(minimalCSV),
			},
		}

		err := LoadImageDigestsFromCSV(fs)
		require.NoError(t, err)

		assert.Len(t, config.Config.ImageDigests, 2)

		v1270 := config.Config.ImageDigests["v1.27.0"]
		assert.Equal(t, "gcr.io/istio-release/pilot:1.27.0", v1270.IstiodImage)
		assert.Equal(t, "gcr.io/istio-release/proxyv2:1.27.0", v1270.ProxyImage)
		assert.Equal(t, "gcr.io/istio-release/install-cni:1.27.0", v1270.CNIImage)
		assert.Equal(t, "gcr.io/istio-release/ztunnel:1.27.0", v1270.ZTunnelImage)

		v1281 := config.Config.ImageDigests["v1.28.1"]
		assert.Equal(t, "gcr.io/istio-release/pilot:1.28.1", v1281.IstiodImage)
		assert.Equal(t, "gcr.io/istio-release/proxyv2:1.28.1", v1281.ProxyImage)
		assert.Equal(t, "gcr.io/istio-release/install-cni:1.28.1", v1281.CNIImage)
		assert.Equal(t, "gcr.io/istio-release/ztunnel:1.28.1", v1281.ZTunnelImage)
	})

	t.Run("handles alpha version with commit hash", func(t *testing.T) {
		config.Config = config.OperatorConfig{}
		csv := `apiVersion: operators.coreos.com/v1alpha1
kind: ClusterServiceVersion
spec:
  install:
    spec:
      deployments:
        - name: op
          spec:
            template:
              metadata:
                annotations:
                  images.v1_30-alpha_abc123.istiod: gcr.io/istio-testing/pilot:1.30-alpha.abc123
                  images.v1_30-alpha_abc123.proxy: gcr.io/istio-testing/proxyv2:1.30-alpha.abc123
`
		fs := fstest.MapFS{
			"op.clusterserviceversion.yaml": &fstest.MapFile{Data: []byte(csv)},
		}

		err := LoadImageDigestsFromCSV(fs)
		require.NoError(t, err)

		v := config.Config.ImageDigests["v1.30-alpha.abc123"]
		assert.Equal(t, "gcr.io/istio-testing/pilot:1.30-alpha.abc123", v.IstiodImage)
		assert.Equal(t, "gcr.io/istio-testing/proxyv2:1.30-alpha.abc123", v.ProxyImage)
	})

	t.Run("no-op when ImageDigests already set", func(t *testing.T) {
		config.Config = config.OperatorConfig{
			ImageDigests: map[string]config.IstioImageConfig{
				"v1.27.0": {IstiodImage: "already-set"},
			},
		}
		fs := fstest.MapFS{
			"test.clusterserviceversion.yaml": &fstest.MapFile{Data: []byte(minimalCSV)},
		}

		err := LoadImageDigestsFromCSV(fs)
		require.NoError(t, err)

		assert.Len(t, config.Config.ImageDigests, 1)
		assert.Equal(t, "already-set", config.Config.ImageDigests["v1.27.0"].IstiodImage)
	})

	t.Run("error when no CSV file found", func(t *testing.T) {
		config.Config = config.OperatorConfig{}
		fs := fstest.MapFS{
			"something-else.yaml": &fstest.MapFile{Data: []byte("foo: bar")},
		}

		err := LoadImageDigestsFromCSV(fs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no *.clusterserviceversion.yaml file found")
	})

	t.Run("error when CSV has no image annotations", func(t *testing.T) {
		config.Config = config.OperatorConfig{}
		csv := `apiVersion: operators.coreos.com/v1alpha1
kind: ClusterServiceVersion
spec:
  install:
    spec:
      deployments:
        - name: op
          spec:
            template:
              metadata:
                annotations:
                  unrelated: value
`
		fs := fstest.MapFS{
			"op.clusterserviceversion.yaml": &fstest.MapFile{Data: []byte(csv)},
		}

		err := LoadImageDigestsFromCSV(fs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no image annotations found")
	})
}
