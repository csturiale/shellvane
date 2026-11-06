# Connessione {#top}

## Apri una sessione {#open}

- Doppio clic su un host, Invio su di esso, oppure la palette dei comandi ([[K]], poi `@` per cercare solo gli host).
- **Connessione rapida**: scrivi `utente@host:porta` nella barra in alto. Porta e utente sono facoltativi.
- Più sessioni verso lo stesso server condividono una connessione: dividere un pannello apre un nuovo canale invece di rifare il login.

## Chiavi degli host {#hostkey}

Alla prima connessione Shellvane mostra l'impronta della chiave del server. Confrontala con quella che ti dà l'amministratore (`ssh-keygen -lf /etc/ssh/ssh_host_ed25519_key.pub` sul server):

- **Fidati e ricorda** la salva negli host conosciuti di Shellvane.
- **Fidati una volta** si connette senza salvarla.
- **Rifiuta** interrompe.

Se un server conosciuto presenta una chiave **diversa**, la connessione viene rifiutata: può essere una reinstallazione, o qualcuno nel mezzo. Rimuovi la vecchia chiave solo quando sai perché è cambiata.

## Stati {#states}

Un pannello è *in connessione*, *connesso*, *chiuso* (la shell è terminata, con il suo codice di uscita) o *fallito* (con il motivo). I pannelli chiusi o falliti offrono **Riconnetti**; le connessioni cadute si riconnettono da sole, vedi [Riconnessione](shellvane:topic/continuity). I terminali continuano a girare mentre sei su un'altra pagina (impostazioni, file, guida): terminano solo quando finisce la loro shell o connessione, o quando li chiudi.

## Terminale locale {#local}

**Nuovo terminale locale** ([[T]], o l'icona del terminale nella barra delle tab) apre una shell di questo computer: la tua shell di login, o quella scelta in Impostazioni › Terminale locale (bash, zsh, fish, PowerShell, cmd, WSL, Git Bash). I pannelli locali funzionano come quelli SSH: split, trascinamento, broadcast, regole, workspace.

[Nuovo terminale locale](shellvane:action/newLocal)

## Telnet e console seriali {#telnet-serial}

Per apparati di rete e schede senza SSH, scrivi in **Connessione rapida** (o nella palette dei comandi):

- `telnet://host` o `telnet://host:2323`: una sessione Telnet, che invia il tipo di terminale e la dimensione della finestra. Telnet non è cifrato: usalo solo su reti di cui ti fidi.
- `serial://COM3?baud=115200` su Windows, `serial:///dev/ttyUSB0?baud=115200` su macOS e Linux: una console seriale. Le impostazioni facoltative sono `data` (5–8, predefinito 8), `parity` (`none`, `odd`, `even`, `mark`, `space`) e `stop` (`1`, `1.5`, `2`); il predefinito è 9600 8N1. Le porte seriali di questo computer vengono proposte nell'elenco della connessione rapida e nella palette a 115200. Su Linux il tuo utente deve poter accedere alla porta (di solito il gruppo `dialout`).

Il pannello mostra un'etichetta **Telnet** o **Seriale** con l'indirizzo o la porta e le sue impostazioni. Questi pannelli vengono salvati nei workspace come gli altri.

## Cartella, ambiente e comando {#session}

Nelle opzioni **Avanzate** di un host o di un gruppo:

- **Cartella iniziale**: la shell parte da lì (`~/progetti` è nella tua home);
- **Variabili d'ambiente**: righe `NOME=valore` esportate prima che parta la shell. Le imposta Shellvane stesso, quindi funzionano anche quando l'`AcceptEnv` del server rifiuterebbe il `SetEnv` di OpenSSH;
- **Comando remoto**: viene eseguito al posto della shell interattiva, come il `RemoteCommand` di OpenSSH (es. `htop`, `sudo -i`, `docker exec -it app bash`); il pannello termina quando esce. Con tmux o screen viene eseguito dentro la sessione persistente.

I gruppi le trasmettono ai loro host come le altre impostazioni. L'import di `~/.ssh/config` porta con sé `SetEnv` e `RemoteCommand`.

## Raggiungibilità, Wake-on-LAN e port knocking {#wake}

Un pallino sull'icona di un host indica se la sua porta SSH risponde (verde, con la latenza nel suggerimento) o no (rosso). Gli host raggiunti tramite jump host o proxy non vengono controllati. Il controllo avviene ogni minuto mentre il pannello host è aperto e si può disattivare in **Impostazioni**.

Con un **indirizzo MAC** nelle opzioni **Avanzate**, il pulsante di accensione dell'host invia un pacchetto Wake-on-LAN (a `255.255.255.255:9` o all'indirizzo di broadcast impostato). **Accendi prima di connetterti** lo invia a ogni connessione e attende fino a due minuti che l'host risponda.

Il **port knocking** tocca una sequenza di porte prima di connettersi, es. `7000,8000/udp,9000`, per i server il cui firewall apre SSH solo dopo il "bussare" giusto.

## Mosh, desktop remoto e VNC {#launchers}

Se `mosh` è installato, il pulsante Wi-Fi di un host lo apre con Mosh in un pannello: la sessione sopravvive a sospensione, cambi di rete e connessioni instabili. L'accesso SSH di Mosh usa porta, utente, jump host e ProxyCommand dell'host, e l'agent di Shellvane quando è attivo; sul server serve `mosh-server`.

**Desktop remoto** nelle opzioni **Avanzate** di un host aggiunge un pulsante schermo che apre il desktop dell'host con il client di sistema: RDP (Desktop remoto su Windows e macOS, FreeRDP o Remmina su Linux) o VNC (un visualizzatore VNC, o quello integrato di macOS). **Tramite SSH** apre prima un inoltro temporaneo alla porta sull'host, così il desktop non richiede porte aperte ed è cifrato da SSH.

## Inoltro X11 {#x11}

**Inoltro X11** nelle opzioni **Avanzate** di un host mostra su questo computer i programmi grafici avviati sul server (`xclock`, `gitk`, `virt-manager`…), come `ssh -X`. Serve un server X locale: il desktop su Linux, XQuartz su macOS, VcXsrv o X410 su Windows. Il server deve avere `X11Forwarding yes` e `xauth`.

Il server riceve un cookie casuale; ogni connessione X che apre deve presentarlo, e Shellvane lo sostituisce con il cookie reale del tuo display, che non lascia mai questo computer.
