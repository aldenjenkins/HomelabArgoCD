#!/usr/bin/env bash
# Sync talhelper-generated machine configs to PXE directories on the NAS.
# Maps hostname -> MAC address from talconfig.yaml, then rsyncs each
# clusterconfig/<cluster>-<hostname>.yaml to homeserver:/mnt/data/share/pxe/configs/<mac>/node.yaml
set -euo pipefail

# --- config ----------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TALCONFIG="${TALCONFIG:-$SCRIPT_DIR/talconfig.yaml}"
CLUSTERCONFIG_DIR="${CLUSTERCONFIG_DIR:-$SCRIPT_DIR/clusterconfig}"
NAS_HOST="${NAS_HOST:-homeserver}"
NAS_BASE="${NAS_BASE:-/mnt/data/share/pxe/configs}"
DRY_RUN="${DRY_RUN:-0}"   # set DRY_RUN=1 to preview only

# Force destination perms regardless of local file mode (local files may be 0600).
# F644 -> files 0644, D755 -> dirs 0755. PXE HTTP server needs world-readable.
CHMOD_SPEC="F644,D755"

# --- deps ------------------------------------------------------------------
for bin in yq rsync ssh; do
  command -v "$bin" >/dev/null || { echo "missing dependency: $bin" >&2; exit 1; }
done

# --- read cluster name + hostname->mac map from talconfig -------------------
CLUSTER_NAME="$(yq -r '.clusterName' "$TALCONFIG")"
if [[ -z "$CLUSTER_NAME" || "$CLUSTER_NAME" == "null" ]]; then
  echo "could not read .clusterName from $TALCONFIG" >&2
  exit 1
fi

# yq emits "<hostname>\t<mac>" per node. We take the first interface's MAC.
mapfile -t NODE_LINES < <(
  yq -r '
    .nodes[]
    | [.hostname, (.networkInterfaces[0].deviceSelector.hardwareAddr // "")]
    | @tsv
  ' "$TALCONFIG"
)

# --- sync each node ---------------------------------------------------------
ok=0; missing=0; failed=0
for line in "${NODE_LINES[@]}"; do
  hostname="${line%$'\t'*}"
  mac="${line#*$'\t'}"

  src="$CLUSTERCONFIG_DIR/${CLUSTER_NAME}-${hostname}.yaml"
  dest_dir="$NAS_BASE/$mac"
  dest="$dest_dir/node.yaml"

  if [[ -z "$mac" ]]; then
    echo "✗ $hostname: no MAC in talconfig, skipping" >&2
    ((missing++)) || true
    continue
  fi
  if [[ ! -f "$src" ]]; then
    echo "✗ $hostname: $src not found, skipping" >&2
    ((missing++)) || true
    continue
  fi

  echo "→ $hostname ($mac)"
  echo "    $src"
  echo "    $NAS_HOST:$dest"

  if [[ "$DRY_RUN" == "1" ]]; then
    rsync --dry-run -av --chmod="$CHMOD_SPEC" \
      --rsync-path="mkdir -p '$dest_dir' && rsync" \
      "$src" "$NAS_HOST:$dest" && ((ok++)) || { ((failed++)) || true; }
  else
    if rsync -a --chmod="$CHMOD_SPEC" \
        --rsync-path="mkdir -p '$dest_dir' && rsync" \
        "$src" "$NAS_HOST:$dest"; then
      ((ok++)) || true
    else
      echo "✗ $hostname: rsync failed" >&2
      ((failed++)) || true
    fi
  fi
done

echo
echo "done: $ok synced, $missing skipped, $failed failed"
[[ $failed -eq 0 ]]
