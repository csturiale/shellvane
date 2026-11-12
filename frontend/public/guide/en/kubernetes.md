# Kubernetes context per pane {#top}

Each terminal pane can have its own `kubectl` context and namespace, independent from every other pane — on this computer and on servers. kubectl, helm, k9s and kubectx all follow it.

## How it works {#how}

Shellvane never runs `kubectl config use-context` on your kubeconfig (that would switch every terminal at once). Instead each pane's `KUBECONFIG` starts with a small file of its own, followed by your usual kubeconfig. The pane's file only names the current context (and, with a namespace, a copy of that context with the other namespace): your credentials are not copied. Switching rewrites the pane's file, so it takes effect for the next command, even inside tmux.

## Local terminals {#local}

Every local pane has its own file. Click the ship-wheel badge in the pane header, pick a context (read from `kubectl config get-contexts`) and optionally a namespace, then **Apply**. The default for new local terminals is in Settings › Local terminal.

On macOS and Linux Shellvane reads the `PATH` of your login shell at startup, so `kubectl`, `gcloud`, `aws` and their credential plugins installed with Homebrew or in `~/.local/bin` are found even when the app is opened from the Dock or the Finder; kubectl waits for it at startup, and when the shell's startup files are too slow to read the usual folders (Homebrew, `~/.local/bin`, `~/bin`, krew, the Google Cloud SDK, Docker and Rancher Desktop) are added anyway. If listing contexts fails, the message shows kubectl's error.

## Servers {#remote}

In an SSH pane kubectl runs **on the server**, with the server's kubeconfig (its `~/.kube/config`): the picker lists the server's contexts, not this computer's. On a server without kubectl the badge says so once, then goes away.

Set **Kubernetes context and namespace** under Advanced on a host or group (e.g. group *Production* → `prod-eu`): its panes get their own file under `~/.kube/shellvane` on the server, and the picker lists the server's contexts. A pane opened without a context gets one by reopening (a tmux session is resumed).

## Environments with a tunnel {#tunnel}

When a cluster is reached only through a server, an [environment](shellvane:topic/environments) opens **local** terminals with the context it names and routes kubectl through its tunnel: a SOCKS tunnel becomes the context's `proxy-url`, a port forward to the API becomes its `server`, in the pane's own file. Your kubeconfig and its credentials (e.g. `gke-gcloud-auth-plugin`) stay on this computer and are not changed; when the environment starts again after a restart, its panes get the context and the tunnel's current port back.

## Production safety {#prod}

The context is shown in the pane header and on the tab. Contexts matching the **Production contexts** pattern (Settings › Local terminal, default `prod|prd`) are shown in red. The pattern is matched against the kubectl context's name (as `kubectl config get-contexts` shows it), not against the environment's name on the pane's badge.

**Confirm risky commands in production** (on by default): when you press Enter on a `kubectl delete`, `drain`, `cordon`, `taint`, `replace`, `scale --replicas=0`, `rollout restart|undo`, `apply --prune`, `edit`, `patch`, `set`, `label`, `annotate`, or a `helm upgrade|install|uninstall|rollback`, and the pane's context or the one the command names (`--context`, `--kube-context`) is a production one, Shellvane asks before the command reaches the shell. A context chosen inside the shell with `kubectl config use-context` is not seen: use the pane's context picker or `--context`.

## Port-forwards {#forward}

In a local terminal with a context (an environment's kubectl terminal, or one where you chose a context), the terminal menu has **Port-forward to a service or pod…**. Pick a service or running pod of the namespace and its port; the local port is the same one when it is a high one (8080, 3000…), any free one otherwise, or one you type. The forward reaches the cluster as the terminal does (the environment's tunnel and gcloud profile), so it works for private clusters behind a jump host.

The forwards are listed in **Tunnels › Kubernetes port-forwards**, with the local address, a button to open web ports in the browser (https for 443 and 8443) and **Stop**. When kubectl's forward ends (the pod was replaced, the connection dropped) it starts again by itself, after 1, 2, 4… up to 30 seconds; a forward to a service then reaches the new pod. Forwards last while the app runs.

## Limits {#limits}

- kubectl and the contexts must exist where the pane runs: Shellvane does not copy your local kubeconfig to servers.
- A shell startup file that sets `KUBECONFIG` itself (e.g. in `.bashrc`) takes precedence.
- Running `kubectl config use-context` in a pane changes that pane's file only.
