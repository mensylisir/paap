#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$ROOT_DIR/scripts/check-disk-space.sh"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

write_fake_df() {
  local available_mb="$1"
  cat > "$TMP_DIR/df" <<EOF
#!/usr/bin/env bash
cat <<'DF'
Filesystem     1048576-blocks Used Available Capacity Mounted on
fakefs                    100   90        ${available_mb}      90% /fake
DF
EOF
  chmod +x "$TMP_DIR/df"
}

write_fake_df 3
if PATH="$TMP_DIR:$PATH" PAAP_DISK_CHECK_PATHS="/fake" PAAP_MIN_FREE_MB=4 "$SCRIPT" low-space >"$TMP_DIR/low.out" 2>"$TMP_DIR/low.err"; then
  echo "expected low-space check to fail" >&2
  exit 1
fi
grep -q "low-space" "$TMP_DIR/low.err"
grep -q "3 MiB free" "$TMP_DIR/low.err"

write_fake_df 8
PATH="$TMP_DIR:$PATH" PAAP_DISK_CHECK_PATHS="/fake" PAAP_MIN_FREE_MB=4 "$SCRIPT" enough-space >"$TMP_DIR/high.out"
grep -q "enough-space" "$TMP_DIR/high.out"
grep -q "8 MiB free" "$TMP_DIR/high.out"
