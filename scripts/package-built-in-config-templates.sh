#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_DIR="$ROOT_DIR/docs/examples/buildin-config-in-templates"
TARGET_DIR="$ROOT_DIR/data/config-templates"

mkdir -p "$TARGET_DIR"

if [[ ! -d "$SOURCE_DIR" ]]; then
  echo "source directory not found: $SOURCE_DIR" >&2
  exit 1
fi

shopt -s nullglob
template_dirs=("$SOURCE_DIR"/*)
if [[ ${#template_dirs[@]} -eq 0 ]]; then
  echo "no built-in config template directories found under $SOURCE_DIR" >&2
  exit 1
fi

for template_dir in "${template_dirs[@]}"; do
  [[ -d "$template_dir" ]] || continue
  name="$(basename "$template_dir")"

  if [[ ! -f "$template_dir/template.json" ]]; then
    echo "config template $name is missing template.json" >&2
    exit 1
  fi

  package_items=(template.json)
  [[ -f "$template_dir/schema.json" ]] && package_items+=(schema.json)
  [[ -d "$template_dir/files" ]] && package_items+=(files)

  tmp_file="$(mktemp "$TARGET_DIR/.${name}.XXXXXX.tar.gz")"
  tar \
    --sort=name \
    --mtime='UTC 1970-01-01' \
    --owner=0 \
    --group=0 \
    --numeric-owner \
    -C "$template_dir" \
    -cf - "${package_items[@]}" | gzip -n > "$tmp_file"
  chmod 0644 "$tmp_file"
  mv "$tmp_file" "$TARGET_DIR/$name.tar.gz"
  echo "packaged $name -> data/config-templates/$name.tar.gz"
done

