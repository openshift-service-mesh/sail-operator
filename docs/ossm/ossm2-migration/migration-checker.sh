#!/bin/bash

# This script checks your SMCP for fields/features that need to be disabled
# before safely migrationg to OSSM 3.0.

set -o pipefail -eu

BLUE='\033[1;34m'
YELLOW='\033[1;33m'
GREEN='\033[1;32m'
BLANK='\033[0m'

warning() {
  echo -e "${YELLOW}[WARNING] $1${BLANK}"
}

success() {
  echo -e "${GREEN}$1${BLANK}"
}

if ! command -v jq 2>&1 >/dev/null
then
    echo "jq must be installed and present in PATH."
    exit 1
fi

if ! command -v oc 2>&1 >/dev/null
then
    echo "oc must be installed and present in PATH."
    exit 1
fi

TOTAL_WARNINGS=0

check_smcp() {
    local name=$1
    local namespace=$2

    local num_warnings=0

    echo -e "-----------------------\nServiceMeshControlPlane\nName: ${BLUE}$name${BLANK}\nNamespace: ${BLUE}$namespace${BLANK}\n" 

    local smcp
    smcp=$(oc get smcp "$name" -n "$namespace" -o json)

    if [ "$(echo "$smcp" | jq -r '.spec.security.manageNetworkPolicy')" != "false" ]; then
        warning "Network Policy is still enabled. Please set '.spec.security.manageNetworkPolicy' = false"
        ((num_warnings+=1))
    fi

    if [ "$(echo "$smcp" | jq -r '.spec.gateways.openshiftRoute.enabled')" == "true" ]; then
        warning "IOR is still enabled. Please set '.spec.gateways.openshiftRoute.enabled' = false"
        ((num_warnings+=1))
    fi

    # Addons
    if [ "$(echo "$smcp" | jq -r '.spec.addons.prometheus.enabled')" != "false" ]; then
        warning "Prometheus addon is still enabled. Please disable the addon by setting '.spec.addons.prometheus.enabled' = false"
        ((num_warnings+=1))
    fi
    
    if [ "$(echo "$smcp" | jq -r '.spec.addons.kiali.enabled')" != "false" ]; then
        warning "Kiali addon is still enabled. Please disable the addon by setting '.spec.addons.kiali.enabled' = false"
        ((num_warnings+=1))
    fi
    
    if [ "$(echo "$smcp" | jq -r '.spec.addons.grafana.enabled')" != "false" ]; then
        warning "Grafana addon is enabled. Grafana is no longer supported with Service Mesh 3.x."
        ((num_warnings+=1))
    fi

    if [ "$(echo "$smcp" | jq -r '.spec.tracing.type')" != "None" ]; then
        warning "Tracing addon is still enabled. Please disable the addon by setting '.spec.tracing.type' = None"
        ((num_warnings+=1))
    fi

    if [ "$(echo "$smcp" | jq -r '.spec.gateways.enabled')" != "false" ]; then
        warning "Gateways are still enabled. Please disable gateways by setting '.spec.gateways.enabled' = false"
        ((num_warnings+=1))
    fi

    if [ "$num_warnings" -gt 0 ]; then
        echo -e "\n${YELLOW}$num_warnings warnings${BLANK}"
    else
        success "No issues detected with the ServiceMeshControlPlane $name/$namespace."
    fi

    ((TOTAL_WARNINGS += num_warnings))
}

# Find smcps and check each one.
# Format is name/namespace.
for smcp in $(oc get smcp -A -o jsonpath='{range .items[*]}{.metadata.name}/{.metadata.namespace}{" "}{end}'); do
    IFS="/" read -r name namespace <<< "$smcp"
    check_smcp "$name" "$namespace"
done

echo -e "-----------------------"

if [ "$TOTAL_WARNINGS" -eq 0 ]; then
    success "No issues detected. Proceed with the 2.6 --> 3.0 migration."
else
    warning "Detected $TOTAL_WARNINGS issues with installed ServiceMeshControlPlanes. Please fix these before proceeding with the migration."
fi
