# 0006. Open core: community edition and extension points

- Status: accepted
- Date: 2026-11-06

## Context

Shellvane is open source under the GNU AGPL-3.0. The official builds published
on termoraplus.com also include some features that are developed and
licensed separately, free as well (for example an assistant that explains commands and
errors). The open-source code must stay complete and useful on its own, and
must not contain dead code, license checks or hidden switches for features it
does not have.

## Decision

The community edition (this repository) has a few **extension points**, and
registers no extension. Features of the official builds plug in only through
them:

- **Go**: `app.RegisterExtension(app.Extension{…})`, called from an `init`
  function, adds Wails services built from `app.ExtensionDeps` (database,
  vault, sessions, data folder, event emitter, service options). `main.go`
  appends `app.ExtensionServices(…)` to the application's services.
  `ExtensionDeps.AddBroker(prefix, broker)` lets an extension give hosts
  logins for credential references starting with its prefix.
- **Frontend**: `frontend/src/app/extensions/extensions.ts` lists the
  `AppExtension`s of the build (empty here). An extension can add sections to
  Settings, components rendered once at the root (dialogs), entries to a
  terminal pane's context menu (from the selected text), full-page views with
  their own activity-bar entry (loaded when opened), guide topics, and translations merged over
  the app's own at load time.
- **Sharing**: one extension may provide sharing of objects with other people
  (`SharingExtension`): the `Sharing` service answers the public code (does
  an object wait for its commands to be approved, entries of a host group's
  menu, palette commands, logins hosts can use) and its components are shown
  where objects are listed and edited (share button, note, review dialog,
  badge on a host group). Without it nothing is shared and nothing is shown.

Rules:

- Everything in this repository works without a license or an account; it
  contains no license verification.
- An extension point is added only when a real feature needs it, and is
  documented here.
- Error codes of extensions follow `core/errs` like everything else; their
  strings live with the extension.

## Contributions

The official builds combine this code with separately licensed features, so
contributions are accepted under a Contributor License Agreement (see
`CONTRIBUTING.md`): the contributor keeps the copyright and grants the
project the right to distribute the contribution under other terms as well.
The community edition stays AGPL-3.0.

## Consequences

- The community edition builds, tests and runs on its own (CI checks it).
- Adding a feature to the official builds may need a new extension point in
  this repository: a small, reviewable change.
- Forks are free to add their own extensions in the same way.
