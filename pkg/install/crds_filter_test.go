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
)

func TestResourceToCRDFilename(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		expected string
	}{
		{
			name:     "wasmplugins",
			resource: "wasmplugins.extensions.istio.io",
			expected: "extensions.istio.io_wasmplugins.yaml",
		},
		{
			name:     "envoyfilters",
			resource: "envoyfilters.networking.istio.io",
			expected: "networking.istio.io_envoyfilters.yaml",
		},
		{
			name:     "destinationrules",
			resource: "destinationrules.networking.istio.io",
			expected: "networking.istio.io_destinationrules.yaml",
		},
		{
			name:     "virtualservices",
			resource: "virtualservices.networking.istio.io",
			expected: "networking.istio.io_virtualservices.yaml",
		},
		{
			name:     "authorizationpolicies",
			resource: "authorizationpolicies.security.istio.io",
			expected: "security.istio.io_authorizationpolicies.yaml",
		},
		{
			name:     "invalid - no group",
			resource: "wasmplugins",
			expected: "",
		},
		{
			name:     "invalid - empty",
			resource: "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resourceToCRDFilename(tt.resource)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCRDFilenameToResource(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{
			name:     "wasmplugins",
			filename: "extensions.istio.io_wasmplugins.yaml",
			expected: "wasmplugins.extensions.istio.io",
		},
		{
			name:     "envoyfilters",
			filename: "networking.istio.io_envoyfilters.yaml",
			expected: "envoyfilters.networking.istio.io",
		},
		{
			name:     "no underscore",
			filename: "invalid.yaml",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := crdFilenameToResource(tt.filename)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		pattern  string
		expected bool
	}{
		{
			name:     "exact match",
			resource: "wasmplugins.extensions.istio.io",
			pattern:  "wasmplugins.extensions.istio.io",
			expected: true,
		},
		{
			name:     "wildcard suffix matches istio resources",
			resource: "virtualservices.networking.istio.io",
			pattern:  "*.istio.io",
			expected: true, // * matches any characters including dots
		},
		{
			name:     "wildcard matches wasmplugins",
			resource: "wasmplugins.extensions.istio.io",
			pattern:  "*.istio.io",
			expected: true,
		},
		{
			name:     "wildcard matches short name",
			resource: "networking.istio.io",
			pattern:  "*.istio.io",
			expected: true,
		},
		{
			name:     "no match - different domain",
			resource: "gateways.gateway.networking.k8s.io",
			pattern:  "*.istio.io",
			expected: false,
		},
		{
			name:     "no match - exact pattern mismatch",
			resource: "wasmplugins.extensions.istio.io",
			pattern:  "virtualservices.networking.istio.io",
			expected: false,
		},
		{
			name:     "empty pattern",
			resource: "wasmplugins.extensions.istio.io",
			pattern:  "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesPattern(tt.resource, tt.pattern)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchesAnyPattern(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		patterns string
		expected bool
	}{
		{
			name:     "matches first pattern",
			resource: "wasmplugins.extensions.istio.io",
			patterns: "wasmplugins.extensions.istio.io,envoyfilters.networking.istio.io",
			expected: true,
		},
		{
			name:     "matches second pattern",
			resource: "envoyfilters.networking.istio.io",
			patterns: "wasmplugins.extensions.istio.io,envoyfilters.networking.istio.io",
			expected: true,
		},
		{
			name:     "no match",
			resource: "virtualservices.networking.istio.io",
			patterns: "wasmplugins.extensions.istio.io,envoyfilters.networking.istio.io",
			expected: false,
		},
		{
			name:     "empty patterns",
			resource: "wasmplugins.extensions.istio.io",
			patterns: "",
			expected: false,
		},
		{
			name:     "patterns with spaces",
			resource: "wasmplugins.extensions.istio.io",
			patterns: " wasmplugins.extensions.istio.io , envoyfilters.networking.istio.io ",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesAnyPattern(tt.resource, tt.patterns)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShouldManageResource(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		ignore   string
		include  string
		expected bool
	}{
		{
			name:     "included resource - should manage",
			resource: "wasmplugins.extensions.istio.io",
			ignore:   "",
			include:  "wasmplugins.extensions.istio.io",
			expected: true,
		},
		{
			name:     "include overrides ignore",
			resource: "wasmplugins.extensions.istio.io",
			ignore:   "wasmplugins.extensions.istio.io",
			include:  "wasmplugins.extensions.istio.io",
			expected: true, // INCLUDE takes precedence
		},
		{
			name:     "ignored and not included - should not manage",
			resource: "virtualservices.networking.istio.io",
			ignore:   "virtualservices.networking.istio.io",
			include:  "wasmplugins.extensions.istio.io",
			expected: false,
		},
		{
			name:     "not in any filter - default manage",
			resource: "wasmplugins.extensions.istio.io",
			ignore:   "",
			include:  "",
			expected: true,
		},
		{
			name:     "Gateway API mode - included resource",
			resource: "wasmplugins.extensions.istio.io",
			ignore:   gatewayAPIIgnoreResources,
			include:  gatewayAPIIncludeResources,
			expected: true,
		},
		{
			name:     "Gateway API mode - envoyfilters included",
			resource: "envoyfilters.networking.istio.io",
			ignore:   gatewayAPIIgnoreResources,
			include:  gatewayAPIIncludeResources,
			expected: true,
		},
		{
			name:     "Gateway API mode - destinationrules included",
			resource: "destinationrules.networking.istio.io",
			ignore:   gatewayAPIIgnoreResources,
			include:  gatewayAPIIncludeResources,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := shouldManageResource(tt.resource, tt.ignore, tt.include)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResourceFilteringConstants(t *testing.T) {
	// Verify the constants have expected values
	assert.Equal(t, "X_PILOT_IGNORE_RESOURCES", envPilotIgnoreResources)
	assert.Equal(t, "X_PILOT_INCLUDE_RESOURCES", envPilotIncludeResources)
	assert.Equal(t, "*.istio.io", gatewayAPIIgnoreResources)
	assert.Contains(t, gatewayAPIIncludeResources, "wasmplugins.extensions.istio.io")
	assert.Contains(t, gatewayAPIIncludeResources, "envoyfilters.networking.istio.io")
	assert.Contains(t, gatewayAPIIncludeResources, "destinationrules.networking.istio.io")
}

func TestGatewayAPIFiltersWorkTogether(t *testing.T) {
	// Test that our Gateway API filter values work correctly together
	// IGNORE: *.istio.io (all istio resources)
	// INCLUDE: wasmplugins, envoyfilters, destinationrules (exceptions)

	// These should be managed (explicitly included, overrides IGNORE)
	assert.True(t, shouldManageResource("wasmplugins.extensions.istio.io", gatewayAPIIgnoreResources, gatewayAPIIncludeResources),
		"wasmplugins should be managed (in INCLUDE)")
	assert.True(t, shouldManageResource("envoyfilters.networking.istio.io", gatewayAPIIgnoreResources, gatewayAPIIncludeResources),
		"envoyfilters should be managed (in INCLUDE)")
	assert.True(t, shouldManageResource("destinationrules.networking.istio.io", gatewayAPIIgnoreResources, gatewayAPIIncludeResources),
		"destinationrules should be managed (in INCLUDE)")

	// These should NOT be managed (matched by IGNORE, not in INCLUDE)
	assert.False(t, shouldManageResource("virtualservices.networking.istio.io", gatewayAPIIgnoreResources, gatewayAPIIncludeResources),
		"virtualservices should NOT be managed (in IGNORE, not in INCLUDE)")
	assert.False(t, shouldManageResource("gateways.networking.istio.io", gatewayAPIIgnoreResources, gatewayAPIIncludeResources),
		"gateways should NOT be managed (in IGNORE, not in INCLUDE)")
	assert.False(t, shouldManageResource("serviceentries.networking.istio.io", gatewayAPIIgnoreResources, gatewayAPIIncludeResources),
		"serviceentries should NOT be managed (in IGNORE, not in INCLUDE)")
	assert.False(t, shouldManageResource("authorizationpolicies.security.istio.io", gatewayAPIIgnoreResources, gatewayAPIIncludeResources),
		"authorizationpolicies should NOT be managed (in IGNORE, not in INCLUDE)")

	// Non-istio resources should be managed (not matched by IGNORE)
	assert.True(t, shouldManageResource("gateways.gateway.networking.k8s.io", gatewayAPIIgnoreResources, gatewayAPIIncludeResources),
		"k8s gateway resources should be managed (not in IGNORE)")
}

func TestAllIstioCRDs(t *testing.T) {
	resources, err := allIstioCRDs()
	require.NoError(t, err)

	// Should have Istio CRDs but no sailoperator.io CRDs
	assert.NotEmpty(t, resources)
	for _, r := range resources {
		assert.NotContains(t, r, "sailoperator.io", "sailoperator.io CRDs should be excluded")
		assert.Contains(t, r, "istio.io", "all CRDs should be *.istio.io")
	}

	// Verify expected CRDs are present
	assert.Contains(t, resources, "wasmplugins.extensions.istio.io")
	assert.Contains(t, resources, "envoyfilters.networking.istio.io")
	assert.Contains(t, resources, "destinationrules.networking.istio.io")
	assert.Contains(t, resources, "virtualservices.networking.istio.io")
}

func TestTargetCRDsFromValues(t *testing.T) {
	t.Run("include all CRDs", func(t *testing.T) {
		targets, err := targetCRDsFromValues(nil, true)
		require.NoError(t, err)
		assert.NotEmpty(t, targets)
		// Should contain all istio CRDs
		assert.Contains(t, targets, "wasmplugins.extensions.istio.io")
	})

	t.Run("filtered by PILOT_INCLUDE_RESOURCES", func(t *testing.T) {
		values := &v1.Values{
			Pilot: &v1.PilotConfig{
				Env: map[string]string{
					envPilotIgnoreResources:  gatewayAPIIgnoreResources,
					envPilotIncludeResources: gatewayAPIIncludeResources,
				},
			},
		}
		targets, err := targetCRDsFromValues(values, false)
		require.NoError(t, err)
		assert.Len(t, targets, 3)
		assert.Contains(t, targets, "wasmplugins.extensions.istio.io")
		assert.Contains(t, targets, "envoyfilters.networking.istio.io")
		assert.Contains(t, targets, "destinationrules.networking.istio.io")
	})

	t.Run("nil values", func(t *testing.T) {
		targets, err := targetCRDsFromValues(nil, false)
		require.NoError(t, err)
		assert.Empty(t, targets)
	})

	t.Run("no filters defined", func(t *testing.T) {
		values := &v1.Values{
			Pilot: &v1.PilotConfig{
				Env: map[string]string{},
			},
		}
		targets, err := targetCRDsFromValues(values, false)
		require.NoError(t, err)
		assert.Empty(t, targets)
	})
}
