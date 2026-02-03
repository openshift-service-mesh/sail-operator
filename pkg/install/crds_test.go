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
			ignore:   GatewayAPIIgnoreResources,
			include:  GatewayAPIIncludeResources,
			expected: true,
		},
		{
			name:     "Gateway API mode - envoyfilters included",
			resource: "envoyfilters.networking.istio.io",
			ignore:   GatewayAPIIgnoreResources,
			include:  GatewayAPIIncludeResources,
			expected: true,
		},
		{
			name:     "Gateway API mode - destinationrules included",
			resource: "destinationrules.networking.istio.io",
			ignore:   GatewayAPIIgnoreResources,
			include:  GatewayAPIIncludeResources,
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
	assert.Equal(t, "X_PILOT_IGNORE_RESOURCES", EnvPilotIgnoreResources)
	assert.Equal(t, "X_PILOT_INCLUDE_RESOURCES", EnvPilotIncludeResources)
	assert.Equal(t, "*.istio.io", GatewayAPIIgnoreResources)
	assert.Contains(t, GatewayAPIIncludeResources, "wasmplugins.extensions.istio.io")
	assert.Contains(t, GatewayAPIIncludeResources, "envoyfilters.networking.istio.io")
	assert.Contains(t, GatewayAPIIncludeResources, "destinationrules.networking.istio.io")
}

func TestGatewayAPIFiltersWorkTogether(t *testing.T) {
	// Test that our Gateway API filter values work correctly together
	// IGNORE: *.istio.io (all istio resources)
	// INCLUDE: wasmplugins, envoyfilters, destinationrules (exceptions)

	// These should be managed (explicitly included, overrides IGNORE)
	assert.True(t, shouldManageResource("wasmplugins.extensions.istio.io", GatewayAPIIgnoreResources, GatewayAPIIncludeResources),
		"wasmplugins should be managed (in INCLUDE)")
	assert.True(t, shouldManageResource("envoyfilters.networking.istio.io", GatewayAPIIgnoreResources, GatewayAPIIncludeResources),
		"envoyfilters should be managed (in INCLUDE)")
	assert.True(t, shouldManageResource("destinationrules.networking.istio.io", GatewayAPIIgnoreResources, GatewayAPIIncludeResources),
		"destinationrules should be managed (in INCLUDE)")

	// These should NOT be managed (matched by IGNORE, not in INCLUDE)
	assert.False(t, shouldManageResource("virtualservices.networking.istio.io", GatewayAPIIgnoreResources, GatewayAPIIncludeResources),
		"virtualservices should NOT be managed (in IGNORE, not in INCLUDE)")
	assert.False(t, shouldManageResource("gateways.networking.istio.io", GatewayAPIIgnoreResources, GatewayAPIIncludeResources),
		"gateways should NOT be managed (in IGNORE, not in INCLUDE)")
	assert.False(t, shouldManageResource("serviceentries.networking.istio.io", GatewayAPIIgnoreResources, GatewayAPIIncludeResources),
		"serviceentries should NOT be managed (in IGNORE, not in INCLUDE)")
	assert.False(t, shouldManageResource("authorizationpolicies.security.istio.io", GatewayAPIIgnoreResources, GatewayAPIIncludeResources),
		"authorizationpolicies should NOT be managed (in IGNORE, not in INCLUDE)")

	// Non-istio resources should be managed (not matched by IGNORE)
	assert.True(t, shouldManageResource("gateways.gateway.networking.k8s.io", GatewayAPIIgnoreResources, GatewayAPIIncludeResources),
		"k8s gateway resources should be managed (not in IGNORE)")
}
