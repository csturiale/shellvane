# Procedures {#top}

A procedure is a Markdown page of operating instructions with commands: a runbook in a Git repository, a README, a page of notes, or a web page such as Confluence sent from the browser. The **Procedures** panel shows it beside your terminals, so you paste or run its commands in the focused pane instead of switching between a document and the terminal.

## Sources {#sources}

Click **+** in the panel header and choose where the procedures are:

- **Git repository**: the URL (`git@bitbucket.org:acme/runbooks.git` or `https://…`), optionally a branch and a folder inside the repository. Shellvane keeps a read-only copy, cloned with your own Git access (SSH key or credential helper, as in a terminal), and never writes back. **Update from the repository** fetches the latest version.
- **Folder on this computer**: a folder of Markdown files, e.g. a clone you already keep up to date yourself.

Every `.md` and `.markdown` file of the source is listed with its first heading as title; filter the list to find one. With several sources, a menu above the list switches between them.

## Web pages {#web}

Procedures on Confluence, a wiki or any page behind your company sign-in come in through the browser, already signed in. Click the link icon of the panel (**From a web page**):

- **Send to Shellvane**: drag the button to the browser's bookmarks bar (or copy its code and create a bookmark with it as address). On a procedure's page, click the bookmark: the selected text, or else the page's article, is turned into Markdown (headings, lists, tables, links and code blocks with their language) and opens in Procedures › **Received pages**. The browser may ask once whether to open Shellvane. A long page travels through the clipboard; Shellvane takes from the clipboard only what the bookmark put there, recognized by a marker, never anything else you copied.
- **Paste a page**: copy the page, or a part of it, in the browser and paste it in the dialog; the formatting becomes Markdown, which you can still correct before saving.

Received pages are kept on this computer (the last 200), with the page's address at the top; delete one with its bin icon. The content of a page is never trusted: nothing in it runs by itself.

## Commands {#commands}

Each code block of the procedure gets a number and its actions:

- **Paste** types the command in the focused pane without pressing Enter, so you can read or change it first.
- **Run** types it and presses Enter.
- Blocks that are not shell commands (YAML, JSON, SQL…) only offer **Copy**.

Nothing runs by itself: every command is a click. In a session transcript (a `console` block, or lines that start with `$ `) only the commands are sent, without the prompt and without the output shown in the document. When the focused pane broadcasts to others, the commands go to all of them, and dangerous commands ask for confirmation as for snippets ([Broadcast](shellvane:topic/broadcast#safety)).

A command pasted or run is marked as done: the counter in the header shows how far you are, the circle of a block marks it done or not by hand, and **Start again** clears the marks. Links to other Markdown files of the source open them in the panel; web links open in your browser. Raw HTML and images of the document are not shown.

## Placeholders {#placeholders}

Write `<name>` or `{{name}}` in a command, e.g. `kubectl -n <namespace> rollout restart deploy/web`. The first time a command with a placeholder is used, Shellvane asks its value; the value then fills that placeholder in every command of the source until the app is closed. **Values of the placeholders** in the header changes them. A value left empty keeps the placeholder as written.

## Make a runbook {#runbook}

**Make a runbook from this procedure** turns its commands into the steps of a new runbook (placeholders become variables) and opens Automation, where you choose the hosts and run them on many servers at once ([Runbooks](shellvane:topic/runbooks)).
