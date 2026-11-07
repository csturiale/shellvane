# Riga di comando, link, tray e uso portable {#top}

## Riga di comando {#cli}

`shellvane-cli` lavora sugli stessi dati dell'app:

- `shellvane-cli list` stampa gli host salvati con gruppo e indirizzo;
- `shellvane-cli connect <host>` apre un host salvato (per nome), o `utente@host:porta`, in Shellvane in esecuzione;
- `shellvane-cli connect --ssh <host>` esegue l'`ssh` di sistema per un host salvato nel terminale corrente, con porta, utente, jump host e ProxyCommand, e l'agent di Shellvane quando è attivo;
- `shellvane-cli open ssh://… | sftp://…` apre un link in Shellvane in esecuzione;
- `shellvane-cli tunnel list`, `tunnel up <nome>`, `tunnel down <nome>` gestiscono i tunnel salvati;
- `shellvane-cli import ssh-config [file]` importa `~/.ssh/config`.

La CLI comunica con l'app tramite un socket locale che solo il tuo utente può aprire.

## Link ssh:// e sftp:// {#links}

Shellvane gestisce i link `ssh://utente@host:porta` e `sftp://utente@host:porta/percorso` dal browser, dalla documentazione o da altre app. Un link che corrisponde a un host salvato (stesso indirizzo, porta e utente) apre quell'host, con le sue credenziali, jump host e opzioni; altrimenti apre una connessione rapida. I link `sftp://` aprono la cartella in File. Un link cliccato mentre Shellvane è aperto va alla finestra già aperta.

## Modalità portable {#portable}

Metti un file vuoto chiamato `shellvane.portable` accanto all'eseguibile di Shellvane (su macOS, accanto a `Shellvane.app`): Shellvane tiene allora tutti i suoi dati in una cartella `ShellvaneData` lì accanto, per esempio su una chiavetta USB. La variabile d'ambiente `SHELLVANE_DATA_DIR` sceglie esplicitamente la cartella dei dati.

## Tray {#tray}

L'icona nella tray elenca gli host recenti (clicca per aprirne uno), i tunnel salvati con un segno di spunta su quelli attivi (clicca per avviarli o fermarli senza aprire una finestra), **Blocca il vault** quando è sbloccato, ed **Esci**.

## Aggiornamenti {#updates}

**Impostazioni → Aggiornamenti** controlla se ci sono nuove versioni (e, se non lo disattivi, lo fa a ogni avvio e ogni 6 ore mentre l'app resta aperta). Quando ne trova una compare un avviso nell'angolo: **Installa** la scarica e la installa, poi propone **Riavvia ora** o **Più tardi** (la nuova versione parte la prossima volta che apri l'app); **Ricordamelo domani** lo nasconde per un giorno, **Salta questa versione** fino alla successiva e **Non mostrare questi avvisi** lo disattiva (Impostazioni → Aggiornamenti lo riattiva; la barra di stato mostra comunque la nuova versione). Vengono proposte solo le release definitive, non le pre-release (`v1.2.0-rc1`); se non ne è ancora pubblicata nessuna, la pagina dice che non c'è nulla da aggiornare. Gli aggiornamenti arrivano dalle release del repository GitHub pubblico del progetto (github.com/csturiale/shellvane), dove ogni release pubblica un manifest firmato con la chiave del progetto; Shellvane installa un aggiornamento solo se quella firma e il checksum del pacchetto sono validi, e conserva la versione precedente fino al successivo avvio. Il controllo comunica a GitHub solo la versione di Shellvane e il sistema (ad esempio `Shellvane/1.2.0 (windows/amd64)`), che è il modo in cui il progetto conta quante copie di ogni versione sono in uso; non c'è nessun identificativo e nulla su di te, sui tuoi host o su come usi l'app. Su macOS viene sostituita l'intera app con quella nuova. Se macOS esegue Shellvane da dove è stato scaricato (usa allora una copia temporanea in sola lettura) o da una cartella in cui non puoi scrivere, sposta prima Shellvane in Applicazioni; fino ad allora usa la pagina della release.

**macOS dice che Shellvane non può essere aperto** ("Apple non può verificare…", *Sposta nel Cestino*): le release non ancora firmate con un Developer ID Apple vengono bloccate alla prima apertura. Apri **Impostazioni di Sistema › Privacy e sicurezza**, scorri fino a Sicurezza e scegli **Apri comunque** per Shellvane (una volta sola), oppure esegui `xattr -dr com.apple.quarantine /Applications/Shellvane.app` nel Terminale.

**Homebrew** (macOS, Apple silicon): `brew tap csturiale/shellvane && brew trust csturiale/shellvane && brew install --cask shellvane && brew install shellvane-cli` installa l'app e la CLI (la CLI anche su Linux). Le versioni recenti di Homebrew caricano un tap di terzi solo dopo `brew trust`; senza, si fermano con "Refusing to load cask … from untrusted tap". Installata così, macOS apre l'app senza l'avviso di sicurezza.

## Installazione {#install}

Ogni release è su [shellvane.com/download](https://shellvane.com/download/) e contiene: un installer per Windows (e il solo `shellvane-windows-amd64.exe`), l'app macOS per Mac Apple silicon (M1 o successivi; i Mac Intel non sono supportati) come immagine disco (`.dmg`: trascina Shellvane in Applicazioni) e in uno zip e, per Linux, un `.deb`, un `.rpm` e un AppImage (che si aggiorna da solo). Anche `shellvane-cli` viene pubblicato per ogni sistema. Le release con un suffisso nella versione (`v1.2.0-rc1`) sono pre-release.
