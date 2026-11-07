# Snippet {#top}

Gli snippet sono comandi salvati, organizzati in cartelle, inviati al pannello attivo (e agli altri durante il broadcast).

## Variabili {#variables}

- `{{servizio}}` chiede un valore prima dell'invio.
- `{{env:dev|staging|prod}}` chiede di scegliere uno dei valori.
- `{{host}}`, `{{user}}`, `{{date}}` vengono compilati dal pannello attivo e dall'orologio.

## Invia {#send}

Clicca **Invia** nel pannello Snippet, oppure trova lo snippet nella palette dei comandi (`#` cerca solo gli snippet). **Esegui** preme Invio dopo il testo; senza, il comando viene solo scritto, per poterlo controllare.

## Esempio {#example}

`journalctl -u {{servizio}} --since "{{da:1 hour ago|today}}" -f`

[Apri Snippet](shellvane:view/snippets)

## Importa ed esporta {#share}

Il pulsante di download in cima al pannello importa uno o più file:

- un file salvato da Shellvane (`shellvane-snippets.json`);
- uno script di shell o un file di testo con dei comandi: ogni comando diventa uno snippet, con il nome del commento `#` subito sopra (altrimenti il comando stesso) e in una cartella con il nome del file. Una riga che finisce con `\` continua nella successiva.

Gli snippet già presenti con la stessa cartella, lo stesso nome e lo stesso testo vengono saltati. Il pulsante di upload salva gli snippet mostrati (tutti, o quelli che corrispondono al filtro) come `shellvane-snippets.json`, da passare a un collega.

## Galleria della comunità {#gallery}

Il pulsante **Galleria della comunità** nella barra laterale, sopra la Guida (anche dalla palette dei comandi, dal pulsante negozio in cima al pannello Snippet e da Impostazioni › Terminale per gli schemi colore), apre la galleria di shellvane.com: raccolte di snippet (Kubernetes, diagnostica Linux…) e schemi colore del terminale, da aggiungere con un clic. La galleria è firmata dal progetto Shellvane e ogni file viene verificato prima di essere aggiunto; gli snippet vengono scritti solo quando li esegui, quindi leggili prima. **Proponi il tuo** spiega come inviare una raccolta o uno schema.
