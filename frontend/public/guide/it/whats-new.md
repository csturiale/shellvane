# Novità {#top}

## Questa versione {#current}

**Connessioni**
- Ogni chiave viene provata a turno (quella dell'host, quelle dell'agent e di `~/.ssh`); per host o gruppo si scelgono i metodi di autenticazione e il loro ordine, o si propone solo la chiave dell'host: [Autenticazione](shellvane:topic/auth)
- **Telnet** (`telnet://host`) e **console seriali** (`serial://COM3?baud=115200`) dalla connessione rapida e dalla palette: [Connessione](shellvane:topic/connecting)
- **Mosh**, **desktop remoto (RDP)** e **VNC** con i client di sistema, **inoltro X11**, raggiungibilità degli host, **Wake-on-LAN** e **port knocking**: [Connessione](shellvane:topic/connecting)
- Cartella iniziale, variabili d'ambiente e comando remoto per host: [Connessione](shellvane:topic/connecting)
- Un **comando prima di connettersi**, con preimpostazioni per **AWS EC2 Instance Connect** e **Google Cloud OS Login**: [Jump host e proxy](shellvane:topic/jump-proxy#preconnect)
- Diagnostica delle connessioni con l'handshake degli ultimi tentativi, e support bundle anonimizzato: [Privacy e sicurezza](shellvane:topic/security)

**Host**
- **Esporta un gruppo**, con o senza i suoi host, e importalo su un altro computer: [Host](shellvane:topic/hosts)
- Tag, preferiti, host recenti, duplica, connetti a un intero gruppo, note in Markdown: [Host](shellvane:topic/hosts)
- Import da MobaXterm, SecureCRT, WinSCP, FileZilla, Tabby, mRemoteNG e Termius, oltre a OpenSSH, PuTTY e KiTTY: [Import](shellvane:topic/import)
- **Sincronizzazione cifrata end-to-end** tra computer tramite una cartella condivisa, un server WebDAV, un repository Git o un bucket S3: [Sincronizzazione](shellvane:topic/sync)

**Terminale**
- **File attraverso il terminale** con ZMODEM: `sz` salva in Download, `rz` apre il selettore di file: [Terminale](shellvane:topic/terminal#zmodem)
- **Sposta una tab in una finestra a sé**, con le sessioni ancora connesse: [Terminale](shellvane:topic/terminal#detach)
- Il pannello laterale si può **nascondere e ridimensionare** per avere più spazio per i terminali: [Terminale](shellvane:topic/terminal#panel)
- Un **+** dopo le tab apre una nuova tab su una shell locale, un host (con ricerca) o un indirizzo scritto a mano: [Terminale](shellvane:topic/terminal#splits)
- **Menu contestuali** su tab, pannelli, host e gruppi (chiudi le altre, a sinistra, a destra, tutte…), e le tab staccate possono **tornare** nella finestra principale: [Terminale](shellvane:topic/terminal#menus)
- Un'impostazione per la **dimensione dell'interfaccia** di tutta l'app, separata dal carattere del terminale; le scorciatoie compaiono come ⌘ su macOS: [Terminale](shellvane:topic/terminal#appearance)
- **Schemi di colori** personalizzati con editor e import (iTerm2, Windows Terminal, Base16, Alacritty, kitty, Ghostty), colori dell'app abbinati: [Terminale](shellvane:topic/terminal)
- **Notifiche** desktop dai programmi (OSC 9/777) e alla fine dei comandi lunghi, **immagini inline** (Sixel), il **carico del server** sul suo pannello: [Terminale](shellvane:topic/terminal)
- **Log** e **registrazioni** delle sessioni con lettore, ricerca, massimizza, zoom per pannello: [Terminale](shellvane:topic/terminal)
- **Barra di broadcast** e selezione dei pannelli per gruppo o tag: [Broadcast](shellvane:topic/broadcast)
- **Snippet** importati da script di shell o elenchi di comandi, ed esportati in un file: [Snippet](shellvane:topic/snippets)
- **Runbook**: passi su molti host con condizioni, conferme e griglia dei risultati: [Runbook](shellvane:topic/runbooks)

**Team**
- Un **vault di team**: accessi e note condivisi cifrati end-to-end tramite una cartella condivisa, WebDAV, Git o S3, usati direttamente dagli host, e **gruppi di host condivisi** sempre aggiornati sui computer di tutti i membri (nell'app di termoraplus.com)

**Plugin**
- **Host da AWS, Google Cloud, Azure, Kubernetes, Ansible, Terraform, Proxmox e NetBox**, aggiornati in un gruppo, token nel vault; pannelli **Docker, systemd e log** sugli host connessi; installa solo ciò che serve, dallo **store** o da file, o scrivine uno tuo; accessi da **HashiCorp Vault, Teleport e Boundary**: [Plugin](shellvane:topic/plugins)

**File e tunnel**
- **Anteprima** dei file, **confronto e sincronizzazione di cartelle**, ripresa dei trasferimenti, modifica su questo computer: [File](shellvane:topic/files)
- Porte in ascolto sul server, inoltrate in un clic, e tunnel verso **socket Unix**: [Tunnel](shellvane:topic/tunnels)

**Chiavi e cassaforte**
- Chiavi su **token hardware** (YubiKey, smart card, HSM) tramite PKCS#11, con il PIN chiesto all'accesso: [Chiavi SSH](shellvane:topic/keys#token)
- Dove è usata ogni chiave, export (OpenSSH, PuTTY), **rotazione** su tutti gli host, **agent SSH di Shellvane** per gli altri programmi: [Chiavi SSH](shellvane:topic/keys)
- **Chiave di recupero**, **blocco automatico** e **sblocco con il portachiavi di sistema** della cassaforte, confermato con **Touch ID** o **Windows Hello**: [Cassaforte](shellvane:topic/vault)

**Primi passi**
- Un **tour iniziale** configura la cassaforte e importa i tuoi host: [Primi passi](shellvane:topic/getting-started)

**Integrazione**
- `shellvane-cli`, link `ssh://` e `sftp://`, **modalità portabile**, icona nella **tray**: [Integrazione](shellvane:topic/integration)
- **Aggiornamenti firmati** installati sul posto, anche su macOS; installer per Windows, macOS e Linux (.deb, .rpm, AppImage): [Integrazione](shellvane:topic/integration)
