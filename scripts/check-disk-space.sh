#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
LABEL="${1:-disk-check}"
MIN_FREE_MB="${PAAP_MIN_FREE_MB:-10240}"
MODE="${PAAP_DISK_CHECK_MODE:-fail}"

disk_check_paths() {
  if [[ -n "${PAAP_DISK_CHECK_PATHS:-}" ]]; then
    tr ':' '\n' <<< "$PAAP_DISK_CHECK_PATHS"
    return
  fi

  echo "$ROOT_DIR"
  if command -v docker >/dev/null 2>&1; then
    docker info --format '{{.DockerRootDir}}' 2>/dev/null || true
  fi
}

fail_or_warn() {
  local message="$1"
  if [[ "$MODE" == "warn" ]]; then
    echo "warning: [$LABEL] $message" >&2
    return
  fi
  echo "error: [$LABEL] $message" >&2
  exit 1
}

echo "=== Disk space check: $LABEL ==="
checked=0
seen=":"
while IFS= read -r path; do
  [[ -n "$path" ]] || continue
  if [[ "$seen" == *":$path:"* ]]; then
    continue
  fi
  seen="${seen}${path}:"

  if ! line="$(df -Pm "$path" 2>/dev/null | awk 'NR == 2 {print $4 " " $5 " " $6}')"; then
    fail_or_warn "cannot inspect filesystem for $path"
    continue
  fi
  available_mb="$(awk '{print $1}' <<< "$line")"
  capacity="$(awk '{print $2}' <<< "$line")"
  mount="$(awk '{print $3}' <<< "$line")"
  if [[ -z "$available_mb" || ! "$available_mb" =~ ^[0-9]+$ ]]; then
    fail_or_warn "cannot parse free space for $path"
    continue
  fi

  echo "[$LABEL] $path: ${available_mb} MiB free (${capacity:-unknown} used on ${mount:-unknown})"
  checked=$((checked + 1))
  if (( available_mb < MIN_FREE_MB )); then
    fail_or_warn "$path has only ${available_mb} MiB free; require at least ${MIN_FREE_MB} MiB before image-heavy operations"
  fi
done < <(disk_check_paths)

if (( checked == 0 )); then
  fail_or_warn "no filesystem paths were checked"
fi

if command -v docker >/dev/null 2>&1; then
  docker system df 2>/dev/null || true
fi
