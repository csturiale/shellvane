# Runbooks {#top}

A runbook is a saved sequence of commands that runs on many hosts at once. Results appear in a grid: one row per host, one column per step, with the exit code and the output of every cell.

Open **Automation → Runbooks** to create one, or use **Run on several hosts** on a snippet to start from it. Runbooks can be exported and imported as YAML, so you can keep them in a repository and share them.

[Open Automation](shellvane:view/automation)

## Targets {#targets}

Choose where a runbook runs:

- **Hosts**: any saved host;
- **Groups**: every host of the group and of its subgroups;
- **Tags**: every host with one of the tags.

A host selected more than once runs only once. The start dialog shows how many hosts match before you run.

Hosts run **in parallel** (optionally limited to a maximum number at once) or **in sequence**. In sequence you can stop at the first host that fails, which is useful for a rolling change: if the first server breaks, the others are left untouched.

## Steps {#steps}

Each step is a shell command run on its own (a new exec channel, not the interactive shell). Options:

- **Run when**: always, only if the previous step succeeded, only if it failed, or only if its output matches a regular expression. "Previous step" is the last step that actually ran on that host.
- **Continue if it fails**: otherwise a failed step skips the rest of the steps on that host.
- **Ask before running**: the cell waits for you; choose **Continue** or **Stop** per host, or continue on all hosts at once.
- **Timeout**: the step fails if it takes longer.

Write `{{name}}` in a command to ask for a value when the runbook starts; declared variables can have a default and a list of choices. `{{steps.<step name>.output}}` inserts the output (trimmed) of an earlier step on the same host.

**Dry run** shows the resolved command of every step for every host without running anything. When a run is over, export the results as CSV or JSON. Every executed command is written to the audit log.
