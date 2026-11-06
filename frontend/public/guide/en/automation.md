# Automation {#top}

## Output rules {#rules}

The Automation section holds rules that watch terminal output line by line. Each rule has a regular expression, an action and a scope: all sessions, a group (and its subgroups) or a host.

- **Highlight** colors the matching text.
- **Notify** shows a notification (a system notification when Shellvane is in the background), optionally with a sound. Clicking it brings the pane to the front.
- **Respond** types an answer when a prompt appears.

Lines you type yourself never trigger rules: typing `echo panic:` does not match `panic:`, but the command's output does. Use **Test** in the editor to try a pattern on a sample line. Rules apply to sessions opened after a change.

[Open Automation](shellvane:view/automation)

## Automatic responses {#respond}

A respond rule types fixed text or a vault credential (e.g. a sudo password), then Enter if chosen. Because the server's output triggers it:

- it can only apply to a group or a host, never to everything;
- at most 3 responses per minute per rule;
- never in panes that receive broadcast input;
- every response is written to the **audit log** (never the secret).

## Startup scripts {#scripts}

The **Startup commands** field of a host or group is a small script. Plain lines are typed after login, as before; these steps add control:

- `expect <regex>` waits until the output matches (default timeout 15 s);
- `timeout <seconds>` changes the timeout of the following expects;
- `wait <seconds>` pauses;
- `sendSecret <vault scope>` types a vault credential and presses Enter;
- `send <text>` types a line (useful when it starts with one of these words);
- `# …` is a comment.

Example:

```
expect \$ $
cd /srv/app
sudo -v
expect password for
sendSecret sudo:web
```

If a step fails, a notification tells which one; the shell stays usable.

## Audit log {#audit}

The Audit log tab lists what automation did, newest first, and exports it as CSV or JSON.
