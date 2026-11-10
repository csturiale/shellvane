# Kubernetes context per pane {#top}

Each terminal pane can have its own `kubectl` context and namespace, independent from every other pane — on this computer and on servers. kubectl, helm, k9s and kubectx all follow it.

## How it works {#how}

Shellvane never runs `kubectl config use-context` on your kubeconfig (that would switch every terminal at once). Instead each pane's `KUBECONFIG` starts with a small file of its own, followed by your usual kubeconfig. The pane's file only names the current context (and, with a namespace, a copy of that context with the other namespace): your credentials are not copied. Switching rewrites the pane's file, so it takes effect for the next command, even inside tmux.

## Local terminals {#local}

Every local pane has its own file. Click the ship-wheel badge in the pane header, pick a context (read from `kubectl config get-contexts`) and optionally a namespace, then **Apply**. The default for new local terminals is in Settings › Local terminal.

On macOS and Linux Shellvane reads the `PATH` of your login shell at startup, so `kubectl`, `gcloud`, `aws` and their credential plugins installed with Homebrew or in `~/.local/bin` are found even when the app is opened from the Dock or the Finder; kubectl waits for it at startup, and when the shell's startup files are too slow to read the usual folders (Homebrew, `~/.local/bin`, `~/bin`, krew, the Google Cloud SDK, Docker and Rancher Desktop) are added anyway. If listing contexts fails, the message shows kubectl's error.

## Servers {#remote}

Set **Kubernetes context and namespace** under Advanced on a host or group (e.g. group *Production* → `prod-eu`): its panes get their own file under `~/.kube/shellvane` on the server, and the picker lists the server's contexts. A pane opened without a context gets one by reopening (a tmux session is resumed).

## Production safety {#prod}

The context is shown in the pane header and on the tab. Contexts matching the **Production contexts** pattern (Settings › Local terminal, default `prod`) are shown in red. The pattern is matched against the kubectl context's name (as `kubectl config get-contexts` shows it), not against the environment's name on the pane's badge.

## Limits {#limits}

- kubectl and the contexts must exist where the pane runs: Shellvane does not copy your local kubeconfig to servers.
- A shell startup file that sets `KUBECONFIG` itself (e.g. in `.bashrc`) takes precedence.
- Running `kubectl config use-context` in a pane changes that pane's file only.
