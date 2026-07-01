#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CONTAINER_CLI="${CONTAINER_CLI:-docker}"
KIND_CLUSTER="${KIND_CLUSTER:-}"
SYSTEM_REGISTRY="${PAAP_SYSTEM_REGISTRY:-}"
SYSTEM_REGISTRY_SCHEME="${PAAP_SYSTEM_REGISTRY_SCHEME:-http}"
KUBECTL_BIN="${KUBECTL_BIN:-kubectl}"
LIST_FILE=""
DRY_RUN=false

usage() {
  cat <<'USAGE'
Usage: scripts/configure-kind-system-image-mirror.sh --registry <host:port> [--list <file>] [--dry-run]

Writes containerd hosts.toml mirror entries into every node of a kind cluster so
platform/template/addon image pulls for docker.io, quay.io, ghcr.io, etc. resolve
through the PAAP system registry.

The target registry must contain images mirrored with scripts/mirror-offline-images.sh.

Environment:
  KIND_CLUSTER                 kind cluster name, default from current context or "kind"
  CONTAINER_CLI                docker/podman-compatible CLI, default docker
  PAAP_SYSTEM_REGISTRY         default target registry host:port
  PAAP_SYSTEM_REGISTRY_SCHEME  mirror endpoint scheme, default http
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --registry)
      SYSTEM_REGISTRY="$2"
      shift 2
      ;;
    --list)
      LIST_FILE="$2"
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

if [[ -z "$SYSTEM_REGISTRY" ]]; then
  echo "ERROR: --registry or PAAP_SYSTEM_REGISTRY is required" >&2
  exit 1
fi
SYSTEM_REGISTRY="${SYSTEM_REGISTRY#http://}"
SYSTEM_REGISTRY="${SYSTEM_REGISTRY#https://}"
SYSTEM_REGISTRY="${SYSTEM_REGISTRY%/}"

tmp_list=""
if [[ -z "$LIST_FILE" ]]; then
  tmp_list="$(mktemp)"
  trap 'rm -f "$tmp_list"' EXIT
  "$SCRIPT_DIR/discover-offline-images.sh" > "$tmp_list"
  LIST_FILE="$tmp_list"
fi

if [[ -z "$KIND_CLUSTER" ]]; then
  current_context="$("$KUBECTL_BIN" config current-context 2>/dev/null || true)"
  if [[ "$current_context" == kind-* ]]; then
    KIND_CLUSTER="${current_context#kind-}"
  else
    KIND_CLUSTER="kind"
  fi
fi

image_registry_host() {
  local image="$1"
  local without_digest="${image%@*}"
  local without_tag="$without_digest"
  local first last_segment

  last_segment="${without_tag##*/}"
  if [[ "$last_segment" == *:* ]]; then
    without_tag="${without_tag%:*}"
  fi

  first="${without_tag%%/*}"
  if [[ "$without_tag" == */* && ( "$first" == *.* || "$first" == *:* || "$first" == "localhost" ) ]]; then
    printf '%s' "$first"
  else
    printf 'docker.io'
  fi
}

tmp_hosts="$(mktemp)"
trap 'rm -f "$tmp_hosts" ${tmp_list:+"$tmp_list"}' EXIT

while IFS= read -r image; do
  image="${image%%#*}"
  image="$(awk '{$1=$1; print}' <<<"$image")"
  [[ -n "$image" ]] || continue
  image_registry_host "$image"
  printf '\n'
done < "$LIST_FILE" | sort -u > "$tmp_hosts"

mirror_endpoint="${SYSTEM_REGISTRY_SCHEME}://${SYSTEM_REGISTRY}"

if [[ "$DRY_RUN" == "true" ]]; then
  while IFS= read -r host; do
    [[ -n "$host" ]] || continue
    echo "$host -> $mirror_endpoint"
  done < "$tmp_hosts"
  exit 0
fi

kind_label="${KIND_CLUSTER#kind-}"
node_containers="$("$CONTAINER_CLI" ps \
  --filter "label=io.x-k8s.kind.cluster=${kind_label}" \
  --format '{{.Names}}' | awk 'NF')"

if [[ -z "$node_containers" ]]; then
  echo "ERROR: no kind node containers found for cluster ${kind_label}" >&2
  exit 1
fi

echo "Configuring PAAP system image mirror for kind cluster ${kind_label}:"
while IFS= read -r host; do
  [[ -n "$host" ]] || continue
  echo "  $host -> $mirror_endpoint"
done < "$tmp_hosts"

for node in $node_containers; do
  echo "-> ${node}"
  while IFS= read -r host; do
    [[ -n "$host" ]] || continue
    "$CONTAINER_CLI" exec "$node" sh -ceu '
      host="$1"
      endpoint="$2"
      dir="/etc/containerd/certs.d/${host}"
      mkdir -p "$dir"
      cat > "${dir}/hosts.toml" <<EOF
server = "https://${host}"

[host."${endpoint}"]
  capabilities = ["pull", "resolve"]
  skip_verify = true
EOF
    ' sh "$host" "$mirror_endpoint"
  done < "$tmp_hosts"
done

echo "containerd mirror hosts.toml entries updated."
