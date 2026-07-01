#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
MANIFEST="$SCRIPT_DIR/init-templates.yaml"
CONTENT="$(cat "$MANIFEST")"

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

assert_contains 'sort > /staged-service-templates/.service-template-list'
assert_contains 'done < /service-templates/.service-template-list'
assert_not_contains 'find /service-templates -type f -name'

echo "init-templates tests passed"
