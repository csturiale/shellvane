# Security: threat model and code review

## Reporting a vulnerability

Please do not open a public issue. Use **Report a vulnerability** in the
repository's *Security* tab (GitHub private vulnerability reporting). You will
get an answer within a week; fixes are released as soon as they are ready and
credited unless you prefer otherwise.

Last full code review: November 2026.

## What Shellvane protects

- **Vault**: Argon2id (64 MiB, t=3, OWASP), a random data key wrapped with
  XChaCha20-Poly1305; each secret is encrypted with an AEAD bound to its ID and
  type (secrets cannot be swapped in the database). Optional unlock from the
  system keychain and Touch ID/Windows Hello.
- **SSH**: a single connection point (`core/sshengine`), always verifying the
  host key (Shellvane's own known hosts; a changed key = refusal). Agent
  forwarding with confirmation of each signature; Shellvane's agent with
  confirmation of each signature, socket/pipe accessible only to the user.
- **Updates**: Ed25519-signed manifest (key in the binary), SHA-256 of the
  package, only newer versions; extraction without symbolic links or paths
  outside the folder.
- **Plugins**: the official store's index is signed with the update key
  (`plugins/index.json.sig`, verified by the app: someone who controlled only
  the website could not make it install anything else), packages with a
  mandatory SHA-256, safe extraction (no `..`, absolute paths or symbolic
  links; size limits), validated IDs. An installed plugin is trusted code: it
  runs with the user's permissions.
- **Gallery**: the index of snippets and themes is signed the same way, and
  each file is installed only if it matches the SHA-256 in the index.
- **Team** (in the app from termoraplus.com): end-to-end encrypted file, member list signed by the admins, items
  signed by whoever writes them, owner with a signed handover, signed versions
  against rollbacks, shared commands only after approval.

## Found and fixed in this review

| Severity | Problem | Fix |
| --- | --- | --- |
| Critical | Any team member could share a host group with `ProxyCommand`, `PasswordCommand`, `CertificateCommand` or `PreConnectCommand`: commands run on every member's computer on connection (besides startup/remote commands and X11 forwarding). | Commands of shared groups arrive only after approval (as for environments and rules); until then the copy has none. Pasted exports are imported without commands, which are listed. |
| High | The team ID in an invitation was not validated and ended up in paths: a crafted invitation could delete or write folders outside Shellvane's (`../..`). | IDs accepted only as 16 hex digits, checked on invitation, opening, storage and leaving; the file's ID must match. |
| High | `ProxyCommand`: `%h` and `%r` inserted unchecked into `/bin/sh -c`; a host or user with shell characters ran commands. | Hosts/users with unsafe characters refused (as in OpenSSH 9.6); hostnames and users validated on save. |
| High | Arguments to the system `ssh`/`mosh` and `git`: a host, user or address starting with `-` became an option (e.g. `-oProxyCommand=…`). | `--` before the destination for `mosh` and `git clone`; destinations starting with `-` refused; hostnames validated. |
| High | SFTP: when downloading a folder, file names sent by the server (`..`, names with `/`) could write outside the destination. | Names `.`/`..` or with separators ignored. |
| Medium | Shared environments: variable defaults and choices (inserted into commands) and tunnel addresses were not part of the command review. | Included in the review: every change asks for a new approval. |
| Medium | "Edit" on a remote file opened it with the default app: a `.command`, `.exe`, `.desktop`… file was executed. | Executable extensions are not opened for editing. |
| Medium | Links in the app window: terminal links (also OSC 8 sent by the server, with any scheme such as `file:`) and those in host notes could open in the webview itself, which has access to Shellvane's functions. | Every link goes to the system browser, and only if `http`/`https`; the app window never leaves Shellvane. |
| Medium | No Content Security Policy: injected content could have run scripts with access to the app's functions (local commands too). | CSP: only the app's scripts, no inline or remote scripts, no plugins or frames (`unsafe-eval` stays for messageformat translations). |
| Medium | Unsigned plugin store index: whoever controlled the website decided what to install. | Signed with the update key and verified by the app for the official store. |
| Low | Git HTTPS token sent as a header to any URL git contacted. | Header limited to the repository's URL. |
| Low | Windows pipes with a fixed name: on a shared computer another user could take them first. | Pipe name with the user's SID. |

## Known limits (accepted or to be decided)

- **Team rollback** is detected only against the highest version that computer
  has already seen: a computer that never saw the new version cannot notice.
- **Server mode** (`-tags server`): no authentication, listens only on
  localhost; used for tests, not distributed.
- **WebDAV over http://** and **Telnet** transmit in clear: the user's choice.
- **Installed plugins** and **approved rules/environments** run code with the
  user's permissions by design.
