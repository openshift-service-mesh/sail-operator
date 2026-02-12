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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultVersion(t *testing.T) {
	t.Run("multiple stable versions returns highest", func(t *testing.T) {
		fs := fstest.MapFS{
			"v1.27.5/charts/.keep": &fstest.MapFile{},
			"v1.28.0/charts/.keep": &fstest.MapFile{},
			"v1.28.3/charts/.keep": &fstest.MapFile{},
		}
		v, err := DefaultVersion(fs)
		require.NoError(t, err)
		assert.Equal(t, "v1.28.3", v)
	})

	t.Run("pre-release versions skipped", func(t *testing.T) {
		fs := fstest.MapFS{
			"v1.28.3/charts/.keep":          &fstest.MapFile{},
			"v1.30-alpha.abc/charts/.keep":   &fstest.MapFile{},
			"v1.29.0-beta.1/charts/.keep":    &fstest.MapFile{},
			"v1.29.0-rc.2/charts/.keep":      &fstest.MapFile{},
		}
		v, err := DefaultVersion(fs)
		require.NoError(t, err)
		assert.Equal(t, "v1.28.3", v)
	})

	t.Run("no stable versions returns error", func(t *testing.T) {
		fs := fstest.MapFS{
			"v1.30-alpha.abc/charts/.keep": &fstest.MapFile{},
		}
		_, err := DefaultVersion(fs)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no stable version")
	})

	t.Run("non-semver directories silently skipped", func(t *testing.T) {
		fs := fstest.MapFS{
			"not-a-version/charts/.keep": &fstest.MapFile{},
			"v1.27.0/charts/.keep":       &fstest.MapFile{},
			"resources.go":               &fstest.MapFile{},
		}
		v, err := DefaultVersion(fs)
		require.NoError(t, err)
		assert.Equal(t, "v1.27.0", v)
	})

	t.Run("single version works", func(t *testing.T) {
		fs := fstest.MapFS{
			"v1.28.2/charts/.keep": &fstest.MapFile{},
		}
		v, err := DefaultVersion(fs)
		require.NoError(t, err)
		assert.Equal(t, "v1.28.2", v)
	})

	t.Run("empty FS returns error", func(t *testing.T) {
		fs := fstest.MapFS{}
		_, err := DefaultVersion(fs)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no stable version")
	})
}

func TestNormalizeVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"v1.24.0", "v1.24.0"},
		{"1.24.0", "v1.24.0"},
		{"v1.28.3-rc.1", "v1.28.3-rc.1"},
		{"1.28.3-rc.1", "v1.28.3-rc.1"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, NormalizeVersion(tt.input))
		})
	}
}

func TestValidateVersion(t *testing.T) {
	fs := fstest.MapFS{
		"v1.28.3/charts/.keep": &fstest.MapFile{},
		"resources.go":         &fstest.MapFile{},
	}

	t.Run("existing directory passes", func(t *testing.T) {
		err := ValidateVersion(fs, "v1.28.3")
		assert.NoError(t, err)
	})

	t.Run("missing directory returns error", func(t *testing.T) {
		err := ValidateVersion(fs, "v1.99.0")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("file instead of directory returns error", func(t *testing.T) {
		err := ValidateVersion(fs, "resources.go")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a directory")
	})
}
