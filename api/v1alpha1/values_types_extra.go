// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	k8sv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type SDSConfigToken struct {
	Aud string `json:"aud,omitempty"`
}

type CNIValues struct {
	// Configuration for the Istio CNI plugin.
	Cni *CNIConfig `json:"cni,omitempty"`

	// Part of the global configuration applicable to the Istio CNI component.
	Global *CNIGlobalConfig `json:"global,omitempty"`
}

// CNIGlobalConfig is a subset of the Global Configuration used in the Istio CNI chart.
type CNIGlobalConfig struct {
	// Default k8s resources settings for all Istio control plane components.
	//
	// See https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#resource-requests-and-limits-of-pod-and-container
	DefaultResources *k8sv1.ResourceRequirements `json:"defaultResources,omitempty"`
	// Specifies the docker hub for Istio images.
	Hub string `json:"hub,omitempty"`
	// Specifies the image pull policy for the Istio images. one of Always, Never, IfNotPresent.
	// Defaults to Always if :latest tag is specified, or IfNotPresent otherwise. Cannot be updated.
	//
	// More info: https://kubernetes.io/docs/concepts/containers/images#updating-images
	ImagePullPolicy  k8sv1.PullPolicy `json:"imagePullPolicy,omitempty"`
	ImagePullSecrets []string         `json:"imagePullSecrets,omitempty"`
	LogAsJSON        *bool            `json:"logAsJson,omitempty"`
	// Specifies the global logging level settings for the Istio CNI component.
	Logging *GlobalLoggingConfig `json:"logging,omitempty"`
	// Specifies the tag for the Istio CNI image.
	// +kubebuilder:validation:XIntOrString
	Tag     *intstr.IntOrString `json:"tag,omitempty"`
	Variant string              `json:"variant,omitempty"`
}
