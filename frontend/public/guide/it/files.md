# File (SFTP) {#top}

La vista File ha due pannelli indipendenti. Ognuno mostra **questo computer** o un host salvato, così puoi copiare tra questo computer e un server, o **direttamente tra due server**.

## Naviga {#browse}

Scegli la posizione in alto in un pannello (un elenco con ricerca: scrivi parte di un nome, indirizzo, gruppo o tag), poi naviga con doppio clic sulle cartelle, la barra del percorso o il pulsante "su".

## Copia {#copy}

Seleziona file o cartelle e usa la freccia verso l'altro pannello, oppure trascinali. Le cartelle vengono copiate con tutto il contenuto. La coda dei trasferimenti mostra l'avanzamento e permette di annullare.

## Permessi {#permissions}

**Permessi** modifica lettura/scrittura/esecuzione per proprietario, gruppo e altri, i bit speciali (setuid, setgid, sticky) e il valore ottale; su una cartella può applicarsi a tutto il contenuto.

## Altre azioni {#actions}

Nuova cartella, rinomina, elimina (con conferma), aggiorna. I file trascinati sulla finestra vengono caricati nel pannello remoto attivo.

## Conflitti e ripresa {#conflicts}

**Se il file esiste** (barra della vista File) decide cosa succede quando il file c'è già: sovrascrivi, salta, mantieni entrambi (rinominato "nome (1).est"), sovrascrivi solo se più recente, oppure **riprendi**, che aggiunge la parte mancante di un file il cui trasferimento si è interrotto (un trasferimento annullato in modalità ripresa mantiene il file parziale).

## Modifica su questo computer {#edit}

Il pulsante con la matita sul file scarica il file e lo apre con l'applicazione predefinita. A ogni salvataggio Shellvane lo ricarica sul server; la barra mostra lo stato. **Fatto** smette di osservarlo ed elimina la copia locale.

## Proprietari, link, segnalibri {#more}

- Il dialog dei permessi cambia anche proprietario e gruppo numerici (chown).
- I link simbolici mostrano dove puntano; il pulsante del link ne crea uno.
- La stella aggiunge ai segnalibri la cartella corrente di quell'host; i segnalibri compaiono nell'intestazione del pannello.

## Segui il terminale {#cwd}

Quando la shell comunica la cartella corrente con OSC 7, nell'intestazione del pannello compare un pulsante cartella che la apre in File. Quasi tutte le shell richiedono una riga nel file di avvio, ad esempio per bash:

```
PROMPT_COMMAND='printf "\033]7;file://%s%s\033\\" "$HOSTNAME" "$PWD"'
```

per zsh: `precmd() { printf '\e]7;file://%s%s\e\\' "$HOST" "$PWD" }`. Shellvane non modifica mai da sé i file sul server.

## Server senza SFTP {#nosftp}

Se un server non ha il sottosistema SFTP, Shellvane usa comandi di shell (come scp): il badge "senza SFTP" contrassegna il pannello. Servono una shell POSIX e GNU find, ed è più lento.

[Apri File](shellvane:view/files)

## Anteprima {#preview}

Il pulsante con l'occhio di un file ne mostra l'inizio (testo fino a 512 KB) o l'immagine stessa (PNG, JPEG, GIF, WebP, BMP, AVIF), senza scaricarlo. I file SVG vengono mostrati come testo.

## Confronta e sincronizza cartelle {#compare}

**Confronta cartelle…** confronta le cartelle mostrate sui due lati, comprese le sottocartelle: file presenti su un solo lato, più recenti da un lato (per data di modifica, con un secondo di tolleranza) o di dimensione diversa. Scegli la direzione, spunta cosa copiare (i file nuovi e più recenti sono già spuntati) ed eventualmente elimina ciò che esiste solo nella cartella da aggiornare. Le copie avvengono come normali trasferimenti.
