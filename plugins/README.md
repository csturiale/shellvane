# Third-party plugins in the store

Each `<id>.json` here lists a plugin published by its author: the store at
shellvane.com/plugins includes it, and the app installs the package only
if its SHA-256 matches the one in this file. How to propose one: the in-app
guide, topic Contribute › Publish a plugin in the store
(shellvane.com/docs/contribute/#plugins).

```json
{
  "id": "my-plugin",
  "name": "My plugin",
  "version": "1.0.0",
  "description": "What it shows or does, in one line.",
  "author": "Your name",
  "homepage": "https://github.com/you/plugin",
  "url": "https://github.com/you/plugin/releases/download/v1.0.0/my-plugin-1.0.0.zip",
  "sha256": "<sha256 of the zip>"
}
```

Check an entry before opening the pull request:

```sh
python3 scripts/store-entries.py --fetch plugins
```
