#!/bin/bash
#
# Copyright 2024 Red Hat, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#	http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# This script checks your SMCP and other resources for fields/features that need to be disabled
# before safely migrationg to OSSM 3.0.

set -o pipefail -eu

BLUE='\033[1;34m'
YELLOW='\033[1;33m'
GREEN='\033[1;32m'
BLANK='\033[0m'
WARNING_EMOJI='\u2757'
SPACER="-----------------------"

# TODO: autodetect latest versions
LATEST_VERSION=v2.6
LATEST_CHART_VERSION=2.6.4
LATEST_KIALI_VERSION=2.4.0

TOTAL_WARNINGS=0

SKIP_PROXY_CHECK=false

# process command line args
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
    --skip-proxy-check)
      SKIP_PROXY_CHECK="${2}"
      shift;shift
      ;;
    -h|--help)
      cat <<HELPMSG
Valid command line arguments:
  --skip-proxy-check
    If 'true', will skip checking proxies for the latest version.
    Default: false
HELPMSG
      exit 1
      ;;
    *)
      echo "ERROR: Unknown argument [$key]. Use --help to see valid arguments."
      exit 1
      ;;
  esac
done

print_section() {
    cat <<EOM
****************************************************
*
* $1
*
****************************************************
EOM
}

warning() {
  echo -e "${YELLOW}${WARNING_EMOJI}$1${BLANK}"
}

success() {
  echo -e "${GREEN}$1${BLANK}"
}

add_warning() {
    ((TOTAL_WARNINGS+=1))
    warning "$1"
}

check_for_new_warnings() {
    local previous_warnings=$1
    local success_message=$2

    local new_warnings=$((TOTAL_WARNINGS - previous_warnings))
    if [ "$new_warnings" -ne 0 ]; then
        echo -e "\n${YELLOW}$new_warnings warnings${BLANK}"
    else
        success "$success_message"
    fi
}

if ! command -v jq > /dev/null 2>&1
then
    echo "jq must be installed and present in PATH."
    exit 1
fi

if ! command -v oc > /dev/null 2>&1
then
    echo "oc must be installed and present in PATH."
    exit 1
fi

if ! oc whoami > /dev/null 2>&1
then
    echo "Unable to use oc. Ensure your cluster is online and you have logged in with 'oc login'"
    exit 1
fi

check_smcp() {
    local name=$1
    local namespace=$2

    local num_warnings=$TOTAL_WARNINGS

    echo -e "ServiceMeshControlPlane\nName: ${BLUE}$name${BLANK}\nNamespace: ${BLUE}$namespace${BLANK}\n"

    local smcp
    smcp=$(oc get smcp "$name" -n "$namespace" -o json)

    if [ "$(echo "$smcp" | jq -r '.spec.security.manageNetworkPolicy')" != "false" ]; then
        add_warning "Network Policy is still enabled. Please set '.spec.security.manageNetworkPolicy' = false"
    fi

    local current_version
    current_version=$(echo "$smcp" | jq -r '.spec.version')
    if [ "$current_version" != "$LATEST_VERSION" ]; then
        add_warning "Your ServiceMeshControlPlane is not on the latest version. Current version: '$current_version'. Latest version: '$LATEST_VERSION'. Please upgrade your ServiceMeshControlPlane to the latest version."
    fi

    local current_chart_version
    current_chart_version=$(echo "$smcp" | jq -r '.status.chartVersion')
    if [ "$current_chart_version" != "$LATEST_CHART_VERSION" ]; then
        add_warning "Your ServiceMeshControlPlane does not have the latest z-stream release. If your ServiceMeshControlPlane is already on the latest version, please ensure your Service Mesh operator is also updated to the latest version. Current version: '$current_chart_version'. Latest version: '$LATEST_CHART_VERSION'."
    fi

    # Addons
    if [ "$(echo "$smcp" | jq -r '.spec.addons.prometheus.enabled')" != "false" ]; then
        add_warning "Prometheus addon is still enabled. Please disable the addon by setting '.spec.addons.prometheus.enabled' = false"
    fi
    
    if [ "$(echo "$smcp" | jq -r '.spec.addons.kiali.enabled')" != "false" ]; then
        add_warning "Kiali addon is still enabled. Please disable the addon by setting '.spec.addons.kiali.enabled' = false"
    fi
    
    if [ "$(echo "$smcp" | jq -r '.spec.addons.grafana.enabled')" != "false" ]; then
        add_warning "Grafana addon is enabled. Grafana is no longer supported with Service Mesh 3.x."
    fi

    if [ "$(echo "$smcp" | jq -r '.spec.tracing.type')" != "None" ]; then
        add_warning "Tracing addon is still enabled. Please disable the addon by setting '.spec.tracing.type' = None"
    fi

    if [ "$(echo "$smcp" | jq -r '.spec.gateways.enabled')" != "false" ]; then
        add_warning "Gateways are still enabled. Please disable gateways by setting '.spec.gateways.enabled' = false"
    fi

    # IOR is included in the above check since if this top level gateways field
    # is disabled then IOR is disabled too because there won't be any gateways but
    # we're checking it here to remind users to disable it.
    # Default is 'false' so only log a warning if someone has set it to 'true'.
    if [ "$(echo "$smcp" | jq -r '.spec.gateways.openshiftRoute.enabled')" == "true" ]; then
        add_warning "IOR is still enabled. Please disable IOR gateways by setting '.spec.gateways.openshiftRoute.enabled' = false"
    fi

    check_for_new_warnings $num_warnings "No issues detected with the ServiceMeshControlPlane $name/$namespace."
    echo -e "$SPACER"
}

check_federation() {
    print_section "Federation"
    local num_warnings=$TOTAL_WARNINGS

    if [ "$(oc get exportedservicesets.federation.maistra.io -A -o jsonpath='{.items}' | jq 'length')" != 0 ]; then
        add_warning "Detected federation resources 'exportedservicesets'. Migrating federation to 3.0 is not supported. Please remove your federation resources."
    fi

    if [ "$(oc get importedservicesets.federation.maistra.io -A -o jsonpath='{.items}' | jq 'length')" != 0 ]; then
        add_warning "Detected federation resources 'importedservicesets'. Migrating federation to 3.0 is not supported. Please remove your federation resources."
    fi

    check_for_new_warnings $num_warnings "No federation resources found in the cluster."
}

check_proxies_updated() {
    print_section "Proxies"
    echo -e "Checking proxies are up to date...\n"
    local num_warnings=$TOTAL_WARNINGS
    # Find pods and check each one.
    # Format is name/namespace/version.
    for pod in $(oc get pods -A -l maistra-version -o jsonpath='{range .items[*]}{.metadata.name}/{.metadata.namespace}/{.metadata.labels.maistra-version}{" "}{end}'); do
        IFS="/" read -r name namespace version <<< "$pod"
        # label version format: 2.6.4 --> 2.6
        local sanitized_version
        sanitized_version=$(cut -c1-3 <<< "$version")
        # latest version format: v2.6 --> 2.6
        local sanitized_latest_version
        sanitized_latest_version=$(cut -c2- <<< "$LATEST_VERSION")
        if [ "$sanitized_version" != "$sanitized_latest_version" ]; then
            add_warning "pod: $name/$namespace is running a proxy at an older version: $sanitized_version Please update your ServiceMeshControlPlane to the latest version: ${LATEST_VERSION} and then restart this workload."
        fi
    done

    check_for_new_warnings $num_warnings "All proxies are on the latest version."
}

check_smcps() {
    print_section "ServiceMeshControlPlanes"
    # Find smcps and check each one.
    # Format is name/namespace.
    for smcp in $(oc get smcp -A -o jsonpath='{range .items[*]}{.metadata.name}/{.metadata.namespace}{" "}{end}'); do
        IFS="/" read -r name namespace <<< "$smcp"
        check_smcp "$name" "$namespace"
    done
}

check_kiali() {
    local name=$1
    local namespace=$2

    local num_warnings=$TOTAL_WARNINGS

    echo -e "Kiali\nName: ${BLUE}$name${BLANK}\nNamespace: ${BLUE}$namespace${BLANK}\n"

    local kiali
    kiali=$(oc get kiali "$name" -n "$namespace" -o json)

    local current_version
    current_version=$(echo "$kiali" | jq -r '.spec.version')
    if [[ "$current_version" != "$LATEST_KIALI_VERSION" && "$current_version" != "default" ]]; then
        add_warning "Your Kiali is not on the latest version. Current version: '$current_version'. Latest version: '$LATEST_KIALI_VERSION'. Please upgrade your Kiali to the latest version."
    fi

    check_for_new_warnings $num_warnings "Kiali $name/$namespace is on the latest version."
    echo -e "$SPACER"
}

check_kialis() {
    print_section "Kiali"

    if ! oc get crds kialis.kiali.io > /dev/null 2>&1
    then
        echo "Kiali CRD is not detected. Skipping Kiali checks..."
        return
    fi

    # Find kialis and check each one.
    # Format is name/namespace.
    for kiali in $(oc get kiali -A -o jsonpath='{range .items[*]}{.metadata.name}/{.metadata.namespace}{" "}{end}'); do
        IFS="/" read -r name namespace <<< "$kiali"
        check_kiali "$name" "$namespace"
    done

    local num_warnings=$TOTAL_WARNINGS

    # Check Kiali operator
    # Find kiali-ossm subscription and then use that to find the operator/csv namespace
    local operator_namespace
    operator_namespace=$(oc get subscriptions.operators.coreos.com -A -o jsonpath='{.items[?(@.metadata.name=="kiali-ossm")].metadata.namespace}')
    local operator_name
    operator_name=$(kubectl get csv -n "$operator_namespace" -l "operators.coreos.com/kiali-ossm.$operator_namespace" -o jsonpath='{.items[0].metadata.name}')
    local operator_version
    operator_version=$(oc get csv -n "$operator_namespace" "$operator_name" -o jsonpath='{.spec.version}')
    if [ "$operator_version" != "$LATEST_KIALI_VERSION" ]; then
        add_warning "Your Kiali operator is not on the latest version. Current version: '$operator_version'. Latest version: '$LATEST_KIALI_VERSION'. Please upgrade your Kiali operator to the latest version."
    fi

    check_for_new_warnings $num_warnings "Kiali operator is up to date"
}

check_istio_crds() {
    print_section "Istio CRDs"

    local num_warnings=$TOTAL_WARNINGS

    # Assumes that if any one of the CRDs has a v1 then the rest do.
    if [ "$(oc get crds -l chart=istio -o json | jq '.items[] | select(.spec.versions[].name == "v1") | .metadata.name')" == "" ]; then
        add_warning "v1 istio CRDs not found. Ensure you have installed the OpenShift Service Mesh 3 operator."
    fi

    check_for_new_warnings $num_warnings "Istio CRDs are up to date"
}

check_smcps
check_federation
if [ "$SKIP_PROXY_CHECK" != "true" ]; then
    check_proxies_updated
fi
check_kialis
check_istio_crds

print_section "Summary"
if [ "$TOTAL_WARNINGS" -eq 0 ]; then
    success "No issues detected. Proceed with the 2.6 --> 3.0 migration."
else
    warning "Detected $TOTAL_WARNINGS issues. Please fix these before proceeding with the migration."
fi
