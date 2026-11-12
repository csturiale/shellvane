# Procedure {#top}

Una procedura è una pagina Markdown di istruzioni operative con dei comandi: un runbook in un repository Git, un README, una pagina di appunti, o una pagina web come Confluence inviata dal browser. Il pannello **Procedure** la mostra accanto ai terminali, così incolli o esegui i suoi comandi nel pannello attivo invece di passare di continuo dal documento al terminale.

## Sorgenti {#sources}

Fai clic su **+** nell'intestazione del pannello e scegli dove sono le procedure:

- **Repository Git**: l'URL (`git@bitbucket.org:acme/runbooks.git` o `https://…`), facoltativamente un branch e una cartella del repository. Shellvane tiene una copia in sola lettura, clonata con il tuo accesso Git (chiave SSH o credential helper, come in un terminale), e non scrive mai nulla. **Aggiorna dal repository** scarica l'ultima versione.
- **Cartella di questo computer**: una cartella di file Markdown, ad esempio un clone che tieni già aggiornato tu.

Tutti i file `.md` e `.markdown` della sorgente sono elencati con il primo titolo come nome; filtra l'elenco per trovarne uno. Con più sorgenti, un menu sopra l'elenco passa dall'una all'altra.

## Pagine web {#web}

Le procedure su Confluence, su un wiki o su qualsiasi pagina dietro l'accesso aziendale arrivano dal browser, già autenticato. Fai clic sull'icona del link nel pannello (**Da una pagina web**):

- **Invia a Shellvane**: trascina il pulsante nella barra dei preferiti del browser (oppure copia il suo codice e crea un preferito con quel codice come indirizzo). Sulla pagina di una procedura, fai clic sul preferito: il testo selezionato, o altrimenti l'articolo della pagina, diventa Markdown (titoli, elenchi, tabelle, link e blocchi di codice con il loro linguaggio) e si apre in Procedure › **Pagine ricevute**. Il browser può chiedere una volta se aprire Shellvane. Una pagina lunga passa dagli appunti; Shellvane prende dagli appunti solo ciò che vi ha messo il preferito, riconosciuto da un marcatore, mai altro che hai copiato.
- **Incolla una pagina**: copia la pagina, o una sua parte, nel browser e incollala nel dialog; la formattazione diventa Markdown, che puoi ancora correggere prima di salvare.
- **Importa un file**: un documento Word (.docx) o PDF, oppure un file Markdown, HTML o di testo. Titoli, elenchi e tabelle restano; i paragrafi in uno stile codice (Code, HTML Preformatted…) o in un font a spaziatura fissa (Consolas, Courier, Menlo…) diventano blocchi di comandi con **Incolla** ed **Esegui**. Un PDF non ha stili, solo font e dimensioni: controlla il risultato prima di salvare. I PDF scansionati (immagini) non hanno testo da leggere.

Le pagine ricevute restano su questo computer (le ultime 200), con l'indirizzo della pagina in cima; eliminane una con l'icona del cestino. Il contenuto di una pagina non è mai considerato affidabile: niente parte da solo.

## Comandi {#commands}

Ogni blocco di codice della procedura ha un numero e le sue azioni:

- **Incolla** scrive il comando nel pannello attivo senza premere Invio, così puoi leggerlo o modificarlo prima.
- **Esegui** lo scrive e preme Invio.
- I blocchi che non sono comandi di shell (YAML, JSON, SQL…) offrono solo **Copia**.

Niente parte da solo: ogni comando è un clic. In una trascrizione di sessione (un blocco `console`, o righe che iniziano con `$ `) vengono inviati solo i comandi, senza il prompt e senza l'output mostrato nel documento. Quando il pannello attivo invia anche ad altri (broadcast), i comandi vanno a tutti, e i comandi pericolosi chiedono conferma come per gli snippet ([Broadcast](shellvane:topic/broadcast#safety)).

Un comando incollato o eseguito viene segnato come fatto: il contatore nell'intestazione mostra a che punto sei, il cerchio di un blocco lo segna fatto o no a mano, e **Ricomincia** azzera i segni. Ogni comando incollato o eseguito viene anche scritto in **Automazione › Registro** (quando, quale terminale o ambiente, quale procedura, il comando su una riga), esportabile in CSV o JSON come traccia di un change. I link ad altri file Markdown della sorgente li aprono nel pannello; i link web si aprono nel browser. HTML e immagini del documento non vengono mostrati.

## Segnaposto {#placeholders}

Scrivi `<nome>` o `{{nome}}` in un comando, ad esempio `kubectl -n <namespace> rollout restart deploy/web`. La prima volta che usi un comando con un segnaposto, Shellvane ne chiede il valore; il valore riempie poi quel segnaposto in tutti i comandi della sorgente finché l'app resta aperta. **Valori dei segnaposto** nell'intestazione li modifica. Un valore lasciato vuoto mantiene il segnaposto com'è scritto.

Un valore può anche essere ricordato come [valore personale](shellvane:topic/environments#personal), tenuto su questo computer anche dopo la chiusura dell'app: **Ricorda ovunque** riempie quel nome in ogni procedura, snippet e ambiente, **Solo qui** solo in questa sorgente. Le procedure da un repository Git condiviso funzionano così per tutti, ciascuno con i propri valori. Quando il terminale attivo appartiene a un [ambiente](shellvane:topic/environments#variables), le sue variabili riempiono i segnaposto con lo stesso nome (ad esempio `<namespace>`, `<project>`), dopo i valori scritti in questa sessione.

## Procedure di un ambiente {#environment}

Con un terminale di un [ambiente](shellvane:topic/environments) attivo, il pulsante puntina di una procedura aperta la collega a quell'ambiente. Da lì in poi, ogni volta che un suo terminale è attivo, il pannello mostra **Per <ambiente>:** con le sue procedure, a un clic, e i loro segnaposto prendono le variabili dell'ambiente. I collegamenti restano su questo computer; la puntina di nuovo li toglie.

## Crea un runbook {#runbook}

**Crea un runbook da questa procedura** trasforma i suoi comandi nei passi di un nuovo runbook (i segnaposto diventano variabili) e apre Automazione, dove scegli gli host e li esegui su molti server insieme ([Runbook](shellvane:topic/runbooks)).
