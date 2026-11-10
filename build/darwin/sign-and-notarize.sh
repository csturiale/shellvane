#!/usr/bin/env bash
# Signs the macOS app, the CLI and the disk image with a Developer ID
# certificate and has Apple notarize them, so Gatekeeper opens them without
# warnings. Run on a Mac by scripts/release-macos.sh.
#
# Usage: sign-and-notarize.sh <path/to/App.app> <cli binary> <dmg to create> <volume name>
#
# Environment, either a certificate already in your keychain:
#   MACOS_SIGN_IDENTITY   e.g. "Developer ID Application: Name (TEAMID)"
#   NOTARY_PROFILE        a notarytool profile saved with
#                         xcrun notarytool store-credentials
# or the same as the former GitHub workflow:
#   MACOS_CERT_P12        Developer ID Application certificate (.p12), base64
#   MACOS_CERT_PASSWORD   its password
#   notarization, either an App Store Connect API key:
#     APPLE_API_KEY       the .p8 key, base64
#     APPLE_API_KEY_ID, APPLE_API_ISSUER
#   or an Apple ID:
#     APPLE_ID, APPLE_TEAM_ID, APPLE_APP_PASSWORD (an app-specific password)
set -euo pipefail

app=$1 cli=$2 dmg=$3 volume=$4
here=$(cd "$(dirname "$0")" && pwd)
work=$(mktemp -d)
trap 'security delete-keychain "$work/sign.keychain-db" >/dev/null 2>&1 || true; rm -rf "$work"' EXIT

kc_args=()
if [[ -n ${MACOS_SIGN_IDENTITY:-} ]]; then
  # A certificate already in the user's keychains.
  identity=$MACOS_SIGN_IDENTITY
else
  # A keychain of its own holding only the certificate.
  kc="$work/sign.keychain-db"
  kcpass=$(openssl rand -hex 16)
  security create-keychain -p "$kcpass" "$kc"
  security set-keychain-settings -lut 3600 "$kc"
  security unlock-keychain -p "$kcpass" "$kc"
  echo "$MACOS_CERT_P12" | base64 --decode >"$work/cert.p12"
  security import "$work/cert.p12" -k "$kc" -P "$MACOS_CERT_PASSWORD" -T /usr/bin/codesign
  security set-key-partition-list -S apple-tool:,apple: -s -k "$kcpass" "$kc" >/dev/null
  security list-keychains -d user -s "$kc" $(security list-keychains -d user | tr -d '"')
  identity=$(security find-identity -v -p codesigning "$kc" | awk -F'"' '/Developer ID Application/ {print $2; exit}')
  if [[ -z $identity ]]; then
    echo "No 'Developer ID Application' identity in MACOS_CERT_P12" >&2
    exit 1
  fi
  kc_args=(--keychain "$kc")
fi
echo "Signing as: $identity"

sign() {
  codesign --force --timestamp --options runtime ${kc_args[@]+"${kc_args[@]}"} --sign "$identity" "$@"
}

notarize() {
  if [[ -n ${NOTARY_PROFILE:-} ]]; then
    xcrun notarytool submit "$1" --wait --keychain-profile "$NOTARY_PROFILE"
  elif [[ -n ${APPLE_API_KEY:-} ]]; then
    echo "$APPLE_API_KEY" | base64 --decode >"$work/key.p8"
    xcrun notarytool submit "$1" --wait --key "$work/key.p8" --key-id "$APPLE_API_KEY_ID" --issuer "$APPLE_API_ISSUER"
  else
    xcrun notarytool submit "$1" --wait --apple-id "$APPLE_ID" --team-id "$APPLE_TEAM_ID" --password "$APPLE_APP_PASSWORD"
  fi
}

# The app: nested code first, then the bundle, with the hardened runtime.
find "$app/Contents" -type f \( -name '*.dylib' -o -name '*.so' \) -print0 | while IFS= read -r -d '' f; do sign "$f"; done
sign --entitlements "$here/entitlements.plist" "$app"
codesign --verify --deep --strict --verbose=2 "$app"
ditto -c -k --keepParent "$app" "$work/app.zip"
notarize "$work/app.zip"
xcrun stapler staple "$app"

# The CLI: a bare binary cannot be stapled; Gatekeeper checks it online.
sign "$cli"
ditto -c -k "$cli" "$work/cli.zip"
notarize "$work/cli.zip"

# The disk image, made from the stapled app.
mkdir -p "$work/dmg"
cp -R "$app" "$work/dmg/$(basename "$app")"
ln -s /Applications "$work/dmg/Applications"
hdiutil create -volname "$volume" -srcfolder "$work/dmg" -ov -format UDZO "$dmg"
sign "$dmg"
notarize "$dmg"
xcrun stapler staple "$dmg"
spctl --assess --type execute --verbose=2 "$app"
