# Import {#top}

**Import** (in the Hosts panel header or the command palette) reads hosts from other tools. A preview shows what will be created; existing hosts are not duplicated, and a backup of Shellvane's database is taken first.

## Preview {#preview}

The preview shows the hosts in their folders, nested as in the source tool. Fold a folder with its arrow; its checkbox chooses or leaves out every host below it.

- **User for hosts without one** is set on the chosen hosts that have no user (many PuTTY and KiTTY sessions don't); the addresses in the preview show it.
- **Put everything in a new group** (on by default, named after the tool) holds the import in one group; turn it off to put the imported folders and hosts at the top level, where folders with the same name as yours are merged.

## Sources {#sources}

- **OpenSSH**: `~/.ssh/config` with `Include`, `Host` patterns, `HostName`, `User`, `Port`, `IdentityFile`, `ProxyJump`, `ProxyCommand`, `SetEnv`, `RemoteCommand`, `PreferredAuthentications`, `IdentitiesOnly`, `LocalForward`/`RemoteForward`/`DynamicForward` (as tunnels).
- **PuTTY**: saved sessions (Windows registry, or the files of PuTTY on Linux).
- **KiTTY**: sessions from the registry or the portable `Sessions` folder, keeping KiTTY's folders as groups (subfolders of a portable `Sessions` folder too). The folder can also be given as a `.zip`.
- **MobaXterm**: SSH sessions of `MobaXterm.ini` or a `.mxtsessions` export, with their folders.
- **SecureCRT**: the `Sessions` folder; subfolders become groups.
- **WinSCP**: SFTP and SCP sites of `WinSCP.ini` (export it from the registry with Tools → Export configuration).
- **FileZilla**: SFTP sites of `sitemanager.xml`, with their folders.
- **Tabby**: SSH profiles of `config.yaml`, with groups and jump hosts.
- **mRemoteNG**: SSH connections of `confCons.xml`, with their folders.
- **Termius**: export the hosts as an SSH config file in Termius, then choose that file.

Sessions of other protocols (RDP, VNC, FTP, Telnet, serial…) are listed as not imported; Telnet and serial consoles open from quick connect instead ([Connecting](shellvane:topic/connecting#telnet-serial)). To move hosts between two Shellvane installations, export a group ([Hosts](shellvane:topic/hosts#templates)) or use [sync](shellvane:topic/sync). Saved passwords are never imported, not even obfuscated ones: save them in the vault at the first login.

Keys referenced by `IdentityFile` can then be imported in Keys.

[Import hosts](shellvane:action/import)
