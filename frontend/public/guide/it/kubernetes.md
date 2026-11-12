# Contesto Kubernetes per pannello {#top}

Ogni pannello del terminale può avere il proprio contesto e namespace di `kubectl`, indipendente da tutti gli altri — su questo computer e sui server. kubectl, helm, k9s e kubectx lo seguono.

## Come funziona {#how}

Shellvane non esegue mai `kubectl config use-context` sul tuo kubeconfig (cambierebbe tutti i terminali insieme). Invece il `KUBECONFIG` di ogni pannello inizia con un piccolo file tutto suo, seguito dal tuo kubeconfig abituale. Il file del pannello indica solo il contesto corrente (e, con un namespace, una copia di quel contesto con l'altro namespace): le tue credenziali non vengono copiate. Cambiare contesto riscrive il file del pannello, quindi vale dal comando successivo, anche dentro tmux.

## Terminali locali {#local}

Ogni pannello locale ha il proprio file. Clicca il badge con il timone nell'intestazione del pannello, scegli un contesto (letto da `kubectl config get-contexts`) ed eventualmente un namespace, poi **Applica**. Il valore predefinito per i nuovi terminali locali è in Impostazioni › Terminale locale.

Su macOS e Linux Shellvane legge all'avvio il `PATH` della tua shell di login, così `kubectl`, `gcloud`, `aws` e i loro plugin di credenziali installati con Homebrew o in `~/.local/bin` vengono trovati anche quando l'app è aperta dal Dock o dal Finder; all'avvio kubectl lo aspetta, e se i file di avvio della shell sono troppo lenti da leggere vengono aggiunte comunque le cartelle abituali (Homebrew, `~/.local/bin`, `~/bin`, krew, Google Cloud SDK, Docker e Rancher Desktop). Se l'elenco dei contesti fallisce, il messaggio mostra l'errore di kubectl.

## Server {#remote}

In un pannello SSH kubectl gira **sul server**, con il kubeconfig del server (il suo `~/.kube/config`): il selettore elenca i contesti del server, non quelli di questo computer. Su un server senza kubectl il badge lo dice una volta, poi sparisce.

Imposta **Contesto e namespace Kubernetes** in Avanzate su un host o gruppo (ad esempio gruppo *Produzione* → `prod-eu`): i suoi pannelli ricevono un proprio file in `~/.kube/shellvane` sul server e il selettore elenca i contesti del server. Un pannello aperto senza contesto ne riceve uno riaprendosi (una sessione tmux viene ripresa).

## Ambienti con tunnel {#tunnel}

Quando un cluster si raggiunge solo passando da un server, un [ambiente](shellvane:topic/environments) apre terminali **locali** con il contesto indicato e fa passare kubectl dal suo tunnel: un tunnel SOCKS diventa il `proxy-url` del contesto, un port forward verso l'API diventa il suo `server`, nel file del pannello. Il tuo kubeconfig e le sue credenziali (es. `gke-gcloud-auth-plugin`) restano su questo computer e non vengono modificati; quando l'ambiente riparte dopo un riavvio, i suoi pannelli riprendono il contesto e la porta attuale del tunnel.

## Sicurezza in produzione {#prod}

Il contesto è mostrato nell'intestazione del pannello e sulla tab. I contesti che corrispondono al pattern **Contesti di produzione** (Impostazioni › Terminale locale, predefinito `prod|prd`) sono mostrati in rosso. Il pattern si confronta con il nome del contesto kubectl (come lo mostra `kubectl config get-contexts`), non con il nome dell'ambiente sul badge del pannello.

**Conferma i comandi rischiosi in produzione** (attivo per default): quando premi Invio su `kubectl delete`, `drain`, `cordon`, `taint`, `replace`, `scale --replicas=0`, `rollout restart|undo`, `apply --prune`, `edit`, `patch`, `set`, `label`, `annotate`, o su `helm upgrade|install|uninstall|rollback`, e il contesto del pannello o quello indicato dal comando (`--context`, `--kube-context`) è di produzione, Shellvane chiede conferma prima che il comando arrivi alla shell. Un contesto scelto dentro la shell con `kubectl config use-context` non viene visto: usa il selettore di contesto del pannello o `--context`.

## Port-forward {#forward}

In un terminale locale con un contesto (il terminale kubectl di un ambiente, o uno in cui hai scelto un contesto), il menu del terminale ha **Port-forward verso un service o un pod…**. Scegli un service o un pod attivo del namespace e la sua porta; la porta locale è la stessa se è alta (8080, 3000…), altrimenti una libera, oppure quella che scrivi. Il forward raggiunge il cluster come il terminale (tunnel e profilo gcloud dell'ambiente), quindi funziona anche con cluster privati dietro un jump host.

I forward sono elencati in **Tunnel › Port-forward Kubernetes**, con l'indirizzo locale, un pulsante per aprire nel browser le porte web (https per 443 e 8443) e **Ferma**. Quando il forward di kubectl termina (il pod è stato sostituito, la connessione è caduta) riparte da solo, dopo 1, 2, 4… fino a 30 secondi; un forward verso un service raggiunge così il nuovo pod. I forward durano finché l'app è aperta.

## Limiti {#limits}

- kubectl e i contesti devono esistere dove gira il pannello: Shellvane non copia il tuo kubeconfig locale sui server.
- Un file di avvio della shell che imposta `KUBECONFIG` da sé (ad esempio in `.bashrc`) ha la precedenza.
- Eseguire `kubectl config use-context` in un pannello cambia solo il file di quel pannello.
