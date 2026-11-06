# Privacy e sicurezza {#top}

## Cosa resta su questo computer {#local}

Host, impostazioni, host conosciuti, regole e log di audit sono in un database locale; i segreti sono nella [cassaforte](shellvane:topic/vault) cifrata. Shellvane non invia nulla da nessuna parte, a parte le connessioni che apri tu, il controllo degli aggiornamenti (una volta al giorno se non lo disattivi: scarica da termoraplus.com il manifest firmato indicando solo la versione di Shellvane e il sistema, ad esempio `Shellvane/1.2.0 (darwin/arm64)`, e nulla su di te; vedi [Aggiornamenti](shellvane:topic/integration#updates)) e, se la configuri, i file cifrati inviati alla cartella, al server WebDAV, al repository Git o al bucket S3 di [sincronizzazione](shellvane:topic/sync). Telnet non è cifrato: usalo solo su reti di cui ti fidi. Un gruppo esportato non contiene mai chiavi, password o note.

## Conferme {#confirm}

Shellvane chiede prima di: fidarsi di una nuova chiave di host (e rifiuta una chiave cambiata), ogni firma con un agent inoltrato, snippet pericolosi in broadcast, eliminare host, gruppi, chiavi e regole.

## Limiti dell'automazione {#automation}

Le risposte automatiche si attivano esplicitamente per gruppo o host, hanno un limite di frequenza, sono disattivate in broadcast e registrate. Vedi [Automazione](shellvane:topic/automation#respond).

## Comandi che configuri {#commands}

I comandi per password, certificati, ProxyCommand e prima di connettersi girano su questo computer con i tuoi permessi; usa solo comandi di cui ti fidi.

## Diagnostica e support bundle {#diagnostics}

Quando una connessione fallisce, **Dettagli connessione** sul pannello mostra ogni passo dell'handshake: l'indirizzo raggiunto, la versione del server, gli algoritmi negoziati (scambio di chiavi, chiave host, cifratura, MAC), l'impronta della chiave host, i metodi di autenticazione ammessi dal server e quelli falliti, e l'errore esatto. **Impostazioni → Diagnostica** elenca gli ultimi 50 tentativi dall'avvio di Shellvane.

**Copia report** copia un tentativo come testo per un collega o una segnalazione, con nomi degli host, utenti e indirizzi sostituiti da pseudonimi (`host-01`, `user-01`, `10.255.0.x`).

Il **support bundle** è un file ZIP con versione e sistema, le tue impostazioni, la struttura di gruppi e host e i tentativi di connessione recenti. Prima di salvare, **Anteprima** mostra ogni file esattamente come verrà scritto. Gli stessi pseudonimi valgono in tutti i file, così restano correlabili; la tabella delle corrispondenze non viene mai salvata. Vault, chiavi private, password, seed TOTP, note e comandi (password, certificato, proxy, prima di connettersi e comandi di avvio) non vengono mai inclusi: le impostazioni che potrebbero contenerli riportano solo "(set)". Il bundle viene salvato nella cartella dell'applicazione e non viene inviato da nessuna parte.
