#!/bin/bash

# Copyright Istio Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -eux -o pipefail

function check_prerequisites() {
  if command -v oc &> /dev/null; then
    COMMAND="oc"
    echo "Using OpenShift CLI (oc)"
  elif command -v kubectl &> /dev/null; then
    COMMAND="kubectl"
    echo "Using Kubernetes CLI (kubectl)"
  else
    echo "Error: Neither 'oc' nor 'kubectl' is available. Please install one of them."
    exit 1
  fi
}

function cleanup_sail_crds() {
  echo "Cleaning up Sail operator CRDs..."
  if ${COMMAND} get crds -o name | grep ".*\.sail" > /dev/null 2>&1; then
    ${COMMAND} get crds -o name | grep ".*\.sail" | xargs -r -n 1 ${COMMAND} delete
    echo "Sail CRDs cleaned up successfully"
  else
    echo "No Sail CRDs found to clean up"
  fi
}

function cleanup_istio_crds() {
  echo "Cleaning up Istio CRDs..."
  if ${COMMAND} get crds -o name | grep ".*\.istio" > /dev/null 2>&1; then
    ${COMMAND} get crds -o name | grep ".*\.istio" | xargs -r -n 1 ${COMMAND} delete
    echo "Istio CRDs cleaned up successfully"
  else
    echo "No Istio CRDs found to clean up"
  fi
}

function main() {
  echo "Starting e2e environment cleanup..."
  
  check_prerequisites
  cleanup_sail_crds
  cleanup_istio_crds
  
  echo "E2E environment cleanup completed successfully"
}

main "$@"