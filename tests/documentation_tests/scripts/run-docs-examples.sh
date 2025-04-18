#!/bin/bash

# Copyright Istio Authors

# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -eu -o pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"
TEST_DIR="$ROOT_DIR/tests/documentation_tests"

export KIND_CLUSTER_NAME="docs-automation"
export IP_FAMILY="ipv4"
export ISTIOCTL="${ROOT_DIR}/bin/istioctl"
export IMAGE_BASE="sail-operator"
export TAG="latest"
export LOCAL_REGISTRY="localhost:5000"
export OCP=false
export KIND_REGISTRY_NAME="kind-registry"
export KIND_REGISTRY_PORT="5000"
export KIND_REGISTRY="localhost:${KIND_REGISTRY_PORT}"
# Use the local registry instead of the default HUB
export HUB="${KIND_REGISTRY}"
# Workaround make inside make: ovewrite this variable so it is not recomputed in Makefile.core.mk
export IMAGE="${HUB}/${IMAGE_BASE}:${TAG}"
export ARTIFACTS="${ARTIFACTS:-$(mktemp -d)}"
export KUBECONFIG="${KUBECONFIG:-"${ARTIFACTS}/config"}"
export HELM_TEMPL_DEF_FLAGS="--include-crds --values chart/values.yaml"

# Validate that istioctl is installed
if ! command -v istioctl &> /dev/null; then
  echo "istioctl could not be found. Please install it."
  exit 1
fi

# Validate that kubectl is installed
if ! command -v kubectl &> /dev/null; then
  echo "kubectl could not be found. Please install it."
  exit 1
fi

# Discover .md files with bash tags
FILES_TO_CHECK=()
for file in "$TEST_DIR"/*.md; do
  if grep -q "bash { name=" "$file"; then
    FILES_TO_CHECK+=("$file")
  fi
done

# Build list of file-tag pairs
TAGS_LIST=()
for file in "${FILES_TO_CHECK[@]}"; do
  TAGS=$(grep -oP 'bash { name=[^ ]+ tag=\K[^}]+(?=})' "$file" | sort -u)
  for tag in $TAGS; do
    TAGS_LIST+=("$file -t $tag")
  done
done

echo "Tags list:"
for tag in "${TAGS_LIST[@]}"; do
  echo "$tag"
done

# Run each test in its own KIND cluster
for tag in "${TAGS_LIST[@]}"; do
  (
    echo "Setting up cluster for: $tag"

    # Source setup and build scripts to preserve trap and env
    source "${ROOT_DIR}/tests/e2e/setup/setup-kind.sh"

    # Build and push the operator image from source
    source "${ROOT_DIR}/tests/e2e/setup/build-and-push-operator.sh"
    build_and_push_operator_image

    # Ensure kubeconfig is set to the current cluster
    # TODO: check why KUBECONFIG is not properly set
    kind export kubeconfig --name="${KIND_CLUSTER_NAME}"

    # Deploy operator
    kubectl create ns sail-operator || echo "namespace sail-operator already exists"
    # shellcheck disable=SC2086
    helm template chart chart ${HELM_TEMPL_DEF_FLAGS} --set image="${IMAGE}" --namespace sail-operator | kubectl apply --server-side=true -f -
    kubectl wait --for=condition=available --timeout=600s deployment/sail-operator -n sail-operator

    # Run the actual doc test
    FILE=$(echo "$tag" | cut -d' ' -f1)
    TAG=$(echo "$tag" | cut -d' ' -f3-)

    echo "Running: runme run --filename $FILE -t $TAG --skip-prompts"
    runme run --filename "$FILE" -t "$TAG" --skip-prompts
  )
done
