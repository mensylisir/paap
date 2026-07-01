#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$SCRIPT_DIR/deploy.sh"
CONTENT="$(cat "$SCRIPT")"

assert_contains() {
  local needle="$1"
  if [[ "$CONTENT" != *"$needle"* ]]; then
    echo "missing expected text: $needle" >&2
    exit 1
  fi
}

assert_not_contains() {
  local needle="$1"
  if [[ "$CONTENT" == *"$needle"* ]]; then
    echo "unexpected text: $needle" >&2
    exit 1
  fi
}

assert_contains '"$CONTAINER_CLI" build -t "$ASSETS_IMAGE" -f Dockerfile.assets .'
assert_contains 'sed "s#image: paap-assets:.*#image: $ASSETS_IMAGE#g" "$SCRIPT_DIR/init-templates.yaml" | kubectl apply -f -'
assert_not_contains 'Installing kpack'
assert_not_contains 'kpack-v0.17.0.yaml'
assert_not_contains 'inspect_namespace kpack'

echo "deploy tests passed"
