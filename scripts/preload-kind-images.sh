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

docker_pull_with_retry() {
  local pull_image="$1"
  local attempts="${PULL_RETRIES:-3}"
  local attempt=1
  while true; do
    if docker pull "$pull_image"; then
      return
    fi
    if [ "$attempt" -ge "$attempts" ]; then
      return 1
    fi
    echo "retry: $pull_image ($attempt/$attempts failed)" >&2
    sleep $((attempt * 3))
    attempt=$((attempt + 1))
  done
}

pull_or_tag_image() {
  local image="$1"
  local legacy_image
  if docker image inspect "$image" >/dev/null 2>&1; then
    echo "local: $image"
    return
  fi

  if [[ "$image" == paap-* ]]; then
    echo "missing local PAAP image: $image" >&2
    echo "Build PAAP images first, for example: make docker-build-server docker-build-operator" >&2
    exit 1
  fi

  # Older Bitnami chart tags were moved out of docker.io/bitnami, but the
  # pinned charts still render the original image names. Pull the legacy image
  # and retag it back to the name Kubernetes will request inside kind.
  if [[ "$image" == docker.io/bitnami/* ]]; then
    legacy_image="${image/docker.io\/bitnami\//docker.io\/bitnamilegacy\/}"
    if ! docker image inspect "$legacy_image" >/dev/null 2>&1; then
      echo "pull:  $legacy_image (for $image)"
      docker_pull_with_retry "$legacy_image"
    else
      echo "local: $legacy_image (for $image)"
    fi
    docker tag "$legacy_image" "$image"
    echo "tag:   $legacy_image -> $image"
    return
  fi

  echo "pull:  $image"
  docker_pull_with_retry "$image"
}

load_image_into_kind() {
  local image="$1"
  local node
  if [ "${KIND_LOAD_MODE:-direct}" = "kind" ]; then
    if kind load docker-image "$image" --name "$KIND_CLUSTER"; then
      return
    fi
    echo "kind load failed for $image; falling back to docker save + ctr import" >&2
  fi

  while read -r node; do
    [ -n "$node" ] || continue
    echo "load:  $image -> $node"
    docker save "$image" | docker exec --privileged -i "$node" \
      ctr --namespace=k8s.io images import --snapshotter=overlayfs -
  done < <(kind get nodes --name "$KIND_CLUSTER")
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
    pull_or_tag_image "$image"
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
  load_image_into_kind "$image"
done < "$FINAL_LIST"

echo "Loaded $(wc -l < "$FINAL_LIST" | tr -d ' ') images into kind."
