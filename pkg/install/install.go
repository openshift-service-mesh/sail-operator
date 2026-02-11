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

// Package install provides a simplified API for one-shot istiod installation.
// It is designed for embedding in other operators (like OpenShift Ingress)
// that need to install Istio without running a continuous controller.
package install

import (
	"context"
	"fmt"
	"io/fs"
	"os"

	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	"github.com/istio-ecosystem/sail-operator/pkg/helm"
	sharedreconcile "github.com/istio-ecosystem/sail-operator/pkg/reconcile"
	"github.com/istio-ecosystem/sail-operator/pkg/revision"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	defaultNamespace  = "istio-system"
	defaultProfile    = "openshift"
	defaultHelmDriver = "secret"
	defaultRevision   = v1.DefaultRevision
)

// Status represents the result of an Install operation, covering both
// CRD management and Helm installation.
type Status struct {
	// CRDState is the aggregate ownership state of the target Istio CRDs.
	CRDState CRDManagementState

	// CRDMessage is a human-readable description of the CRD state.
	CRDMessage string

	// CRDs contains per-CRD detail (name, ownership, found on cluster).
	CRDs []CRDInfo

	// Installed is true if the Helm install/upgrade completed successfully.
	Installed bool

	// Version is the resolved Istio version (set even if Installed is false).
	Version string

	// Error is non-nil if something went wrong during CRD management or Helm installation.
	// CRD ownership problems (UnknownManagement, MixedOwnership) set this but do not
	// prevent Helm installation from being attempted.
	Error error
}

// Options for installing istiod
type Options struct {
	// Namespace is the target namespace for installation.
	// Defaults to "istio-system" if not specified.
	Namespace string

	// Version is the Istio version to install.
	// Defaults to the latest supported version if not specified.
	Version string

	// Revision is the Istio revision name.
	// Defaults to "default" if not specified.
	Revision string

	// Values are Helm value overrides.
	// Use GatewayAPIDefaults() to get pre-configured values for Gateway API mode,
	// then modify as needed before passing here.
	Values *v1.Values

	// OwnerRef is an optional owner reference for garbage collection.
	// If set, installed resources will be owned by this resource.
	OwnerRef *metav1.OwnerReference

	// ManageCRDs controls whether the installer manages Istio CRDs.
	// When true (default), CRDs are classified by ownership and installed/updated
	// if we own them or none exist.
	// Set to false to skip CRD management entirely.
	ManageCRDs *bool

	// IncludeAllCRDs controls which CRDs are managed.
	// When true, all *.istio.io CRDs from the embedded FS are managed.
	// When false (default), only CRDs matching PILOT_INCLUDE_RESOURCES are managed.
	IncludeAllCRDs *bool
}

// applyDefaults fills in default values for Options.
// Version is not defaulted here — it requires access to the resource FS,
// so it is resolved in Install() via DefaultVersion().
func (o *Options) applyDefaults() {
	if o.Namespace == "" {
		o.Namespace = defaultNamespace
	}
	if o.Revision == "" {
		o.Revision = defaultRevision
	}
	if o.ManageCRDs == nil {
		o.ManageCRDs = ptr.To(true)
	}
	if o.IncludeAllCRDs == nil {
		o.IncludeAllCRDs = ptr.To(false)
	}
}

// Installer provides one-shot istiod installation for OpenShift.
type Installer struct {
	chartManager *helm.ChartManager
	resourceFS   fs.FS
	kubeConfig   *rest.Config
	client       client.Client // for CRD operations
}

// NewInstaller creates a new Installer.
//
// Parameters:
//   - kubeConfig: Kubernetes client configuration (required)
//   - resourceFS: Filesystem containing Helm charts and profiles (required).
//     Use resources.FS for embedded resources or FromDirectory() for a filesystem path.
func NewInstaller(kubeConfig *rest.Config, resourceFS fs.FS) (*Installer, error) {
	if kubeConfig == nil {
		return nil, fmt.Errorf("kubeConfig is required")
	}
	if resourceFS == nil {
		return nil, fmt.Errorf("resourceFS is required")
	}

	cl, err := client.New(kubeConfig, client.Options{})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	// Populate default image refs from the provided FS (no-op if already set by config.Read)
	if err := SetImageDefaults(resourceFS, defaultRegistry, ImageNames{
		Istiod:  defaultIstiodImage,
		Proxy:   defaultProxyImage,
		CNI:     defaultCNIImage,
		ZTunnel: defaultZTunnelImage,
	}); err != nil {
		return nil, fmt.Errorf("failed to set image defaults: %w", err)
	}

	return &Installer{
		chartManager: helm.NewChartManager(kubeConfig, defaultHelmDriver),
		resourceFS:   resourceFS,
		kubeConfig:   kubeConfig,
		client:       cl,
	}, nil
}

// Install installs or upgrades istiod with the specified options.
//
// This method:
//   - Resolves the Istio version
//   - Classifies and manages CRDs (if ManageCRDs is true)
//   - Computes final values (digests, vendor defaults, profiles, FIPS, overrides)
//   - Installs both the base chart and istiod chart via Helm
//   - Returns a DriftReconciler for optional drift detection and a Status
//
// CRD ownership problems are reported in Status.Error but do not prevent
// the Helm installation from being attempted.
//
// For Gateway API mode, use GatewayAPIDefaults() to get pre-configured values:
//
//	values := install.GatewayAPIDefaults()
//	values.Pilot.Env["PILOT_GATEWAY_API_CONTROLLER_NAME"] = "my-controller"
//	reconciler, status := installer.Install(ctx, Options{Values: values})
//	if status.Error != nil { /* handle */ }
//	reconciler.Start(ctx)  // Optional: start drift detection
func (i *Installer) Install(ctx context.Context, opts Options) (*DriftReconciler, Status) {
	status := i.doInstall(ctx, opts)

	if !status.Installed {
		return nil, status
	}

	// Create drift reconciler with the same options used for installation
	driftReconciler, err := newDriftReconciler(i, opts)
	if err != nil {
		status.Error = combineErrors(status.Error, fmt.Errorf("failed to create drift reconciler: %w", err))
		return nil, status
	}

	return driftReconciler, status
}

// doInstall is the internal implementation that does CRDs + Helm.
// Used by both Install() (public) and the drift reconciler (internal).
func (i *Installer) doInstall(ctx context.Context, opts Options) Status {
	opts.applyDefaults()

	status := Status{}

	// Default version from FS if not specified
	if opts.Version == "" {
		v, err := DefaultVersion(i.resourceFS)
		if err != nil {
			status.Error = fmt.Errorf("failed to determine default version: %w", err)
			return status
		}
		opts.Version = v
	}

	// Validate version directory exists in the resource FS
	if err := ValidateVersion(i.resourceFS, opts.Version); err != nil {
		status.Error = fmt.Errorf("invalid version %q: %w", opts.Version, err)
		return status
	}
	status.Version = opts.Version

	// Compute final values using the same pipeline as the Operator:
	// - applies image digests from configuration
	// - applies vendor-specific default values
	// - applies profiles and platform defaults
	// - applies FIPS values
	// - applies overrides (namespace, revision)
	values, err := revision.ComputeValues(
		opts.Values,
		opts.Namespace,
		opts.Version,
		config.PlatformOpenShift,
		defaultProfile, // "openshift" YAML profile
		"",             // no user profile for library
		i.resourceFS,
		opts.Revision,
	)
	if err != nil {
		status.Error = fmt.Errorf("failed to compute values: %w", err)
		return status
	}

	// Manage CRDs (classify ownership, install/update if appropriate)
	if ptr.Deref(opts.ManageCRDs, true) {
		crdState, crdInfos, crdMsg, crdErr := i.manageCRDs(ctx, values, ptr.Deref(opts.IncludeAllCRDs, false))
		status.CRDState = crdState
		status.CRDs = crdInfos
		status.CRDMessage = crdMsg
		if crdErr != nil {
			// CRD errors are recorded but don't prevent Helm installation
			status.Error = crdErr
		}
	}

	// Create reconciler config
	reconcilerCfg := sharedreconcile.Config{
		ResourceFS:        i.resourceFS,
		Platform:          config.PlatformOpenShift,
		DefaultProfile:    defaultProfile, // always "openshift" YAML profile
		OperatorNamespace: opts.Namespace, // base chart goes to same namespace
		ChartManager:      i.chartManager,
	}

	// Create IstiodReconciler and install
	istiodReconciler := sharedreconcile.NewIstiodReconciler(reconcilerCfg, i.client)

	// Validate (version, namespace, values, target namespace exists)
	if err := istiodReconciler.Validate(ctx, opts.Version, opts.Namespace, values); err != nil {
		status.Error = combineErrors(status.Error, fmt.Errorf("validation failed: %w", err))
		return status
	}

	// Install
	if err := istiodReconciler.Install(ctx, opts.Version, opts.Namespace, values, opts.Revision, opts.OwnerRef); err != nil {
		status.Error = combineErrors(status.Error, fmt.Errorf("installation failed: %w", err))
		return status
	}

	status.Installed = true
	return status
}

// combineErrors returns the first non-nil error, or wraps both if both are non-nil.
func combineErrors(existing, new error) error {
	if existing == nil {
		return new
	}
	if new == nil {
		return existing
	}
	return fmt.Errorf("%w; %w", existing, new)
}

// Uninstall removes the istiod installation from the specified namespace.
// If revision is empty, it defaults to "default".
func (i *Installer) Uninstall(ctx context.Context, namespace, revision string) error {
	if namespace == "" {
		namespace = defaultNamespace
	}
	if revision == "" {
		revision = defaultRevision
	}

	// Create reconciler config
	reconcilerCfg := sharedreconcile.Config{
		ResourceFS:        i.resourceFS,
		Platform:          config.PlatformOpenShift,
		DefaultProfile:    defaultProfile,
		OperatorNamespace: namespace,
		ChartManager:      i.chartManager,
	}

	// Create IstiodReconciler and uninstall
	istiodReconciler := sharedreconcile.NewIstiodReconciler(reconcilerCfg, i.client)

	if err := istiodReconciler.Uninstall(ctx, namespace, revision); err != nil {
		return fmt.Errorf("uninstallation failed: %w", err)
	}

	return nil
}

// FromDirectory creates an fs.FS from a filesystem directory path.
// This is a convenience function for consumers who want to load resources
// from the filesystem instead of using embedded resources.
//
// Example:
//
//	installer, _ := install.NewInstaller(kubeConfig, install.FromDirectory("/var/lib/sail-operator/resources"))
func FromDirectory(path string) fs.FS {
	return os.DirFS(path)
}
