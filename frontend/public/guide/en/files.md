# Files (SFTP) {#top}

The Files view has two independent panes. Each shows **this computer** or any saved host, so you can copy between this computer and a server, or **directly between two servers**.

## Browse {#browse}

Choose the location at the top of a pane (a searchable list: type part of a name, address, group or tag), then navigate by double-clicking folders, using the path bar or the up button.

## Copy {#copy}

Select files or folders and use the arrow toward the other pane, or drag them across. Folders are copied with all their content. The transfer queue shows progress and lets you cancel.

## Permissions {#permissions}

**Permissions** edits read/write/execute for owner, group and others, the special bits (setuid, setgid, sticky) and the octal value; on a folder it can apply to everything inside.

## Other actions {#actions}

New folder, rename, delete (with confirmation), refresh. Files dropped on the window are uploaded to the focused remote pane.

## Conflicts and resume {#conflicts}

**If the file exists** (toolbar of the Files view) decides what happens when a file is already there: overwrite, skip, keep both (renamed "name (1).ext"), overwrite only if newer, or **resume**, which appends the missing part of a file whose transfer was interrupted (a cancelled resumable transfer keeps the partial file).

## Edit on this computer {#edit}

The pencil-on-file button downloads a file and opens it with your default application. Every time you save, Shellvane uploads it again; the toolbar shows the state. **Done** stops watching and deletes the local copy.

## Owners, links, bookmarks {#more}

- The permissions dialog also changes the numeric owner and group (chown).
- Symbolic links show where they point; the link button creates one.
- The star bookmarks the current folder of that host; bookmarks appear in the pane header.

## Follow the terminal {#cwd}

When the shell reports its current folder with OSC 7, a folder button appears in the pane header: it opens that folder in Files. Most shells need one line in their startup file, e.g. for bash:

```
PROMPT_COMMAND='printf "\033]7;file://%s%s\033\\" "$HOSTNAME" "$PWD"'
```

for zsh: `precmd() { printf '\e]7;file://%s%s\e\\' "$HOST" "$PWD" }`. Shellvane never changes files on the server by itself.

## Servers without SFTP {#nosftp}

If a server has no SFTP subsystem, Shellvane uses shell commands instead (like scp): a "no SFTP" badge marks the pane. It needs a POSIX shell and GNU find, and is slower.

[Open Files](shellvane:view/files)

## Preview {#preview}

The eye button of a file shows its beginning (text up to 512 KB) or the image itself (PNG, JPEG, GIF, WebP, BMP, AVIF), without downloading it. SVG files are shown as text.

## Compare and sync folders {#compare}

**Compare folders…** compares the folders shown on the two sides, including subfolders: files only on one side, newer on one side (by modification time, within a second), or different in size. Choose the direction, tick what to copy (new and newer files are ticked), and optionally delete what exists only in the folder being updated. Copies run as normal transfers.
