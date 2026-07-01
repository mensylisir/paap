#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

tmp_list="$(mktemp)"
trap 'rm -f "$tmp_list"' EXIT

cat >"$tmp_list" <<'EOF'
docker.io/library/nginx:1.25
quay.io/argoproj/argocd:v2.13.1
registry:2.8.3
gitea/gitea:1.23.8
ghcr.io/buildpacks-community/kpack/controller@sha256:80e71f484f0aa5f54eb549f5d5e015ac5373c9bc616f12891d676a7e1dfb80bd
EOF

output="$("$SCRIPT_DIR/mirror-offline-images.sh" --dry-run --registry 127.0.0.1:5000 --list "$tmp_list")"

for expected in \
  "docker.io/library/nginx:1.25 -> 127.0.0.1:5000/library/nginx:1.25" \
  "quay.io/argoproj/argocd:v2.13.1 -> 127.0.0.1:5000/argoproj/argocd:v2.13.1" \
  "registry:2.8.3 -> 127.0.0.1:5000/library/registry:2.8.3" \
  "gitea/gitea:1.23.8 -> 127.0.0.1:5000/gitea/gitea:1.23.8" \
  "ghcr.io/buildpacks-community/kpack/controller@sha256:80e71f484f0aa5f54eb549f5d5e015ac5373c9bc616f12891d676a7e1dfb80bd -> 127.0.0.1:5000/buildpacks-community/kpack/controller:sha256-80e71f484f0a"; do
  if [[ "$output" != *"$expected"* ]]; then
    echo "missing expected mapping: $expected" >&2
    echo "--- output ---" >&2
    echo "$output" >&2
    exit 1
  fi
done

echo "mirror-offline-images tests passed"
