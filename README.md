# Shellvane

Cross-platform desktop manager for SSH, SFTP, tunnels and multi-host
workflows. Built with Go, [Wails v3](https://v3.wails.io) and Angular.

- What it does and what comes next: [ROADMAP.md](ROADMAP.md)
- Decisions: [docs/adr](docs/adr) · Security: [docs/SECURITY.md](docs/SECURITY.md)
- Contributing: [CONTRIBUTING.md](CONTRIBUTING.md)
- License: [GNU AGPL-3.0](LICENSE)

## Layout

| Path | Contents |
| --- | --- |
| `main.go` | Desktop app entry point (Wails) |
| `app/` | Service layer exposed to the frontend; extension points |
| `core/` | UI-independent logic: SSH, SFTP, tunnels, vault, importers, sync, plugins… |
| `cmd/shellvane-cli/` | Command-line companion |
| `frontend/` | Angular UI; translations in `frontend/src/i18n`, guide in `frontend/public/guide` |
| `gallery/` | Community snippets and color schemes |
| `examples/plugins/` | An example external plugin |
| `build/` | Packaging for Windows, macOS and Linux |
| `test/integration/` | OpenSSH test servers (Docker) |

## Requirements

- Go 1.26+
- Node.js 24 LTS
- Wails CLI: `go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0`
- Linux only: `libgtk-4-dev libwebkitgtk-6.0-dev` (Debian/Ubuntu names)

## Common tasks

```sh
wails3 dev                      # run with hot reload
wails3 build                    # production build into bin/
wails3 package                  # installers / packages for the current OS

go test ./core/... ./app/... ./cmd/...   # Go tests
cd frontend && npm test                  # frontend tests
cd frontend && npm run i18n:check
```

## Internationalization

English is the source language; Italian is maintained alongside it. No
user-facing string is hardcoded:

- Web UI: [Transloco](https://jsverse.gitbook.io/transloco) with ICU messages,
  files in `frontend/src/i18n/*.json`.
- Strings rendered by Go (menus, tray, CLI): `core/i18n`, files in
  `core/i18n/locales/*.json`.
- Errors from Go carry a stable code (`core/errs`) that the UI translates
  under `errors.<code>`.

To add a language, add the JSON file in both places and register it in
`frontend/src/app/core/i18n/languages.ts`. CI fails if keys differ from English.
