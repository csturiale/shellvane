# Ambienti {#top}

Un ambiente avvia, in ordine e con un clic, tutto ciò che serve per lavorare su un sistema: un login cloud su questo computer, una sessione SSH a un jump host con i suoi comandi, tunnel, terminali locali il cui kubectl passa da un tunnel, e un layout.

Caso tipico: un cluster GKE raggiungibile solo da un bastion. L'ambiente fa il login con `gcloud`, si connette al bastion, lì esegue `get-credentials`, apre un tunnel SOCKS e apre terminali locali in cui `kubectl` parla con il cluster attraverso il tunnel.

[Apri Ambienti](shellvane:view/environments)

## Passi {#steps}

Ogni passo è facoltativo; i passi senza impostazioni vengono saltati.

1. **Login su questo computer** — un comando di verifica e un comando di login.
2. **Jump host** — una sessione SSH a un host salvato, con comandi aggiuntivi dopo il suo script di avvio e un contesto kubectl per quel pannello.
3. **Tunnel** — tunnel salvati, avviati dopo la connessione; il passo attende che siano attivi.
4. **kubectl locale** — terminali locali con un contesto kubectl, eventualmente attraverso uno dei tunnel.
5. **Layout** — un workspace salvato da aprire.

Il pannello Ambienti mostra ogni passo mentre viene eseguito. Se uno fallisce, l'esecuzione si ferma lì, con il motivo e **Riprova da qui**. **Ferma** chiude i terminali e ferma i tunnel aperti dall'ambiente.

## Login cloud {#login}

Il comando di **verifica** deve riuscire solo quando sei autenticato, ad esempio `gcloud auth print-access-token`, `aws sts get-caller-identity --profile {{profile}}`, `az account show`. Se fallisce, il comando di **login** (`gcloud auth login`, `aws sso login`, `az login`) viene eseguito in un terminale locale per completarlo (browser, codice) e l'ambiente attende che la verifica riesca (fino a 10 minuti). I preset compilano entrambi i campi.

Verifica e preparazione girano nella tua shell (zsh, bash…), leggendo i suoi file di avvio come fa un terminale, quindi gli strumenti messi nel PATH da `~/.zshrc` vengono trovati. Mentre il passo attende, mostra perché la verifica non riesce ancora; **Continua senza verificare** prosegue comunque.

Ogni passo in corso dice cosa sta facendo (verifica del login, comando di preparazione, connessione dei tunnel, apertura dei terminali) e da quanto. I passi senza terminale — la verifica, la preparazione, i tunnel — girano in background, quindi questa riga è il posto dove seguirli.

Se il jump host non ha una password salvata (né una chiave o un accesso di team), la password viene chiesta nella solita finestra appena il tunnel si connette: spunta **Salva nel vault** e gli avvii successivi sono automatici. Il passo attende la risposta fino a cinque minuti.

I login necessari sul jump host vanno nei suoi comandi, ad esempio `gcloud auth login --no-launch-browser`.

## Sessioni ripristinate e altri terminali {#terminals}

Ogni terminale aperto da un ambiente mostra il suo badge nell'intestazione: il nome dell'ambiente, e il suo stato quando non è pronto. Quando Shellvane ripristina la sessione precedente, quei terminali tornano come shell semplici: l'ambiente riparte da solo su di essi — verifica del login (aprendo il login se serve), tunnel, route di kubectl — senza riaprirli. Finché non è pronto il badge lo dice, in rosso.

- **Login scaduto**: la verifica del login viene ripetuta ogni 5 minuti. Quando fallisce (ad esempio `gcloud` chiede di autenticarsi di nuovo) il badge e il pannello Ambienti indicano *login scaduto*: clicca il badge oppure **Rifai il login e riprendi**.
- **Un altro terminale**: clicca il badge di un ambiente pronto, usa **Nuovo terminale nell'ambiente** nel menu del pannello, oppure il pulsante terminale dell'ambiente nel pannello Ambienti. Apre una shell locale con il contesto kubectl e il tunnel dell'ambiente (o il suo host remoto se non ha terminali locali). Aprine quanti ne servono.
- Le variabili con cui è partito l'ambiente sono salvate con la sessione, così il ripristino le riusa.
- **Chiudere l'ultimo terminale** di un ambiente avviato lo ferma: si chiudono i tunnel che ha avviato, ma non quelli già attivi prima, avviati all'apertura o usati da un altro ambiente avviato. Un avviso offre **Annulla** per qualche secondo (login e tunnel di nuovo, senza terminali). Gli ambienti che non aprono terminali restano attivi fino a **Stop**. Si disattiva in **Impostazioni › Sessioni › Ferma un ambiente con il suo ultimo terminale**.

## Variabili {#variables}

Scrivi `{{nome}}` in qualsiasi comando, contesto o namespace. Le variabili vengono chieste all'avvio dell'ambiente, con un valore predefinito o un elenco di scelte, ad esempio `cluster` con scelte `prod-eu, prod-us`.

## kubectl attraverso un tunnel {#kubectl}

Scegli un tunnel in **kubectl locale**:

- un tunnel **dinamico (SOCKS)**: il cluster del contesto riceve `proxy-url: socks5://…`;
- un tunnel **locale** verso l'API server (ad esempio locale `6443` → `10.0.0.2:443`): l'indirizzo del cluster diventa `https://127.0.0.1:6443`, mantenendo il nome TLS originale così il certificato viene ancora verificato.

Come per ogni pannello locale, viene scritto nel file kubectl del pannello: il tuo kubeconfig non cambia. Il contesto deve esistere nel tuo kubeconfig locale (ad esempio dopo `gcloud container clusters get-credentials` su questo computer): mettilo nel campo **Preparazione** del passo 1 e viene eseguito a ogni avvio, dopo il login.

### Esempio: cluster GKE privato dietro un jump host {#gke}

Il solito script (trova il cluster, ne prende le credenziali, `ssh -L porta:endpoint:443 jumphost`, modifica il kubeconfig con `127.0.0.1` e `tls-server-name`) diventa un ambiente:

1. Salva il jump host come host, e su di esso un tunnel: **dinamico (SOCKS)**, oppure **locale** `25901` → `<endpoint del cluster>:443`.
2. Crea un ambiente e clicca **GKE tramite jump host**: aggiunge le variabili `project`, `region` e `cluster`, il login gcloud, la preparazione `get-credentials` e il contesto `gke_{{project}}_{{region}}_{{cluster}}`.
3. Nel passo 3 spunta il tunnel, nel passo 4 sceglilo come tunnel di kubectl. Usa le **scelte** delle variabili per selezionare il cluster all'avvio.

Avviandolo esegue il login se serve, scrive il contesto, apre il tunnel e apre terminali locali in cui `kubectl`, `helm` e `k9s` raggiungono il cluster; indirizzo e nome TLS sono impostati nel file kubectl del singolo pannello, quindi il tuo kubeconfig mantiene l'endpoint reale. Un ambiente per stadio (sit, uat, prd), ognuno con il suo jump host e tunnel.

## Condividi {#share}

Nell'app di shellvane.com, il pulsante team accanto all'ambiente lo condivide: i membri lo ricevono con i suoi tunnel, tenuto aggiornato, e ne approvano i comandi prima che giri sul loro computer.


L'icona di copia mette l'ambiente negli appunti in YAML, con i **nomi** di host, tunnel e workspace al posto degli ID interni e senza segreti. **Importa** lo rilegge e collega i nomi ai tuoi host, tunnel e workspace; si possono incollare più ambienti insieme, separati da righe con `---`. I tunnel importati da una configurazione OpenSSH prendono il nome dall'host e dalle porte (`bastion SOCKS 1080`, `bastion L 8080 → db:5432`), così gli ambienti possono farvi riferimento.
