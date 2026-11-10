#!/usr/bin/env bash
# Makes a compressed disk image (UDZO) of a folder.
#
# Usage: make-dmg.sh <folder> <dmg to create> <volume name>
#
# hdiutil create -srcfolder mounts a writable image to fill it; on managed
# Macs, security software that blocks writable disk images makes it fail
# with "Read-only file system". Then the image is built without mounting
# anything: makehybrid writes an HFS+ image of the folder, convert
# compresses it.
set -euo pipefail
src=$1 dmg=$2 volume=$3
rm -f "$dmg"
if hdiutil create -volname "$volume" -srcfolder "$src" -ov -format UDZO "$dmg"; then
  exit 0
fi
echo "hdiutil create failed: building the image without mounting it (makehybrid)" >&2
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
# makehybrid keeps symbolic links as links on HFS+; should it follow the
# Applications shortcut instead (copying /Applications), the image would be
# far larger than the folder: it is made again without the shortcut.
content_kb=$(find "$src" -mindepth 1 -maxdepth 1 ! -type l -exec du -sk {} + | awk '{s+=$1} END {print s+0}')
limit_kb=$((content_kb * 2 + 102400))
# Runs makehybrid, stopping it as soon as the image grows past the limit;
# fails (status 3) when it did.
hybrid() {
  rm -f "$tmp/raw.dmg"
  hdiutil makehybrid -hfs -hfs-volume-name "$volume" -default-volume-name "$volume" -o "$tmp/raw.dmg" "$1" &
  local pid=$!
  while kill -0 "$pid" 2>/dev/null; do
    if [[ -f $tmp/raw.dmg ]] && (( $(du -sk "$tmp/raw.dmg" | cut -f1) > limit_kb )); then
      kill "$pid" 2>/dev/null; wait "$pid" 2>/dev/null || true
      return 3
    fi
    sleep 1
  done
  wait "$pid" || return $?
  (( $(du -sk "$tmp/raw.dmg" | cut -f1) <= limit_kb )) || return 3
}
status=0
hybrid "$src" || status=$?
if (( status == 3 )); then
  echo "The image followed the Applications shortcut: made again without it (drag the app to Applications in the Finder)." >&2
  ditto "$src" "$tmp/src"
  find "$tmp/src" -maxdepth 1 -type l -delete
  hybrid "$tmp/src"
elif (( status != 0 )); then
  exit "$status"
fi
hdiutil convert "$tmp/raw.dmg" -format UDZO -o "$dmg"
hdiutil imageinfo "$dmg" >/dev/null
