# Jump host, proxy e accesso cloud {#top}

Si trovano tutti in **Avanzate** nei dialog di host e gruppi, e sono ereditati.

## Jump host {#jump}

Aggiungi uno o più host salvati come **jump host** (ProxyJump): la connessione passa da loro in ordine. Ogni salto usa il proprio utente, chiave e password, ed è condiviso: molti server dietro lo stesso bastion richiedono un solo login su di esso. I pannelli mostrano il percorso ("via bastion"). I cicli vengono rifiutati.

## Accesso cloud: AWS SSM, Azure Bastion, Google IAP {#cloud}

I server in reti private si raggiungono spesso con il servizio di accesso del provider cloud invece che con una VPN o un indirizzo pubblico. Imposta **Accesso tramite** nelle opzioni avanzate dell'host (o su un gruppo, per tutti i suoi host):

- **AWS Systems Manager (SSM)**: l'ID dell'istanza (vuoto: l'hostname), e facoltativamente regione e profilo AWS. Richiede la CLI `aws` e il plugin Session Manager, con login fatto (`aws sso login`); l'istanza non ha bisogno di indirizzo pubblico né di porte aperte.
- **Azure Bastion**: l'ID risorsa della VM, nome e gruppo di risorse del Bastion, facoltativamente la sottoscrizione. Richiede la CLI di Azure con le estensioni `bastion` e `ssh`, con login fatto (`az login`), e un Bastion Standard con supporto al client nativo. Shellvane apre `az network bastion tunnel` su una porta locale libera e si collega lì; il tunnel si chiude con la connessione.
- **Google Cloud IAP**: il nome dell'istanza (vuoto: l'hostname), facoltativamente zona e progetto. Richiede `gcloud`, con login fatto, e una regola firewall che permetta a IAP di raggiungere la porta 22.

Gli strumenti vengono eseguiti direttamente, senza shell; chiavi dell'host, chiavi, jump host dopo il primo, tunnel e trasferimenti di file funzionano come sempre. Se uno strumento manca o non ha il login fatto, l'errore di connessione lo dice con il messaggio dello strumento stesso.

## Proxy {#proxy}

Il **proxy** vale per la prima connessione di rete: `socks5://host:1080` (con `utente:password@` se serve) o `http://proxy:3128` (HTTP CONNECT).

## ProxyCommand {#proxycommand}

**ProxyCommand** esegue un programma e parla SSH attraverso il suo input e output, come OpenSSH. `%h`, `%p` e `%r` vengono sostituiti con host, porta e utente. Sostituisce il proxy.

Preset:

- **AWS SSM**: `aws ssm start-session --target %h --document-name AWS-StartSSHSession --parameters portNumber=%p` (l'host è l'ID dell'istanza).
- **Google Cloud IAP**: `gcloud compute start-iap-tunnel %h %p --listen-on-stdin`.
- **Cloudflare Access**: `cloudflared access ssh --hostname %h`.
- **Teleport**: `tsh proxy ssh %r@%h:%p`.

Gli strumenti devono essere installati e autenticati su questo computer.

## Comando prima di connettersi {#preconnect}

**Comando prima di connettersi** (in **Avanzate**) gira su questo computer prima di ogni connessione, con `$SHELLVANE_HOST`, `$SHELLVANE_PORT`, `$SHELLVANE_USER`, `$SHELLVANE_HOST_NAME` e, quando l'host ha una **Chiave SSH**, la sua chiave pubblica in `$SHELLVANE_PUBLIC_KEY_FILE`. Se fallisce, Shellvane non si connette e mostra la sua ultima riga di errore. Preimpostazioni:

- **AWS EC2 Instance Connect** invia la chiave all'istanza per 60 secondi (`aws ec2-instance-connect send-ssh-public-key`). Usa l'ID dell'istanza come nome host, di solito insieme al ProxyCommand **AWS SSM**, e l'utente dell'istanza (`ec2-user`, `ubuntu`…).
- **Google Cloud OS Login** aggiunge la chiave al tuo profilo OS Login per un'ora.

È utile anche per avviare una VPN o accendere un bastion prima di connettersi.

## Importazione {#import}

L'importazione di una configurazione OpenSSH mantiene `ProxyJump` (collegato agli host importati, o creati) e `ProxyCommand`.
