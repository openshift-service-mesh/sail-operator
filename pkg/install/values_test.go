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

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func TestGatewayAPIDefaults(t *testing.T) {
	defaults := GatewayAPIDefaults()

	// Check Global settings
	require.NotNil(t, defaults.Global)
	assert.NotNil(t, defaults.Global.DefaultPodDisruptionBudget)
	assert.Equal(t, false, *defaults.Global.DefaultPodDisruptionBudget.Enabled)

	// Check Pilot settings
	require.NotNil(t, defaults.Pilot)
	assert.Equal(t, true, *defaults.Pilot.Enabled)
	assert.NotNil(t, defaults.Pilot.Cni)
	assert.Equal(t, false, *defaults.Pilot.Cni.Enabled)

	// Check Gateway API env vars
	require.NotNil(t, defaults.Pilot.Env)
	assert.Equal(t, "true", defaults.Pilot.Env["PILOT_ENABLE_GATEWAY_API"])
	assert.Equal(t, "false", defaults.Pilot.Env["PILOT_ENABLE_ALPHA_GATEWAY_API"])
	assert.Equal(t, "true", defaults.Pilot.Env["PILOT_ENABLE_GATEWAY_API_STATUS"])
	assert.Equal(t, "true", defaults.Pilot.Env["PILOT_ENABLE_GATEWAY_API_DEPLOYMENT_CONTROLLER"])
	assert.Equal(t, "false", defaults.Pilot.Env["PILOT_ENABLE_GATEWAY_API_GATEWAYCLASS_CONTROLLER"])
	assert.Equal(t, "false", defaults.Pilot.Env["PILOT_MULTI_NETWORK_DISCOVER_GATEWAY_API"])
	assert.Equal(t, "false", defaults.Pilot.Env["ENABLE_GATEWAY_API_MANUAL_DEPLOYMENT"])
	assert.Equal(t, "true", defaults.Pilot.Env["PILOT_ENABLE_GATEWAY_API_CA_CERT_ONLY"])
	assert.Equal(t, "false", defaults.Pilot.Env["PILOT_ENABLE_GATEWAY_API_COPY_LABELS_ANNOTATIONS"])

	// Check resource filtering env vars (X_ prefixed until Istio feature is ready)
	assert.Equal(t, gatewayAPIIgnoreResources, defaults.Pilot.Env[envPilotIgnoreResources])
	assert.Equal(t, gatewayAPIIncludeResources, defaults.Pilot.Env[envPilotIncludeResources])

	// Check SidecarInjectorWebhook settings
	require.NotNil(t, defaults.SidecarInjectorWebhook)
	assert.Equal(t, false, *defaults.SidecarInjectorWebhook.EnableNamespacesByDefault)

	// Check MeshConfig settings
	require.NotNil(t, defaults.MeshConfig)
	assert.Equal(t, "/dev/stdout", *defaults.MeshConfig.AccessLogFile)
	assert.Equal(t, v1.MeshConfigIngressControllerModeOff, defaults.MeshConfig.IngressControllerMode)

	// Check proxy headers
	require.NotNil(t, defaults.MeshConfig.DefaultConfig)
	require.NotNil(t, defaults.MeshConfig.DefaultConfig.ProxyHeaders)
	assert.Equal(t, true, *defaults.MeshConfig.DefaultConfig.ProxyHeaders.Server.Disabled)
	assert.Equal(t, true, *defaults.MeshConfig.DefaultConfig.ProxyHeaders.EnvoyDebugHeaders.Disabled)
	assert.Equal(t, v1.ProxyConfigProxyHeadersMetadataExchangeModeInMesh, defaults.MeshConfig.DefaultConfig.ProxyHeaders.MetadataExchangeHeaders.Mode)
}

func TestMergeValues(t *testing.T) {
	t.Run("nil base returns overlay", func(t *testing.T) {
		overlay := &v1.Values{
			Global: &v1.GlobalConfig{Hub: ptr.To("overlay-hub")},
		}

		result := MergeValues(nil, overlay)

		assert.Equal(t, overlay, result)
	})

	t.Run("nil overlay returns base", func(t *testing.T) {
		base := &v1.Values{
			Global: &v1.GlobalConfig{Hub: ptr.To("base-hub")},
		}

		result := MergeValues(base, nil)

		assert.Equal(t, base, result)
	})

	t.Run("overlay takes precedence", func(t *testing.T) {
		base := &v1.Values{
			Global: &v1.GlobalConfig{
				Hub:               ptr.To("base-hub"),
				PriorityClassName: ptr.To("base-priority"),
			},
		}
		overlay := &v1.Values{
			Global: &v1.GlobalConfig{
				Hub: ptr.To("overlay-hub"),
			},
		}

		result := MergeValues(base, overlay)

		assert.Equal(t, "overlay-hub", *result.Global.Hub)
	})

	t.Run("env maps are merged", func(t *testing.T) {
		base := &v1.Values{
			Pilot: &v1.PilotConfig{
				Env: map[string]string{
					"KEY1": "value1",
					"KEY2": "value2",
				},
			},
		}
		overlay := &v1.Values{
			Pilot: &v1.PilotConfig{
				Env: map[string]string{
					"KEY2": "overlay-value2",
					"KEY3": "value3",
				},
			},
		}

		result := MergeValues(base, overlay)

		assert.Equal(t, "value1", result.Pilot.Env["KEY1"])
		assert.Equal(t, "overlay-value2", result.Pilot.Env["KEY2"])
		assert.Equal(t, "value3", result.Pilot.Env["KEY3"])
	})

	t.Run("does not mutate base", func(t *testing.T) {
		base := &v1.Values{
			Global: &v1.GlobalConfig{Hub: ptr.To("base-hub")},
		}
		overlay := &v1.Values{
			Global: &v1.GlobalConfig{Hub: ptr.To("overlay-hub")},
		}

		_ = MergeValues(base, overlay)

		assert.Equal(t, "base-hub", *base.Global.Hub)
	})

	t.Run("lists are replaced not merged", func(t *testing.T) {
		base := &v1.Values{
			Pilot: &v1.PilotConfig{
				ExtraContainerArgs: []string{"--foo", "--bar"},
			},
		}
		overlay := &v1.Values{
			Pilot: &v1.PilotConfig{
				ExtraContainerArgs: []string{"--baz"},
			},
		}

		result := MergeValues(base, overlay)

		// Lists are replaced entirely, not merged (matches Helm semantics)
		assert.Equal(t, []string{"--baz"}, result.Pilot.ExtraContainerArgs)
	})
}
