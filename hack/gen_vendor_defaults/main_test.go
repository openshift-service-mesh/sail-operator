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

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Masterminds/semver/v3"
)

const repoRoot = "../.."

// TestGeneratedFileIsUpToDate is the guard against pkg/istiovalues/vendor_defaults.yaml
// drifting away from the source file and the supported version list. It is the
// reason a versions.ossm.yaml bump can no longer silently skip a version.
func TestGeneratedFileIsUpToDate(t *testing.T) {
	versions, err := readVersions(filepath.Join(repoRoot, defaultVersionsPath))
	if err != nil {
		t.Fatal(err)
	}
	rules, err := readRules(filepath.Join(repoRoot, defaultSrcPath))
	if err != nil {
		t.Fatal(err)
	}
	want, _, err := expand(rules, versions)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(repoRoot, defaultOutPath))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is out of date; run 'make gen-vendor-defaults'", defaultOutPath)
	}
}

// TestSourceRulesMatchSomething catches a constraint that no shipped version
// satisfies. The generator only warns (a workaround may legitimately age out),
// so this surfaces it as a skip rather than silence.
func TestSourceRulesMatchSomething(t *testing.T) {
	versions, err := readVersions(filepath.Join(repoRoot, defaultVersionsPath))
	if err != nil {
		t.Fatal(err)
	}
	rules, err := readRules(filepath.Join(repoRoot, defaultSrcPath))
	if err != nil {
		t.Fatal(err)
	}
	if unmatched := unmatchedRules(rules, versions); len(unmatched) > 0 {
		t.Skipf("rule(s) %s match no installable version on this branch; "+
			"remove them once the workaround is no longer needed anywhere",
			strings.Join(unmatched, ", "))
	}
}

func TestReadRulesRejectsInvalidSource(t *testing.T) {
	testcases := []struct {
		name    string
		src     string
		wantErr string
	}{
		{
			name: "missing resource type level",
			src: `
v1.24.2:
  pilot:
    env:
      SOME_VAR: "false"
`,
			wantErr: `"pilot" is not a resource type`,
		},
		{
			name: "unparseable constraint",
			src: `
not-a-constraint:
  istio: {}
`,
			wantErr: "is not a valid semver constraint",
		},
		{
			name: "valid constraint and resource type",
			src: `
">=1.26.0 <1.27.8":
  istio:
    pilot:
      env:
        SOME_VAR: "false"
`,
			wantErr: "",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "src.yaml")
			if err := os.WriteFile(path, []byte(tc.src), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := readRules(path)
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tc.wantErr != "" && err == nil:
				t.Fatalf("expected error containing %q, got none", tc.wantErr)
			case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
				t.Fatalf("expected error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

func TestExpandAppliesConstraintsPerVersion(t *testing.T) {
	src := `
">=1.26.0 <1.27.8":
  istio:
    pilot:
      env:
        WORKAROUND: "false"
">=1.27.0":
  istio:
    pilot:
      env:
        NEWER: "true"
`
	path := filepath.Join(t.TempDir(), "src.yaml")
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	rules, err := readRules(path)
	if err != nil {
		t.Fatal(err)
	}

	versions := make([]*semver.Version, 0, 4)
	for _, v := range []string{"v1.28.0", "v1.27.3", "v1.26.2", "v1.25.0"} {
		versions = append(versions, semver.MustParse(v))
	}

	out, entries, err := expand(rules, versions)
	if err != nil {
		t.Fatal(err)
	}
	if entries != 3 { // v1.25.0 matches neither rule
		t.Errorf("expected 3 version entries, got %d:\n%s", entries, out)
	}
	for _, want := range []string{"v1.28.0:", "v1.27.3:", "v1.26.2:"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output:\n%s", want, out)
		}
	}
	if strings.Contains(out, "v1.25.0:") {
		t.Errorf("v1.25.0 matches no rule but appears in output:\n%s", out)
	}
	// v1.27.3 satisfies both constraints, so both keys must be merged into it.
	v1273 := section(out, "v1.27.3:")
	if !strings.Contains(v1273, "WORKAROUND") || !strings.Contains(v1273, "NEWER") {
		t.Errorf("expected v1.27.3 to merge both rules, got:\n%s", v1273)
	}
}

// section returns the generated block for the given version key.
func section(out, key string) string {
	_, rest, found := strings.Cut(out, key)
	if !found {
		return ""
	}
	var b strings.Builder
	for line := range strings.SplitSeq(rest, "\n") {
		if strings.HasPrefix(line, "v") && strings.HasSuffix(line, ":") {
			break
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}
