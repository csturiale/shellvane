# Snippets {#top}

Snippets are saved commands, organized in folders, sent to the focused pane (and to the others when broadcasting).

## Variables {#variables}

- `{{service}}` asks for a value before sending.
- `{{env:dev|staging|prod}}` asks to choose one of the values.
- `{{host}}`, `{{user}}`, `{{date}}` are filled in from the focused pane and the clock.

When asked, each value can be remembered as a [personal value](shellvane:topic/environments#personal): **Remember everywhere** fills that name in every snippet, environment and procedure, **Only here** in this snippet alone. A snippet whose values are all remembered is sent without asking; **My values** (the person icon) changes them. Personal values stay on this computer: a shared snippet carries only the names.

## Send {#send}

Click **Send** in the Snippets panel, or find the snippet in the command palette (`#` searches snippets only); a snippet with variables opens the Snippets panel and asks for them. **Run** presses Enter after the text; without it the command is only typed, for you to review.

## Example {#example}

`journalctl -u {{service}} --since "{{since:1 hour ago|today}}" -f`

[Open Snippets](shellvane:view/snippets)

## Import and export {#share}

The download button at the top of the panel imports one or more files:

- a file saved by Shellvane (`shellvane-snippets.json`);
- a shell script or a text file of commands: each command becomes a snippet, named by the `#` comment just above it (otherwise by the command itself) and filed in a folder named after the file. A line ending in `\` continues on the next one.

Snippets already present with the same folder, name and text are skipped. The upload button saves the snippets shown (all of them, or those matching the filter) as `shellvane-snippets.json`, to pass to a colleague.

## Community gallery {#gallery}

The **Community gallery** button in the side bar, above the Guide (also in the command palette, the shop button at the top of the Snippets panel and Settings › Terminal for color schemes), opens the gallery of shellvane.com: collections of snippets (Kubernetes, Linux diagnostics…) and terminal color schemes, added in one click. The gallery is signed by the Shellvane project and every file is checked against it before it is added; snippets are only typed when you run them, so read them first. **Propose yours** explains how to send a collection or a scheme.
