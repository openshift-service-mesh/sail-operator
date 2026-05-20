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

# To be able to run this script, it's needed to pass the flag --ocp or --kind
set -eu -o pipefail

WD=$(dirname "$0")
WD=$(cd "${WD}" || exit; pwd)

check_arguments() {
  if [ $# -eq 0 ]; then
    echo "No arguments provided"
    exit 1
  fi
}

parse_flags() {
  SKIP_BUILD=${SKIP_BUILD:-false}
  SKIP_DEPLOY=${SKIP_DEPLOY:-false}
  SKIP_CLEANUP=${SKIP_CLEANUP:-false}
  OLM=${OLM:-false}
  DESCRIBE=false
  MULTICLUSTER=${MULTICLUSTER:-false}
  while [ $# -gt 0 ]; do
    case "$1" in
      --ocp)
        shift
        OCP=true
        ;;
      --kind)
        shift
        OCP=false
        ;;
      --multicluster)
        shift
        MULTICLUSTER=true
        ;;
      --skip-build)
        shift
        SKIP_BUILD=true
        ;;
      --skip-deploy)
        shift
        # no point building if we don't deploy
        SKIP_BUILD=true
        SKIP_DEPLOY=true
        ;;
      --olm)
        shift
        OLM=true
        ;;
      --describe)
        shift
        DESCRIBE=true
        ;;
      *)
        echo "Invalid flag: $1"
        exit 1
        ;;
    esac
  done

  if [ "${DESCRIBE}" == "true" ]; then
    WD=$(dirname "$0")
    while IFS= read -r -d '' file; do
      if [[ $file == *"_test.go" ]]; then
        go run github.com/onsi/ginkgo/v2/ginkgo outline -format indent "${file}"
      fi
    done < <(find "${WD}" -type f -name "*_test.go" -print0)
    exit 0
  fi

  if [ "${OCP}" == "true" ]; then
    echo "Running on OCP"
  else
    echo "Running on kind"
  fi

  if [ "${MULTICLUSTER}" == "true" ]; then
    echo "Running on multicluster"
  fi

  if [ "${SKIP_BUILD}" == "true" ]; then
    echo "Skipping build"
  fi

  if [ "${SKIP_DEPLOY}" == "true" ]; then
    echo "Skipping deploy"
  fi

  if [ "${OLM}" == "true" ]; then
    echo "OLM deployment enabled"
  fi
}

initialize_variables() {
  VERSIONS_YAML_FILE=${VERSIONS_YAML_FILE:-"versions.yaml"}
  VERSIONS_YAML_DIR=${VERSIONS_YAML_DIR:-"pkg/istioversions"}
  NAMESPACE="${NAMESPACE:-sail-operator}"
  DEPLOYMENT_NAME="${DEPLOYMENT_NAME:-sail-operator}"
  CONTROL_PLANE_NS="${CONTROL_PLANE_NS:-istio-system}"
  COMMAND="kubectl"
  ARTIFACTS="${ARTIFACTS:-$(mktemp -d)}"
  KUBECONFIG="${KUBECONFIG:-"${ARTIFACTS}/config"}"
  ISTIOCTL_PATH="${ISTIOCTL:-"istioctl"}"
  LOCALBIN="${LOCALBIN:-${HOME}/bin}"
  OPERATOR_SDK=${LOCALBIN}/operator-sdk
  IP_FAMILY=${IP_FAMILY:-ipv4}
  ISTIO_MANIFEST="chart/samples/istio-sample.yaml"
  CI=${CI:-"false"}
  USE_INTERNAL_REGISTRY=${USE_INTERNAL_REGISTRY:-"false"}
  FIPS_CLUSTER=${FIPS_CLUSTER:-"false"}
  # In Prow CI, HEAD is a temporary merge commit not visible in the PR history.
  # PULL_PULL_SHA holds the actual PR head commit — use it when available.
  if [ -n "${PULL_PULL_SHA:-}" ]; then
    COMMIT_HASH="${PULL_PULL_SHA:0:8}"
  else
    COMMIT_HASH=$(git rev-parse --short HEAD)
  fi
  echo "DEBUG commit hash resolution:"
  echo "  PULL_PULL_SHA: ${PULL_PULL_SHA:-not set}"
  echo "  git HEAD (full): $(git rev-parse HEAD)"
  echo "  COMMIT_HASH resolved to: ${COMMIT_HASH}"

  # Debug logging and fallback for GINKGO_FLAGS
  echo "CI environment: ${CI}"
  echo "GINKGO_FLAGS received: '${GINKGO_FLAGS:-}'"

  # Fallback: Generate GINKGO_FLAGS if empty and CI=true
  if [ -z "${GINKGO_FLAGS:-}" ] && [ "${CI}" == "true" ]; then
    GINKGO_FLAGS="--no-color"
    echo "Generated GINKGO_FLAGS fallback: '${GINKGO_FLAGS}'"
  fi

  # export to be sure that the variables are available in the subshell
  export IMAGE_BASE="${IMAGE_BASE:-sail-operator}"
  export TAG="${TAG:-latest}"
  export HUB="${HUB:-localhost:5000}"

  # Handle OCP registry scenarios
  # Note: Makefile.core.mk sets HUB=quay.io/sail-dev and TAG=1.29-latest by default
  if [ "${OCP}" == "true" ]; then
    # Debug output for troubleshooting
    echo "DEBUG: CI='${CI}', HUB='${HUB}'"

    if [ "${CI}" == "true" ] && [ "${HUB}" == "quay.io/sail-dev" ]; then
      # Scenario 2: CI mode with default HUB -> use external registry with proper CI tag
      echo "CI mode detected for OCP, using external registry ${HUB}"
      export USE_INTERNAL_REGISTRY="false"
      # Use PR_NUMBER and commit hash to identify the image, avoid race conditions in CI when multiple runs are pushing to the same default tag
      # Use TARGET_ARCH to differentiate tags for different architectures in CI
      if [ -n "${PR_NUMBER:-}" ]; then
        TAG="pr-${PR_NUMBER}-${COMMIT_HASH}-${TARGET_ARCH}"
        export TAG
        echo "Using PR-based tag: ${TAG}"
      else
        TAG="ci-test-${COMMIT_HASH}-${TARGET_ARCH}"
        export TAG
        echo "Using commit-based tag: ${TAG}"
      fi
    elif [ "${CI}" == "true" ]; then
      # Additional CI mode check - handle CI mode regardless of HUB value
      echo "CI mode detected for OCP with custom HUB (${HUB}), using external registry"
      export USE_INTERNAL_REGISTRY="false"
    elif [ "${HUB}" != "quay.io/sail-dev" ]; then
      # Scenario 3: Custom registry provided by user
      echo "Using custom registry: ${HUB}"
      export USE_INTERNAL_REGISTRY="false"
    else
      # Scenario 1: Local development -> use internal OCP registry
      echo "Local development mode, will use OCP internal registry"
      export USE_INTERNAL_REGISTRY="true"
    fi
  fi

  echo "Setting Istio manifest file: ${ISTIO_MANIFEST}"
  ISTIO_NAME=$(yq eval '.metadata.name' "${WD}/../../$ISTIO_MANIFEST")

  if [ "${OCP}" == "true" ]; then COMMAND="oc"; fi
}

log_cluster_state() {
  # Log detailed cluster state for debugging MachineConfig reconciliation issues
  if [ "${OCP}" != "true" ]; then
    return 0
  fi

  echo "=== CLUSTER STATE SNAPSHOT ($(date -u +%Y-%m-%dT%H:%M:%SZ)) ==="

  echo "--- Cluster Operators Status ---"
  oc get clusteroperator -o json | jq -r '
    .items[] |
    select(.status.conditions[] |
      (.type == "Progressing" and .status == "True") or
      (.type == "Degraded" and .status == "True") or
      (.type == "Available" and .status == "False")
    ) |
    "\(.metadata.name): " +
    ([.status.conditions[] | select(.type == "Progressing" or .type == "Degraded" or .type == "Available") | "\(.type)=\(.status)"] | join(", "))
  ' 2>/dev/null || echo "All operators stable or unable to query"

  echo "--- MachineConfigPools Status ---"
  oc get mcp -o json | jq -r '
    .items[] |
    "\(.metadata.name): Updated=\(.status.conditions[] | select(.type=="Updated") | .status), Updating=\(.status.conditions[] | select(.type=="Updating") | .status), machineCount=\(.status.machineCount), readyMachineCount=\(.status.readyMachineCount), updatedMachineCount=\(.status.updatedMachineCount)"
  ' 2>/dev/null || echo "Unable to query MachineConfigPools"

  echo "--- MachineConfigs (sorted by creation time, last 10) ---"
  oc get machineconfig --sort-by='.metadata.creationTimestamp' -o json | jq -r '
    .items[-10:] | .[] |
    "\(.metadata.creationTimestamp) \(.metadata.name)"
  ' 2>/dev/null || echo "Unable to query MachineConfigs"

  echo "--- Node MachineConfig Annotations ---"
  oc get nodes -o json | jq -r '
    .items[] |
    "\(.metadata.name): currentConfig=\(.metadata.annotations["machineconfiguration.openshift.io/currentConfig"] // "N/A"), desiredConfig=\(.metadata.annotations["machineconfiguration.openshift.io/desiredConfig"] // "N/A"), state=\(.metadata.annotations["machineconfiguration.openshift.io/state"] // "N/A")"
  ' 2>/dev/null || echo "Unable to query node annotations"

  echo "--- Image Config (registrySources) ---"
  oc get image.config.openshift.io/cluster -o json | jq '.spec.registrySources // "not configured"' 2>/dev/null || echo "Unable to query image config"

  echo "=== END CLUSTER STATE SNAPSHOT ==="
}

# Global variable for background monitoring PID
MC_MONITOR_PID=""

# Captures current MachineConfig state as a baseline (logged to stdout for CI capture)
capture_machineconfig_baseline() {
  if [ "${OCP}" != "true" ]; then
    return 0
  fi

  # Check if MachineConfig API exists (not available on HyperShift hosted clusters)
  if ! oc api-resources --api-group=machineconfiguration.openshift.io 2>/dev/null | grep -q machineconfigs; then
    echo "[MC-MONITOR] MachineConfig API not available (likely HyperShift hosted cluster) - skipping monitoring"
    export MC_MONITOR_DISABLED=true
    return 0
  fi

  echo "[MC-MONITOR] Capturing MachineConfig baseline at $(date -u +%Y-%m-%dT%H:%M:%SZ)..."

  # Create directory for MachineConfig dumps
  MC_DUMP_DIR="${ARTIFACTS:-/tmp}/machineconfig-dumps"
  mkdir -p "${MC_DUMP_DIR}"
  echo "[MC-MONITOR] MachineConfig dumps will be saved to: ${MC_DUMP_DIR}"

  # Dump all MachineConfigs at baseline
  echo "[MC-MONITOR] Saving baseline MachineConfig content..."
  oc get machineconfig -o json | jq -c '.items[]' 2>/dev/null | while read -r mc_json; do
    mc_name=$(echo "$mc_json" | jq -r '.metadata.name')
    echo "$mc_json" | jq '.' > "${MC_DUMP_DIR}/${mc_name}.baseline.json" 2>/dev/null
  done
  echo "[MC-MONITOR] Baseline MachineConfigs saved to ${MC_DUMP_DIR}/*.baseline.json"

  # Store rendered MachineConfig hashes for each pool
  echo "[MC-MONITOR] MCP configurations:"
  oc get mcp -o json | jq -r '
    .items[] | "[MC-MONITOR]   \(.metadata.name): \(.spec.configuration.name // "none")"
  ' 2>/dev/null || echo "[MC-MONITOR]   Unable to capture MCP baseline"

  # Store node desiredConfig values
  echo "[MC-MONITOR] Node configurations:"
  oc get nodes -o json | jq -r '
    .items[] | "[MC-MONITOR]   \(.metadata.name): \(.metadata.annotations["machineconfiguration.openshift.io/desiredConfig"] // "N/A")"
  ' 2>/dev/null || echo "[MC-MONITOR]   Unable to capture node baseline"

  # Store list of MachineConfigs with their creation timestamps and generation
  echo "[MC-MONITOR] All MachineConfigs (with generation):"
  oc get machineconfig -o json | jq -r '
    .items[] | "[MC-MONITOR]   \(.metadata.creationTimestamp) gen=\(.metadata.generation) \(.metadata.name)"
  ' 2>/dev/null || echo "[MC-MONITOR]   Unable to capture MC list"

  # Capture cluster operator versions - helps identify if cluster is mid-upgrade
  echo "[MC-MONITOR] Cluster operator versions:"
  oc get clusteroperator -o json | jq -r '
    .items[] | select(.metadata.name == "machine-config" or .metadata.name == "etcd" or .metadata.name == "kube-apiserver") |
    "[MC-MONITOR]   \(.metadata.name): version=\(.status.versions[0].version // "unknown")"
  ' 2>/dev/null || echo "[MC-MONITOR]   Unable to capture CO versions"

  # Check if cluster version operator is progressing
  echo "[MC-MONITOR] ClusterVersion state:"
  oc get clusterversion version -o json | jq -r '
    "[MC-MONITOR]   version: \(.status.desired.version)",
    "[MC-MONITOR]   history: \([.status.history[0:2][] | "\(.version) \(.state)"] | join(", "))",
    "[MC-MONITOR]   conditions: \([.status.conditions[] | select(.status == "True") | .type] | join(", "))"
  ' 2>/dev/null || echo "[MC-MONITOR]   Unable to capture ClusterVersion"

  # Capture source CRs that can trigger MachineConfig changes
  echo "[MC-MONITOR] Source CRs that trigger MachineConfig changes:"

  echo "[MC-MONITOR]   KubeletConfigs:"
  oc get kubeletconfig -o json 2>/dev/null | jq -r '
    .items[] | "[MC-MONITOR]     \(.metadata.name) gen=\(.metadata.generation) created=\(.metadata.creationTimestamp)"
  ' 2>/dev/null || echo "[MC-MONITOR]     (none or unable to list)"

  echo "[MC-MONITOR]   ContainerRuntimeConfigs:"
  oc get containerruntimeconfig -o json 2>/dev/null | jq -r '
    .items[] | "[MC-MONITOR]     \(.metadata.name) gen=\(.metadata.generation) created=\(.metadata.creationTimestamp)"
  ' 2>/dev/null || echo "[MC-MONITOR]     (none or unable to list)"

  echo "[MC-MONITOR]   ImageContentSourcePolicies:"
  oc get imagecontentsourcepolicy -o json 2>/dev/null | jq -r '
    .items[] | "[MC-MONITOR]     \(.metadata.name) gen=\(.metadata.generation)"
  ' 2>/dev/null || echo "[MC-MONITOR]     (none or unable to list)"

  echo "[MC-MONITOR]   ImageDigestMirrorSets:"
  oc get imagedigestmirrorset -o json 2>/dev/null | jq -r '
    .items[] | "[MC-MONITOR]     \(.metadata.name) gen=\(.metadata.generation)"
  ' 2>/dev/null || echo "[MC-MONITOR]     (none or unable to list)"

  echo "[MC-MONITOR]   ImageTagMirrorSets:"
  oc get imagetagmirrorset -o json 2>/dev/null | jq -r '
    .items[] | "[MC-MONITOR]     \(.metadata.name) gen=\(.metadata.generation)"
  ' 2>/dev/null || echo "[MC-MONITOR]     (none or unable to list)"

  # ControllerConfig is the key resource that triggers kubelet MachineConfig changes
  echo "[MC-MONITOR]   ControllerConfig (owns kubelet MachineConfigs):"
  oc get controllerconfig -o json 2>/dev/null | jq -r '
    .items[] | "[MC-MONITOR]     \(.metadata.name) gen=\(.metadata.generation) resourceVersion=\(.metadata.resourceVersion)"
  ' 2>/dev/null || echo "[MC-MONITOR]     (none or unable to list)"

  # Save baseline ControllerConfig for comparison
  oc get controllerconfig machine-config-controller -o json > "${MC_DUMP_DIR}/controllerconfig-baseline.json" 2>/dev/null || true
  echo "[MC-MONITOR]   Baseline ControllerConfig saved to ${MC_DUMP_DIR}/controllerconfig-baseline.json"

  # Show ALL certificate details at baseline
  echo "[MC-MONITOR]   === BASELINE CERTIFICATES ==="
  oc get controllerconfig machine-config-controller -o json 2>/dev/null | jq -r '
    "[MC-MONITOR]   Certificate count: \(.status.controllerCertificates | length // 0)",
    (.status.controllerCertificates[]? |
      "[MC-MONITOR]     [\(.bundleFile // "unknown")]",
      "[MC-MONITOR]       signer: \(.signer)",
      "[MC-MONITOR]       subject: \(.subject // "N/A")",
      "[MC-MONITOR]       notBefore: \(.notBefore)",
      "[MC-MONITOR]       notAfter: \(.notAfter)"
    )
  ' 2>/dev/null || echo "[MC-MONITOR]     (unable to get certificate info)"

  # Show spec field hashes at baseline
  echo "[MC-MONITOR]   === BASELINE SPEC FIELDS ==="
  oc get controllerconfig machine-config-controller -o json 2>/dev/null | jq -r '
    "[MC-MONITOR]     rootCAData (first 40 chars): \(.spec.rootCAData | .[0:40] // "N/A")",
    "[MC-MONITOR]     kubeAPIServerServingCAData (first 40 chars): \(.spec.kubeAPIServerServingCAData | .[0:40] // "N/A")",
    "[MC-MONITOR]     cloudProviderCAData present: \(if .spec.cloudProviderCAData then "yes" else "no" end)",
    "[MC-MONITOR]     additionalTrustBundle present: \(if .spec.additionalTrustBundle then "yes (\(.spec.additionalTrustBundle | length) bytes)" else "no" end)",
    "[MC-MONITOR]     releaseImage: \(.spec.releaseImage // "N/A")"
  ' 2>/dev/null || echo "[MC-MONITOR]     (unable to get spec fields)"
}

# Background monitoring function that logs MachineConfig changes during test execution
# Writes directly to stdout so output is captured in CI logs even if pod is killed
monitor_machineconfig_changes() {
  local interval="${1:-120}"  # Default: check every 2 minutes

  echo "[MC-MONITOR] === Started at $(date -u +%Y-%m-%dT%H:%M:%SZ) ==="
  echo "[MC-MONITOR] Monitoring interval: ${interval} seconds"

  # Capture initial state (including generation for detecting modifications)
  local prev_mc_list
  prev_mc_list=$(oc get machineconfig -o json | jq -r '[.items[] | {name: .metadata.name, created: .metadata.creationTimestamp, gen: .metadata.generation}] | sort_by(.created)' 2>/dev/null)
  local prev_node_configs
  prev_node_configs=$(oc get nodes -o json | jq -r '[.items[] | {name: .metadata.name, desired: .metadata.annotations["machineconfiguration.openshift.io/desiredConfig"], current: .metadata.annotations["machineconfiguration.openshift.io/currentConfig"]}]' 2>/dev/null)

  echo "[MC-MONITOR] Initial MachineConfig count: $(echo "$prev_mc_list" | jq 'length')"
  echo "[MC-MONITOR] Initial node configs:"
  echo "$prev_node_configs" | jq -r '.[] | "[MC-MONITOR]   \(.name): current=\(.current), desired=\(.desired)"'

  # Capture initial image.config for change detection
  PREV_IMAGE_CONFIG=$(oc get image.config.openshift.io/cluster -o json | jq -c '.spec // {}' 2>/dev/null)
  echo "[MC-MONITOR] Initial image.config: $PREV_IMAGE_CONFIG"

  # Log initial cluster operator state
  echo "[MC-MONITOR] Initial cluster operator state:"
  oc get clusteroperator -o json | jq -r '
    .items[] | select(.status.conditions[] | .type == "Progressing" and .status == "True") |
    "[MC-MONITOR]   PROGRESSING: \(.metadata.name)"
  ' 2>/dev/null || echo "[MC-MONITOR]   All operators stable"

  # Capture initial source CR state for change detection
  local prev_kubeletconfigs prev_containerruntimeconfigs prev_controllerconfig
  prev_kubeletconfigs=$(oc get kubeletconfig -o json 2>/dev/null | jq -c '[.items[] | {name: .metadata.name, gen: .metadata.generation}]' 2>/dev/null || echo "[]")
  prev_containerruntimeconfigs=$(oc get containerruntimeconfig -o json 2>/dev/null | jq -c '[.items[] | {name: .metadata.name, gen: .metadata.generation}]' 2>/dev/null || echo "[]")
  prev_controllerconfig=$(oc get controllerconfig -o json 2>/dev/null | jq -c '[.items[] | {name: .metadata.name, gen: .metadata.generation, rv: .metadata.resourceVersion}]' 2>/dev/null || echo "[]")

  local fast_poll_count=0
  local first_iteration=true
  local dump_counter=0

  while true; do
    # Sleep at the start, but skip on first iteration and use fast poll interval when active
    if [ "$first_iteration" = "true" ]; then
      first_iteration=false
      sleep 5  # Short initial delay
    elif [ "$fast_poll_count" -gt 0 ]; then
      echo "[MC-MONITOR]   Fast polling active (${fast_poll_count} iterations remaining)"
      fast_poll_count=$((fast_poll_count - 1))
      sleep 15
    else
      sleep "${interval}"
    fi

    local timestamp
    timestamp=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    echo "[MC-MONITOR] --- Check at ${timestamp} ---"

    # Check for new MachineConfigs (including generation for modification detection)
    local current_mc_list
    current_mc_list=$(oc get machineconfig -o json | jq -r '[.items[] | {name: .metadata.name, created: .metadata.creationTimestamp, gen: .metadata.generation}] | sort_by(.created)' 2>/dev/null)

    # Check for NEW MachineConfigs
    local new_mcs
    new_mcs=$(jq -rn --argjson prev "$prev_mc_list" --argjson curr "$current_mc_list" '
      ($curr | map(.name)) - ($prev | map(.name)) | .[]
    ' 2>/dev/null)

    if [ -n "$new_mcs" ]; then
      echo "[MC-MONITOR] *** NEW MACHINECONFIGS DETECTED ***"
      echo "$new_mcs" | while read -r mc_name; do
        if [ -n "$mc_name" ]; then
          echo "[MC-MONITOR]   - $mc_name"
          # Get details about who created this MachineConfig
          oc get machineconfig "$mc_name" -o json | tee "${MC_DUMP_DIR}/${mc_name}.new.json" | jq -r '
            "[MC-MONITOR]     created: \(.metadata.creationTimestamp)",
            "[MC-MONITOR]     ownerReferences: \(.metadata.ownerReferences // [] | map(.kind + "/" + .name) | join(", "))",
            "[MC-MONITOR]     generated-by: \(.metadata.annotations["machineconfiguration.openshift.io/generated-by-controller-version"] // "N/A")",
            "[MC-MONITOR]     release-image: \(.metadata.annotations["machineconfiguration.openshift.io/release-image-version"] // "N/A")"
          ' 2>/dev/null
          echo "[MC-MONITOR]     full content saved to: ${MC_DUMP_DIR}/${mc_name}.new.json"
        fi
      done
    else
      echo "[MC-MONITOR] No new MachineConfigs"
    fi

    # Check for MODIFIED MachineConfigs (generation changed)
    local modified_mcs
    modified_mcs=$(jq -rn --argjson prev "$prev_mc_list" --argjson curr "$current_mc_list" '
      ($prev | map({(.name): .gen}) | add) as $prev_gens |
      $curr | map(select($prev_gens[.name] != null and $prev_gens[.name] != .gen) |
        .name) | .[]
    ' 2>/dev/null)

    if [ -n "$modified_mcs" ]; then
      echo "[MC-MONITOR] *** MACHINECONFIGS MODIFIED (generation changed) ***"
      echo "$modified_mcs" | while read -r mc_name; do
        if [ -n "$mc_name" ]; then
          local old_gen new_gen
          old_gen=$(echo "$prev_mc_list" | jq -r --arg name "$mc_name" '.[] | select(.name == $name) | .gen')
          new_gen=$(echo "$current_mc_list" | jq -r --arg name "$mc_name" '.[] | select(.name == $name) | .gen')
          echo "[MC-MONITOR]   - $mc_name gen=${old_gen}->${new_gen}"

          # Save current state and show diff against baseline
          oc get machineconfig "$mc_name" -o json > "${MC_DUMP_DIR}/${mc_name}.modified.json" 2>/dev/null
          echo "[MC-MONITOR]     modified content saved to: ${MC_DUMP_DIR}/${mc_name}.modified.json"

          # Show diff if baseline exists
          if [ -f "${MC_DUMP_DIR}/${mc_name}.baseline.json" ]; then
            echo "[MC-MONITOR]     diff from baseline:"
            diff -u "${MC_DUMP_DIR}/${mc_name}.baseline.json" "${MC_DUMP_DIR}/${mc_name}.modified.json" 2>/dev/null | head -50 | sed 's/^/[MC-MONITOR]       /'
          fi
        fi
      done
    fi

    prev_mc_list="$current_mc_list"

    # Check for node config changes
    local current_node_configs
    current_node_configs=$(oc get nodes -o json | jq -r '[.items[] | {name: .metadata.name, desired: .metadata.annotations["machineconfiguration.openshift.io/desiredConfig"], current: .metadata.annotations["machineconfiguration.openshift.io/currentConfig"], state: .metadata.annotations["machineconfiguration.openshift.io/state"]}]' 2>/dev/null)

    # Compare with previous state
    local config_changes
    config_changes=$(jq -n --argjson prev "$prev_node_configs" --argjson curr "$current_node_configs" '
      $curr | to_entries | map(
        . as $e |
        ($prev | .[$e.key]) as $prev_node |
        if $prev_node.desired != $e.value.desired or $prev_node.current != $e.value.current then
          {
            node: $e.value.name,
            prev_desired: ($prev_node.desired // "unknown"),
            curr_desired: $e.value.desired,
            prev_current: ($prev_node.current // "unknown"),
            curr_current: $e.value.current,
            state: $e.value.state
          }
        else empty end
      )
    ' 2>/dev/null)

    if [ -n "$config_changes" ] && [ "$config_changes" != "[]" ]; then
      echo "[MC-MONITOR] *** NODE CONFIG CHANGES DETECTED ***"
      echo "$config_changes" | jq -r '.[] | "[MC-MONITOR]   \(.node): desired changed from \(.prev_desired) to \(.curr_desired), state=\(.state)"'

      # When desiredConfig changes, identify the source MachineConfigs that differ
      echo "[MC-MONITOR]   Investigating what caused the rendered config change..."
      local old_rendered new_rendered
      old_rendered=$(echo "$config_changes" | jq -r '.[0].prev_desired // empty')
      new_rendered=$(echo "$config_changes" | jq -r '.[0].curr_desired // empty')

      if [ -n "$old_rendered" ] && [ -n "$new_rendered" ] && [ "$old_rendered" != "$new_rendered" ]; then
        echo "[MC-MONITOR]   Comparing rendered configs: $old_rendered vs $new_rendered"

        # Show what controller generated the new rendered config
        echo "[MC-MONITOR]   New rendered config source info:"
        oc get machineconfig "$new_rendered" -o json 2>/dev/null | jq -r '
          "[MC-MONITOR]     generated-by: \(.metadata.annotations["machineconfiguration.openshift.io/generated-by-controller-version"] // "N/A")",
          "[MC-MONITOR]     release-image: \(.metadata.annotations["machineconfiguration.openshift.io/release-image-version"] // "N/A")"
        ' 2>/dev/null || true

        # List source MachineConfigs (non-rendered) with generation to identify what changed
        echo "[MC-MONITOR]   Source MachineConfigs (non-rendered) with gen > 1:"
        oc get machineconfig -o json | jq -r '
          .items[] | select(.metadata.name | startswith("rendered-") | not) |
          select(.metadata.generation > 1) |
          "[MC-MONITOR]     \(.metadata.name) gen=\(.metadata.generation)"
        ' 2>/dev/null || echo "[MC-MONITOR]     (none with gen > 1)"

        # Check MachineConfigPool source config list
        echo "[MC-MONITOR]   MCP source configs:"
        oc get mcp -o json 2>/dev/null | jq -r '
          .items[] | "[MC-MONITOR]     \(.metadata.name): \(.spec.configuration.source // [] | map(.name) | join(", "))"
        ' 2>/dev/null || true
      fi

      prev_node_configs="$current_node_configs"
    else
      echo "[MC-MONITOR] No node config changes"
    fi

    # Check MCP status
    local updating_mcps
    updating_mcps=$(oc get mcp -o json | jq -r '[.items[] | select(.status.conditions[] | .type == "Updating" and .status == "True") | .metadata.name] | join(", ")' 2>/dev/null)

    if [ -n "$updating_mcps" ] && [ "$updating_mcps" != "" ]; then
      echo "[MC-MONITOR] *** MCP UPDATING: $updating_mcps ***"

      # Check if worker node is in Working state (drain imminent - dump everything NOW)
      local worker_draining
      worker_draining=$(oc get nodes -o json | jq -r '
        .items[] | select(.metadata.labels["node-role.kubernetes.io/worker"] != null) |
        select(.metadata.annotations["machineconfiguration.openshift.io/state"] == "Working") |
        .metadata.name
      ' 2>/dev/null)

      if [ -n "$worker_draining" ]; then
        echo "[MC-MONITOR] !!! WORKER NODE DRAINING - DUMPING ALL STATE TO STDOUT !!!"
        echo "[MC-MONITOR] Draining worker(s): $worker_draining"

        # Show what config change triggered the drain
        echo "[MC-MONITOR] === NODE CONFIG CHANGE DETAILS ==="
        oc get nodes -o json | jq -r '
          .items[] | select(.metadata.labels["node-role.kubernetes.io/worker"] != null) |
          "[MC-MONITOR]   Node: \(.metadata.name)",
          "[MC-MONITOR]     currentConfig: \(.metadata.annotations["machineconfiguration.openshift.io/currentConfig"] // "N/A")",
          "[MC-MONITOR]     desiredConfig: \(.metadata.annotations["machineconfiguration.openshift.io/desiredConfig"] // "N/A")",
          "[MC-MONITOR]     state: \(.metadata.annotations["machineconfiguration.openshift.io/state"] // "N/A")"
        ' 2>/dev/null

        # Get the new rendered config and show what's different
        local new_rendered
        new_rendered=$(oc get nodes -l node-role.kubernetes.io/worker -o json | jq -r '.items[0].metadata.annotations["machineconfiguration.openshift.io/desiredConfig"] // empty' 2>/dev/null)
        if [ -n "$new_rendered" ]; then
          echo "[MC-MONITOR] === NEW RENDERED CONFIG: $new_rendered ==="
          oc get machineconfig "$new_rendered" -o json 2>/dev/null | jq -r '
            "[MC-MONITOR]   created: \(.metadata.creationTimestamp)",
            "[MC-MONITOR]   ownerReferences: \(.metadata.ownerReferences // [] | map(.kind + "/" + .name) | join(", "))",
            "[MC-MONITOR]   generated-by: \(.metadata.annotations["machineconfiguration.openshift.io/generated-by-controller-version"] // "N/A")"
          ' 2>/dev/null
        fi

        # Compare baseline vs current ControllerConfig
        if [ -f "${MC_DUMP_DIR}/controllerconfig-baseline.json" ]; then
          echo "[MC-MONITOR] === CONTROLLERCONFIG COMPARISON ==="

          # Get current state
          local current_cc
          current_cc=$(oc get controllerconfig machine-config-controller -o json 2>/dev/null)

          # Generation comparison
          local base_gen curr_gen
          base_gen=$(jq -r '.metadata.generation' "${MC_DUMP_DIR}/controllerconfig-baseline.json")
          curr_gen=$(echo "$current_cc" | jq -r '.metadata.generation')
          echo "[MC-MONITOR] Generation: baseline=$base_gen current=$curr_gen"

          # Full spec hash comparison
          local base_spec_hash curr_spec_hash
          base_spec_hash=$(jq -Sc '.spec' "${MC_DUMP_DIR}/controllerconfig-baseline.json" | md5sum | cut -d' ' -f1)
          curr_spec_hash=$(echo "$current_cc" | jq -Sc '.spec' | md5sum | cut -d' ' -f1)
          echo "[MC-MONITOR] Spec MD5: baseline=$base_spec_hash current=$curr_spec_hash"

          if [ "$base_spec_hash" != "$curr_spec_hash" ]; then
            echo "[MC-MONITOR] *** SPEC CHANGED - showing diff ***"
            diff <(jq -S '.spec' "${MC_DUMP_DIR}/controllerconfig-baseline.json" 2>/dev/null) \
                 <(echo "$current_cc" | jq -S '.spec' 2>/dev/null) 2>/dev/null | \
              head -100 | while IFS= read -r line; do echo "[MC-MONITOR]   $line"; done
          fi

          # Certificate comparison
          local base_cert_hash curr_cert_hash
          base_cert_hash=$(jq -Sc '.status.controllerCertificates' "${MC_DUMP_DIR}/controllerconfig-baseline.json" | md5sum | cut -d' ' -f1)
          curr_cert_hash=$(echo "$current_cc" | jq -Sc '.status.controllerCertificates' | md5sum | cut -d' ' -f1)
          echo "[MC-MONITOR] Certs MD5: baseline=$base_cert_hash current=$curr_cert_hash"

          if [ "$base_cert_hash" != "$curr_cert_hash" ]; then
            echo "[MC-MONITOR] *** CERTIFICATES CHANGED ***"
            echo "[MC-MONITOR] --- BASELINE CERTIFICATES ---"
            jq '.status.controllerCertificates' "${MC_DUMP_DIR}/controllerconfig-baseline.json" 2>/dev/null
            echo "[MC-MONITOR] --- CURRENT CERTIFICATES ---"
            echo "$current_cc" | jq '.status.controllerCertificates' 2>/dev/null
          else
            echo "[MC-MONITOR] Certificates UNCHANGED"
          fi

          echo "[MC-MONITOR] === END CONTROLLERCONFIG COMPARISON ==="
        fi
      fi
    fi

    # Check cluster operator status - this helps identify which config change triggered reconciliation
    local progressing_operators
    progressing_operators=$(oc get clusteroperator -o json | jq -r '
      [.items[] | select(.status.conditions[] | .type == "Progressing" and .status == "True") |
       {name: .metadata.name, message: (.status.conditions[] | select(.type == "Progressing") | .message // "no message")}] |
      if length > 0 then . else empty end
    ' 2>/dev/null)

    if [ -n "$progressing_operators" ] && [ "$progressing_operators" != "[]" ]; then
      echo "[MC-MONITOR] *** CLUSTER OPERATORS PROGRESSING ***"
      echo "$progressing_operators" | jq -r '.[] | "[MC-MONITOR]   \(.name): \(.message | .[0:200])"' 2>/dev/null
    fi

    # Check for image.config changes (insecure registries, etc.)
    local current_image_config
    current_image_config=$(oc get image.config.openshift.io/cluster -o json | jq -c '.spec // {}' 2>/dev/null)
    if [ -n "${PREV_IMAGE_CONFIG:-}" ] && [ "$current_image_config" != "$PREV_IMAGE_CONFIG" ]; then
      echo "[MC-MONITOR] *** IMAGE CONFIG CHANGED ***"
      echo "[MC-MONITOR]   Previous: $PREV_IMAGE_CONFIG"
      echo "[MC-MONITOR]   Current:  $current_image_config"
    fi
    PREV_IMAGE_CONFIG="$current_image_config"

    # Check for source CR changes (KubeletConfig, ContainerRuntimeConfig)
    local curr_kubeletconfigs curr_containerruntimeconfigs
    curr_kubeletconfigs=$(oc get kubeletconfig -o json 2>/dev/null | jq -c '[.items[] | {name: .metadata.name, gen: .metadata.generation}]' 2>/dev/null || echo "[]")
    curr_containerruntimeconfigs=$(oc get containerruntimeconfig -o json 2>/dev/null | jq -c '[.items[] | {name: .metadata.name, gen: .metadata.generation}]' 2>/dev/null || echo "[]")

    if [ "$curr_kubeletconfigs" != "$prev_kubeletconfigs" ]; then
      echo "[MC-MONITOR] *** KUBELETCONFIG CHANGED ***"
      echo "[MC-MONITOR]   Previous: $prev_kubeletconfigs"
      echo "[MC-MONITOR]   Current:  $curr_kubeletconfigs"
      prev_kubeletconfigs="$curr_kubeletconfigs"
    fi

    if [ "$curr_containerruntimeconfigs" != "$prev_containerruntimeconfigs" ]; then
      echo "[MC-MONITOR] *** CONTAINERRUNTIMECONFIG CHANGED ***"
      echo "[MC-MONITOR]   Previous: $prev_containerruntimeconfigs"
      echo "[MC-MONITOR]   Current:  $curr_containerruntimeconfigs"
      prev_containerruntimeconfigs="$curr_containerruntimeconfigs"
    fi

    # Check ControllerConfig changes (this is what triggers kubelet MachineConfig regeneration)
    local curr_controllerconfig
    curr_controllerconfig=$(oc get controllerconfig -o json 2>/dev/null | jq -c '[.items[] | {name: .metadata.name, gen: .metadata.generation, rv: .metadata.resourceVersion}]' 2>/dev/null || echo "[]")
    if [ "$curr_controllerconfig" != "$prev_controllerconfig" ]; then
      echo "[MC-MONITOR] *** CONTROLLERCONFIG CHANGED ***"
      echo "[MC-MONITOR]   Previous: $prev_controllerconfig"
      echo "[MC-MONITOR]   Current:  $curr_controllerconfig"

      # Save current ControllerConfig
      local cc_timestamp
      cc_timestamp=$(date +%s)
      oc get controllerconfig machine-config-controller -o json > "${MC_DUMP_DIR}/controllerconfig-${cc_timestamp}.json" 2>/dev/null || true

      # DUMP EVERYTHING about the ControllerConfig change
      echo "[MC-MONITOR]   === FULL CONTROLLERCONFIG DUMP ==="

      # Metadata
      echo "[MC-MONITOR]   --- Metadata ---"
      oc get controllerconfig machine-config-controller -o json 2>/dev/null | jq -r '
        "[MC-MONITOR]     generation: \(.metadata.generation)",
        "[MC-MONITOR]     resourceVersion: \(.metadata.resourceVersion)",
        "[MC-MONITOR]     uid: \(.metadata.uid)"
      ' 2>/dev/null || true

      # Status fields
      echo "[MC-MONITOR]   --- Status ---"
      oc get controllerconfig machine-config-controller -o json 2>/dev/null | jq -r '
        "[MC-MONITOR]     observedGeneration: \(.status.observedGeneration // "N/A")",
        "[MC-MONITOR]     controllerCertificates count: \(.status.controllerCertificates | length // 0)"
      ' 2>/dev/null || true

      # ALL certificates with full details
      echo "[MC-MONITOR]   --- ALL Certificates (current) ---"
      oc get controllerconfig machine-config-controller -o json 2>/dev/null | jq -r '
        .status.controllerCertificates[]? |
        "[MC-MONITOR]     [\(.bundleFile // "unknown")]",
        "[MC-MONITOR]       signer: \(.signer)",
        "[MC-MONITOR]       subject: \(.subject // "N/A")",
        "[MC-MONITOR]       notBefore: \(.notBefore)",
        "[MC-MONITOR]       notAfter: \(.notAfter)"
      ' 2>/dev/null || echo "[MC-MONITOR]     (unable to get certificates)"

      # Compare with baseline certificates
      if [ -f "${MC_DUMP_DIR}/controllerconfig-baseline.json" ]; then
        echo "[MC-MONITOR]   --- Certificate Comparison (baseline vs current) ---"

        # Get baseline cert summary
        local baseline_certs current_certs
        baseline_certs=$(jq -r '[.status.controllerCertificates[]? | "\(.signer)|\(.notBefore)|\(.notAfter)"] | sort | .[]' "${MC_DUMP_DIR}/controllerconfig-baseline.json" 2>/dev/null)
        current_certs=$(jq -r '[.status.controllerCertificates[]? | "\(.signer)|\(.notBefore)|\(.notAfter)"] | sort | .[]' "${MC_DUMP_DIR}/controllerconfig-${cc_timestamp}.json" 2>/dev/null)

        echo "[MC-MONITOR]     Baseline certificates:"
        echo "$baseline_certs" | while read -r cert; do
          echo "[MC-MONITOR]       $cert"
        done

        echo "[MC-MONITOR]     Current certificates:"
        echo "$current_certs" | while read -r cert; do
          echo "[MC-MONITOR]       $cert"
        done

        # Show new/changed certs
        echo "[MC-MONITOR]     --- NEW or CHANGED certificates ---"
        comm -13 <(echo "$baseline_certs" | sort) <(echo "$current_certs" | sort) 2>/dev/null | while read -r cert; do
          echo "[MC-MONITOR]       + $cert"
        done

        echo "[MC-MONITOR]     --- REMOVED certificates ---"
        comm -23 <(echo "$baseline_certs" | sort) <(echo "$current_certs" | sort) 2>/dev/null | while read -r cert; do
          echo "[MC-MONITOR]       - $cert"
        done
      fi

      # Spec field hashes (these trigger regeneration)
      echo "[MC-MONITOR]   --- Spec Field Hashes ---"
      oc get controllerconfig machine-config-controller -o json 2>/dev/null | jq -r '
        "[MC-MONITOR]     rootCAData (first 40 chars of base64): \(.spec.rootCAData | .[0:40] // "N/A")",
        "[MC-MONITOR]     kubeAPIServerServingCAData (first 40 chars): \(.spec.kubeAPIServerServingCAData | .[0:40] // "N/A")",
        "[MC-MONITOR]     cloudProviderCAData present: \(if .spec.cloudProviderCAData then "yes" else "no" end)",
        "[MC-MONITOR]     additionalTrustBundle present: \(if .spec.additionalTrustBundle then "yes (\(.spec.additionalTrustBundle | length) bytes)" else "no" end)",
        "[MC-MONITOR]     imageRegistryBundleData count: \(.spec.imageRegistryBundleData | length // 0)",
        "[MC-MONITOR]     imageRegistryBundleUserData count: \(.spec.imageRegistryBundleUserData | length // 0)",
        "[MC-MONITOR]     releaseImage: \(.spec.releaseImage // "N/A")",
        "[MC-MONITOR]     internalRegistryPullSecret present: \(if .spec.internalRegistryPullSecret then "yes" else "no" end)",
        "[MC-MONITOR]     ipFamilies: \(.spec.ipFamilies // "N/A")",
        "[MC-MONITOR]     platform: \(.spec.platform // "N/A")",
        "[MC-MONITOR]     proxy: \(.spec.proxy // "N/A")",
        "[MC-MONITOR]     pullSecret present: \(if .spec.pullSecret then "yes" else "no" end)"
      ' 2>/dev/null || true

      # Compare spec fields with baseline - FULL comparison
      if [ -f "${MC_DUMP_DIR}/controllerconfig-baseline.json" ]; then
        echo "[MC-MONITOR]   --- FULL Spec Diff (baseline vs current) ---"

        # Full spec hash comparison (not truncated)
        local baseline_spec_hash current_spec_hash
        baseline_spec_hash=$(jq -Sc '.spec' "${MC_DUMP_DIR}/controllerconfig-baseline.json" 2>/dev/null | md5sum | cut -d' ' -f1)
        current_spec_hash=$(jq -Sc '.spec' "${MC_DUMP_DIR}/controllerconfig-${cc_timestamp}.json" 2>/dev/null | md5sum | cut -d' ' -f1)

        echo "[MC-MONITOR]     Baseline spec MD5: $baseline_spec_hash"
        echo "[MC-MONITOR]     Current spec MD5:  $current_spec_hash"

        if [ "$baseline_spec_hash" = "$current_spec_hash" ]; then
          echo "[MC-MONITOR]     Spec is IDENTICAL - generation change triggered by OTHER mechanism"
          echo "[MC-MONITOR]     (Note: generation only changes on spec change, but MCO may use annotations or other triggers)"
        else
          echo "[MC-MONITOR]     *** SPEC CHANGED! Full diff below ***"
          # Show what changed using diff
          echo "[MC-MONITOR]   --- SPEC DIFF (< baseline, > current) ---"
          diff <(jq -S '.spec' "${MC_DUMP_DIR}/controllerconfig-baseline.json" 2>/dev/null) \
               <(jq -S '.spec' "${MC_DUMP_DIR}/controllerconfig-${cc_timestamp}.json" 2>/dev/null) 2>/dev/null | \
            head -100 | while IFS= read -r line; do
              echo "[MC-MONITOR]     $line"
            done
        fi

        # Check each spec field individually for length changes
        echo "[MC-MONITOR]   --- Spec Field Sizes (baseline -> current) ---"
        for field in rootCAData kubeAPIServerServingCAData cloudProviderCAData additionalTrustBundle pullSecret internalRegistryPullSecret; do
          local base_len curr_len
          base_len=$(jq -r ".spec.${field} | if . then (. | length | tostring) else \"null\" end" "${MC_DUMP_DIR}/controllerconfig-baseline.json" 2>/dev/null)
          curr_len=$(jq -r ".spec.${field} | if . then (. | length | tostring) else \"null\" end" "${MC_DUMP_DIR}/controllerconfig-${cc_timestamp}.json" 2>/dev/null)
          if [ "$base_len" != "$curr_len" ]; then
            echo "[MC-MONITOR]     ${field}: $base_len -> $curr_len (CHANGED)"
          fi
        done

        # Full certificate data comparison (not just metadata)
        echo "[MC-MONITOR]   --- Certificate Data Hash Comparison ---"
        local baseline_cert_hash current_cert_hash
        baseline_cert_hash=$(jq -Sc '.status.controllerCertificates' "${MC_DUMP_DIR}/controllerconfig-baseline.json" 2>/dev/null | md5sum | cut -d' ' -f1)
        current_cert_hash=$(jq -Sc '.status.controllerCertificates' "${MC_DUMP_DIR}/controllerconfig-${cc_timestamp}.json" 2>/dev/null | md5sum | cut -d' ' -f1)

        echo "[MC-MONITOR]     Baseline certs MD5: $baseline_cert_hash"
        echo "[MC-MONITOR]     Current certs MD5:  $current_cert_hash"

        if [ "$baseline_cert_hash" != "$current_cert_hash" ]; then
          echo "[MC-MONITOR]     *** CERTIFICATES CHANGED - showing full diff ***"
          diff <(jq -S '.status.controllerCertificates' "${MC_DUMP_DIR}/controllerconfig-baseline.json" 2>/dev/null) \
               <(jq -S '.status.controllerCertificates' "${MC_DUMP_DIR}/controllerconfig-${cc_timestamp}.json" 2>/dev/null) 2>/dev/null | \
            head -200 | while IFS= read -r line; do
              echo "[MC-MONITOR]     $line"
            done
        fi

        # Check annotations (sometimes MCO uses these for triggering)
        echo "[MC-MONITOR]   --- Annotation Comparison ---"
        local baseline_annotations current_annotations
        baseline_annotations=$(jq -Sc '.metadata.annotations // {}' "${MC_DUMP_DIR}/controllerconfig-baseline.json" 2>/dev/null)
        current_annotations=$(jq -Sc '.metadata.annotations // {}' "${MC_DUMP_DIR}/controllerconfig-${cc_timestamp}.json" 2>/dev/null)

        if [ "$baseline_annotations" != "$current_annotations" ]; then
          echo "[MC-MONITOR]     *** ANNOTATIONS CHANGED ***"
          echo "[MC-MONITOR]     Baseline: $baseline_annotations"
          echo "[MC-MONITOR]     Current:  $current_annotations"
        else
          echo "[MC-MONITOR]     Annotations unchanged"
        fi
      fi

      echo "[MC-MONITOR]   === END CONTROLLERCONFIG DUMP ==="
      echo "[MC-MONITOR]   Full ControllerConfig saved to ${MC_DUMP_DIR}/controllerconfig-${cc_timestamp}.json"
      prev_controllerconfig="$curr_controllerconfig"

      # Switch to faster polling for next 10 iterations to catch delayed MachineConfig regeneration
      echo "[MC-MONITOR]   Switching to fast polling (15s) to catch delayed MachineConfig regeneration..."
      fast_poll_count=10
    fi

    # Periodic status check (every 6 iterations = ~3 minutes) - minimal output
    dump_counter=$((dump_counter + 1))
    if [ "$dump_counter" -ge 6 ]; then
      dump_counter=0
      # One-line status with hashes for tracking
      local curr_gen curr_spec_hash
      curr_gen=$(oc get controllerconfig machine-config-controller -o jsonpath='{.metadata.generation}' 2>/dev/null)
      curr_spec_hash=$(oc get controllerconfig machine-config-controller -o json 2>/dev/null | jq -Sc '.spec' | md5sum | cut -d' ' -f1)
      echo "[MC-MONITOR] Periodic: ControllerConfig gen=$curr_gen spec=$curr_spec_hash"
    fi
  done
}

# Starts background MachineConfig monitoring
start_machineconfig_monitor() {
  if [ "${OCP}" != "true" ]; then
    return 0
  fi

  echo "Starting background MachineConfig monitor..."
  capture_machineconfig_baseline

  # Check if monitoring was disabled (e.g., HyperShift hosted cluster)
  if [ "${MC_MONITOR_DISABLED:-false}" = "true" ]; then
    echo "MachineConfig monitoring disabled - skipping background monitor"
    return 0
  fi

  # Start the monitor in background (30 second interval for better change detection)
  monitor_machineconfig_changes 30 &
  MC_MONITOR_PID=$!
  echo "MachineConfig monitor started with PID: ${MC_MONITOR_PID}"
}

# Stops background MachineConfig monitoring
stop_machineconfig_monitor() {
  if [ -n "${MC_MONITOR_PID}" ] && kill -0 "${MC_MONITOR_PID}" 2>/dev/null; then
    echo "[MC-MONITOR] === Stopping (PID: ${MC_MONITOR_PID}) ==="
    kill "${MC_MONITOR_PID}" 2>/dev/null || true
    wait "${MC_MONITOR_PID}" 2>/dev/null || true
    MC_MONITOR_PID=""
  fi
}

# Pauses the worker MachineConfigPool to prevent node drains during tests
# This is necessary because OpenShift certificate rotation triggers MachineConfig
# regeneration ~50-60 minutes after cluster creation, which causes node drains
pause_worker_mcp() {
  if [ "${OCP}" != "true" ]; then
    return 0
  fi

  # Check if MachineConfigPool API is available (not on HyperShift)
  if ! oc api-resources --api-group=machineconfiguration.openshift.io 2>/dev/null | grep -q machineconfigpool; then
    echo "MachineConfigPool API not available (likely HyperShift) - skipping MCP pause"
    return 0
  fi

  echo "Pausing worker MachineConfigPool to prevent node drains during tests..."
  if oc patch mcp/worker --type merge -p '{"spec":{"paused":true}}' 2>/dev/null; then
    echo "Worker MachineConfigPool paused successfully"
    export MCP_PAUSED=true
  else
    echo "WARNING: Failed to pause worker MachineConfigPool - tests may be interrupted by node drains"
  fi
}

# Unpauses the worker MachineConfigPool after tests complete
unpause_worker_mcp() {
  if [ "${MCP_PAUSED:-false}" != "true" ]; then
    return 0
  fi

  echo "Unpausing worker MachineConfigPool..."
  if oc patch mcp/worker --type merge -p '{"spec":{"paused":false}}' 2>/dev/null; then
    echo "Worker MachineConfigPool unpaused successfully"
    export MCP_PAUSED=false
  else
    echo "WARNING: Failed to unpause worker MachineConfigPool - manual intervention may be required"
  fi
}

check_mcp_stability() {
  # Check if MachineConfigPools are stable (not updating)
  if [ "${OCP}" != "true" ]; then
    return 0
  fi

  # Check if MachineConfigPool API exists (not available on HyperShift hosted clusters)
  if ! oc api-resources --api-group=machineconfiguration.openshift.io 2>/dev/null | grep -q machineconfigpools; then
    echo "MachineConfigPool API not available (likely HyperShift hosted cluster) - skipping stability check"
    return 0
  fi

  echo "Checking MachineConfigPool stability..."

  local updating_mcps
  updating_mcps=$(oc get mcp -o json | jq -r '
    [.items[] | select(.status.conditions[] | .type == "Updating" and .status == "True") | .metadata.name] | join(", ")
  ' 2>/dev/null)

  if [ -n "$updating_mcps" ] && [ "$updating_mcps" != "" ]; then
    echo "WARNING: MachineConfigPools are currently updating: $updating_mcps"
    echo "This may cause node drains during test execution."
    return 1
  fi

  # Check if any node has desiredConfig != currentConfig
  local nodes_pending_update
  nodes_pending_update=$(oc get nodes -o json | jq -r '
    [.items[] | select(
      .metadata.annotations["machineconfiguration.openshift.io/currentConfig"] !=
      .metadata.annotations["machineconfiguration.openshift.io/desiredConfig"]
    ) | .metadata.name] | join(", ")
  ' 2>/dev/null)

  if [ -n "$nodes_pending_update" ] && [ "$nodes_pending_update" != "" ]; then
    echo "WARNING: Nodes have pending MachineConfig updates: $nodes_pending_update"
    return 1
  fi

  echo "All MachineConfigPools are stable."
  return 0
}

check_cluster_operators() {
  # This function is only relevant for OCP clusters
  if [ "${OCP}" != "true" ]; then
    echo "Skipping ClusterOperator check on non-OCP cluster."
    return 0
  fi

  # Check if jq is installed
  if ! command -v jq &> /dev/null; then
    echo "ERROR: jq is required for the cluster operator health check. Please install jq."
    exit 1
  fi

  # Log initial cluster state for debugging
  log_cluster_state

  local timeout_seconds=600
  echo "Validating OpenShift cluster operators are stable..."
  local end_time=$(( $(date +%s) + timeout_seconds ))

  while [ "$(date +%s)" -lt $end_time ]; do
    # This command uses jq to count operators that are not Available, or are Progressing, or are Degraded.
    # A healthy cluster should have a count of 0.
    local unstable_operators
    unstable_operators=$(oc get clusteroperator -o json | jq '[.items[] | select(.status.conditions[] | (.type == "Available" and .status == "False") or (.type == "Progressing" and .status == "True") or (.type == "Degraded" and .status == "True"))] | length')

    if [[ $unstable_operators -eq 0 ]]; then
      echo "All cluster operators are stable."
      # Also check MCP stability
      check_mcp_stability || echo "Proceeding despite MCP warning..."
      return 0
    fi

    echo -n "."
    sleep 15
  done

  echo "ERROR: Timeout reached. Not all cluster operators are stable."
  oc get clusteroperator # Print the final status for debugging
  log_cluster_state # Log final state for debugging
  exit 1
}

install_operator() {
  echo "Installing sail-operator (KUBECONFIG=${KUBECONFIG})"
  "${COMMAND}" create namespace "${NAMESPACE}"
  helm install sail-operator "${SOURCE_DIR}"/chart --namespace "${NAMESPACE}" --set image="${HUB}/${IMAGE_BASE}:${TAG}" --set operatorLogLevel=3
}

await_operator() {
  echo "Awaiting operator deployment on (KUBECONFIG=${KUBECONFIG})"
  local name="${DEPLOYMENT_NAME}"
  if [ "${OLM}" == "true" ]; then
    local csv_name
    local csv_file
    csv_file=$(find "${WD}/../../bundle/manifests/" -name "*.clusterserviceversion.yaml" | head -1)
    csv_name=$(yq eval '.spec.install.spec.deployments[0].name' "${csv_file}" 2>/dev/null || true)
    if [ -n "${csv_name}" ]; then
      echo "OLM mode: using deployment name from bundle CSV: ${csv_name}"
      name="${csv_name}"
      DEPLOYMENT_NAME="${csv_name}"
    fi
  fi
  "${COMMAND}" wait --for=condition=available deployment/"${name}" -n "${NAMESPACE}" --timeout=5m
}

# shellcheck disable=SC2329  # Function is invoked indirectly via trap
uninstall_operator() {
  echo "Uninstalling sail-operator (KUBECONFIG=${KUBECONFIG})"
  helm uninstall sail-operator --namespace "${NAMESPACE}"
  "${COMMAND}" delete namespace "${NAMESPACE}"
}

# Ensure cleanup always runs and that the original test exit code is preserved
# shellcheck disable=SC2329  # Function is invoked indirectly via trap
cleanup() {
  # Do not let cleanup errors affect the final exit code
  set +e

  # Unpause worker MachineConfigPool if we paused it
  # DISABLED FOR DEBUGGING - investigating certificate rotation root cause
  # unpause_worker_mcp

  # Stop MachineConfig monitoring and print summary
  stop_machineconfig_monitor

  if [ "${OLM}" != "true" ] && [ "${SKIP_DEPLOY}" != "true" ] && [ "${SKIP_CLEANUP}" != "true" ]; then
    if [ "${MULTICLUSTER}" == true ]; then
      KUBECONFIG="${KUBECONFIG}" uninstall_operator || true
      # shellcheck disable=SC2153  # KUBECONFIG2 is set by multicluster setup scripts
      KUBECONFIG="${KUBECONFIG2}" uninstall_operator || true
    else
      uninstall_operator || true
    fi
  fi
  echo "JUnit report: ${ARTIFACTS}/report.xml"
}

trap cleanup EXIT INT TERM

# Main script flow
check_arguments "$@"
parse_flags "$@"
initialize_variables

# Export necessary vars
export COMMAND OCP HUB IMAGE_BASE TAG NAMESPACE USE_INTERNAL_REGISTRY

if [ "${SKIP_BUILD}" == "false" ]; then
  "${WD}/setup/build-and-push-operator.sh"

  if [ "${OCP}" = "true" ]; then
    # This is a workaround when pulling the image from internal registry
    # To avoid errors of certificates meanwhile we are pulling the operator image from the internal registry
    # We need to set image $HUB to a fixed known value after the push
    # Convert from route URL to service URL format for image pulling
    if [[ "${HUB}" == *"/istio-images" ]]; then
      HUB="image-registry.openshift-image-registry.svc:5000/istio-images"
      echo "Using internal registry service URL: ${HUB}"
    else
      echo "Using external registry: ${HUB}"
    fi

    # Workaround for OCP helm operator installation issues:
    # To avoid any cleanup issues, after we build and push the image we check if the namespace exists and delete it if it does.
    # The test logic already handles the namespace creation and deletion during the test run. 
    if ${COMMAND} get ns "${NAMESPACE}" &>/dev/null; then
      echo "Namespace ${NAMESPACE} already exists. Deleting it to avoid conflicts."
      ${COMMAND} delete ns "${NAMESPACE}"
    fi
  fi
  # If OLM is enabled, deploy the operator using OLM
  # If PR_NUMBER is set we will tag the BUNDLE_IMG with the PR number and commit hash to avoid conflicts.
  if [ "${OLM}" == "true" ] && [ "${SKIP_DEPLOY}" == "false" ] && [ "${MULTICLUSTER}" == "false" ]; then
    IMAGE_TAG_BASE="${HUB}/${IMAGE_BASE}"
    if [ "${CI}" == "true" ]; then
      if [ -n "${PR_NUMBER:-}" ]; then
        BUNDLE_IMG="${IMAGE_TAG_BASE}-bundle:pr-${PR_NUMBER}-${COMMIT_HASH}-${TARGET_ARCH}"
      else
        BUNDLE_IMG="${IMAGE_TAG_BASE}-bundle:ci-test-${COMMIT_HASH}-${TARGET_ARCH}"
      fi
    else
      BUNDLE_IMG="${IMAGE_TAG_BASE}-bundle:ci-test-${COMMIT_HASH}-${TARGET_ARCH}"
    fi

    IMAGE="${HUB}/${IMAGE_BASE}:${TAG}" \
    IMAGE_TAG_BASE="${IMAGE_TAG_BASE}" \
    BUNDLE_IMG="${BUNDLE_IMG}" \
    OCP="${OCP}" \
    make bundle bundle-build bundle-push

    if [ "${OCP}" == "false" ]; then
      # Install OLM in the cluster because it's not available by default in kind.
      OLM_INSTALL_ARGS=""
      if [ "${OLM_VERSION}" != "" ]; then
        OLM_INSTALL_ARGS+="--version ${OLM_VERSION}"
      fi

      # Ensure kubeconfig is set to the kind cluster
      kind export kubeconfig --name="${KIND_CLUSTER_NAME}"
      # shellcheck disable=SC2086
      ${OPERATOR_SDK} olm install ${OLM_INSTALL_ARGS}

      ${COMMAND} wait catalogsource operatorhubio-catalog -n olm --for 'jsonpath={.status.connectionState.lastObservedState}=READY' --timeout=5m
    else
      # On OCP, wait for different CatalogSources as operatorhubio-catalog might not exist
      ${COMMAND} wait catalogsource redhat-operators -n openshift-marketplace --for 'jsonpath={.status.connectionState.lastObservedState}=READY' --timeout=5m || true
    fi

    ${COMMAND} create ns "${NAMESPACE}" || true
    ${OPERATOR_SDK} run bundle "${BUNDLE_IMG}" -n "${NAMESPACE}" --skip-tls --timeout 5m || exit 1

    await_operator

    SKIP_DEPLOY=true
  fi
fi

export SKIP_DEPLOY IP_FAMILY ISTIO_MANIFEST NAMESPACE CONTROL_PLANE_NS DEPLOYMENT_NAME MULTICLUSTER ARTIFACTS ISTIO_NAME COMMAND KUBECONFIG ISTIOCTL_PATH SKIP_CLEANUP GINKGO_FLAGS FIPS_CLUSTER

if [ "${OLM}" != "true" ] && [ "${SKIP_DEPLOY}" != "true" ]; then
  # shellcheck disable=SC2153
  if [ "${MULTICLUSTER}" == true ]; then
    KUBECONFIG="${KUBECONFIG}" install_operator
    KUBECONFIG="${KUBECONFIG2}" install_operator
    KUBECONFIG="${KUBECONFIG}" await_operator
    KUBECONFIG="${KUBECONFIG2}" await_operator
  else
    install_operator
    await_operator
  fi
fi

# Check that all cluster operators are stable before running the tests. This only applies to OCP clusters.
# This is to avoid test failures due to cluster instability.
check_cluster_operators

# Start background MachineConfig monitoring to detect changes during test execution
start_machineconfig_monitor

# Pause worker MachineConfigPool to prevent node drains during tests
# This is necessary because certificate rotation triggers MachineConfig updates ~50-60 min after cluster creation
# DISABLED FOR DEBUGGING - investigating certificate rotation root cause
# pause_worker_mcp

set +e
# Disable to avoid failing the test run before generating the report.xml
# Capture the test exit code and allow cleanup via trap to run
# shellcheck disable=SC2086
IMAGE="${HUB}/${IMAGE_BASE}:${TAG}" \
go run github.com/onsi/ginkgo/v2/ginkgo -tags e2e \
--timeout 60m --junit-report="${ARTIFACTS}/report.xml" ${GINKGO_FLAGS:-} "${WD}"/...
TEST_EXIT_CODE=$?

exit "${TEST_EXIT_CODE}"
