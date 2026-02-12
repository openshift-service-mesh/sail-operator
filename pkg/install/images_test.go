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
