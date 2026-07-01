#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="${PAAP_OFFLINE_PROJECT_DIR:-$(cd "$SCRIPT_DIR/.." && pwd)}"
IMAGE_LIST="${IMAGE_LIST:-$PROJECT_DIR/deploy/k8s/kind-images.txt}"

INCLUDE_STATIC="${PAAP_OFFLINE_INCLUDE_STATIC:-true}"
INCLUDE_DEPLOY_MANIFESTS="${PAAP_OFFLINE_INCLUDE_DEPLOY_MANIFESTS:-true}"
INCLUDE_CHARTS="${PAAP_OFFLINE_INCLUDE_CHARTS:-true}"
INCLUDE_CHART_HELM="${PAAP_OFFLINE_INCLUDE_CHART_HELM:-true}"
INCLUDE_CHART_TEXT="${PAAP_OFFLINE_INCLUDE_CHART_TEXT:-false}"
INCLUDE_PLATFORM_ADDONS="${PAAP_OFFLINE_INCLUDE_PLATFORM_ADDONS:-true}"
INCLUDE_SERVICE_TEMPLATES="${PAAP_OFFLINE_INCLUDE_SERVICE_TEMPLATES:-true}"
INCLUDE_CONFIG_TEMPLATES="${PAAP_OFFLINE_INCLUDE_CONFIG_TEMPLATES:-true}"

WORKDIR="$(mktemp -d)"
RAW_LIST="$(mktemp)"
trap 'rm -rf "$WORKDIR"; rm -f "$RAW_LIST"' EXIT

emit_static_list() {
  [ -f "$IMAGE_LIST" ] || return 0
  sed 's/#.*$//' "$IMAGE_LIST" | awk 'NF { print $1 }'
}

extract_images_from_text() {
  awk '
    function clean(value) {
      gsub(/^[[:space:]"'\''`]+/, "", value)
      gsub(/[[:space:],"'\''`]+$/, "", value)
      return value
    }
    function emit(value) {
      value = clean(value)
      if (value == "") return
      if (value ~ /\{\{/) return
      if (value ~ /\$\{/) return
      if (value ~ /__/) return
      if (value ~ /^https?:\/\//) return
      print value
    }
    /^[[:space:]-]*image:[[:space:]]*/ {
      value = $0
      sub(/^[[:space:]-]*image:[[:space:]]*/, "", value)
      emit(value)
    }
    /"image"[[:space:]]*:[[:space:]]*"/ {
      value = $0
      sub(/^.*"image"[[:space:]]*:[[:space:]]*"/, "", value)
      sub(/".*$/, "", value)
      emit(value)
    }
    /^[[:space:]]*FROM[[:space:]]+/ {
      emit($2)
    }
  '
}

scan_files() {
  local root="$1"
  [ -d "$root" ] || return 0
  find "$root" -type f \( \
    -name '*.yaml' -o -name '*.yml' -o -name '*.json' -o -name 'Dockerfile' -o -name '*.dockerfile' \
  \) -print0 | while IFS= read -r -d '' file; do
    extract_images_from_text < "$file"
  done
}

scan_tar_text() {
  local archive="$1"
  local entry
  tar -tzf "$archive" 2>/dev/null | while IFS= read -r entry; do
    case "$entry" in
      *.yaml|*.yml|*.json|*/Dockerfile|Dockerfile|*.dockerfile)
        tar -xOzf "$archive" "$entry" 2>/dev/null | extract_images_from_text || true
        ;;
    esac
  done
}

scan_archives() {
  local root="$1"
  [ -d "$root" ] || return 0
  find "$root" -type f -name '*.tar.gz' -print0 | while IFS= read -r -d '' archive; do
    scan_tar_text "$archive"
  done
}

discover_chart_images_with_helm() {
  if [ "$INCLUDE_CHART_HELM" != "true" ]; then
    return 0
  fi
  if ! command -v helm >/dev/null 2>&1; then
    echo "helm not found; skipping rendered chart image discovery" >&2
    return 0
  fi

  find "$PROJECT_DIR/data/charts" -type f -name '*.tar.gz' -print0 2>/dev/null | while IFS= read -r -d '' archive; do
    name="$(basename "$archive" .tar.gz)"
    target="$WORKDIR/chart-$name"
    mkdir -p "$target"
    tar -xzf "$archive" -C "$target" 2>/dev/null || continue
    chart="$target/chart"
    preset="$target/preset-values.yaml"
    [ -d "$chart" ] || continue
    args=(template "$name" "$chart" --namespace offline-discovery)
    if [ -f "$preset" ]; then
      args+=(--values "$preset")
    fi
    helm "${args[@]}" 2>/dev/null | extract_images_from_text || true
  done
}

if [ "$INCLUDE_STATIC" = "true" ]; then
  emit_static_list >> "$RAW_LIST"
fi

if [ "$INCLUDE_DEPLOY_MANIFESTS" = "true" ]; then
  scan_files "$PROJECT_DIR/deploy/k8s" >> "$RAW_LIST"
fi

if [ "$INCLUDE_CHARTS" = "true" ]; then
  if [ "$INCLUDE_CHART_TEXT" = "true" ]; then
    scan_archives "$PROJECT_DIR/data/charts" >> "$RAW_LIST"
  fi
  discover_chart_images_with_helm >> "$RAW_LIST"
fi

if [ "$INCLUDE_PLATFORM_ADDONS" = "true" ]; then
  scan_archives "$PROJECT_DIR/data/platform-addons" >> "$RAW_LIST"
fi

if [ "$INCLUDE_SERVICE_TEMPLATES" = "true" ]; then
  scan_files "$PROJECT_DIR/data/service-templates" >> "$RAW_LIST"
fi

if [ "$INCLUDE_CONFIG_TEMPLATES" = "true" ]; then
  scan_archives "$PROJECT_DIR/data/config-templates" >> "$RAW_LIST"
fi

awk 'NF { print $1 }' "$RAW_LIST" | sort -u
