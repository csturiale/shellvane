# Contesto Kubernetes per pannello {#top}

Ogni pannello del terminale può avere il proprio contesto e namespace di `kubectl`, indipendente da tutti gli altri — su questo computer e sui server. kubectl, helm, k9s e kubectx lo seguono.

## Come funziona {#how}

Shellvane non esegue mai `kubectl config use-context` sul tuo kubeconfig (cambierebbe tutti i terminali insieme). Invece il `KUBECONFIG` di ogni pannello inizia con un piccolo file tutto suo, seguito dal tuo kubeconfig abituale. Il file del pannello indica solo il contesto corrente (e, con un namespace, una copia di quel contesto con l'altro namespace): le tue credenziali non vengono copiate. Cambiare contesto riscrive il file del pannello, quindi vale dal comando successivo, anche dentro tmux.

## Terminali locali {#local}

Ogni pannello locale ha il proprio file. Clicca il badge con il timone nell'intestazione del pannello, scegli un contesto (letto da `kubectl config get-contexts`) ed eventualmente un namespace, poi **Applica**. Il valore predefinito per i nuovi terminali locali è in Impostazioni › Terminale locale.

Su macOS e Linux Shellvane legge all'avvio il `PATH` della tua shell di login, così `kubectl`, `gcloud`, `aws` e i loro plugin di credenziali installati con Homebrew o in `~/.local/bin` vengono trovati anche quando l'app è aperta dal Dock o dal Finder; all'avvio kubectl lo aspetta, e se i file di avvio della shell sono troppo lenti da leggere vengono aggiunte comunque le cartelle abituali (Homebrew, `~/.local/bin`, `~/bin`, krew, Google Cloud SDK, Docker e Rancher Desktop). Se l'elenco dei contesti fallisce, il messaggio mostra l'errore di kubectl.

## Server {#remote}

Imposta **Contesto e namespace Kubernetes** in Avanzate su un host o gruppo (ad esempio gruppo *Produzione* → `prod-eu`): i suoi pannelli ricevono un proprio file in `~/.kube/shellvane` sul server e il selettore elenca i contesti del server. Un pannello aperto senza contesto ne riceve uno riaprendosi (una sessione tmux viene ripresa).

## Sicurezza in produzione {#prod}

Il contesto è mostrato nell'intestazione del pannello e sulla tab. I contesti che corrispondono al pattern **Contesti di produzione** (Impostazioni › Terminale locale, predefinito `prod`) sono mostrati in rosso. Il pattern si confronta con il nome del contesto kubectl (come lo mostra `kubectl config get-contexts`), non con il nome dell'ambiente sul badge del pannello.

## Limiti {#limits}

- kubectl e i contesti devono esistere dove gira il pannello: Shellvane non copia il tuo kubeconfig locale sui server.
- Un file di avvio della shell che imposta `KUBECONFIG` da sé (ad esempio in `.bashrc`) ha la precedenza.
- Eseguire `kubectl config use-context` in un pannello cambia solo il file di quel pannello.
