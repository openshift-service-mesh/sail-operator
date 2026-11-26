VERSION = 3.0.1
OPERATOR_NAME = servicemeshoperator3
CHANNELS = "stable,stable-3.0"
DEFAULT_CHANNEL = stable
HELM_VALUES_FILE = ossm/values.yaml
USE_IMAGE_DIGESTS = false
PATCH_HELM_VALUES = false
OCP = true

# Regenerate the vendor defaults whenever anything else is generated, so that a
# versions.ossm.yaml bump cannot leave pkg/istiovalues/vendor_defaults.yaml stale.
gen-all-except-bundle: gen-vendor-defaults

.PHONY: gen-vendor-defaults
gen-vendor-defaults: ## Expand ossm/vendor_defaults.src.yaml into pkg/istiovalues/vendor_defaults.yaml.
	@go run ./hack/gen_vendor_defaults -versions $(VERSIONS_YAML_DIR)/$(VERSIONS_YAML_FILE)

.PHONY: build-fips
build-fips: ## Build sail-operator binary for FIPS mode.
	GOARCH="$(TARGET_ARCH)" CGO_ENABLED=1 LDFLAGS="$(LD_FLAGS) -tags strictfipsruntime" common/scripts/gobuild.sh $(REPO_ROOT)/out/$(TARGET_OS)_$(TARGET_ARCH)/sail-operator cmd/main.go
