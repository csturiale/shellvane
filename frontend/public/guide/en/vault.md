# Vault {#top}

The vault is an encrypted store on this computer for passwords, key passphrases, private keys and TOTP secrets.

## How it is protected {#security}

The master password is turned into a key with Argon2id; that key unlocks a random data key; every secret is encrypted with XChaCha20-Poly1305 and bound to its name, so secrets cannot be swapped. The master password itself is never stored. Without it the vault cannot be opened: there is no recovery.

## Lock and unlock {#lock}

- The lock icon in the title bar locks or unlocks it; it also locks by itself after a period of inactivity.
- When a connection needs a saved secret while the vault is locked, Shellvane opens it with the system keychain (Touch ID, Windows Hello) when that is enabled, otherwise it asks for the master password (or you can skip and type the password). When the keychain unlock is on but did not open the vault, the dialog says why (Touch ID cancelled or unavailable, secret no longer in the keychain).
- Everything else that needs the vault while it is locked (saving a plugin's token, Sync now, a team action, adding a key, the assistant's settings) opens it the same way — the system keychain first, after Touch ID or Windows Hello when they are on, the master password only if that is not set up or is cancelled — and then carries on; messages about the locked vault have an **Unlock the vault** button, and so does the vault status at the bottom of the window.
- Only one unlock runs at a time: when several panes of an environment, the app's start and a dialog all need the vault, Touch ID or Windows Hello is asked once and the others go on. If the system keychain no longer has Shellvane's secret (it was deleted, or the keychain was reset), Touch ID is not asked for nothing: the keychain unlock turns itself off, and the next master password turns it on again with a new secret (with Touch ID or Windows Hello again if it was required). This also happens after installing a build of Shellvane signed differently.
- When a login asks for a password, passphrase or PIN, **Save in vault** works even if the vault is locked or not created yet: the dialog asks for the master password there (or creates the vault), then saves the secret after a successful login.

## What is saved {#content}

Settings › Vault lists the saved credentials (never their values) and lets you delete them or change the master password.

[Open the vault settings](shellvane:view/settings)

## Auto-lock and recovery {#recovery}

The vault locks after a period of inactivity (15 minutes by default, or never): using the vault or typing in a terminal counts as activity and when the computer goes to sleep; both are set in **Settings → Vault**. **Never offer to save passwords** hides the "save in vault" option of password prompts.

A **recovery key** is a random code that can set a new master password if you forget it. It is shown once when created: write it down and keep it away from this computer. Creating a new one replaces the old one; it can be removed at any time. Without a recovery key a forgotten master password means the saved secrets are lost.

## Unlock with the system keychain {#keychain}

In **Settings → Vault**, **Unlock with the system keychain** keeps a random secret in the macOS Keychain, the Windows Credential Manager or, on Linux, the Secret Service (GNOME Keyring, KWallet). From then on the vault opens by itself when Shellvane starts, and the locked vault shows **Use the keychain** after an automatic lock. The system may ask you to allow access the first time.

The secret opens the vault only on this computer: the master password and the recovery key keep working everywhere, and changing the master password does not turn the option off. Anyone who can use your session on this computer can open the vault, so leave the option off on shared computers. Turning it off deletes the secret from the keychain.

**Ask for Touch ID** (macOS) or **Ask for Windows Hello** (Windows), shown once the keychain unlock is on, adds a confirmation: the vault then opens after your fingerprint, your face, or the device's password or PIN, and not by itself. Turning it on asks once, to check that it works; cancelling leaves the vault locked, and the master password keeps working. It is not available on Linux. Windows Hello's dialog can open behind Shellvane's window: look for it in the taskbar.
