#!/usr/bin/env python3
"""Checks the third-party plugins listed in plugins/*.json and prints their
plugin store entries as a JSON array.

Each file names a package published by its author (url) and the SHA-256 the
project reviewed. With --fetch every package is downloaded: its hash must
match and its plugin.json must have the same id and version; capabilities
and requirements are read from it. Without --fetch only the files are
checked (fields, https, hash format).

Usage: scripts/store-entries.py [--fetch] [--skip-bad] [dir]   (default plugins)
  --skip-bad  leave out entries that fail (with a warning) instead of
              failing: the website keeps publishing the others.
"""
import hashlib
import io
import json
import re
import sys
import urllib.request
import zipfile
from pathlib import Path

REQUIRED = ("id", "name", "version", "description", "url", "sha256")
OPTIONAL = ("author", "homepage")
ID = re.compile(r"^[a-z0-9][a-z0-9-]{1,62}$")
SHA = re.compile(r"^[0-9a-f]{64}$")
MAX_PACKAGE = 64 << 20  # the app's own limit


class Bad(Exception):
    pass


def check_file(path: Path) -> dict:
    try:
        entry = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, ValueError) as err:
        raise Bad(f"not valid JSON: {err}")
    if not isinstance(entry, dict):
        raise Bad("must be a JSON object")
    unknown = set(entry) - set(REQUIRED) - set(OPTIONAL)
    if unknown:
        raise Bad(f"unknown fields: {', '.join(sorted(unknown))}")
    for key in REQUIRED:
        if not isinstance(entry.get(key), str) or not entry[key].strip():
            raise Bad(f'"{key}" is missing or empty')
    for key in OPTIONAL:
        if key in entry and not isinstance(entry[key], str):
            raise Bad(f'"{key}" must be a string')
    if not ID.match(entry["id"]):
        raise Bad('"id": lowercase letters, digits and hyphens')
    if path.stem != entry["id"]:
        raise Bad(f'the file must be named {entry["id"]}.json')
    if not entry["url"].startswith("https://"):
        raise Bad('"url" must be https')
    if entry.get("homepage") and not entry["homepage"].startswith("https://"):
        raise Bad('"homepage" must be https')
    entry["sha256"] = entry["sha256"].lower()
    if not SHA.match(entry["sha256"]):
        raise Bad('"sha256" must be 64 hexadecimal characters')
    return entry


def manifest_of(data: bytes) -> dict:
    try:
        zf = zipfile.ZipFile(io.BytesIO(data))
    except zipfile.BadZipFile:
        raise Bad("the package is not a zip")
    names = [n for n in zf.namelist() if n.rsplit("/", 1)[-1] == "plugin.json"]
    # At the root or in a single top-level folder, as the app accepts it.
    names = [n for n in names if n.count("/") <= 1]
    if len(names) != 1:
        raise Bad("the package must hold one plugin.json, at its root or in one top-level folder")
    try:
        return json.loads(zf.read(names[0]))
    except ValueError as err:
        raise Bad(f"plugin.json is not valid JSON: {err}")


def fetch(entry: dict) -> dict:
    req = urllib.request.Request(entry["url"], headers={"User-Agent": "shellvane-store-check"})
    try:
        with urllib.request.urlopen(req, timeout=60) as resp:
            data = resp.read(MAX_PACKAGE + 1)
    except OSError as err:
        raise Bad(f"cannot download the package: {err}")
    if len(data) > MAX_PACKAGE:
        raise Bad("the package is larger than 64 MB")
    got = hashlib.sha256(data).hexdigest()
    if got != entry["sha256"]:
        raise Bad(f"the package's SHA-256 is {got}, not the one listed (was it replaced?)")
    m = manifest_of(data)
    for key in ("id", "version"):
        if m.get(key) != entry[key]:
            raise Bad(f'plugin.json has {key} "{m.get(key)}", the entry "{entry[key]}"')
    entry["capabilities"] = m.get("capabilities", [])
    entry["requires"] = m.get("requires", [])
    return entry


def main(argv: list[str]) -> int:
    do_fetch = "--fetch" in argv
    skip_bad = "--skip-bad" in argv
    args = [a for a in argv if not a.startswith("--")]
    folder = Path(args[0] if args else "plugins")
    entries, failed = [], False
    for path in sorted(folder.glob("*.json")) if folder.is_dir() else []:
        try:
            entry = check_file(path)
            if do_fetch:
                entry = fetch(entry)
            entries.append({k: entry.get(k, "") for k in REQUIRED + OPTIONAL} | {
                "capabilities": entry.get("capabilities", []), "requires": entry.get("requires", [])})
        except Bad as err:
            print(f"{path}: {err}", file=sys.stderr)
            failed = True
    print(json.dumps(entries, indent=2))
    return 1 if failed and not skip_bad else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
