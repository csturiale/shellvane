# Sync between computers {#top}

Shellvane can keep several computers in step through a folder they already share: Syncthing, Dropbox, iCloud Drive, OneDrive, Google Drive or a network share. There is no Shellvane server.

## Set it up {#setup}

1. Unlock the vault (the sync key is kept in it).
2. In **Settings → Sync** choose where the encrypted files go and a **sync passphrase**, then **Start syncing**:
   - **Shared folder**: a folder kept in step by iCloud Drive, Google Drive, Dropbox, OneDrive, Syncthing or a network share. The folders of the cloud drives installed on this computer are offered as buttons (iCloud Drive on macOS and Windows; Google Drive for desktop, also as a drive letter on Windows; Dropbox; OneDrive; Syncthing's `~/Sync`). Shellvane uses the `Shellvane-sync` subfolder; the drive's own app does the syncing, so it must be installed and signed in on each computer.
   - **WebDAV server**: the URL of a folder on Nextcloud, ownCloud, a NAS or any WebDAV server, with user and password (an app password is best). Shellvane creates the folder if it is missing; the password is kept in the vault.
   - **S3 bucket**: Amazon S3 or a compatible service (MinIO, Cloudflare R2, Backblaze B2, Wasabi): endpoint, bucket, region, access key and secret key (best limited to that bucket). Files go in the bucket's `Shellvane-sync` folder; the secret key is kept in the vault.
   - **Git repository**: the URL of a private repository you can already push to with `git` (SSH key or credential helper), e.g. `git@github.com:me/shellvane-sync.git`. Shellvane keeps a clone in its data folder, pulls before each sync and pushes after it.
3. On each other computer choose the same place (the folder as it appears there, the same URL) and the same passphrase.

Sync runs every five minutes while the vault is unlocked, and on demand with **Sync now**.

## What is synced {#what}

Hosts and groups (with their settings, tags and notes), snippets, tunnels, automation rules, environments, runbooks and color schemes. The vault (saved passwords, keys, TOTP seeds), workspaces and preferences stay on each computer: a host that uses a vault key needs that key on each computer.

## How it works {#how}

Each computer writes only its own file in `Shellvane-sync` (or in the WebDAV folder, or commits only that file to the repository), so nothing ever has to be merged by the service. A file is encrypted end to end with XChaCha20-Poly1305 under a key derived from the passphrase (Argon2id): the folder service, the WebDAV server, the Git host or the S3 service, and anyone with access to them, sees nothing in clear. For every item the most recent change wins; deletions are propagated for 90 days. An item that cannot be applied (a runbook or scheme whose name is already used by another one) is listed after the sync.

**Stop syncing on this computer** forgets the setup here (and the local Git clone); the folder, server or repository and the other computers are untouched.
