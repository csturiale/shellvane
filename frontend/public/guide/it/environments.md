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

### Password del login da un plugin delle credenziali {#login-secret}

Quando la pagina di accesso che il login apre richiede una password conservata in un gestore di password, imposta **Password per il login**. Il campo compare quando in **Plugin** è configurato un plugin delle credenziali (CyberArk, Conjur, HashiCorp Vault…) o quando un login è condiviso in un team. Scegli la configurazione del plugin e l'account **come è registrato nel vault**: il suo **utente** (ad esempio l'account di dominio `a123456`) e, se non è quella del plugin, la sua **Safe**; l'**indirizzo** è facoltativo e serve solo se più account hanno lo stesso utente. Questo account non deve per forza essere quello con cui accedi: metti l'email di accesso nel comando di login, ad esempio `gcloud auth login alice.admin@example.com --update-adc`, e la pagina di accesso si apre su quell'account. In questi campi funzionano le variabili.

Quando la verifica fallisce, Shellvane legge prima la password dell'account dal plugin, accedendo se serve (per CyberArk, con il suo MFA o l'accesso dal browser; la sessione viene poi riusata), la copia negli appunti per 45 secondi (configurabili nelle Impostazioni) e solo dopo esegue il login: incolla la password nella pagina di accesso. La password non viene mai salvata, mostrata né scritta nel terminale, e gli appunti vengono svuotati dopo, a meno che tu non abbia copiato altro. Se la password non si può leggere, il passo dice perché e il login parte comunque, così puoi scriverla tu.

**Apri la pagina di accesso in una finestra privata** (attiva per default con i preset Google Cloud e Azure) fa aprire a `gcloud`, `az` e `aws sso` la loro pagina in una finestra in incognito o InPrivate di Chrome, Edge, Brave o Firefox, tramite la variabile `BROWSER` del terminale del login; con **Automatico** è il tuo browser predefinito, o il primo di questi se il predefinito (Safari) non ha una finestra privata: una sessione Google o Microsoft già aperta nel browser non può così accedere con l'account sbagliato. Safari non ha una finestra privata apribile in questo modo, quindi viene usato un altro browser installato; se non ce ne sono, si apre il browser di sempre.

In **Impostazioni → Login degli ambienti** scegli il browser delle pagine di accesso (automatico, uno dell'elenco, o un **comando personalizzato** con `%s` per il link, ad esempio un browser con un profilo dedicato) e per quanti secondi una password di login resta negli appunti (45 per default). Ogni ambiente può scegliere un altro browser nel suo passo 1.

Non tutti gli strumenti leggono `BROWSER`: molti scritti in Go (kubelogin, `vault login -method=oidc`, `tsh login`) aprono direttamente il browser di sistema, e alcuni stampano solo il link (`gcloud auth login --no-launch-browser`, codici dispositivo). Per questi, attiva **Apri i link di accesso stampati nel terminale del login**: Shellvane riconosce nell'output del terminale i link OAuth, OpenID Connect, SAML e dei codici dispositivo e li apre una volta nel browser scelto (al massimo 5 per terminale). Usa l'opzione dello strumento che stampa il link invece di aprirlo, altrimenti la pagina si apre due volte.

**Usa questo browser in tutti i terminali locali** (Impostazioni) applica la stessa scelta ai terminali locali aperti a mano, con le sue opzioni **finestra privata** e **apri i link di accesso**; il login di un ambiente mantiene la scelta dell'ambiente.

### Account Google diversi nello stesso momento {#gcloud}

gcloud tiene account, progetto e credenziali application default in un'unica cartella di configurazione, quindi per impostazione predefinita tutti i terminali usano lo stesso account Google. Assegna a un ambiente un **profilo gcloud** (es. `prod-admin`): verifica, login, preparazione e terminali locali (con kubectl e il suo plugin GKE) usano una cartella di configurazione propria (`CLOUDSDK_CONFIG`). L'ambiente "prod" può restare collegato come `alice-admin@…` mentre "dev" usa un altro account, entrambi aperti insieme. Gli ambienti con lo stesso profilo condividono il login. Il preset Google Cloud accede con `gcloud auth login --update-adc`, che aggiorna anche le credenziali application default del profilo.

## Sessioni ripristinate e altri terminali {#terminals}

Ogni terminale aperto da un ambiente mostra il suo badge nell'intestazione: il nome dell'ambiente, e il suo stato quando non è pronto. Quando Shellvane ripristina la sessione precedente, quei terminali tornano come shell semplici: l'ambiente riparte da solo su di essi — verifica del login (aprendo il login se serve), tunnel, route di kubectl — senza riaprirli. Finché non è pronto il badge lo dice, in rosso.

- **Login scaduto**: la verifica del login viene ripetuta ogni 5 minuti. Quando fallisce (ad esempio `gcloud` chiede di autenticarsi di nuovo) il badge e il pannello Ambienti indicano *login scaduto*: clicca il badge oppure **Rifai il login e riprendi**.
- **Un altro terminale**: clicca il badge di un ambiente pronto, usa **Nuovo terminale nell'ambiente** nel menu del pannello, oppure il pulsante terminale dell'ambiente nel pannello Ambienti. Apre una shell locale con il contesto kubectl e il tunnel dell'ambiente (o il suo host remoto se non ha terminali locali). Aprine quanti ne servono.
- Le variabili con cui è partito l'ambiente sono salvate con la sessione, così il ripristino le riusa.
- **Chiudere l'ultimo terminale** di un ambiente avviato lo ferma: si chiudono i tunnel che ha avviato, ma non quelli già attivi prima, avviati all'apertura o usati da un altro ambiente avviato. Un avviso offre **Annulla** per qualche secondo (login e tunnel di nuovo, senza terminali). Gli ambienti che non aprono terminali restano attivi fino a **Stop**. Si disattiva in **Impostazioni › Sessioni › Ferma un ambiente con il suo ultimo terminale**.

## Variabili {#variables}

Scrivi `{{nome}}` in qualsiasi comando, contesto o namespace. Le variabili vengono chieste all'avvio dell'ambiente, con un valore predefinito o un elenco di scelte, ad esempio `cluster` con scelte `prod-eu, prod-us`.


### Valori personali {#personal}

Spunta **personale** su una variabile il cui valore appartiene a ciascuna persona, come l'email di accesso (`gcloud auth login {{googleEmail}} --update-adc`) o l'utente e la Safe nel vault (anche i campi di **Password per il login** accettano variabili). Un valore personale viene chiesto la prima volta, poi ricordato su questo computer e non più richiesto; **I miei valori** (l'icona della persona sull'ambiente) lo modifica. Non è mai salvato nell'ambiente: esportazioni e condivisioni nel team contengono solo il nome della variabile, così tutti nel team usano lo stesso ambiente con i propri valori e ne ricevono gli aggiornamenti senza una copia locale da tenere allineata.

Accanto a ogni valore personale scegli dove ricordarlo:

- **Ricorda ovunque** (predefinito): il valore è tenuto per nome, e lo riceve ogni ambiente, [snippet](shellvane:topic/snippets#variables) e [procedura](shellvane:topic/procedures#placeholders) che usa lo stesso nome. Scrivi `googleEmail` una volta e tutti i tuoi ambienti GKE lo hanno.
- **Solo qui**: questo ambiente tiene un valore suo, che vince su quello usato ovunque. Serve per un secondo account, ad esempio `nome@cliente.com` per i cluster di un cliente mentre gli altri tengono `nome@azienda.com`. Scegliere di nuovo **Ricorda ovunque** lo elimina.

**Impostazioni › Valori personali** elenca i valori tenuti ovunque: lì li modifichi, aggiungi o rimuovi. Eliminare un ambiente cancella solo i suoi valori.

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
