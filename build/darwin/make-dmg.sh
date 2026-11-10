#!/usr/bin/env bash
# Makes a compressed disk image (UDZO) of a folder.
#
# Usage: make-dmg.sh <folder> <dmg to create> <volume name>
#
# hdiutil create -srcfolder mounts a writable image to fill it; on managed
# Macs, security software that blocks writable disk images makes it fail
# with "Read-only file system". Then the image is built without mounting
# anything: makehybrid writes an HFS+ image of the folder, convert
# compresses it (without the Applications shortcut: drag the app to the
# Applications folder of the Finder instead).
set -euo pipefail
src=$1 dmg=$2 volume=$3
rm -f "$dmg"
if hdiutil create -volname "$volume" -srcfolder "$src" -ov -format UDZO "$dmg"; then
  exit 0
fi
echo "hdiutil create failed: building the image without mounting it (makehybrid)" >&2
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
# A copy without symbolic links (the Applications shortcut): makehybrid
# could follow them and copy what they point to.
ditto "$src" "$tmp/src"
find "$tmp/src" -maxdepth 1 -type l -delete
hdiutil makehybrid -hfs -hfs-volume-name "$volume" -default-volume-name "$volume" -o "$tmp/raw.dmg" "$tmp/src"
hdiutil convert "$tmp/raw.dmg" -format UDZO -o "$dmg"
hdiutil imageinfo "$dmg" >/dev/null
