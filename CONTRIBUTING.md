# Contributing to Shellvane

Thanks for helping! Bug reports, translations, snippets and themes for the
gallery, and code are all welcome.

## Before you start

- For anything larger than a small fix, open an issue first so we can agree on
  the approach.
- Security problems: do not open an issue, see [docs/SECURITY.md](docs/SECURITY.md).
- Contributions are accepted under the project's Contributor License
  Agreement, signed once from the pull request (a bot asks for it). You keep
  the copyright; the CLA lets the project ship your change in the official
  builds as well, which also include separately licensed features
  ([ADR 0006](docs/adr/0006-open-core.md)). The code here stays AGPL-3.0.

## Conventions

- All code is in English: identifiers, comments, log messages, commit
  messages, ADRs and documentation.
- No user-facing strings hardcoded in code. UI strings go through i18n
  (frontend: Transloco, `frontend/src/i18n/*.json`; Go-rendered UI and CLI:
  `core/i18n`, `core/i18n/locales/*.json`). English is the source language;
  Italian is maintained too. Keep all locale files in sync (CI checks it), and
  the in-app guide (`frontend/public/guide/{en,it}/`) too: every feature is
  documented there, with the same `{#anchors}` in every language.
- Angular components: always separate files for TypeScript, template and
  styles (`x.ts`, `x.html`, `x.scss`); no inline `template` or `styles`.
- Every terminal pane must show which connection it is (label,
  user@host:port, group path and color, state) so the user can always tell
  panes apart.
- The Go core returns coded errors (`core/errs`), never translated text.
- `core/` must not import Wails; `app/` adapts the core to the UI.
- Important decisions are recorded as ADRs in `docs/adr/`.

## Checks

Run before opening a pull request (CI runs the same):

```sh
go vet ./core/... ./app/... ./cmd/...
go test ./core/... ./app/... ./cmd/...
cd frontend && npm test && npm run i18n:check
```

Integration tests against real OpenSSH servers are in `test/integration/`
(Docker).

## Gallery

Snippet collections and color schemes live in `gallery/`: a `snippets/*.json`
file is a Shellvane snippet export (`"kind": "shellvane-snippets"`), a
`themes/*.json` file a scheme in Windows Terminal format (export it from
Settings › Terminal). `go test ./app/ -run GalleryContent` checks that every
file imports. Commands that change something should have `"run": false`, so
they are pasted but not executed.
