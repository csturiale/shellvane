#!/bin/sh
# Generates the throwaway key pair used by the key-only test server.
set -eu
dir="$(dirname "$0")/.keys"
mkdir -p "$dir"
[ -f "$dir/id_ed25519" ] || ssh-keygen -q -t ed25519 -N "" -C shellvane-test -f "$dir/id_ed25519"
echo "Test key: $dir/id_ed25519"
