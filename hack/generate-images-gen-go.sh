#!/bin/bash
#
# Script to generate pkg/install/images.gen.go from ossm/values.yaml annotations
# This is needed when PATCH_HELM_VALUES=false (vendor builds) where make gen doesn't
# automatically update images.gen.go
#
# Usage:
#   ./hack/generate-images-gen-go.sh [values_file_path]
#
# Arguments:
#   values_file_path  Path to values.yaml file (default: ossm/values.yaml)

set -euo pipefail

VALUES_FILE="${1:-ossm/values.yaml}"

if [ ! -f "${VALUES_FILE}" ]; then
    echo "❌ Values file not found: ${VALUES_FILE}"
    exit 1
fi

SCRIPTPATH="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT=$(cd "${SCRIPTPATH}/.." && pwd)
GENERATED_GO_FILE="${ROOT}/pkg/install/images.gen.go"

echo "Generating ${GENERATED_GO_FILE} from ${VALUES_FILE}"

# First, read existing images from the current images.gen.go file (if it exists)
declare -A EXISTING_ISTIOD_IMAGES
declare -A EXISTING_PROXY_IMAGES
declare -A EXISTING_CNI_IMAGES
declare -A EXISTING_ZTUNNEL_IMAGES

if [ -f "${GENERATED_GO_FILE}" ]; then
    echo "Reading existing images from ${GENERATED_GO_FILE}"

    # Parse existing Go file to extract current images
    # Format: "v1.30.1": { IstiodImage: "...", ...}
    current_version=""
    while IFS= read -r line; do
        # Match version line like: "v1.30.1": {
        if [[ "$line" =~ \"v([0-9.]+)\":[[:space:]]*\{ ]]; then
            current_version="${BASH_REMATCH[1]}"
        # Match IstiodImage line
        elif [[ "$line" =~ IstiodImage:[[:space:]]*\"(.+)\" ]] && [ -n "$current_version" ]; then
            EXISTING_ISTIOD_IMAGES["${current_version}"]="${BASH_REMATCH[1]}"
        # Match ProxyImage line
        elif [[ "$line" =~ ProxyImage:[[:space:]]*\"(.+)\" ]] && [ -n "$current_version" ]; then
            EXISTING_PROXY_IMAGES["${current_version}"]="${BASH_REMATCH[1]}"
        # Match CNIImage line
        elif [[ "$line" =~ CNIImage:[[:space:]]*\"(.+)\" ]] && [ -n "$current_version" ]; then
            EXISTING_CNI_IMAGES["${current_version}"]="${BASH_REMATCH[1]}"
        # Match ZTunnelImage line
        elif [[ "$line" =~ ZTunnelImage:[[:space:]]*\"(.+)\" ]] && [ -n "$current_version" ]; then
            EXISTING_ZTUNNEL_IMAGES["${current_version}"]="${BASH_REMATCH[1]}"
        # Reset version on closing brace
        elif [[ "$line" =~ ^[[:space:]]*\},[[:space:]]*$ ]]; then
            current_version=""
        fi
    done < "${GENERATED_GO_FILE}"
fi

# Extract all image annotations from values.yaml
# Format: images.v1_30_1.istiod: registry.redhat.io/...
# We need to parse these and group them by version

declare -A VERSIONS
declare -A ISTIOD_IMAGES
declare -A PROXY_IMAGES
declare -A CNI_IMAGES
declare -A ZTUNNEL_IMAGES

# Helper function to check if a value is a placeholder
is_placeholder() {
    local value="$1"
    [[ "$value" =~ ^\$\{ ]]
}

# Parse annotations using grep and awk
while IFS= read -r line; do
    # Extract version and component from annotation key
    # Example: images.v1_30_1.istiod: registry.redhat.io/...
    if [[ "$line" =~ images\.v([0-9_]+)\.(istiod|proxy|cni|ztunnel):[[:space:]]*(.+)$ ]]; then
        version_underscore="${BASH_REMATCH[1]}"
        component="${BASH_REMATCH[2]}"
        image="${BASH_REMATCH[3]}"

        # Convert version from underscore to dot notation (1_30_1 -> 1.30.1)
        version="${version_underscore//_/.}"

        # Store the version
        VERSIONS["${version}"]=1

        # If image is a placeholder (${...}), use existing image from current file
        # Otherwise use the new image from values.yaml
        if is_placeholder "${image}"; then
            # Use existing image if available, otherwise skip this component
            case "$component" in
                "istiod")
                    if [ -n "${EXISTING_ISTIOD_IMAGES[$version]}" ]; then
                        ISTIOD_IMAGES["${version}"]="${EXISTING_ISTIOD_IMAGES[$version]}"
                    fi
                    ;;
                "proxy")
                    if [ -n "${EXISTING_PROXY_IMAGES[$version]}" ]; then
                        PROXY_IMAGES["${version}"]="${EXISTING_PROXY_IMAGES[$version]}"
                    fi
                    ;;
                "cni")
                    if [ -n "${EXISTING_CNI_IMAGES[$version]}" ]; then
                        CNI_IMAGES["${version}"]="${EXISTING_CNI_IMAGES[$version]}"
                    fi
                    ;;
                "ztunnel")
                    if [ -n "${EXISTING_ZTUNNEL_IMAGES[$version]}" ]; then
                        ZTUNNEL_IMAGES["${version}"]="${EXISTING_ZTUNNEL_IMAGES[$version]}"
                    fi
                    ;;
            esac
        else
            # Use the new image from values.yaml
            case "$component" in
                "istiod")
                    ISTIOD_IMAGES["${version}"]="${image}"
                    ;;
                "proxy")
                    PROXY_IMAGES["${version}"]="${image}"
                    ;;
                "cni")
                    CNI_IMAGES["${version}"]="${image}"
                    ;;
                "ztunnel")
                    ZTUNNEL_IMAGES["${version}"]="${image}"
                    ;;
            esac
        fi
    fi
done < <(grep -E "images\.v[0-9_]+\.(istiod|proxy|cni|ztunnel):" "${VALUES_FILE}")

# Check if we found any images
if [ ${#VERSIONS[@]} -eq 0 ]; then
    echo "❌ No image annotations found in ${VALUES_FILE}"
    echo "   Expected format: images.vX_Y_Z.component: image-url"
    exit 1
fi

# Generate the Go file
cat common/scripts/copyright-banner-go.txt > "${GENERATED_GO_FILE}"
cat >> "${GENERATED_GO_FILE}" << 'GOHEADER'
// Code generated by hack/generate-images-gen-go.sh. DO NOT EDIT.

package install

import "github.com/istio-ecosystem/sail-operator/pkg/config"

func init() {
	config.Config.ImageDigests = map[string]config.IstioImageConfig{
GOHEADER

# Sort versions in reverse order (newest to oldest)
for version in $(printf '%s\n' "${!VERSIONS[@]}" | sort -Vr); do
    istiod="${ISTIOD_IMAGES[$version]:-}"
    proxy="${PROXY_IMAGES[$version]:-}"
    cni="${CNI_IMAGES[$version]:-}"
    ztunnel="${ZTUNNEL_IMAGES[$version]:-}"

    # Validate all components are present
    if [ -z "$istiod" ] || [ -z "$proxy" ] || [ -z "$cni" ] || [ -z "$ztunnel" ]; then
        echo "⚠️  Warning: Missing components for version ${version}"
        echo "   istiod: ${istiod:-MISSING}"
        echo "   proxy: ${proxy:-MISSING}"
        echo "   cni: ${cni:-MISSING}"
        echo "   ztunnel: ${ztunnel:-MISSING}"
        continue
    fi

    cat >> "${GENERATED_GO_FILE}" << GOENTRY
		"v${version}": {
			IstiodImage:  "${istiod}",
			ProxyImage:   "${proxy}",
			CNIImage:     "${cni}",
			ZTunnelImage: "${ztunnel}",
		},
GOENTRY
done

cat >> "${GENERATED_GO_FILE}" << 'GOFOOTER'
	}
}
GOFOOTER

echo "✅ Generated ${GENERATED_GO_FILE}"
echo "   Found ${#VERSIONS[@]} versions"
