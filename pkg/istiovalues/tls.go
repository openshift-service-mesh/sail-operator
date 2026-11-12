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

package istiovalues

import (
	"crypto/tls"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	v1 "github.com/istio-ecosystem/sail-operator/api/v1"
	"github.com/istio-ecosystem/sail-operator/pkg/config"
	configv1 "github.com/openshift/api/config/v1"
	openshiftcrypto "github.com/openshift/library-go/pkg/crypto"

	"istio.io/istio/pkg/log"
)

var (
	istio1_29 = semver.MustParse("1.29.0")
	istio1_30 = semver.MustParse("1.30.0")
)

// ApplyTLSConfig applies TLS configuration to the Istio values.
// If TLS settings are already set, they are not overridden.
func ApplyTLSConfig(tlsConfig *config.TLSConfig, istioVersion string, values *v1.Values) {
	if values == nil {
		return
	}

	// ECDH curves are handled independently of the cipher suites, so this runs before the
	// early return below.
	applyECDHCurves(tlsConfig, istioVersion, values)

	if tlsConfig == nil || len(tlsConfig.CipherSuites) == 0 {
		return
	}

	cipherNames := make([]string, len(tlsConfig.CipherSuites))
	for i, id := range tlsConfig.CipherSuites {
		cipherNames[i] = tls.CipherSuiteName(id)
	}

	if values.MeshConfig == nil {
		values.MeshConfig = &v1.MeshConfig{}
	}

	if values.MeshConfig.TlsDefaults == nil {
		values.MeshConfig.TlsDefaults = &v1.MeshConfigTLSConfig{}
	}
	if len(values.MeshConfig.TlsDefaults.CipherSuites) == 0 {
		values.MeshConfig.TlsDefaults.CipherSuites = cipherNames
	}

	if values.MeshConfig.TlsDefaults.MinProtocolVersion == "" {
		values.MeshConfig.TlsDefaults.MinProtocolVersion = tlsProtocolVersion(tlsConfig.MinVersion)
	}

	if values.MeshConfig.MeshMTLS == nil {
		values.MeshConfig.MeshMTLS = &v1.MeshConfigTLSConfig{}
	}
	if len(values.MeshConfig.MeshMTLS.CipherSuites) == 0 {
		values.MeshConfig.MeshMTLS.CipherSuites = cipherNames
	}

	if values.MeshConfig.MeshMTLS.MinProtocolVersion == "" {
		values.MeshConfig.MeshMTLS.MinProtocolVersion = tlsProtocolVersion(tlsConfig.MinVersion)
	}

	// Envoy does not support setting TLS ciphers when minProtocolVersion is 1.3
	if values.MeshConfig.TlsDefaults.MinProtocolVersion == v1.MeshConfigTLSConfigTLSProtocolTlsv13 {
		values.MeshConfig.TlsDefaults.CipherSuites = nil
	}

	if values.MeshConfig.MeshMTLS.MinProtocolVersion == v1.MeshConfigTLSConfigTLSProtocolTlsv13 {
		values.MeshConfig.MeshMTLS.CipherSuites = nil
	}

	if tlsConfig.OpenShift != nil {
		// Envoy does not support setting TLS ciphers for TLS 1.3 BUT the openssl
		// backend does allow you to customize these. On openshift the envoy image has a custom openssl
		// config file baked in that will populate the tls1.3 cipher suites through an env var.
		// Also important to note is that all non-FIPS approved ciphers will be silently dropped by
		// openssl, even if they are specified in the openssl config. So we do not need to filter out
		// FIPS ciphers here. We can pass them to the conf and openssl will drop them automatically on
		// a FIPS enabled cluster.
		if values.MeshConfig.DefaultConfig == nil {
			values.MeshConfig.DefaultConfig = &v1.MeshConfigProxyConfig{}
		}
		if values.MeshConfig.DefaultConfig.ProxyMetadata == nil {
			values.MeshConfig.DefaultConfig.ProxyMetadata = map[string]string{}
		}
		if _, ok := values.MeshConfig.DefaultConfig.ProxyMetadata["OPENSSL_TLS1_3_CIPHERSUITES"]; !ok {
			values.MeshConfig.DefaultConfig.ProxyMetadata["OPENSSL_TLS1_3_CIPHERSUITES"] = strings.Join(cipherNames, ":")
		}
	}

	if values.Pilot == nil {
		values.Pilot = &v1.PilotConfig{}
	}
	addExtraContainerArg(values.Pilot, "--tls-cipher-suites", strings.Join(cipherNames, ","))

	if minVersionName := tlsVersionName(tlsConfig.MinVersion); minVersionName != "" {
		v, err := semver.NewVersion(istioVersion)
		if err != nil {
			log.Warnf("failed to parse Istio version %q: %v", istioVersion, err)
		}

		// This flag is only supported on Istio 1.29+. TODO: Remove this check when we drop support for Istio 1.28
		if v.GreaterThanEqual(istio1_29) {
			addExtraContainerArg(values.Pilot, "--tls-min-version", minVersionName)
		} else {
			log.Infof("Istio version %q is less than 1.29 and --tls-min-version flag for istiod is only supported on Istio 1.29+. Skipping sync of flag.", istioVersion)
		}
	}
}

// ApplyZTunnelTLSConfig sets the MESH_CIPHER_SUITES env var on ztunnel.
// Only supported on Istio 1.30+. Existing user values are not overridden.
func ApplyZTunnelTLSConfig(tlsConfig *config.TLSConfig, istioVersion string, values *v1.ZTunnelValues) {
	if tlsConfig == nil || values == nil || len(tlsConfig.CipherSuites) == 0 {
		return
	}

	v, err := semver.NewVersion(istioVersion)
	if err != nil {
		log.Warnf("failed to parse Istio version %q: %v", istioVersion, err)
		return
	}

	if !v.GreaterThanEqual(istio1_30) {
		log.Infof("Istio version %q is less than 1.30; skipping MESH_CIPHER_SUITES sync to ztunnel", istioVersion)
		return
	}

	if values.ZTunnel == nil {
		values.ZTunnel = &v1.ZTunnelConfig{}
	}
	if values.ZTunnel.Env == nil {
		values.ZTunnel.Env = make(map[string]string)
	}

	if _, found := values.ZTunnel.Env["MESH_CIPHER_SUITES"]; found {
		return
	}

	cipherNames := make([]string, len(tlsConfig.CipherSuites))
	for i, id := range tlsConfig.CipherSuites {
		cipherNames[i] = tls.CipherSuiteName(id)
	}
	values.ZTunnel.Env["MESH_CIPHER_SUITES"] = strings.Join(cipherNames, ",")
}

func tlsProtocolVersion(v uint16) v1.MeshConfigTLSConfigTLSProtocol {
	switch v {
	case tls.VersionTLS12:
		return v1.MeshConfigTLSConfigTLSProtocolTlsv12
	case tls.VersionTLS13:
		return v1.MeshConfigTLSConfigTLSProtocolTlsv13
	default:
		return ""
	}
}

func tlsVersionName(v uint16) string {
	switch v {
	case tls.VersionTLS12:
		return "1.2"
	case tls.VersionTLS13:
		return "1.3"
	default:
		return ""
	}
}

// addExtraContainerArg adds an argument to ExtraContainerArgs if not already present.
func addExtraContainerArg(pilot *v1.PilotConfig, argName, argValue string) {
	argNameWithEquals := argName + "="
	for _, arg := range pilot.ExtraContainerArgs {
		if arg == argName || strings.HasPrefix(arg, argNameWithEquals) {
			return
		}
	}
	pilot.ExtraContainerArgs = append(pilot.ExtraContainerArgs, argNameWithEquals+argValue)
}

// defaultECDHCurves mirrors the Groups of the predefined OpenShift profiles: Old,
// Intermediate and Modern all specify exactly these four (see TLSProfiles in
// github.com/openshift/api/config/v1). The names are already normalized here.
var defaultECDHCurves = []string{"X25519MLKEM768", "X25519", "P-256", "P-384"}

// applyECDHCurves sets meshConfig.tlsDefaults.ecdhCurves from the OpenShift TLS profile when
// the cluster profile is honored and provides groups, and otherwise falls back to
// defaultECDHCurves on Istio 1.30+. A user-supplied value is never overridden.
//
// The curves must be set explicitly: when the field is empty Envoy falls back to its own
// defaults, which don't offer X25519MLKEM768, and the curves in /etc/pki/tls/openssl.cnf are
// ignored by Envoy. Offering X25519MLKEM768 by default is required by
// https://redhat.atlassian.net/browse/OSSM-15468. This does not break FIPS, because
// downstream Istio drops the curves when the fips-140-3-redhat compliance policy is in use.
//
// Note that only meshConfig.tlsDefaults is set: Istio rejects the whole mesh config when
// meshConfig.meshMTLS.ecdhCurves is present ("mesh TLS does not support ECDH curves
// configuration"), so mesh mTLS curves cannot be configured this way.
func applyECDHCurves(tlsConfig *config.TLSConfig, istioVersion string, values *v1.Values) {
	// Only applies on OpenShift, where the proxy image is known to support the hybrid group.
	// Elsewhere tlsConfig is nil and Envoy's own defaults are left in place.
	if tlsConfig == nil || tlsConfig.OpenShift == nil {
		return
	}

	// The cluster TLS profile is consulted only when tlsAdherence requires it. Under
	// NoOpinion and LegacyAdheringComponentsOnly the profile's groups are ignored, just as
	// its ciphers are, and defaultECDHCurves applies instead.
	var curves []string
	if openshiftcrypto.ShouldHonorClusterTLSProfile(tlsConfig.OpenShift.TLSAdherencePolicy) {
		// Normalize the names, since OpenShift uses different names than NIST.
		curves = copyECDHCurvesToConfig(tlsConfig.OpenShift.TLSProfileSpec.Groups)
	}
	if len(curves) == 0 {
		// Our defaults are only applied on Istio 1.30+; on older versions Envoy's own
		// defaults are left in place when the profile supplies nothing.
		v, err := semver.NewVersion(istioVersion)
		if err != nil {
			log.Warnf("failed to parse Istio version %q: %v", istioVersion, err)
			return
		}
		if !v.GreaterThanEqual(istio1_30) {
			return
		}
		curves = slices.Clone(defaultECDHCurves)
	}

	if values.MeshConfig == nil {
		values.MeshConfig = &v1.MeshConfig{}
	}
	if values.MeshConfig.TlsDefaults == nil {
		values.MeshConfig.TlsDefaults = &v1.MeshConfigTLSConfig{}
	}
	if len(values.MeshConfig.TlsDefaults.EcdhCurves) == 0 {
		values.MeshConfig.TlsDefaults.EcdhCurves = curves
	}
}

// copyECDHCurvesToConfig creates a new slice containing the normalized ECDHCurves to be
// used on meshConfig fields
func copyECDHCurvesToConfig(groups []configv1.TLSGroup) []string {
	values := make([]string, len(groups))
	for i := range groups {
		values[i] = normalizeECDHCurve(string(groups[i]))
	}
	return values
}

// normalizeECDHCurve maps common curve aliases to Envoy-supported names
func normalizeECDHCurve(curve string) string {
	switch strings.ToLower(strings.TrimSpace(curve)) {
	case "secp256r1", "prime256v1", "p-256", "p256":
		return "P-256"
	case "secp384r1", "p-384", "p384":
		return "P-384"
	case "x25519":
		return "X25519"
	default:
		return curve // Return as-is if unmapped
	}
}
