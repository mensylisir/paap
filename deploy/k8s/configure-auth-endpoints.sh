#!/bin/bash
set -euo pipefail

NAMESPACE="${PAAP_NAMESPACE:-paap-system}"
KUBECONFIG_PATH="${KUBECONFIG:-}"
DRY_RUN="${PAAP_AUTH_CONFIG_DRY_RUN:-false}"
WAIT_ROLLOUT="${PAAP_AUTH_CONFIG_WAIT:-true}"
KUBECTL_BIN="${KUBECTL_BIN:-kubectl}"

usage() {
  cat <<'USAGE'
Usage: deploy/k8s/configure-auth-endpoints.sh [--namespace <namespace>] [--kubeconfig <path>] [--dry-run]

Configures browser-visible PAAP/Keycloak URLs and optional runtime registry host
template for the current cluster.

Overrides:
  PAAP_PUBLIC_URL       Full PAAP URL, for example https://paap.example.com
  KEYCLOAK_PUBLIC_URL   Full Keycloak URL, for example https://auth.example.com
  KEYCLOAK_BACKCHANNEL_ISSUER_URL
                        Full server-to-server Keycloak issuer URL, default
                        http://paap-keycloak.<namespace>.svc.cluster.local:8080/realms/<realm>
  PAAP_ACCESS_HOST      Host/IP used with the detected PAAP NodePort
  KEYCLOAK_ACCESS_HOST  Host/IP used with the detected Keycloak NodePort
  PUBLIC_ACCESS_HOST    Fallback host/IP for both PAAP and Keycloak
  PAAP_PUBLIC_SCHEME    Scheme for detected NodePort URLs, default http
  KEYCLOAK_PUBLIC_SCHEME Scheme for detected NodePort URLs, default http
  KEYCLOAK_REALM        Keycloak realm, default paap
  KEYCLOAK_CLIENT_ID    Keycloak client ID, default paap
  REGISTRY_HOST_TEMPLATE Host template written to PAAP_REGISTRY_HOST_TEMPLATE,
                         for example registry.{app}-{env}.example.com:5443
  REGISTRY_DOMAIN        Builds registry.{app}-{env}.<domain> when
                         REGISTRY_HOST_TEMPLATE is not set
  REGISTRY_PORT          Optional port appended when REGISTRY_DOMAIN is used
  PAAP_AUTH_CONFIG_WAIT  Wait for PAAP/Keycloak rollouts after updating env,
                         default true
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --namespace)
      NAMESPACE="$2"
      shift 2
      ;;
    --kubeconfig)
      KUBECONFIG_PATH="$2"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

kubectl_cmd() {
  if [[ -n "$KUBECONFIG_PATH" ]]; then
    "$KUBECTL_BIN" --kubeconfig "$KUBECONFIG_PATH" "$@"
  else
    "$KUBECTL_BIN" "$@"
  fi
}

die() {
  echo "ERROR: $*" >&2
  exit 1
}

trim_trailing_slash() {
  local value="${1:-}"
  while [[ "$value" == */ ]]; do
    value="${value%/}"
  done
  printf '%s' "$value"
}

require_url() {
	local name="$1"
	local value="$2"
	case "$value" in
    http://*|https://*) ;;
    *) die "$name must include http:// or https://: $value" ;;
	esac
}

require_host_template() {
	local name="$1"
	local value="$2"
	if [[ -z "$value" ]]; then
		return 0
	fi
	case "$value" in
	http://* | https://*) die "$name must be a host template only, not a URL: $value" ;;
	esac
	if [[ "$value" != *"{app}"* || "$value" != *"{env}"* ]]; then
		die "$name must include both {app} and {env}: $value"
	fi
}

first_line() {
	awk 'NF { print; exit }'
}

discover_node_host() {
  local external internal
  external="$(kubectl_cmd get nodes -o jsonpath='{range .items[*]}{range .status.addresses[?(@.type=="ExternalIP")]}{.address}{"\n"}{end}{end}' 2>/dev/null | first_line || true)"
  if [[ -n "$external" ]]; then
    printf '%s' "$external"
    return 0
  fi

  internal="$(kubectl_cmd get nodes -o jsonpath='{range .items[*]}{range .status.addresses[?(@.type=="InternalIP")]}{.address}{"\n"}{end}{end}' 2>/dev/null | first_line || true)"
  if [[ -n "$internal" ]]; then
    printf '%s' "$internal"
    return 0
  fi

  return 1
}

service_node_port() {
  local service="$1"
  local jsonpath="$2"
  local port

  port="$(kubectl_cmd -n "$NAMESPACE" get svc "$service" -o "jsonpath=$jsonpath" 2>/dev/null || true)"
  if [[ -z "$port" ]]; then
    port="$(kubectl_cmd -n "$NAMESPACE" get svc "$service" -o 'jsonpath={.spec.ports[0].nodePort}' 2>/dev/null || true)"
  fi

  printf '%s' "$port"
}

build_nodeport_url() {
  local label="$1"
  local service="$2"
  local jsonpath="$3"
  local full_url="$4"
  local host="$5"
  local scheme="$6"
  local node_port

  if [[ -n "$full_url" ]]; then
    full_url="$(trim_trailing_slash "$full_url")"
    require_url "${label}_PUBLIC_URL" "$full_url"
    printf '%s' "$full_url"
    return 0
  fi

  if [[ -z "$host" ]]; then
    host="$(discover_node_host)" || die "cannot discover a node address; set ${label}_PUBLIC_URL or ${label}_ACCESS_HOST"
  fi
  case "$host" in
    http://*|https://*) die "${label}_ACCESS_HOST/PUBLIC_ACCESS_HOST must be a host only; use ${label}_PUBLIC_URL for a full URL" ;;
  esac

  node_port="$(service_node_port "$service" "$jsonpath")"
  if [[ -z "$node_port" ]]; then
    die "cannot discover NodePort for service $service; set ${label}_PUBLIC_URL"
  fi

  printf '%s://%s:%s' "$scheme" "$host" "$node_port"
}

set_deployment_env() {
	local deployment="$1"
	shift
	if [[ "$DRY_RUN" == "true" ]]; then
		printf 'DRY-RUN set env deployment/%s' "$deployment"
		for item in "$@"; do
			printf ' %q' "$item"
		done
		printf '\n'
		return 0
	fi
	kubectl_cmd -n "$NAMESPACE" set env "deployment/$deployment" "$@"
}

apply_runtime_configmap() {
	if [[ "$DRY_RUN" == "true" ]]; then
		cat <<EOF
DRY-RUN apply configmap/paap-runtime-endpoints
PAAP_PUBLIC_URL=$PAAP_PUBLIC_URL_RESOLVED
KEYCLOAK_PUBLIC_URL=$KEYCLOAK_PUBLIC_URL_RESOLVED
KEYCLOAK_ISSUER_URL=$KEYCLOAK_ISSUER_URL_RESOLVED
KEYCLOAK_BACKCHANNEL_ISSUER_URL=$KEYCLOAK_BACKCHANNEL_ISSUER_URL_RESOLVED
KEYCLOAK_REDIRECT_URL=$KEYCLOAK_REDIRECT_URL_RESOLVED
EOF
		return 0
	fi
	kubectl_cmd -n "$NAMESPACE" create configmap paap-runtime-endpoints \
		--from-literal=PAAP_PUBLIC_URL="$PAAP_PUBLIC_URL_RESOLVED" \
		--from-literal=KEYCLOAK_PUBLIC_URL="$KEYCLOAK_PUBLIC_URL_RESOLVED" \
		--from-literal=KEYCLOAK_ISSUER_URL="$KEYCLOAK_ISSUER_URL_RESOLVED" \
		--from-literal=KEYCLOAK_BACKCHANNEL_ISSUER_URL="$KEYCLOAK_BACKCHANNEL_ISSUER_URL_RESOLVED" \
		--from-literal=KEYCLOAK_REDIRECT_URL="$KEYCLOAK_REDIRECT_URL_RESOLVED" \
		--dry-run=client -o yaml | kubectl_cmd apply -f -
}

wait_for_rollouts() {
	if [[ "$DRY_RUN" == "true" || "$WAIT_ROLLOUT" != "true" ]]; then
		return 0
	fi
	kubectl_cmd -n "$NAMESPACE" rollout status deployment/paap-keycloak --timeout=180s
	kubectl_cmd -n "$NAMESPACE" rollout status deployment/paap-server --timeout=180s
}

KEYCLOAK_REALM="${KEYCLOAK_REALM:-paap}"
KEYCLOAK_CLIENT_ID="${KEYCLOAK_CLIENT_ID:-paap}"
REGISTRY_HOST_TEMPLATE_RESOLVED="${REGISTRY_HOST_TEMPLATE:-}"
if [[ -z "$REGISTRY_HOST_TEMPLATE_RESOLVED" && -n "${REGISTRY_DOMAIN:-}" ]]; then
	REGISTRY_HOST_TEMPLATE_RESOLVED="registry.{app}-{env}.${REGISTRY_DOMAIN}"
	if [[ -n "${REGISTRY_PORT:-}" ]]; then
		REGISTRY_HOST_TEMPLATE_RESOLVED="${REGISTRY_HOST_TEMPLATE_RESOLVED}:${REGISTRY_PORT}"
	fi
fi
require_host_template "REGISTRY_HOST_TEMPLATE/REGISTRY_DOMAIN" "$REGISTRY_HOST_TEMPLATE_RESOLVED"

PAAP_SCHEME="${PAAP_PUBLIC_SCHEME:-http}"
KEYCLOAK_SCHEME="${KEYCLOAK_PUBLIC_SCHEME:-http}"
SHARED_ACCESS_HOST="${PUBLIC_ACCESS_HOST:-}"
PAAP_HOST="${PAAP_ACCESS_HOST:-$SHARED_ACCESS_HOST}"
KEYCLOAK_HOST="${KEYCLOAK_ACCESS_HOST:-$SHARED_ACCESS_HOST}"

PAAP_PUBLIC_URL_RESOLVED="$(build_nodeport_url "PAAP" "paap-server" '{.spec.ports[?(@.port==9090)].nodePort}' "${PAAP_PUBLIC_URL:-}" "$PAAP_HOST" "$PAAP_SCHEME")"
KEYCLOAK_PUBLIC_URL_RESOLVED="$(build_nodeport_url "KEYCLOAK" "paap-keycloak" '{.spec.ports[?(@.name=="http")].nodePort}' "${KEYCLOAK_PUBLIC_URL:-}" "$KEYCLOAK_HOST" "$KEYCLOAK_SCHEME")"

KEYCLOAK_ISSUER_URL_RESOLVED="${KEYCLOAK_PUBLIC_URL_RESOLVED}/realms/${KEYCLOAK_REALM}"
KEYCLOAK_BACKCHANNEL_ISSUER_URL_RESOLVED="$(trim_trailing_slash "${KEYCLOAK_BACKCHANNEL_ISSUER_URL:-http://paap-keycloak.${NAMESPACE}.svc.cluster.local:8080/realms/${KEYCLOAK_REALM}}")"
require_url "KEYCLOAK_BACKCHANNEL_ISSUER_URL" "$KEYCLOAK_BACKCHANNEL_ISSUER_URL_RESOLVED"
KEYCLOAK_REDIRECT_URL_RESOLVED="${PAAP_PUBLIC_URL_RESOLVED}/api/v1/auth/keycloak/callback"

echo "Configuring auth endpoints for namespace $NAMESPACE..."
echo "  PAAP_PUBLIC_URL=$PAAP_PUBLIC_URL_RESOLVED"
echo "  KEYCLOAK_PUBLIC_URL=$KEYCLOAK_PUBLIC_URL_RESOLVED"
echo "  KEYCLOAK_ISSUER_URL=$KEYCLOAK_ISSUER_URL_RESOLVED"
echo "  KEYCLOAK_BACKCHANNEL_ISSUER_URL=$KEYCLOAK_BACKCHANNEL_ISSUER_URL_RESOLVED"
echo "  KEYCLOAK_REDIRECT_URL=$KEYCLOAK_REDIRECT_URL_RESOLVED"
if [[ -n "$REGISTRY_HOST_TEMPLATE_RESOLVED" ]]; then
	echo "  PAAP_REGISTRY_HOST_TEMPLATE=$REGISTRY_HOST_TEMPLATE_RESOLVED"
else
	echo "  PAAP_REGISTRY_HOST_TEMPLATE unchanged"
	echo "  NOTE: source builds require a node-reachable trusted registry host; set REGISTRY_HOST_TEMPLATE or REGISTRY_DOMAIN when enabling CI builds."
fi

set_deployment_env paap-keycloak \
	"KC_HOSTNAME=$KEYCLOAK_PUBLIC_URL_RESOLVED" \
	"KC_HOSTNAME_STRICT=false"

set_deployment_env paap-server \
	"KEYCLOAK_ISSUER_URL=$KEYCLOAK_ISSUER_URL_RESOLVED" \
	"KEYCLOAK_BACKCHANNEL_ISSUER_URL=$KEYCLOAK_BACKCHANNEL_ISSUER_URL_RESOLVED" \
	"KEYCLOAK_REDIRECT_URL=$KEYCLOAK_REDIRECT_URL_RESOLVED" \
	"KEYCLOAK_CLIENT_ID=$KEYCLOAK_CLIENT_ID"

if [[ -n "$REGISTRY_HOST_TEMPLATE_RESOLVED" ]]; then
	set_deployment_env paap-server \
		"PAAP_REGISTRY_HOST_TEMPLATE=$REGISTRY_HOST_TEMPLATE_RESOLVED"
fi

apply_runtime_configmap
wait_for_rollouts

echo "Runtime endpoints configured. Use PAAP_PUBLIC_URL, KEYCLOAK_PUBLIC_URL, and REGISTRY_HOST_TEMPLATE/REGISTRY_DOMAIN for DNS/Ingress deployments."
