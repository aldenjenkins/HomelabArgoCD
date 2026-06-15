#!/usr/bin/env bash
# Sync talhelper-generated machine configs to PXE directories on the NAS,
# and download the matching Talos boot assets (vmlinuz-amd64, initramfs-amd64.xz)
# to the NAS, overwriting whatever is currently there.
#
# Maps hostname -> MAC address from talconfig.yaml, then rsyncs each
# clusterconfig/<cluster>-<hostname>.yaml to homeserver:/mnt/data/share/pxe/configs/<mac>/node.yaml
set -euo pipefail

# --- config ----------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TALCONFIG="${TALCONFIG:-$SCRIPT_DIR/talconfig.yaml}"
TALENV="${TALENV:-$SCRIPT_DIR/talenv.yaml}"
CLUSTERCONFIG_DIR="${CLUSTERCONFIG_DIR:-$SCRIPT_DIR/clusterconfig}"
NAS_HOST="${NAS_HOST:-homeserver}"
NAS_BASE="${NAS_BASE:-/mnt/data/share/pxe/configs}"
NAS_ASSETS_DIR="${NAS_ASSETS_DIR:-/mnt/data/share/pxe/boot/talos-assets}"
TALOS_GITHUB_BASE="${TALOS_GITHUB_BASE:-https://github.com/siderolabs/talos/releases/download}"
DRY_RUN="${DRY_RUN:-0}"   # set DRY_RUN=1 to preview only

# Force destination perms regardless of local file mode (local files may be 0600).
# F644 -> files 0644, D755 -> dirs 0755. PXE HTTP server needs world-readable.
CHMOD_SPEC="F644,D755"

# Boot asset filenames to sync
BOOT_ASSETS=(vmlinuz-amd64 initramfs-amd64.xz)

# --- deps ------------------------------------------------------------------
for bin in yq rsync ssh curl; do
  command -v "$bin" >/dev/null || { echo "missing dependency: $bin" >&2; exit 1; }
done

# --- read cluster name from talconfig ---------------------------------------
CLUSTER_NAME="$(yq -r '.clusterName' "$TALCONFIG")"
if [[ -z "$CLUSTER_NAME" || "$CLUSTER_NAME" == "null" ]]; then
  echo "could not read .clusterName from $TALCONFIG" >&2
  exit 1
fi

# --- read talos version from talenv ----------------------------------------
if [[ ! -f "$TALENV" ]]; then
  echo "talenv.yaml not found: $TALENV" >&2
  exit 1
fi
TALOS_VERSION_RAW="$(yq -r '.talosVersion' "$TALENV")"
if [[ -z "$TALOS_VERSION_RAW" || "$TALOS_VERSION_RAW" == "null" ]]; then
  echo "could not read .talosVersion from $TALENV" >&2
  exit 1
fi
# Normalise to always have a leading "v"
TALOS_VERSION="v${TALOS_VERSION_RAW#v}"

# yq emits "<hostname>\t<mac>" per node. We take the first interface's MAC.
mapfile -t NODE_LINES < <(
  yq -r '
    .nodes[]
    | [.hostname, (.networkInterfaces[0].deviceSelector.hardwareAddr // "")]
    | @tsv
  ' "$TALCONFIG"
)

# --- sync boot assets -------------------------------------------------------
echo "==> Talos boot assets (version: $TALOS_VERSION)"

if [[ "$DRY_RUN" == "1" ]]; then
  for asset in "${BOOT_ASSETS[@]}"; do
    url="$TALOS_GITHUB_BASE/$TALOS_VERSION/$asset"
    echo "    [dry-run] would download: $url -> $NAS_HOST:$NAS_ASSETS_DIR/$asset"
  done
else
  ssh "$NAS_HOST" "mkdir -p '$NAS_ASSETS_DIR'"
  for asset in "${BOOT_ASSETS[@]}"; do
    url="$TALOS_GITHUB_BASE/$TALOS_VERSION/$asset"
    remote_path="$NAS_ASSETS_DIR/$asset"
    echo "    downloading $url"
    echo "             -> $NAS_HOST:$remote_path"
    # Stream directly from GitHub to NAS via ssh, atomic replace via .tmp
    if curl -fsSL --retry 3 "$url" \
        | ssh "$NAS_HOST" "cat > '${remote_path}.tmp' && mv '${remote_path}.tmp' '$remote_path' && chmod 644 '$remote_path'"; then
      echo "    ✓ $asset"
    else
      echo "✗ failed to download $asset from $url" >&2
      ssh "$NAS_HOST" "rm -f '${remote_path}.tmp'" 2>/dev/null || true
      exit 1
    fi
  done
fi

echo

# --- sync each node's machine config ----------------------------------------
echo "==> Machine configs (cluster: $CLUSTER_NAME)"
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
