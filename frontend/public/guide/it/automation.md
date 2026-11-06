# Automazione {#top}

## Regole sull'output {#rules}

La sezione Automazione contiene regole che osservano l'output del terminale riga per riga. Ogni regola ha un'espressione regolare, un'azione e un ambito: tutte le sessioni, un gruppo (e i suoi sottogruppi) o un host.

- **Evidenzia** colora il testo corrispondente.
- **Notifica** mostra una notifica (di sistema quando Shellvane è in secondo piano), con un suono se vuoi. Cliccandola il pannello torna in primo piano.
- **Rispondi** digita una risposta quando compare un prompt.

Le righe che scrivi tu non attivano mai le regole: scrivere `echo panic:` non corrisponde a `panic:`, ma l'output del comando sì. Usa **Prova** nell'editor per provare un pattern su una riga di esempio. Le regole valgono per le sessioni aperte dopo una modifica.

[Apri Automazione](shellvane:view/automation)

## Risposte automatiche {#respond}

Una regola di risposta digita un testo fisso o una credenziale della cassaforte (ad esempio la password di sudo), poi Invio se scelto. Poiché è l'output del server ad attivarla:

- può valere solo per un gruppo o un host, mai per tutto;
- al massimo 3 risposte al minuto per regola;
- mai nei pannelli che ricevono il broadcast;
- ogni risposta viene scritta nel **log di audit** (mai il segreto).

## Script di avvio {#scripts}

Il campo **Comandi di avvio** di un host o gruppo è un piccolo script. Le righe semplici vengono digitate dopo il login, come prima; questi passi aggiungono controllo:

- `expect <regex>` attende che l'output corrisponda (timeout predefinito 15 s);
- `timeout <secondi>` cambia il timeout degli expect successivi;
- `wait <secondi>` fa una pausa;
- `sendSecret <scope della cassaforte>` digita una credenziale della cassaforte e preme Invio;
- `send <testo>` digita una riga (utile quando inizia con una di queste parole);
- `# …` è un commento.

Esempio:

```
expect \$ $
cd /srv/app
sudo -v
expect password for
sendSecret sudo:web
```

Se un passo fallisce, una notifica indica quale; la shell resta utilizzabile.

## Log di audit {#audit}

La scheda Log di audit elenca cosa ha fatto l'automazione, dal più recente, e lo esporta in CSV o JSON.
