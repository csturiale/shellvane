# Contribute {#top}

Color schemes, snippet collections and plugins made by users reach everyone through the community gallery and the plugin store. Everything goes through a pull request on the open-source repository, [github.com/csturiale/shellvane](https://github.com/csturiale/shellvane): you need a GitHub account, not Git, because files can be uploaded from the browser.

## Share a color scheme {#themes}

1. In **Settings › Terminal**, select the scheme and use **Export**: you get a JSON file in Windows Terminal format. Give the scheme a clear `name`, it is the title shown in the gallery.
2. Open the repository on GitHub, go to the `gallery/themes/` folder and use **Add file › Upload files**. GitHub makes a copy of the repository for you (a fork) and offers to open a pull request: describe the scheme in a line.
3. The project checks that it imports and that the text is readable; once accepted it appears on termoraplus.com/gallery and in the app's [community gallery](shellvane:topic/snippets#gallery) with the next update of the website.

One scheme per file, with a lowercase file name such as `solarized-warm.json`.

## Share snippets {#snippets}

1. In the **Snippets** panel put the commands in a folder and export them (`shellvane-snippets.json`). A collection should cover one subject (a tool, a service, a kind of diagnosis).
2. Commands that change something (restarts, deletions, `kubectl apply`…) must have **Press Enter after sending** turned off (`"run": false` in the file): the gallery then pastes them without running them. Use [variables](shellvane:topic/snippets#variables) instead of names of your own hosts, users or paths.
3. Upload the file to `gallery/snippets/` as for a color scheme and open the pull request.

Never include passwords, tokens, internal host names or addresses: the file is public.

## Write a plugin {#write}

A plugin is a folder with a `plugin.json` and a program in any language: the protocol is in [Plugins › Write a plugin](shellvane:topic/plugins#write) and `examples/plugins/disk-usage` in the repository is a complete example. To try it, use **Plugins › Install from file** with the folder: no store or signature is needed for plugins you install yourself.

## Publish a plugin in the store {#plugins}

The code of a third-party plugin stays in your own repository; the store lists it with the address of its package and the package's SHA-256, so the app installs exactly the file that was reviewed.

1. Make the package: a `.zip` of the plugin's folder, with `plugin.json` in it. Publish it at a permanent HTTPS address, for example as a file of a GitHub release (`https://github.com/you/plugin/releases/download/v1.0.0/my-plugin-1.0.0.zip`).
2. Compute its SHA-256: `shasum -a 256 my-plugin-1.0.0.zip` on macOS and Linux, `Get-FileHash my-plugin-1.0.0.zip` in PowerShell.
3. In the repository add `plugins/<id>.json`:

   ```json
   {
     "id": "my-plugin",
     "name": "My plugin",
     "version": "1.0.0",
     "description": "What it shows or does, in one line.",
     "author": "Your name",
     "homepage": "https://github.com/you/plugin",
     "url": "https://github.com/you/plugin/releases/download/v1.0.0/my-plugin-1.0.0.zip",
     "sha256": "…"
   }
   ```

   `id` and `version` must match the package's `plugin.json`. Capabilities and requirements are read from there.
4. Open the pull request, saying what the plugin does and what it runs on the hosts (`host.exec`) or on the computer.

Before accepting it the project downloads the package, checks the SHA-256 and the manifest and reads the code: plugins run with the user's permissions. Each website update downloads the package again and leaves out any whose file changed. For a **new version** publish a new zip at a new address and update `version`, `url` and `sha256` in the same file: never replace a published zip.

## Translations and code {#code}

Fixes, translations and new features are welcome too: read `CONTRIBUTING.md` in the repository first (conventions, checks to run) and, for anything larger than a small fix, open an issue to agree on the approach.

## What happens to your pull request {#review}

The project reviews it on GitHub. Once accepted, the change is applied to the project's main repository with you as its author and published to the open-source repository with the next update; the pull request is closed with a link to that commit. By opening it you agree to the contribution terms in `CONTRIBUTING.md`: your work is published under the AGPL-3.0 and can also be shipped in the official builds.
