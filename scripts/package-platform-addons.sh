#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_DIR="$ROOT_DIR/docs/examples/platform-addons"
TARGET_DIR="$ROOT_DIR/data/platform-addons"

mkdir -p "$TARGET_DIR"

if [[ ! -d "$SOURCE_DIR" ]]; then
  echo "source directory not found: $SOURCE_DIR" >&2
  exit 1
fi

shopt -s nullglob
addon_dirs=("$SOURCE_DIR"/*)
if [[ ${#addon_dirs[@]} -eq 0 ]]; then
  echo "no platform addon directories found under $SOURCE_DIR" >&2
  exit 1
fi

for addon_dir in "${addon_dirs[@]}"; do
  [[ -d "$addon_dir" ]] || continue
  name="$(basename "$addon_dir")"

  for required in addon.yaml README.md manifests; do
    if [[ ! -e "$addon_dir/$required" ]]; then
      echo "platform addon $name is missing $required" >&2
      exit 1
    fi
  done

  if ! find "$addon_dir/manifests" -type f \( -name '*.yaml' -o -name '*.yml' \) | grep -q .; then
    echo "platform addon $name has no manifest yaml files" >&2
    exit 1
  fi

  tmp_file="$(mktemp "$TARGET_DIR/.${name}.XXXXXX.tar.gz")"
  tar \
    --sort=name \
    --mtime='UTC 1970-01-01' \
    --owner=0 \
    --group=0 \
    --numeric-owner \
    -C "$addon_dir" \
    -cf - addon.yaml README.md manifests | gzip -n > "$tmp_file"
  chmod 0644 "$tmp_file"
  mv "$tmp_file" "$TARGET_DIR/$name.tar.gz"
  echo "packaged $name -> data/platform-addons/$name.tar.gz"
done
