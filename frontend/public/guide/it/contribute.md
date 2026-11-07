# Contribuisci {#top}

Schemi colore, raccolte di snippet e plugin creati dagli utenti arrivano a tutti tramite la galleria della comunità e lo store dei plugin. Tutto passa da una pull request sul repository open source, [github.com/csturiale/shellvane](https://github.com/csturiale/shellvane): serve un account GitHub, non Git, perché i file si caricano anche dal browser.

## Condividi uno schema colore {#themes}

1. In **Impostazioni › Terminale** seleziona lo schema e usa **Esporta**: ottieni un file JSON nel formato di Windows Terminal. Dai allo schema un `name` chiaro, è il titolo mostrato nella galleria.
2. Apri il repository su GitHub, vai nella cartella `gallery/themes/` e usa **Add file › Upload files**. GitHub crea una tua copia del repository (un fork) e propone di aprire una pull request: descrivi lo schema in una riga.
3. Il progetto controlla che si importi e che il testo sia leggibile; una volta accettato compare su shellvane.com/gallery e nella [galleria della comunità](shellvane:topic/snippets#gallery) dell'app al successivo aggiornamento del sito.

Uno schema per file, con un nome in minuscolo come `solarized-warm.json`.

## Condividi snippet {#snippets}

1. Nel pannello **Snippet** metti i comandi in una cartella ed esportali (`shellvane-snippets.json`). Una raccolta dovrebbe riguardare un solo argomento (uno strumento, un servizio, un tipo di diagnosi).
2. I comandi che cambiano qualcosa (riavvii, cancellazioni, `kubectl apply`…) devono avere **Premi Invio dopo l'invio** disattivato (`"run": false` nel file): la galleria li incolla senza eseguirli. Usa le [variabili](shellvane:topic/snippets#variables) al posto dei nomi dei tuoi host, utenti o percorsi.
3. Carica il file in `gallery/snippets/` come per uno schema colore e apri la pull request.

Non includere mai password, token, nomi o indirizzi di host interni: il file è pubblico.

## Scrivi un plugin {#write}

Un plugin è una cartella con un `plugin.json` e un programma in qualsiasi linguaggio: il protocollo è in [Plugin › Scrivere un plugin](shellvane:topic/plugins#write) e `examples/plugins/disk-usage` nel repository è un esempio completo. Per provarlo usa **Plugin › Installa da file** con la cartella: per i plugin che installi tu non servono store né firme.

## Pubblica un plugin nello store {#plugins}

Il codice di un plugin di terzi resta nel tuo repository; lo store lo elenca con l'indirizzo del pacchetto e il suo SHA-256, così l'app installa esattamente il file che è stato controllato.

1. Prepara il pacchetto: uno `.zip` della cartella del plugin, con dentro `plugin.json`. Pubblicalo a un indirizzo HTTPS permanente, per esempio come file di una release GitHub (`https://github.com/tu/plugin/releases/download/v1.0.0/my-plugin-1.0.0.zip`).
2. Calcola lo SHA-256: `shasum -a 256 my-plugin-1.0.0.zip` su macOS e Linux, `Get-FileHash my-plugin-1.0.0.zip` in PowerShell.
3. Nel repository aggiungi `plugins/<id>.json`:

   ```json
   {
     "id": "my-plugin",
     "name": "My plugin",
     "version": "1.0.0",
     "description": "Cosa mostra o fa, in una riga.",
     "author": "Il tuo nome",
     "homepage": "https://github.com/tu/plugin",
     "url": "https://github.com/tu/plugin/releases/download/v1.0.0/my-plugin-1.0.0.zip",
     "sha256": "…"
   }
   ```

   `id` e `version` devono coincidere con il `plugin.json` del pacchetto. Capacità e requisiti vengono letti da lì.
4. Apri la pull request, spiegando cosa fa il plugin e cosa esegue sugli host (`host.exec`) o sul computer.

Prima di accettarlo il progetto scarica il pacchetto, controlla SHA-256 e manifest e legge il codice: i plugin girano con i permessi dell'utente. A ogni aggiornamento del sito il pacchetto viene scaricato di nuovo e quelli il cui file è cambiato vengono esclusi. Per una **nuova versione** pubblica un nuovo zip a un nuovo indirizzo e aggiorna `version`, `url` e `sha256` nello stesso file: non sostituire mai uno zip già pubblicato.

## Traduzioni e codice {#code}

Correzioni, traduzioni e nuove funzionalità sono benvenute: leggi prima `CONTRIBUTING.md` nel repository (convenzioni, controlli da eseguire) e, per qualsiasi cosa più grande di una piccola correzione, apri una issue per concordare l'approccio.

## Cosa succede alla tua pull request {#review}

Il progetto la esamina su GitHub. Una volta accettata, la modifica viene applicata al repository principale del progetto con te come autore e pubblicata nel repository open source con l'aggiornamento successivo; la pull request viene chiusa con il link a quel commit. Aprendola accetti i termini di contribuzione in `CONTRIBUTING.md`: il tuo lavoro è pubblicato sotto AGPL-3.0 e può essere incluso anche nelle build ufficiali.
