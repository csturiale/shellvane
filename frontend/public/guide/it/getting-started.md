# Primi passi {#top}

Al primo avvio Shellvane apre un breve tour: lingua, cassaforte, importazione degli host. **Rifai il tour** nella lista qui sotto lo riapre.

Shellvane tiene in un unico posto server, credenziali e terminali. Questa pagina ti porta da un'app vuota alla prima connessione in pochi minuti; ogni passo apre la schermata in cui farlo.

## 1. Crea la cassaforte {#vault}

La cassaforte conserva password, passphrase delle chiavi, chiavi private e segreti TOTP, cifrati con una password principale che non lascia mai il tuo computer. Puoi usare Shellvane anche senza, ma nulla di segreto verrà ricordato.

[Apri le impostazioni della cassaforte](shellvane:view/settings)

## 2. Aggiungi o importa gli host {#hosts}

- **Importa** ciò che hai già: il tuo `~/.ssh/config` di OpenSSH, le sessioni di PuTTY o KiTTY. La sorgente non viene modificata e prima viene fatto un backup.
- Oppure crea gli host a mano, dentro **gruppi** che condividono le impostazioni (utente, porta, chiave, jump host…).

[Importa host](shellvane:action/import) · [Nuovo host](shellvane:action/newHost)

## 3. Connettiti {#connect}

Fai doppio clic su un host, oppure scrivi `utente@host:porta` nella barra di connessione rapida in alto. La prima volta Shellvane mostra l'impronta del server e chiede se fidarsi.

Puoi anche aprire un terminale su questo computer: [Nuovo terminale locale](shellvane:action/newLocal) ([[T]]).

## 4. Usa una chiave invece della password {#key}

Genera una chiave in **Chiavi**, poi usa **Installa su un host** per aggiungerla agli `authorized_keys` del server. Assegnala a un host o a un intero gruppo.

[Apri Chiavi](shellvane:view/keys)

## 5. Vai oltre {#next}

- Dividi i pannelli e spostali: [Terminale](shellvane:topic/terminal)
- Scrivi su più server insieme: [Broadcast](shellvane:topic/broadcast)
- Salva comandi con variabili: [Snippet](shellvane:topic/snippets)
- Inoltra porte: [Tunnel](shellvane:topic/tunnels)
- Copia file, anche da server a server: [File (SFTP)](shellvane:topic/files)
- Non perdere mai una sessione: [Riconnessione e workspace](shellvane:topic/continuity)
- Reagisci all'output, automatizza i login: [Automazione](shellvane:topic/automation)
- Apri sessioni Telnet e console seriali: [Connessione](shellvane:topic/connecting#telnet-serial)
- Condividi un gruppo di host con un collega: [Host](shellvane:topic/hosts#templates)
- Esegui gli stessi passi su molti host: [Runbook](shellvane:topic/runbooks)
- Tieni allineati due computer: [Sincronizzazione](shellvane:topic/sync)

Premi **[[K]]** ovunque per cercare host, snippet e azioni, oppure scrivi `?` per cercare in questa guida.
