# Example plugins

`disk-usage/` is a complete external plugin: a panel with the host's
filesystems and an action that lists the largest folders in a terminal.
Install it from Shellvane's Plugins view with **Install from file**, giving
the folder (or a .zip of it). It needs `python3` on this computer.

The protocol (JSON-RPC 2.0 on stdin/stdout, `host.exec`) and the store
index format are described in the in-app guide, topic Plugins, and in
docs/adr/0003-plugins.md.

A store is a JSON file served over HTTPS:

```json
{
  "plugins": [
    {
      "id": "disk-usage",
      "name": "Disk usage",
      "version": "1.0.0",
      "description": "Filesystems of the host and their largest folders.",
      "author": "Shellvane",
      "homepage": "https://shellvane.com/plugins/disk-usage",
      "capabilities": ["panel", "actions"],
      "requires": ["python3"],
      "url": "https://shellvane.com/plugins/disk-usage-1.0.0.zip",
      "sha256": "<sha256 of the zip>"
    }
  ]
}
```

`scripts/plugin-index.sh` packages every folder here and writes such an
index, ready to publish under https://shellvane.com/plugins/.
