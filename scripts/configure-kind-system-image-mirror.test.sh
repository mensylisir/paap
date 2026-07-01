#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

tmp_list="$(mktemp)"
trap 'rm -f "$tmp_list"' EXIT

cat >"$tmp_list" <<'EOF'
docker.io/library/nginx:1.25
registry:2.8.3
quay.io/argoproj/argocd:v2.13.1
ghcr.io/kedacore/keda:2.17.0
public.ecr.aws/docker/library/redis:7.4.1-alpine
EOF

output="$("$SCRIPT_DIR/configure-kind-system-image-mirror.sh" --dry-run --registry 127.0.0.1:5000 --list "$tmp_list")"

for expected in \
  "docker.io -> http://127.0.0.1:5000" \
  "quay.io -> http://127.0.0.1:5000" \
  "ghcr.io -> http://127.0.0.1:5000" \
  "public.ecr.aws -> http://127.0.0.1:5000"; do
  if [[ "$output" != *"$expected"* ]]; then
    echo "missing expected mirror: $expected" >&2
    echo "--- output ---" >&2
    echo "$output" >&2
    exit 1
  fi
done

echo "configure-kind-system-image-mirror tests passed"
