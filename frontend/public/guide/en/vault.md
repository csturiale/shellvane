# Vault {#top}

The vault is an encrypted store on this computer for passwords, key passphrases, private keys and TOTP secrets.

## How it is protected {#security}

The master password is turned into a key with Argon2id; that key unlocks a random data key; every secret is encrypted with XChaCha20-Poly1305 and bound to its name, so secrets cannot be swapped. The master password itself is never stored. Without it the vault cannot be opened: there is no recovery.

## Lock and unlock {#lock}

- The lock icon in the title bar locks or unlocks it; it also locks by itself after a period of inactivity.
- When a connection needs a saved secret while the vault is locked, Shellvane opens it with the system keychain (Touch ID, Windows Hello) when that is enabled, otherwise it asks for the master password (or you can skip and type the password).
- When a login asks for a password, passphrase or PIN, **Save in vault** works even if the vault is locked or not created yet: the dialog asks for the master password there (or creates the vault), then saves the secret after a successful login.

## What is saved {#content}

Settings › Vault lists the saved credentials (never their values) and lets you delete them or change the master password.

[Open the vault settings](shellvane:view/settings)

## Auto-lock and recovery {#recovery}

The vault locks after a period of inactivity (15 minutes by default, or never) and when the computer goes to sleep; both are set in **Settings → Vault**. **Never offer to save passwords** hides the "save in vault" option of password prompts.

A **recovery key** is a random code that can set a new master password if you forget it. It is shown once when created: write it down and keep it away from this computer. Creating a new one replaces the old one; it can be removed at any time. Without a recovery key a forgotten master password means the saved secrets are lost.

## Unlock with the system keychain {#keychain}

In **Settings → Vault**, **Unlock with the system keychain** keeps a random secret in the macOS Keychain, the Windows Credential Manager or, on Linux, the Secret Service (GNOME Keyring, KWallet). From then on the vault opens by itself when Shellvane starts, and the locked vault shows **Use the keychain** after an automatic lock. The system may ask you to allow access the first time.

The secret opens the vault only on this computer: the master password and the recovery key keep working everywhere, and changing the master password does not turn the option off. Anyone who can use your session on this computer can open the vault, so leave the option off on shared computers. Turning it off deletes the secret from the keychain.

**Ask for Touch ID** (macOS) or **Ask for Windows Hello** (Windows), shown once the keychain unlock is on, adds a confirmation: the vault then opens after your fingerprint, your face, or the device's password or PIN, and not by itself. Turning it on asks once, to check that it works; cancelling leaves the vault locked, and the master password keeps working. It is not available on Linux. Windows Hello's dialog can open behind Shellvane's window: look for it in the taskbar.
