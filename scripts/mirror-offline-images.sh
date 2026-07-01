#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
CONTAINER_CLI="${CONTAINER_CLI:-docker}"
SYSTEM_REGISTRY="${PAAP_SYSTEM_REGISTRY:-}"
LIST_FILE=""
DRY_RUN=false
ALLOW_FLOATING="${PAAP_OFFLINE_ALLOW_FLOATING_IMAGES:-false}"

usage() {
  cat <<'USAGE'
Usage: scripts/mirror-offline-images.sh --registry <host:port> [--list <file>] [--dry-run]

Pulls PAAP platform/template/addon images and pushes them into a system registry
using the repository path expected by containerd host mirrors.

Examples:
  scripts/discover-offline-images.sh > offline-images.txt
  scripts/mirror-offline-images.sh --registry 172.18.0.2:30500 --list offline-images.txt

Environment:
  PAAP_SYSTEM_REGISTRY                 Default target registry host:port
  CONTAINER_CLI                        docker/podman-compatible CLI, default docker
  PAAP_OFFLINE_ALLOW_FLOATING_IMAGES   Allow latest or untagged refs, default false
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

image_has_explicit_reference() {
  local image="$1"
  local without_digest="${image%@*}"
  local last_segment="${without_digest##*/}"
  [[ "$image" == *@sha256:* || "$last_segment" == *:* ]]
}

image_reference_tag() {
  local image="$1"
  local digest="${image#*@sha256:}"
  local without_digest="${image%@*}"
  local last_segment="${without_digest##*/}"
  if [[ "$image" == *@sha256:* && "$digest" != "$image" ]]; then
    printf 'sha256-%.12s' "$digest"
    return 0
  fi
  if [[ "$last_segment" == *:* ]]; then
    printf '%s' "${last_segment##*:}"
    return 0
  fi
  printf 'latest'
}

image_repository_path_for_mirror() {
  local image="$1"
  local without_digest="${image%@*}"
  local without_tag="$without_digest"
  local first path last_segment

  last_segment="${without_tag##*/}"
  if [[ "$last_segment" == *:* ]]; then
    without_tag="${without_tag%:*}"
  fi

  first="${without_tag%%/*}"
  if [[ "$without_tag" == */* && ( "$first" == *.* || "$first" == *:* || "$first" == "localhost" ) ]]; then
    path="${without_tag#*/}"
    if [[ "$first" == "docker.io" && "$path" != */* ]]; then
      path="library/$path"
    fi
  else
    path="$without_tag"
    if [[ "$path" != */* ]]; then
      path="library/$path"
    fi
  fi
  printf '%s' "$path"
}

mirror_target_for_image() {
  local image="$1"
  local path tag
  path="$(image_repository_path_for_mirror "$image")"
  tag="$(image_reference_tag "$image")"
  printf '%s/%s:%s' "$SYSTEM_REGISTRY" "$path" "$tag"
}

pull_with_retry() {
  local image="$1"
  local attempts="${PULL_RETRIES:-3}"
  local attempt=1
  while true; do
    if "$CONTAINER_CLI" pull "$image"; then
      return 0
    fi
    if [[ "$attempt" -ge "$attempts" ]]; then
      return 1
    fi
    echo "retry: $image ($attempt/$attempts failed)" >&2
    sleep $((attempt * 3))
    attempt=$((attempt + 1))
  done
}

declare -A seen_targets

while IFS= read -r image; do
  image="${image%%#*}"
  image="$(awk '{$1=$1; print}' <<<"$image")"
  [[ -n "$image" ]] || continue

  if [[ "$ALLOW_FLOATING" != "true" ]]; then
    if ! image_has_explicit_reference "$image" || [[ "$(image_reference_tag "$image")" == "latest" ]]; then
      echo "ERROR: floating image ref is not allowed in offline mirror list: $image" >&2
      exit 1
    fi
  fi

  target="$(mirror_target_for_image "$image")"
  if [[ -n "${seen_targets[$target]:-}" && "${seen_targets[$target]}" != "$image" ]]; then
    echo "ERROR: mirror target collision: ${seen_targets[$target]} and $image both map to $target" >&2
    exit 1
  fi
  seen_targets[$target]="$image"

  if [[ "$DRY_RUN" == "true" ]]; then
    echo "$image -> $target"
    continue
  fi

  echo "pull: $image"
  pull_with_retry "$image"
  echo "tag:  $image -> $target"
  "$CONTAINER_CLI" tag "$image" "$target"
  echo "push: $target"
  "$CONTAINER_CLI" push "$target"
done < "$LIST_FILE"
