#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"

assert_contains() {
	local haystack="$1"
	local needle="$2"
	if [[ "$haystack" != *"$needle"* ]]; then
		echo "missing expected text: $needle" >&2
		echo "--- output ---" >&2
		echo "$haystack" >&2
		exit 1
	fi
}

with_fake_kubectl() {
	local mode="$1"
	shift
	local tmp_dir
	tmp_dir="$(mktemp -d)"
	trap 'rm -rf "$tmp_dir"' RETURN
	cat >"$tmp_dir/kubectl" <<'FAKE'
#!/bin/bash
set -euo pipefail
args="$*"
mode="${FAKE_KUBECTL_MODE:-dynamic}"

if [[ "$args" == *"get nodes"* && "$args" == *"ExternalIP"* ]]; then
	exit 0
fi
if [[ "$args" == *"get nodes"* && "$args" == *"InternalIP"* ]]; then
	echo "10.10.0.9"
	exit 0
fi
if [[ "$args" == *"-n paap-system get svc paap-server"* ]]; then
	if [[ "$mode" == "explicit-url" ]]; then
		echo "unexpected service lookup for explicit URL mode" >&2
		exit 2
	fi
	echo "31091"
	exit 0
fi
if [[ "$args" == *"-n paap-system get svc paap-keycloak"* ]]; then
	if [[ "$mode" == "explicit-url" ]]; then
		echo "unexpected service lookup for explicit URL mode" >&2
		exit 2
	fi
	echo "31080"
	exit 0
fi
echo "fake kubectl received unexpected args: $args" >&2
exit 2
FAKE
	chmod +x "$tmp_dir/kubectl"
	PATH="$tmp_dir:$PATH" FAKE_KUBECTL_MODE="$mode" "$@"
}

test_detects_node_ip_and_nodeports() {
	local output
	output="$(with_fake_kubectl dynamic "$SCRIPT_DIR/configure-auth-endpoints.sh" --dry-run)"
	assert_contains "$output" "PAAP_PUBLIC_URL=http://10.10.0.9:31091"
	assert_contains "$output" "KEYCLOAK_PUBLIC_URL=http://10.10.0.9:31080"
	assert_contains "$output" "KEYCLOAK_ISSUER_URL=http://10.10.0.9:31080/realms/paap"
	assert_contains "$output" "KEYCLOAK_BACKCHANNEL_ISSUER_URL=http://paap-keycloak.paap-system.svc.cluster.local:8080/realms/paap"
	assert_contains "$output" "KEYCLOAK_REDIRECT_URL=http://10.10.0.9:31091/api/v1/auth/keycloak/callback"
	assert_contains "$output" "DRY-RUN set env deployment/paap-keycloak"
	assert_contains "$output" "DRY-RUN apply configmap/paap-runtime-endpoints"
}

test_uses_explicit_domain_urls_without_nodeport_lookup() {
	local output
	output="$(PAAP_PUBLIC_URL=https://paap.example.test KEYCLOAK_PUBLIC_URL=https://auth.example.test with_fake_kubectl explicit-url "$SCRIPT_DIR/configure-auth-endpoints.sh" --dry-run)"
	assert_contains "$output" "PAAP_PUBLIC_URL=https://paap.example.test"
	assert_contains "$output" "KEYCLOAK_PUBLIC_URL=https://auth.example.test"
	assert_contains "$output" "KEYCLOAK_ISSUER_URL=https://auth.example.test/realms/paap"
	assert_contains "$output" "KEYCLOAK_BACKCHANNEL_ISSUER_URL=http://paap-keycloak.paap-system.svc.cluster.local:8080/realms/paap"
	assert_contains "$output" "KEYCLOAK_REDIRECT_URL=https://paap.example.test/api/v1/auth/keycloak/callback"
}

test_accepts_explicit_backchannel_issuer() {
	local output
	output="$(PAAP_PUBLIC_URL=https://paap.example.test KEYCLOAK_PUBLIC_URL=https://auth.example.test KEYCLOAK_BACKCHANNEL_ISSUER_URL=http://keycloak-internal.auth.svc:8080/realms/paap with_fake_kubectl explicit-url "$SCRIPT_DIR/configure-auth-endpoints.sh" --dry-run)"
	assert_contains "$output" "KEYCLOAK_BACKCHANNEL_ISSUER_URL=http://keycloak-internal.auth.svc:8080/realms/paap"
}

test_detects_node_ip_and_nodeports
test_uses_explicit_domain_urls_without_nodeport_lookup
test_accepts_explicit_backchannel_issuer

echo "configure-auth-endpoints tests passed"
