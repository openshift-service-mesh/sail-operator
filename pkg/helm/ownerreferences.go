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

package helm

import (
	"bytes"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/postrender"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	AnnotationPrimaryResource     = "operator-sdk/primary-resource"
	AnnotationPrimaryResourceType = "operator-sdk/primary-resource-type"
)

// NewOwnerReferencePostRenderer creates a Helm PostRenderer that adds the
// specified OwnerReference to each rendered manifest
func NewOwnerReferencePostRenderer(ownerReference metav1.OwnerReference, ownerNamespace string) postrender.PostRenderer {
	return OwnerReferencePostRenderer{
		ownerReference: ownerReference,
		ownerNamespace: ownerNamespace,
	}
}

type OwnerReferencePostRenderer struct {
	ownerReference metav1.OwnerReference
	ownerNamespace string
}

var _ postrender.PostRenderer = OwnerReferencePostRenderer{}

func (pr OwnerReferencePostRenderer) Run(renderedManifests *bytes.Buffer) (modifiedManifests *bytes.Buffer, err error) {
	modifiedManifests = &bytes.Buffer{}
	encoder := yaml.NewEncoder(modifiedManifests)
	encoder.SetIndent(2)
	decoder := yaml.NewDecoder(renderedManifests)
	for {
		manifest := map[string]any{}

		if err := decoder.Decode(&manifest); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if manifest == nil {
			continue
		}

		manifest, err = pr.addOwnerReference(manifest)
		if err != nil {
			return nil, err
		}

		if err := encoder.Encode(manifest); err != nil {
			return nil, err
		}
	}
	return modifiedManifests, nil
}

func (pr OwnerReferencePostRenderer) addOwnerReference(manifest map[string]any) (map[string]any, error) {
	objNamespace, objHasNamespace, err := unstructured.NestedFieldNoCopy(manifest, "metadata", "namespace")
	if err != nil {
		return nil, err
	}
	if pr.ownerNamespace == "" || objHasNamespace && objNamespace == pr.ownerNamespace {
		ownerReferences, _, err := unstructured.NestedSlice(manifest, "metadata", "ownerReferences")
		if err != nil {
			return nil, err
		}

		ref, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&pr.ownerReference)
		if err != nil {
			return nil, err
		}
		ownerReferences = append(ownerReferences, ref)

		if err := unstructured.SetNestedSlice(manifest, ownerReferences, "metadata", "ownerReferences"); err != nil {
			return nil, err
		}
	} else {
		ownerAPIGroup, _, _ := strings.Cut(pr.ownerReference.APIVersion, "/")
		ownerType := pr.ownerReference.Kind + "." + ownerAPIGroup
		ownerKey := pr.ownerNamespace + "/" + pr.ownerReference.Name
		if err := unstructured.SetNestedField(manifest, ownerType, "metadata", "annotations", AnnotationPrimaryResourceType); err != nil {
			return nil, err
		}
		if err := unstructured.SetNestedField(manifest, ownerKey, "metadata", "annotations", AnnotationPrimaryResource); err != nil {
			return nil, err
		}
	}
	return manifest, nil
}
