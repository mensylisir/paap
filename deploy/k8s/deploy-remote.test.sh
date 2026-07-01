#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SCRIPT="$SCRIPT_DIR/deploy-remote.sh"
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

assert_contains 'SERVER_IMAGE="${SERVER_IMAGE:-paap-server:'
assert_contains 'ASSETS_IMAGE="${ASSETS_IMAGE:-paap-assets:'
assert_contains 'OPERATOR_IMAGE="${OPERATOR_IMAGE:-paap-operator:'
assert_contains '"$SERVER_IMAGE"'
assert_contains '"$ASSETS_IMAGE"'
assert_contains '"$OPERATOR_IMAGE"'
assert_not_contains '"paap-server:v0.1.537"'
assert_not_contains '"paap-assets:v0.1.537"'
assert_not_contains '"paap-operator:v0.1.54"'
assert_contains '--field-selector spec.unschedulable!=true'
assert_contains 'oci-archive:${ARCHIVE}:${IMG}'
assert_not_contains 'oci-archive:${ARCHIVE}:latest'

assert_contains 'apply_manifest_with_image "$SCRIPT_DIR/init-templates.yaml" "paap-assets" "$ASSETS_IMAGE"'
assert_contains 'apply_manifest_with_image "$SCRIPT_DIR/paap-operator.yaml" "paap-operator" "$OPERATOR_IMAGE"'
assert_contains 'apply_manifest_with_image "$SCRIPT_DIR/paap-server.yaml" "paap-server" "$SERVER_IMAGE"'

echo "deploy-remote tests passed"
