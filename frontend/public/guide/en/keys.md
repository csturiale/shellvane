# SSH keys {#top}

The Keys section manages private keys stored encrypted in the vault (the vault must be unlocked).

## Generate {#generate}

**Generate** creates Ed25519 (recommended), RSA 3072/4096 or ECDSA keys. The comment helps you recognize the key on servers.

## Import {#import}

Paste an OpenSSH, PEM or PuTTY `.ppk` (v2 or v3) private key; encrypted keys need their passphrase once. Duplicates are detected by fingerprint.

## Hardware tokens (YubiKey, smart cards) {#token}

A key kept on a YubiKey, a smart card or an HSM never leaves the device: the token signs each login after its PIN. Shellvane talks to it through the maker's PKCS#11 module: OpenSC (most smart cards and the YubiKey PIV applet), Yubico's `ykcs11`, or your HSM's library.

1. Install the module (for example OpenSC) and plug the token in.
2. In **Keys**, the USB button **Add a key from a hardware token** lists the modules found in their usual places; another path can be typed in. **Find tokens**, choose the token and the key, give it a name.
3. Assign it to hosts or groups with **SSH key**, like any other key.

At the first login the PIN is asked; it can be saved in the vault. A wrong PIN is asked again up to three times, then Shellvane stops, before the token locks itself. The token is logged out as soon as the connection is open. A key on a token cannot be exported or rotated, and the Shellvane agent does not offer it to other programs. The public key, to install on servers, is copied like any other.

## Install on a host {#deploy}

**Install** connects to a host (with any method that works now, e.g. the password) and appends the public key to `~/.ssh/authorized_keys`, creating it with the right permissions. Running it twice does not add the key twice.

## Use it {#assign}

Choose the key in the **SSH key** field of a host or group. **Copy** puts the public key on the clipboard.

## Certificates {#certificates}

The badge icon attaches an OpenSSH certificate signed by your CA. Its principals and expiry are shown; an expired certificate is ignored and the bare key is used. For short-lived certificates issued at each login, see [certificate command](shellvane:topic/auth#certificates).

[Open Keys](shellvane:view/keys)

## Where used, export and rotation {#rotation}

The network icon of a key lists the groups that set it and the hosts that use it, directly or through their group.

**Rotate key** replaces it safely: Shellvane creates a new key of the same type, installs it on every host that uses the old one, logs in to each host with the new key alone (no agent, no password) to prove it works, and only then makes those hosts and groups use it. If any host does not accept it, nothing is switched. Optionally the old key is then removed from the hosts' `authorized_keys`. The old key stays in Shellvane until you delete it; deleting a key that hosts use asks first.

**Export** gives the private key as OpenSSH or PuTTY (PPK v3) text, encrypted with a passphrase if you set one. This also converts between the two formats: import a `.ppk`, export it as OpenSSH, or the other way round.

## Shellvane agent {#agent}

**Shellvane agent** (Keys panel) makes the vault's keys available to other programs on this computer, such as `ssh`, `git` and `scp`, without copying them to disk. Click **SSH_AUTH_SOCK** to copy the line to paste in a terminal (or your shell profile); on Windows it is a named pipe.

Every signature asks first, saying which key is used; the vault must be unlocked, and a locked vault offers no key. Programs cannot add or remove keys.
