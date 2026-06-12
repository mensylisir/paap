#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_DIR="$ROOT_DIR/docs/examples/built-in-templates"
TARGET_DIR="$ROOT_DIR/data/charts"

mkdir -p "$TARGET_DIR"

if [[ ! -d "$SOURCE_DIR" ]]; then
  echo "source directory not found: $SOURCE_DIR" >&2
  exit 1
fi

shopt -s nullglob
template_dirs=("$SOURCE_DIR"/*)
if [[ ${#template_dirs[@]} -eq 0 ]]; then
  echo "no built-in template directories found under $SOURCE_DIR" >&2
  exit 1
fi

for template_dir in "${template_dirs[@]}"; do
  [[ -d "$template_dir" ]] || continue
  name="$(basename "$template_dir")"

  for required in platform-manifest.yaml preset-values.yaml chart/Chart.yaml; do
    if [[ ! -f "$template_dir/$required" ]]; then
      echo "template $name is missing $required" >&2
      exit 1
    fi
  done

  tmp_file="$(mktemp "$TARGET_DIR/.${name}.XXXXXX.tar.gz")"
  tar \
    --sort=name \
    --mtime='UTC 1970-01-01' \
    --owner=0 \
    --group=0 \
    --numeric-owner \
    -C "$template_dir" \
    -cf - platform-manifest.yaml preset-values.yaml chart | gzip -n > "$tmp_file"
  chmod 0644 "$tmp_file"
  mv "$tmp_file" "$TARGET_DIR/$name.tar.gz"
  echo "packaged $name -> data/charts/$name.tar.gz"
done
