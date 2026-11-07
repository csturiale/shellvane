# Authentication {#top}

Shellvane tries, in order: the key assigned to the host, your SSH agent, the keys in `~/.ssh`, keyboard-interactive (2FA), then the password.

## Passwords {#password}

When the server asks for a password, Shellvane shows a dialog. With the vault unlocked, tick **Save in vault**: it is stored only after a successful login, and used silently next time (if it is refused, you are asked again).

## Keys and agent {#keys}

- Keys in the vault are assigned per host or group (field **SSH key**).
- Your agent is used automatically: `SSH_AUTH_SOCK` on macOS/Linux; on Windows the OpenSSH agent service, Pageant, or `SSH_AUTH_SOCK` (1Password, Bitwarden, KeePassXC agents work too).
- Encrypted keys in `~/.ssh` ask for their passphrase only if the server accepts the key.
- Every key is tried in turn until the server accepts one: the host's key first, then the agent's, then `~/.ssh/id_ed25519`, `id_ecdsa` and `id_rsa`, each offered once. **Diagnostics** shows which keys were offered.

## Kerberos and Active Directory {#kerberos}

In companies with Active Directory (or MIT/FreeIPA Kerberos), servers often accept the domain login itself: no key, no password typed for each server. Turn on **Kerberos / Active Directory (GSSAPI)** in the host's or group's advanced options:

- With a Kerberos ticket on this computer — the domain login of a Linux workstation, or `kinit user@CORP.EXAMPLE.COM` — Shellvane uses it and connects without asking anything.
- Without a ticket (or on Windows and macOS, whose ticket stores Shellvane cannot read), Shellvane asks the **domain password** once per connection and gets a ticket from the domain controller.
- The realm is taken from the ticket, from `krb5.conf`, or from the host's domain in upper case (`srv1.corp.example.com` → `CORP.EXAMPLE.COM`); domain controllers are found through DNS, as Active Directory publishes them. Set realm and principal only when these do not fit.

The server must accept `GSSAPIAuthentication` and have a key for `host/<its name>`. Kerberos is tried first; keys and passwords follow if it fails. The ticket is not forwarded to the server.

## Methods and their order {#order}

By default Shellvane tries keys, then keyboard-interactive (two-factor codes, PAM prompts), then the password, like OpenSSH. In **Advanced**, **Choose the authentication methods and their order** changes the order for a host or group, and methods left unchecked are never used (OpenSSH's `PreferredAuthentications`).

Servers with a low `MaxAuthTries` close the connection after a few refused keys. Set **Keys offered** to **Only the host's key** to offer just the key chosen in **SSH key** (OpenSSH's `IdentitiesOnly`). Both settings are imported from `~/.ssh/config`.

## Two-factor codes (TOTP) {#totp}

Servers that ask for a verification code (Google Authenticator, Duo…) show their questions in a dialog. To have codes filled in automatically, paste the TOTP secret (base32, or the `otpauth://` URI behind the QR code) in the host's **Two-factor code** field: it is kept in the vault, and the code is computed at each login. A password question in the same challenge is filled from the vault or the password command.

## Password command {#password-command}

**Advanced › Password command** runs a command that prints the password, e.g. `pass show ssh/$SHELLVANE_HOST` or `op read "op://Private/$SHELLVANE_HOST/password"`. The variables `SHELLVANE_HOST`, `SHELLVANE_PORT` and `SHELLVANE_USER` describe the target. Presets cover pass, 1Password, Bitwarden and HashiCorp Vault.

## Certificates {#certificates}

- **Attach a certificate** to a key in Keys (the `-cert.pub` file signed by your CA): it is offered instead of the bare key while it is valid.
- **Certificate command** (Advanced) asks your CA for a short-lived certificate at every login: the public key is in `$SHELLVANE_PUBLIC_KEY` and in the file `$SHELLVANE_PUBLIC_KEY_FILE`; the command prints the certificate. Presets: Vault SSH secrets engine, smallstep step-ca.
- **Credentials from a plugin** (Advanced): HashiCorp Vault, CyberArk, Conjur, Teleport or Boundary give the whole login, without their CLIs' scripts: see [Plugins](shellvane:topic/plugins#credentials).

## Agent forwarding {#forwarding}

**Advanced › Agent forwarding** lets programs on the server (git, ssh to another host) use your agent's keys. Because anyone with root on the server could use it too, Shellvane asks before **every** signature: *Allow once*, *Allow for this connection*, or *Refuse*. Adding or removing keys from the server is always refused.

[More on jump hosts and proxies](shellvane:topic/jump-proxy)
