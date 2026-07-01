#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

mkdir -p \
  "$tmp_dir/deploy/k8s" \
  "$tmp_dir/data/charts/demo/chart/templates" \
  "$tmp_dir/data/platform-addons/addon/manifests" \
  "$tmp_dir/data/service-templates/kubevirt" \
  "$tmp_dir/data/config-templates/config-template"

cat >"$tmp_dir/deploy/k8s/kind-images.txt" <<'EOF'
# static image list
paap-server:test
EOF

cat >"$tmp_dir/deploy/k8s/paap-server.yaml" <<'EOF'
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
        - name: server
          image: paap-server:test
EOF

cat >"$tmp_dir/data/charts/demo/platform-manifest.yaml" <<'EOF'
name: demo
version: test
description: demo
permissions:
  cluster: []
catalog:
  docs:
    overview: "# demo"
    install: "# install"
    quickstart: "# quickstart"
EOF

cat >"$tmp_dir/data/charts/demo/preset-values.yaml" <<'EOF'
image:
  tag: test
EOF

cat >"$tmp_dir/data/charts/demo/chart/Chart.yaml" <<'EOF'
apiVersion: v2
name: demo
version: 0.1.0
appVersion: "1.0.0"
EOF

cat >"$tmp_dir/data/charts/demo/chart/templates/deploy.yaml" <<'EOF'
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
        - name: chart
          image: docker.io/library/nginx:1.25
EOF
tar -czf "$tmp_dir/data/charts/demo.tar.gz" -C "$tmp_dir/data/charts/demo" .
rm -rf "$tmp_dir/data/charts/demo"

cat >"$tmp_dir/data/platform-addons/addon/manifests/install.yaml" <<'EOF'
apiVersion: apps/v1
kind: Deployment
spec:
  template:
    spec:
      containers:
        - name: addon
          image: ghcr.io/example/addon:v1
EOF
tar -czf "$tmp_dir/data/platform-addons/addon.tar.gz" -C "$tmp_dir/data/platform-addons/addon" .
rm -rf "$tmp_dir/data/platform-addons/addon"

cat >"$tmp_dir/data/service-templates/kubevirt/postgresql.yaml" <<'EOF'
runtimeSpec:
  image: docker.io/library/postgres:16
EOF

cat >"$tmp_dir/data/config-templates/config-template/template.json" <<'EOF'
{"image":"docker.io/library/busybox:1.36"}
EOF
tar -czf "$tmp_dir/data/config-templates/config-template.tar.gz" -C "$tmp_dir/data/config-templates/config-template" .
rm -rf "$tmp_dir/data/config-templates/config-template"

output="$(
  PAAP_OFFLINE_PROJECT_DIR="$tmp_dir" \
  PAAP_OFFLINE_INCLUDE_CHART_HELM=false \
  PAAP_OFFLINE_INCLUDE_CHART_TEXT=true \
  "$PROJECT_DIR/scripts/discover-offline-images.sh"
)"

for image in \
  "paap-server:test" \
  "docker.io/library/nginx:1.25" \
  "ghcr.io/example/addon:v1" \
  "docker.io/library/postgres:16" \
  "docker.io/library/busybox:1.36"; do
  if [[ "$output" != *"$image"* ]]; then
    echo "missing image: $image" >&2
    echo "--- output ---" >&2
    echo "$output" >&2
    exit 1
  fi
done

echo "discover-offline-images tests passed"
