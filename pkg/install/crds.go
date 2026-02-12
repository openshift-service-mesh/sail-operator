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
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/chart/crds"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// CRD ownership labels and annotations.
const (
	// LabelManagedByCIO indicates the CRD is managed by the Cluster Ingress Operator.
	LabelManagedByCIO = "ingress.operator.openshift.io/owned"

	// LabelOLMManaged indicates the CRD is managed by OLM (OSSM subscription).
	LabelOLMManaged = "olm.managed"

	// AnnotationHelmKeep prevents Helm from deleting the CRD during uninstall.
	AnnotationHelmKeep = "helm.sh/resource-policy"
)

// CRDManagementState represents the aggregate ownership state of Istio CRDs on the cluster.
type CRDManagementState string

const (
	// CRDManagedByCIO means all target CRDs are owned by the Cluster Ingress Operator.
	// CRDs will be installed or updated.
	CRDManagedByCIO CRDManagementState = "ManagedByCIO"

	// CRDManagedByOLM means all target CRDs are owned by an OSSM subscription via OLM.
	// CRDs are left alone; Helm install still proceeds.
	CRDManagedByOLM CRDManagementState = "ManagedByOLM"

	// CRDUnknownManagement means one or more CRDs exist but are not owned by CIO or OLM.
	// CRDs are left alone; Helm install still proceeds; Status.Error is set.
	CRDUnknownManagement CRDManagementState = "UnknownManagement"

	// CRDMixedOwnership means CRDs have inconsistent ownership (some CIO, some OLM, some unknown, or some missing).
	// CRDs are left alone; Helm install still proceeds; Status.Error is set.
	CRDMixedOwnership CRDManagementState = "MixedOwnership"

	// CRDNoneExist means no target CRDs exist on the cluster yet.
	// CRDs will be installed with CIO ownership labels.
	CRDNoneExist CRDManagementState = "NoneExist"
)

// CRDInfo describes the state of a single CRD on the cluster.
type CRDInfo struct {
	// Name is the CRD name, e.g. "wasmplugins.extensions.istio.io"
	Name string

	// State is the ownership state of this specific CRD.
	// Only meaningful when Found is true.
	State CRDManagementState

	// Found indicates whether this CRD exists on the cluster.
	Found bool
}

// resourceToCRDFilename converts a resource name to its CRD filename.
// The naming convention is: "{plural}.{group}" -> "{group}_{plural}.yaml"
// Example: "wasmplugins.extensions.istio.io" -> "extensions.istio.io_wasmplugins.yaml"
func resourceToCRDFilename(resource string) string {
	parts := strings.SplitN(resource, ".", 2)
	if len(parts) != 2 {
		return ""
	}
	plural := parts[0] // e.g., "wasmplugins"
	group := parts[1]  // e.g., "extensions.istio.io"
	return fmt.Sprintf("%s_%s.yaml", group, plural)
}

// crdFilenameToResource converts a CRD filename back to a resource name.
// The naming convention is: "{group}_{plural}.yaml" -> "{plural}.{group}"
// Example: "extensions.istio.io_wasmplugins.yaml" -> "wasmplugins.extensions.istio.io"
func crdFilenameToResource(filename string) string {
	// Strip .yaml suffix
	name := strings.TrimSuffix(filename, ".yaml")
	// Split on underscore: group_plural
	parts := strings.SplitN(name, "_", 2)
	if len(parts) != 2 {
		return ""
	}
	group := parts[0]  // e.g., "extensions.istio.io"
	plural := parts[1] // e.g., "wasmplugins"
	return fmt.Sprintf("%s.%s", plural, group)
}

// matchesPattern checks if a resource matches a filter pattern.
// Patterns support glob-style wildcards:
//   - "*.istio.io" matches "virtualservices.networking.istio.io"
//   - "virtualservices.networking.istio.io" matches exactly
//
// The resource format is "{plural}.{group}" (e.g., "wasmplugins.extensions.istio.io")
func matchesPattern(resource, pattern string) bool {
	// Handle glob patterns using filepath.Match
	// Pattern "*.istio.io" should match "virtualservices.networking.istio.io"
	matched, err := filepath.Match(pattern, resource)
	if err != nil {
		return false
	}
	return matched
}

// matchesAnyPattern checks if a resource matches any pattern in a comma-separated list.
func matchesAnyPattern(resource, patterns string) bool {
	if patterns == "" {
		return false
	}
	for _, pattern := range strings.Split(patterns, ",") {
		pattern = strings.TrimSpace(pattern)
		if matchesPattern(resource, pattern) {
			return true
		}
	}
	return false
}

// shouldManageResource determines if a resource should be managed based on
// IGNORE and INCLUDE filters. The logic follows Istio's resource filtering:
//   - If resource matches INCLUDE, manage it (INCLUDE overrides IGNORE)
//   - If resource matches IGNORE (and not INCLUDE), skip it
//   - If resource matches neither, manage it (default allow)
func shouldManageResource(resource, ignorePatterns, includePatterns string) bool {
	// INCLUDE takes precedence - if explicitly included, always manage
	if matchesAnyPattern(resource, includePatterns) {
		return true
	}
	// If ignored (and not included), skip
	if matchesAnyPattern(resource, ignorePatterns) {
		return false
	}
	// Default: manage if not matched by any filter
	return true
}

// allIstioCRDs returns all *.istio.io CRD resource names from the embedded CRD filesystem.
// Sail operator CRDs (sailoperator.io) are excluded since those belong to the Sail operator.
func allIstioCRDs() ([]string, error) {
	entries, err := fs.ReadDir(crds.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("failed to read CRD directory: %w", err)
	}

	var resources []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		// Skip Sail operator CRDs
		if strings.HasPrefix(entry.Name(), "sailoperator.io_") {
			continue
		}
		resource := crdFilenameToResource(entry.Name())
		if resource != "" {
			resources = append(resources, resource)
		}
	}
	return resources, nil
}

// targetCRDsFromValues determines the target CRD set based on values and options.
// If includeAllCRDs is true, returns all *.istio.io CRDs.
// Otherwise, returns CRDs derived from PILOT_INCLUDE_RESOURCES in values.
func targetCRDsFromValues(values *v1.Values, includeAllCRDs bool) ([]string, error) {
	if includeAllCRDs {
		return allIstioCRDs()
	}

	if values == nil || values.Pilot == nil || values.Pilot.Env == nil {
		return nil, nil
	}

	ignorePatterns := values.Pilot.Env[EnvPilotIgnoreResources]
	includePatterns := values.Pilot.Env[EnvPilotIncludeResources]

	// If no filters defined, nothing to do
	if ignorePatterns == "" && includePatterns == "" {
		return nil, nil
	}

	var targets []string
	if includePatterns != "" {
		for _, resource := range strings.Split(includePatterns, ",") {
			resource = strings.TrimSpace(resource)
			if !shouldManageResource(resource, ignorePatterns, includePatterns) {
				continue
			}
			// Skip resources that don't map to a valid CRD filename (e.g. wildcards)
			if resourceToCRDFilename(resource) == "" {
				continue
			}
			targets = append(targets, resource)
		}
	}
	return targets, nil
}

// classifyCRD checks a single CRD on the cluster and returns its ownership state.
func classifyCRD(ctx context.Context, cl client.Client, crdName string) CRDInfo {
	existing := &apiextensionsv1.CustomResourceDefinition{}
	err := cl.Get(ctx, client.ObjectKey{Name: crdName}, existing)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return CRDInfo{Name: crdName, Found: false}
		}
		// Treat API errors as unknown management (we can't determine ownership)
		return CRDInfo{Name: crdName, Found: true, State: CRDUnknownManagement}
	}

	labels := existing.GetLabels()

	// Check CIO ownership
	if _, ok := labels[LabelManagedByCIO]; ok {
		return CRDInfo{Name: crdName, Found: true, State: CRDManagedByCIO}
	}

	// Check OLM ownership
	if val, ok := labels[LabelOLMManaged]; ok && val == "true" {
		return CRDInfo{Name: crdName, Found: true, State: CRDManagedByOLM}
	}

	// No recognized ownership labels
	return CRDInfo{Name: crdName, Found: true, State: CRDUnknownManagement}
}

// classifyCRDs checks all target CRDs on the cluster and returns the aggregate state.
func classifyCRDs(ctx context.Context, cl client.Client, targets []string) (CRDManagementState, []CRDInfo) {
	if len(targets) == 0 {
		return CRDNoneExist, nil
	}

	infos := make([]CRDInfo, len(targets))
	for i, target := range targets {
		// Derive the CRD name from the resource name.
		// For "wasmplugins.extensions.istio.io" the CRD name is "wasmplugins.extensions.istio.io" (same as the resource name).
		infos[i] = classifyCRD(ctx, cl, target)
	}

	return aggregateCRDState(infos), infos
}

// aggregateCRDState derives the batch state from individual CRD states.
//
// Rules:
//   - All not found → CRDNoneExist
//   - All found are CIO or unknown (no OLM), with at least one CIO → CRDManagedByCIO
//     (unknown = label drift, missing = deleted; both get fixed by updateCRDs)
//   - All found OLM, all present → CRDManagedByOLM
//   - Pure unknown (no CIO, no OLM) → CRDUnknownManagement
//   - Any mix involving OLM → CRDMixedOwnership
func aggregateCRDState(infos []CRDInfo) CRDManagementState {
	if len(infos) == 0 {
		return CRDNoneExist
	}

	var foundCount, cioCount, olmCount, unknownCount int
	for _, info := range infos {
		if !info.Found {
			continue
		}
		foundCount++
		switch info.State {
		case CRDManagedByCIO:
			cioCount++
		case CRDManagedByOLM:
			olmCount++
		default:
			unknownCount++
		}
	}

	total := len(infos)

	// None exist on cluster
	if foundCount == 0 {
		return CRDNoneExist
	}

	// All found are CIO-owned (possibly with some missing or some that lost labels).
	// No OLM involvement means we can safely reclaim unknowns and reinstall missing.
	if cioCount > 0 && olmCount == 0 {
		return CRDManagedByCIO
	}

	// All found and all OLM — only if none are missing
	if foundCount == total && olmCount == total {
		return CRDManagedByOLM
	}

	// Pure unknown — no CIO, no OLM labels on any found CRD
	if unknownCount > 0 && cioCount == 0 && olmCount == 0 {
		return CRDUnknownManagement
	}

	// Anything else is mixed: CIO+OLM, OLM with missing, etc.
	return CRDMixedOwnership
}

// manageCRDs classifies target CRDs and installs/updates them if we own them (or none exist).
// Returns the aggregate state, per-CRD info, and any error.
func (l *Library) manageCRDs(ctx context.Context, values *v1.Values, includeAllCRDs bool) (CRDManagementState, []CRDInfo, string, error) {
	targets, err := targetCRDsFromValues(values, includeAllCRDs)
	if err != nil {
		return CRDNoneExist, nil, "", fmt.Errorf("failed to determine target CRDs: %w", err)
	}
	if len(targets) == 0 {
		return CRDNoneExist, nil, "no target CRDs configured", nil
	}

	state, infos := classifyCRDs(ctx, l.cl, targets)

	switch state {
	case CRDNoneExist:
		// Install all with CIO labels
		if err := l.installCRDs(ctx, targets); err != nil {
			return state, infos, "", fmt.Errorf("failed to install CRDs: %w", err)
		}
		// Update infos to reflect new state
		for idx := range infos {
			infos[idx].Found = true
			infos[idx].State = CRDManagedByCIO
		}
		return CRDManagedByCIO, infos, "CRDs installed by CIO", nil

	case CRDManagedByCIO:
		// Update existing, reinstall missing, re-label unknowns
		missing := missingCRDNames(infos)
		unlabeled := unlabeledCRDNames(infos)
		if err := l.updateCRDs(ctx, targets); err != nil {
			return state, infos, "", fmt.Errorf("failed to update CRDs: %w", err)
		}
		// Update infos for any previously-missing or unlabeled CRDs
		for idx := range infos {
			if !infos[idx].Found || infos[idx].State == CRDUnknownManagement {
				infos[idx].Found = true
				infos[idx].State = CRDManagedByCIO
			}
		}
		msg := "CRDs updated by CIO"
		if len(missing) > 0 {
			msg = fmt.Sprintf("CRDs updated by CIO; reinstalled: %s", strings.Join(missing, ", "))
		}
		if len(unlabeled) > 0 {
			msg += fmt.Sprintf("; reclaimed: %s", strings.Join(unlabeled, ", "))
		}
		return CRDManagedByCIO, infos, msg, nil

	case CRDManagedByOLM:
		return CRDManagedByOLM, infos, "CRDs managed by OSSM subscription via OLM", nil

	case CRDUnknownManagement:
		missing := missingCRDNames(infos)
		msg := "CRDs exist with unknown management"
		if len(missing) > 0 {
			msg += fmt.Sprintf("; missing from other owner: %s", strings.Join(missing, ", "))
		}
		return CRDUnknownManagement, infos, msg, fmt.Errorf("Istio CRDs are managed by an unknown party")

	case CRDMixedOwnership:
		missing := missingCRDNames(infos)
		msg := "CRDs have mixed ownership"
		if len(missing) > 0 {
			msg += fmt.Sprintf("; missing: %s", strings.Join(missing, ", "))
		}
		return CRDMixedOwnership, infos, msg, fmt.Errorf("Istio CRDs have mixed ownership (CIO/OLM/other)")

	default:
		return state, infos, "", nil
	}
}

// missingCRDNames returns the names of CRDs that were not found on the cluster.
func missingCRDNames(infos []CRDInfo) []string {
	var missing []string
	for _, info := range infos {
		if !info.Found {
			missing = append(missing, info.Name)
		}
	}
	return missing
}

// unlabeledCRDNames returns names of CRDs that exist but have unknown management (no CIO/OLM labels).
func unlabeledCRDNames(infos []CRDInfo) []string {
	var names []string
	for _, info := range infos {
		if info.Found && info.State == CRDUnknownManagement {
			names = append(names, info.Name)
		}
	}
	return names
}

// installCRDs installs all target CRDs with CIO ownership labels and Helm keep annotation.
func (l *Library) installCRDs(ctx context.Context, targets []string) error {
	for _, resource := range targets {
		crd, err := loadCRD(resource)
		if err != nil {
			return err
		}
		applyCIOLabels(crd)
		if err := l.cl.Create(ctx, crd); err != nil {
			return fmt.Errorf("failed to create CRD %s: %w", crd.Name, err)
		}
	}
	return nil
}

// updateCRDs updates existing CIO-owned CRDs and creates any missing ones.
func (l *Library) updateCRDs(ctx context.Context, targets []string) error {
	for _, resource := range targets {
		crd, err := loadCRD(resource)
		if err != nil {
			return err
		}
		applyCIOLabels(crd)

		existing := &apiextensionsv1.CustomResourceDefinition{}
		if err := l.cl.Get(ctx, client.ObjectKey{Name: crd.Name}, existing); err != nil {
			if apierrors.IsNotFound(err) {
				// CRD was deleted — reinstall it
				if err := l.cl.Create(ctx, crd); err != nil {
					return fmt.Errorf("failed to create CRD %s: %w", crd.Name, err)
				}
				continue
			}
			return fmt.Errorf("failed to get existing CRD %s: %w", crd.Name, err)
		}
		crd.ResourceVersion = existing.ResourceVersion
		if err := l.cl.Update(ctx, crd); err != nil {
			return fmt.Errorf("failed to update CRD %s: %w", crd.Name, err)
		}
	}
	return nil
}

// loadCRD reads and unmarshals a CRD from the embedded filesystem.
func loadCRD(resource string) (*apiextensionsv1.CustomResourceDefinition, error) {
	filename := resourceToCRDFilename(resource)
	if filename == "" {
		return nil, fmt.Errorf("invalid resource name: %s", resource)
	}

	data, err := fs.ReadFile(crds.FS, filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read CRD file %s: %w", filename, err)
	}

	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := yaml.Unmarshal(data, crd); err != nil {
		return nil, fmt.Errorf("failed to unmarshal CRD %s: %w", filename, err)
	}
	return crd, nil
}

// applyCIOLabels sets the CIO ownership label and Helm keep annotation on a CRD.
func applyCIOLabels(crd *apiextensionsv1.CustomResourceDefinition) {
	labels := crd.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[LabelManagedByCIO] = "true"
	crd.SetLabels(labels)

	annotations := crd.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[AnnotationHelmKeep] = "keep"
	crd.SetAnnotations(annotations)
}
