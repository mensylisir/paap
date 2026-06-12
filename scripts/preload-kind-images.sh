#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
KIND_CLUSTER="${KIND_CLUSTER:-kind}"
IMAGE_LIST="${IMAGE_LIST:-$PROJECT_DIR/deploy/k8s/kind-images.txt}"
DISCOVERED_LIST="$(mktemp)"
FINAL_LIST="$(mktemp)"
CHART_WORKDIR="$(mktemp -d)"
trap 'rm -f "$DISCOVERED_LIST" "$FINAL_LIST"; rm -rf "$CHART_WORKDIR"' EXIT

echo "=== PAAP kind image preload ==="
echo "Cluster: $KIND_CLUSTER"
echo "Image list: $IMAGE_LIST"
echo "Template package source for image discovery: data/charts/*.tar.gz"
echo "Runtime chart source: data/charts/*.tar.gz copied into /charts and uploaded to MinIO."

extract_images_from_rendered_yaml() {
  awk '
    /^[[:space:]]*image:[[:space:]]*/ {
      value=$0
      sub(/^[[:space:]]*image:[[:space:]]*/, "", value)
      gsub(/["'\'' ]/, "", value)
      if (value != "" && value !~ /\{\{/) print value
    }
  '
}

discover_chart_images() {
  if ! command -v helm >/dev/null 2>&1; then
    echo "helm not found; skipping chart image discovery" >&2
    return
  fi

  for archive in "$PROJECT_DIR"/data/charts/*.tar.gz; do
    [ -f "$archive" ] || continue
    name="$(basename "$archive" .tar.gz)"
    target="$CHART_WORKDIR/$name"
    mkdir -p "$target"
    tar -xzf "$archive" -C "$target"
    chart="$target/chart"
    preset="$target/preset-values.yaml"
    [ -d "$chart" ] || continue
    args=(template "$name" "$chart" --namespace preload)
    if [ -f "$preset" ]; then
      args+=(--values "$preset")
    fi
    helm "${args[@]}" 2>/dev/null | extract_images_from_rendered_yaml || true
  done
}

{
  sed 's/#.*$//' "$IMAGE_LIST" | awk 'NF {print $1}'
  discover_chart_images > "$DISCOVERED_LIST"
  cat "$DISCOVERED_LIST"
} | sort -u > "$FINAL_LIST"

if [ "${PULL_IMAGES:-true}" = "true" ]; then
  while read -r image; do
    [ -n "$image" ] || continue
    if docker image inspect "$image" >/dev/null 2>&1; then
      echo "local: $image"
      continue
    fi
    if [[ "$image" == paap-* ]]; then
      echo "missing local PAAP image: $image" >&2
      echo "Build PAAP images first, for example: make docker-build-server docker-build-operator" >&2
      exit 1
    fi
    echo "pull:  $image"
    docker pull "$image"
  done < "$FINAL_LIST"
else
  echo "PULL_IMAGES=false; verifying all images already exist locally"
  missing=0
  while read -r image; do
    [ -n "$image" ] || continue
    if ! docker image inspect "$image" >/dev/null 2>&1; then
      echo "missing local image: $image" >&2
      missing=1
    fi
  done < "$FINAL_LIST"
  if [ "$missing" -ne 0 ]; then
    echo "Some images are missing locally. Pull them on the host or run with PULL_IMAGES=true before deploying." >&2
    exit 1
  fi
fi

echo "Loading images into kind cluster '$KIND_CLUSTER'..."
while read -r image; do
  [ -n "$image" ] || continue
  kind load docker-image "$image" --name "$KIND_CLUSTER"
done < "$FINAL_LIST"

echo "Loaded $(wc -l < "$FINAL_LIST" | tr -d ' ') images into kind."
