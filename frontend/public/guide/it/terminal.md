# Terminale, tab e pannelli {#top}

## Tab e split {#splits}

- Il **+** dopo l'ultima tab apre una nuova tab: un terminale locale, un host preferito o recente, qualsiasi host salvato cercandone nome, indirizzo, gruppo o tag, oppure un indirizzo scritto a mano (`utente@host:porta`, `telnet://…`, `serial://…`). Funzionano anche frecce e Invio.
- **Dividi a destra** ([[D]]) e **Dividi in basso** ([[E]]) aprono un'altra sessione verso lo stesso host accanto al pannello attivo.
- Trascina il divisore per ridimensionare. **Alt+frecce** spostano il focus tra i pannelli.
- **Chiudi pannello**: [[W]].

## Suggerimenti dei comandi {#suggestions}

Attiva **Impostazioni › Terminale › Suggerisci i comandi mentre scrivi** e, mentre digiti un comando, il resto di uno già eseguito compare in grigio dopo il cursore: premi **→** o **Tab** per prenderlo, o continua a scrivere per ignorarlo. I suggerimenti vengono prima dai comandi eseguiti sulla stessa connessione, poi sulle altre, poi dai tuoi snippet senza variabili. Mentre è mostrato un suggerimento, Tab lo prende; quando non ce n'è, Tab arriva alla shell per il suo completamento, e la sua cronologia (**↑**, **Ctrl+R**) funziona come sempre.

Viene ricordato solo ciò che digiti al prompt della shell, e solo se è comparso sullo schermo: ciò che scrivi quando viene chiesta una password, nei programmi a schermo intero (vim, less, top) o in broadcast non viene mai conservato, e nemmeno i comandi che iniziano con uno spazio o che sembrano contenere una password, un token o una chiave. La cronologia resta su questo computer; **Cancella la cronologia** nelle stesse impostazioni la elimina. I suggerimenti seguono una riga solo mentre la scrivi carattere per carattere: dopo il completamento con Tab, le frecce o le scorciatoie con Ctrl aspettano il comando successivo.

## Inviare e ricevere file nel terminale (ZMODEM) {#zmodem}

Con `lrzsz` installato sul server, i file passano attraverso il terminale stesso, anche dietro jump host e dentro tmux o screen:

- `sz file1 file2` invia i file a questo computer: vengono salvati nella cartella **Download** (senza mai sostituire un file già presente: `nome (1).ext`). Fai clic su un percorso nel riquadro che compare per mostrarlo nella sua cartella.
- `rz` apre qui il selettore di file, e i file scelti vanno nella cartella corrente del server.

Una barra mostra l'avanzamento, con **Annulla**. Chiudendo il selettore senza scegliere, `rz` viene annullato.

## Trasferimenti più veloci con trzsz {#trzsz}

Con [trzsz](https://trzsz.github.io) sul server (`pip install trzsz`, o i pacchetti `trzsz-go`), i trasferimenti sono più veloci di ZMODEM e funzionano anche con le cartelle:

- `tsz file1 file2` (o `tsz -d cartella`) invia a questo computer: una finestra del sistema chiede dove salvare.
- `trz` (o `trz -d` per le cartelle) apre la finestra di scelta file del sistema, e i file scelti vanno nella cartella corrente del server.

Nel terminale compare una barra di avanzamento; Ctrl+C interrompe il trasferimento. Su Linux le finestre richiedono `zenity` installato. Solo per le sessioni SSH.

## Spostare una tab in una finestra a sé {#detach}

Il pulsante **Sposta in una nuova finestra** su una tab la apre in una finestra separata, per un secondo schermo o per tenere sott'occhio un server. Le sessioni restano connesse durante lo spostamento: la nuova finestra mostra l'output recente e la cronologia (gli ultimi 512 KB), le divisioni e le impostazioni di broadcast della tab, e nient'altro. Chiudendo la sua ultima tab la finestra si chiude. Nella finestra separata lo stesso pulsante diventa **Riporta nella finestra principale**, che restituisce la tab, ancora connessa, accanto alle altre. Le tab nelle finestre separate non vengono riaperte al riavvio successivo.

## Più spazio per i terminali {#panel}

Il pannello laterale (host, snippet, tunnel…) si può nascondere: con il pulsante in fondo alla barra delle attività, con un clic sull'icona della sezione già aperta o con **[[L]]**. Un clic su qualsiasi sezione lo mostra di nuovo. Trascina il bordo destro del pannello per allargarlo o stringerlo; trascinandolo quasi chiuso, o con un doppio clic sul bordo, si nasconde. Larghezza e scelta vengono ricordate.

## Cerca, massimizza, zoom {#search}

- **Cerca** nello scrollback con [[F]] o la lente nell'intestazione del pannello: Invio e Maiusc+Invio passano tra i risultati, con opzioni per maiuscole/minuscole ed espressioni regolari.
- **Massimizza** un pannello con [[Invio]] o il pulsante nell'intestazione: occupa tutta la tab e il pannello laterale si fa da parte; di nuovo (o mostrando il pannello) ripristina il layout.
- **Zoom** di un solo pannello con [[=]] e [[-]], [[0]] per ripristinare.
- **Rinomina** una tab con un doppio clic sul titolo.
- Una tab che suona il **campanello** mentre è nascosta mostra 🔔 (e una notifica di sistema quando Shellvane è in secondo piano).

## Menu contestuali {#menus}

Con il tasto destro trovi le azioni di ciò che sta sotto il puntatore:

- una **tab**: rinomina, sposta in una nuova finestra (o di nuovo nella principale), chiudi, chiudi le altre tab, quelle a sinistra o a destra, o tutte;
- un **pannello** (intestazione o terminale): copia, incolla, seleziona tutto, svuota lo scrollback, dividi, massimizza, sposta in una nuova tab, cerca, apri la cartella corrente, log, registrazione, riconnetti, chiudi;
- un **host** o un **gruppo** nel pannello host: vedi [Host](shellvane:topic/hosts#menus).

Nei pannelli stretti l'intestazione mantiene nome, indirizzo, gruppo e stato della connessione (accorciati, per intero nel tooltip) e i pulsanti principali; gli altri sono nel menu del pannello.

## Spostare i pannelli {#drag}

Trascina un pannello dalla sua intestazione:

- su un lato di un altro pannello per dividere lì;
- al centro per scambiarli;
- su una tab per spostarlo in quella tab;
- sulla parte vuota della barra delle tab per dargli una tab tutta sua.

La sessione non viene mai interrotta.

## Aspetto {#appearance}

Impostazioni › Terminale imposta schema di colori, dimensione del carattere, cursore (blocco, barra, sottolineato, lampeggiante), righe di scrollback, copia alla selezione, conferma prima di incollare più righe e prima di chiudere tab con sessioni aperte, Option/Alt come Meta, Backspace come ^H e supporto screen reader. Gruppi e host possono sovrascrivere colori e dimensione, ad esempio uno schema rossastro per la produzione.

Impostazioni › Aspetto › **Dimensione dell'interfaccia** ingrandisce o rimpicciolisce testo e controlli di tutta l'app (80–150%); i terminali mantengono la dimensione del carattere scelta per loro.

## Evidenziazioni {#highlights}

Livelli di log, indirizzi IP, codici HTTP e UUID vengono colorati (i byte non cambiano). Disattiva il set predefinito in Impostazioni › Terminale e aggiungine di tuoi con le [regole di evidenziazione](shellvane:topic/automation#rules).

## Copia e incolla {#clipboard}

Seleziona il testo con il mouse, poi **[[C]]** lo copia e **[[V]]** incolla; Ctrl+C resta il tasto di interruzione della shell. Durante il broadcast il testo incollato raggiunge anche gli altri pannelli. I link sono cliccabili.

## Controllo di flusso {#flow}

Quando un comando inonda il terminale, Shellvane sospende la lettura dal server finché lo schermo non recupera, così l'app resta reattiva e la memoria limitata.

## Log e registrazioni {#logs}

- Il pulsante **log** nell'intestazione di un pannello salva l'output in un file: testo semplice, testo con orario o grezzo (con colori), da scegliere in Impostazioni › Terminale.
- Il pulsante **registra** registra la sessione in formato asciinema v2 (lo indica il badge rosso REC); le registrazioni si riproducono in Shellvane o con `asciinema play`.
- Host e gruppi possono salvare il log o registrare ogni sessione (Avanzate › Log e registrazione).
- **Log e registrazioni** (dalla palette dei comandi, o dall'avviso che compare all'avvio) elenca i file con un visualizzatore e un player: pausa, posizione, velocità da 0,5× a 8×, pause lunghe accorciate.

## Codifica e tipo di terminale {#encoding}

I server datati che non usano UTF-8 (Latin-1, Windows-1252, Shift_JIS, KOI8-R…) si impostano in Avanzate › Codifica: l'output viene convertito in UTF-8 e ciò che scrivi nella codifica del server. Lì si può cambiare anche TERM (predefinito `xterm-256color`).

## Schemi di colori {#themes}

**Impostazioni → Terminale** offre schemi inclusi e schemi tuoi. **Nuovo** e **Duplica** aprono l'editor: ogni colore (sfondo, testo, cursore, selezione e i 16 colori ANSI) con selettore o valore esadecimale, e anteprima dal vivo. I colori lasciati vuoti prendono valori predefiniti sensati.

**Importa…** legge gli schemi di altri terminali: iTerm2 (`.itermcolors`), Windows Terminal (uno schema o un intero `settings.json`), Base16 e Base24 (YAML), Alacritty (TOML o YAML), Kitty e Ghostty. Si possono scegliere più file insieme. **Esporta** copia uno schema in JSON di Windows Terminal, che la maggior parte dei terminali sa importare.

Un host o un gruppo può avere il proprio schema (nella sua finestra), per esempio toni rossi per la produzione. **I colori dell'app seguono lo schema del terminale** colora l'intera finestra con i colori dello schema scelto.

## Notifiche e immagini {#notifications}

I programmi possono chiedere una notifica con le sequenze di escape OSC 9 (`printf '\e]9;Build terminata\a'`) o OSC 777 (`printf '\e]777;notify;Deploy;finito\a'`). Se il pannello è nascosto la sua tab riceve un segno, e quando Shellvane non è in primo piano compare una notifica di sistema.

Con l'**integrazione con la shell** (OSC 133) Shellvane notifica anche quando termina un comando durato più del tempo impostato in **Impostazioni → Terminale**, con il suo codice di uscita. Molti prompt emettono già questi segni (Starship, oh-my-posh, fish 4, gli script di integrazione di VS Code o WezTerm); per bash semplice, aggiungi a `~/.bashrc` sul server:

```bash
PS0='\[\e]133;C\a\]'
PROMPT_COMMAND='printf "\e]133;D;%s\a\e]133;A\a" "$?"'
```

Le immagini vengono mostrate nel terminale con Sixel e il protocollo immagini di iTerm2, ad esempio `img2sixel foto.png`, `chafa -f sixel` o `imgcat`.

## Carico del server {#monitor}

L'icona del battito nell'intestazione del pannello di un host mostra uso della CPU, carico medio, memoria, swap, uso del disco di `/` e tempo di attività del server, aggiornati ogni tre secondi mentre è aperta. I dati sono letti con un breve comando su un canale separato (`/proc` su Linux, o `sysctl`/`vm_stat` su macOS e BSD): sul server non si installa nulla.

## Misurare le prestazioni {#benchmark}

**Impostazioni › Diagnostica › Prestazioni del terminale › Misura** apre una scheda che stampa 8 MB di righe numerate e le cronometra dal primo byte all'ultimo disegnato, poi mostra la velocità con il renderer (WebGL o DOM), il motore web e il sistema. Usa **Copia** per segnalarla: l'obiettivo è 20 MB/s o più.
