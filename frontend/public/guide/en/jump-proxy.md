# Jump hosts, proxies and cloud access {#top}

All of these are under **Advanced** in the host and group dialogs, and are inherited.

## Jump hosts {#jump}

Add one or more saved hosts as **jump hosts** (ProxyJump): the connection goes through them in order. Each hop uses its own user, key and password, and is shared: many servers behind the same bastion need one login to it. Panes show the path ("via bastion"). Loops are refused.

## Cloud access: AWS SSM, Azure Bastion, Google IAP {#cloud}

Servers in private networks are often reached through the cloud provider's access service instead of a VPN or a public address. Set **Access through** in the host's advanced options (or on a group, for all its hosts):

- **AWS Systems Manager (SSM)**: the instance ID (empty: the hostname), and optionally region and AWS profile. Needs the `aws` CLI and the Session Manager plugin, logged in (`aws sso login`); the instance needs no public address and no open port.
- **Azure Bastion**: the VM's resource ID, the Bastion's name and resource group, optionally the subscription. Needs the Azure CLI with the `bastion` and `ssh` extensions, logged in (`az login`), and a Standard Bastion with native client support. Shellvane opens `az network bastion tunnel` on a free local port and connects through it; the tunnel closes with the connection.
- **Google Cloud IAP**: the instance name (empty: the hostname), optionally zone and project. Needs `gcloud`, logged in, and a firewall rule letting IAP reach port 22.

The tools run directly, without a shell; host keys, keys, jump hosts after the first hop, tunnels and file transfers work as usual. If a tool is missing or not logged in, the connection error says so with the tool's own message.

## Proxy {#proxy}

**Proxy** applies to the first network connection: `socks5://host:1080` (with `user:password@` if needed) or `http://proxy:3128` (HTTP CONNECT).

## ProxyCommand {#proxycommand}

**ProxyCommand** runs a program and talks SSH through its input and output, like OpenSSH. `%h`, `%p` and `%r` are replaced with host, port and user. It replaces the proxy.

Presets:

- **AWS SSM**: `aws ssm start-session --target %h --document-name AWS-StartSSHSession --parameters portNumber=%p` (the host is the instance ID).
- **Google Cloud IAP**: `gcloud compute start-iap-tunnel %h %p --listen-on-stdin`.
- **Cloudflare Access**: `cloudflared access ssh --hostname %h`.
- **Teleport**: `tsh proxy ssh %r@%h:%p`.

The tools must be installed and logged in on this computer.

## Command before connecting {#preconnect}

**Command before connecting** (in **Advanced**) runs on this computer before every connection, with `$SHELLVANE_HOST`, `$SHELLVANE_PORT`, `$SHELLVANE_USER`, `$SHELLVANE_HOST_NAME` and, when the host has an **SSH key**, its public key in `$SHELLVANE_PUBLIC_KEY_FILE`. If it fails, Shellvane does not connect and shows its last error line. Presets:

- **AWS EC2 Instance Connect** pushes the key to the instance for 60 seconds (`aws ec2-instance-connect send-ssh-public-key`). Use the instance ID as the host name, usually together with the **AWS SSM** ProxyCommand, and the instance's user (`ec2-user`, `ubuntu`…).
- **Google Cloud OS Login** adds the key to your OS Login profile for one hour.

It is also handy to start a VPN or wake a bastion before connecting.

## Import {#import}

Importing an OpenSSH config keeps `ProxyJump` (linked to the imported hosts, or created) and `ProxyCommand`.
