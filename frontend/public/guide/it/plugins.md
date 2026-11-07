# Plugin {#top}

I plugin portano dentro gli host dai sistemi che già li conoscono: cloud, Kubernetes, gestione della configurazione, virtualizzazione e strumenti di inventario. Si configurano una volta; ogni **Aggiorna host** allinea un gruppo di host con la sorgente. Altri plugin aggiungono [pannelli](shellvane:topic/plugins#panels) agli host connessi.

[Apri Plugin](shellvane:view/plugins)

## Installare {#install}

Niente gira finché non lo installi. La vista Plugin elenca:

- i plugin **inclusi in Shellvane**: **Installa** ne abilita uno, **Disinstalla** lo disabilita di nuovo (prima elimina le sue configurazioni);
- lo **store**: plugin pubblicati a un indirizzo (per default `https://shellvane.com/plugins/index.json`; si può impostare qualsiasi altro store). **Installa** scarica il pacchetto, ne verifica lo SHA-256 con quello dello store e lo installa; **Aggiorna** compare quando lo store ha una versione più recente;
- **Installa da file**: una cartella o un `.zip` di plugin con un `plugin.json`, ad esempio uno scritto da te.

I plugin esterni girano su questo computer con i tuoi permessi, come i tuoi script: installa solo quelli di cui ti fidi.

## Plugin disponibili {#list}

| Plugin | Legge | Serve |
| --- | --- | --- |
| AWS EC2 | istanze in esecuzione, raggruppate per regione; i tag diventano tag | CLI `aws` (profili e SSO) |
| Google Cloud Compute | istanze in esecuzione di un progetto, per zona; le label diventano tag | `gcloud` |
| Azure VMs | VM in esecuzione, per resource group | `az` |
| Kubernetes nodes | i nodi di un contesto, con i loro ruoli | `kubectl` |
| Ansible inventory | inventari statici o dinamici, mantenendo l'albero dei gruppi, `ansible_host`, `ansible_user`, `ansible_port` | `ansible-inventory` |
| Terraform state | macchine di AWS, Google, Azure, DigitalOcean, Hetzner, Linode, Vultr | `terraform` |
| Proxmox VE | VM e container in esecuzione, per nodo, indirizzi dal guest agent | un token API |
| NetBox | dispositivi e VM con un IP primario, per sito o cluster, con un filtro opzionale | un token API |

Se manca lo strumento richiesto il plugin lo segnala; gli strumenti installati con Homebrew o in `~/.local/bin` vengono trovati anche su macOS (Shellvane legge il `PATH` della shell di login).

## Configura e aggiorna {#setup}

1. Clicca **+** e scegli un plugin. Dagli un nome: lo stesso plugin si può aggiungere più volte, ad esempio per due profili AWS.
2. Compila le impostazioni. **Indirizzo** sceglie l'indirizzo pubblico o privato; **Utente SSH** viene impostato su ogni host.
3. **Aggiorna host** salva e riempie il gruppo con il nome del plugin, con sottogruppi per regione, zona, nodo o sito.

Un aggiornamento allinea nome, indirizzo, utente e porta dalla sorgente, aggiunge le nuove macchine e rimuove quelle sparite. Ciò che cambi sugli host resta: preferiti, chiavi, colori, jump host e ogni altra impostazione. Eliminando il plugin i suoi host restano come host normali.

## Segreti {#secrets}

I token API sono conservati nel [vault](shellvane:topic/vault), mai nelle impostazioni: il vault deve essere sbloccato per salvarli o usarli. Lascia vuoto un campo segreto per mantenere il valore salvato. Per un server Proxmox con certificato autofirmato incolla la sua **impronta** SHA-256: viene accettato solo quel certificato.

## Docker, servizi e log {#panels}

Una volta installati, su un host connesso il pulsante a puzzle nell'intestazione del pannello (o **Docker, servizi e log** nel menu del pannello) apre pannelli che usano la stessa connessione SSH, senza installare nulla sul server:

- **Docker**: i container, prima quelli in esecuzione, con **Log** e **Shell** (aperti in un nuovo pannello), **Avvia**, **Ferma** e **Riavvia** (chiedendo conferma).
- **systemd**: servizi falliti e in esecuzione, con **Stato** e **Journal** in un nuovo pannello, e **Avvia**, **Ferma**, **Riavvia** digitati lì con `sudo`, che può così chiedere la password.
- **Log**: gli errori recenti dal journal (o da `/var/log/syslog`, `/var/log/messages`) e **Segui il log** in un nuovo pannello.

Il filtro restringe le righe. I nomi di container e unità vengono controllati prima di finire in un comando.

## Credenziali da un broker {#credentials}

I plugin di credenziali forniscono l'accesso a ogni connessione, così non si conservano password o chiavi a lunga durata. Installane e configurane uno, poi sceglilo nelle opzioni avanzate dell'host o del gruppo in **Credenziali da un plugin o da un team** (i gruppi lo passano ai loro host).

- **HashiCorp Vault**: con un **ruolo SSH**, Shellvane crea una nuova chiave a ogni connessione e il motore SSH di Vault la firma (il certificato vale per l'utente dell'host); con un **percorso KV** (es. `secret/data/ssh/{{host}}`) legge `password` (e `username`) dal segreto. Il token è conservato nel vault di Shellvane; sono supportati i namespace Enterprise e un certificato fissato.
- **Teleport**: dopo `tsh login`, Shellvane usa certificato e chiave del profilo e si connette attraverso `tsh proxy ssh`, sulla porta dei nodi (di solito 3022). Se non hai fatto il login, l'errore indica quale `tsh login` eseguire.
- **Boundary**: Shellvane autorizza una sessione sul target, usa le credenziali fornite da Boundary (utente e password, o una chiave) e si connette attraverso `boundary connect` (serve `nc`).

Se il token del plugin è in un vault bloccato, Shellvane apre prima il vault (portachiavi, Touch ID o password principale). Anche i plugin esterni possono fornire accessi, con `credentials.get`.

## Scrivere un plugin {#write}

Un plugin è una cartella con un `plugin.json` e un programma in qualsiasi linguaggio:

```json
{
  "id": "disk-usage", "name": "Disk usage", "version": "1.0.0",
  "description": "Filesystem dell'host.",
  "protocol": 1,
  "command": ["python3", "main.py"],
  "capabilities": ["panel", "actions"],
  "settings": [{ "key": "token", "label": "Token API", "secret": true, "required": true }]
}
```

Shellvane avvia `command` nella cartella del plugin a ogni chiamata e scrive una richiesta JSON-RPC 2.0 sul suo standard input; il plugin scrive la risposta sullo standard output, un messaggio JSON per riga, ed esce. Le impostazioni `secret` sono conservate nel vault e passate in `params.settings`.

| Metodo | Parametri | Risultato |
| --- | --- | --- |
| `inventory.list` | `settings` | `{"hosts": [{"key", "name", "hostname", "port", "user", "group": [], "tags": [], "notes"}]}` |
| `panel.render` | `settings`, `target` | `{"title", "tables": [{"title", "columns", "rows", "actions"}], "text"}` |
| `actions.list` | `settings`, `target` | `{"actions": [{"id", "label", "confirm"}]}` |
| `actions.run` | `settings`, `target`, `action`, `item` | `{"output"}`, oppure `{"command"}` da digitare in un nuovo terminale sull'host |
| `credentials.get` | `settings`, `target` | `{"password"}`, oppure `{"privateKey", "certificate"}` |

Mentre lavora, un pannello o un'azione può chiedere a Shellvane di eseguire un comando sull'host attraverso la connessione esistente: scrive `{"jsonrpc":"2.0","id":"x","method":"host.exec","params":{"command":"df -h"}}` e legge `{"id":"x","result":{"output":"…","code":0}}`. I plugin non ottengono mai un accesso SSH proprio. Una chiamata che dura più di due minuti viene fermata; una risposta di errore (`{"error":{"code":1,"message":"…"}}`) viene mostrata all'utente.

Il repository contiene un esempio completo in `examples/plugins/disk-usage`, e `scripts/plugin-index.sh` impacchetta i plugin e scrive un indice di store (`{"plugins":[{"id","name","version","description","url","sha256",…}]}`) da pubblicare su qualsiasi server HTTPS.
