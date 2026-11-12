# Command line, links, tray and portable use {#top}

## Command line {#cli}

`shellvane-cli` works with the same data as the app:

- `shellvane-cli list` prints the saved hosts with their group and address;
- `shellvane-cli connect <host>` opens a saved host (by name), or `user@host:port`, in the running Shellvane;
- `shellvane-cli connect --ssh <host>` runs the system's `ssh` for a saved host in the current terminal, with its port, user, jump hosts and ProxyCommand, and Shellvane's agent when it runs;
- `shellvane-cli open ssh://… | sftp://…` opens a link in the running Shellvane;
- `shellvane-cli tunnel list`, `tunnel up <name>`, `tunnel down <name>` control saved tunnels;
- `shellvane-cli import ssh-config [file]` imports `~/.ssh/config`.

The CLI talks to the app through a local socket that only your user can open.

## ssh:// and sftp:// links {#links}

Shellvane handles `ssh://user@host:port` and `sftp://user@host:port/path` links from the browser, documentation or other apps. A link that matches a saved host (same address, port and user) opens that host, with its credentials, jump hosts and options; otherwise it opens a quick connection. `sftp://` links open the folder in Files. A link clicked while Shellvane runs goes to the open window.

`shellvane://procedure` links are made by the **Send to Shellvane** bookmark and bring a web page into Procedures ([Procedures](shellvane:topic/procedures#web)).

## Portable mode {#portable}

Put an empty file named `shellvane.portable` next to the Shellvane executable (on macOS, next to `Shellvane.app`): Shellvane then keeps all its data in a `ShellvaneData` folder beside it, for example on a USB stick. The `SHELLVANE_DATA_DIR` environment variable chooses the data folder explicitly.

## Tray {#tray}

The tray icon lists the recent hosts (click to open one), the saved tunnels with a check mark on the active ones (click to start or stop them without opening a window), **Lock the vault** when it is unlocked, and **Quit**.

## Updates {#updates}

**Settings → Updates** checks for new versions (and, unless turned off, does so at every start and every 6 hours while the app stays open). When one is found a notice appears in the corner: **Install** downloads and installs it, then offers **Restart now** or **Later** (the new version starts the next time you open the app); **Settings** opens Settings › Updates, **Remind me tomorrow** hides it for a day, **Skip this version** until the next one, and **Don't show these notices** turns it off (Settings → Updates turns it back on; the status bar still shows the new version). When a release is out but its package for your system is not published yet (it follows within the hour), the notice says so and Shellvane checks again by itself. Only full releases are offered, not pre-releases (`v1.2.0-rc1`); if none is published yet, the page says there is nothing to update to. Updates come from the releases of the project's public GitHub repository (github.com/csturiale/shellvane), where every release publishes a manifest signed with the project's key; Shellvane installs an update only if that signature and the package's checksum are valid, and keeps the previous version until the next start. The check tells GitHub only Shellvane's version and your system (for example `Shellvane/1.2.0 (windows/amd64)`), which is how the project counts how many copies of each version are in use; there is no identifier and nothing about you, your hosts or your use of the app. On macOS the whole app is replaced with the new one. If macOS runs Shellvane from where it was downloaded (it then uses a temporary read-only copy) or from a folder you cannot write to, move Shellvane to Applications first; until then, use the release page.

**macOS says Shellvane cannot be opened** ("Apple could not verify…", *Move to Trash*): releases not yet signed with an Apple Developer ID are blocked on first open. Open **System Settings › Privacy & Security**, scroll to Security and choose **Open Anyway** for Shellvane (once), or run `xattr -dr com.apple.quarantine /Applications/Shellvane.app` in Terminal.

**Homebrew** (macOS, Apple silicon): `brew tap csturiale/shellvane && brew trust csturiale/shellvane && brew install --cask shellvane && brew install shellvane-cli` installs the app and the CLI (the CLI also on Linux). Recent Homebrew versions load a third-party tap only after `brew trust`; without it they stop with "Refusing to load cask … from untrusted tap". Installed this way, macOS opens the app without the security prompt.

## Network and proxy {#network}

**Settings › Network** sets how Shellvane's own requests reach the internet: update checks, the gallery, the plugin store, usage reports, WebDAV and S3 sync and credential plugins. git (team and sync storage) and kubectl get the same proxy. SSH connections are not affected: they use jump hosts and proxy commands.

- **System** (default): the proxy variables (`HTTPS_PROXY`, `NO_PROXY`…, also those set in your login shell), else the proxy configured in macOS or Windows network settings. Automatic configuration (PAC) scripts are not read: use **Manual** with the proxy the PAC file names.
- **None**: direct connections.
- **Manual**: a proxy address (`http://proxy.example.org:8080`, `https://…` or `socks5://…`, with `user:password@` if needed) and the hosts reached directly.

**Check connection** tries the addresses the app uses (GitHub's release downloads for updates, shellvane.com for the gallery) and says, for each, whether it was reached, through which proxy, or why not. When a company web filter (Zscaler, Netskope, Forcepoint…) refuses a site, the check says so: the proxy works, but the site is not allowed. A new domain is often refused as *uncategorized*: IT can allow it, or have it categorized (Zscaler: Site Review). Updates need `github.com` and `objects.githubusercontent.com`; the gallery, the plugin store and usage reports need `shellvane.com`.

## Installing {#install}

Every release is on [shellvane.com/download](https://shellvane.com/download/) and has: a Windows installer (and the bare `shellvane-windows-amd64.exe`), the macOS app for Apple silicon Macs (M1 or later; Intel Macs are not supported) as a disk image (`.dmg`: drag Shellvane to Applications) and in a zip, and for Linux a `.deb`, an `.rpm` and an AppImage (which updates itself in place). `shellvane-cli` is published for each system too. Releases whose version has a suffix (`v1.2.0-rc1`) are pre-releases.
