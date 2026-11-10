# Cassaforte {#top}

La cassaforte è un archivio cifrato su questo computer per password, passphrase delle chiavi, chiavi private e segreti TOTP.

## Come è protetta {#security}

La password principale viene trasformata in una chiave con Argon2id; quella chiave sblocca una chiave dati casuale; ogni segreto è cifrato con XChaCha20-Poly1305 e legato al suo nome, così i segreti non possono essere scambiati. La password principale non viene mai salvata. Senza di essa la cassaforte non si apre: non esiste recupero.

## Blocca e sblocca {#lock}

- L'icona del lucchetto nella barra del titolo la blocca o sblocca; si blocca anche da sola dopo un periodo di inattività.
- Quando una connessione ha bisogno di un segreto salvato e la cassaforte è bloccata, Shellvane la apre con il portachiavi di sistema (Touch ID, Windows Hello) se è attivo, altrimenti chiede la password principale (oppure puoi saltare e scrivere la password).
- Tutto il resto che ha bisogno della cassaforte mentre è bloccata (salvare il token di un plugin, Sincronizza ora, un'azione del team, aggiungere una chiave, le impostazioni dell'assistente) la apre allo stesso modo — prima con il portachiavi di sistema, dopo Touch ID o Windows Hello se attivi, la password principale solo se non è impostato o lo annulli — e poi prosegue; i messaggi sulla cassaforte bloccata hanno un pulsante **Sblocca il vault**, come lo stato in fondo alla finestra.
- Si sblocca una volta sola: quando più pannelli di un ambiente, l'avvio dell'app e una finestra hanno bisogno della cassaforte insieme, Touch ID o Windows Hello vengono chiesti una volta e gli altri proseguono. Se il portachiavi di sistema non ha più il segreto di Shellvane (eliminato, o portachiavi reimpostato), Touch ID non viene chiesto inutilmente: lo sblocco con il portachiavi si disattiva da solo e la finestra di sblocco propone di riattivarlo dopo la password principale.
- Quando un login chiede una password, una passphrase o un PIN, **Salva nel vault** funziona anche se la cassaforte è bloccata o non ancora creata: la finestra chiede lì la password principale (o crea la cassaforte) e salva il segreto dopo un login riuscito.

## Cosa contiene {#content}

Impostazioni › Cassaforte elenca le credenziali salvate (mai i loro valori) e permette di eliminarle o di cambiare la password principale.

[Apri le impostazioni della cassaforte](shellvane:view/settings)

## Blocco automatico e recupero {#recovery}

Il vault si blocca dopo un periodo di inattività (15 minuti per impostazione predefinita, o mai) e quando il computer va in sospensione; entrambi si impostano in **Impostazioni → Vault**. **Non proporre mai di salvare le password** nasconde l'opzione "salva nel vault" delle richieste di password.

Una **chiave di recupero** è un codice casuale che permette di impostare una nuova master password se la dimentichi. Viene mostrata una sola volta alla creazione: annotala e conservala lontano da questo computer. Crearne una nuova sostituisce la precedente; si può rimuovere in qualsiasi momento. Senza chiave di recupero, una master password dimenticata significa perdere i segreti salvati.

## Sblocco con il portachiavi di sistema {#keychain}

In **Impostazioni → Cassaforte**, **Sblocca con il portachiavi di sistema** conserva un segreto casuale nel Portachiavi di macOS, nel Gestore credenziali di Windows o, su Linux, nel Secret Service (GNOME Keyring, KWallet). Da quel momento la cassaforte si apre da sola all'avvio di Shellvane, e dopo un blocco automatico la cassaforte chiusa mostra **Usa il portachiavi**. Il sistema può chiedere il permesso la prima volta.

Il segreto apre la cassaforte solo su questo computer: master password e chiave di recupero continuano a funzionare ovunque, e cambiare la master password non disattiva l'opzione. Chi può usare la tua sessione su questo computer può aprire la cassaforte, quindi lascia l'opzione disattivata sui computer condivisi. Disattivandola, il segreto viene cancellato dal portachiavi.

**Chiedi Touch ID** (macOS) o **Chiedi Windows Hello** (Windows), visibile quando lo sblocco con il portachiavi è attivo, aggiunge una conferma: la cassaforte si apre allora dopo impronta, volto o password/PIN del dispositivo, e non da sola. Attivandola viene chiesta una volta per verificare che funzioni; annullando, la cassaforte resta chiusa, e la master password continua a funzionare. Non è disponibile su Linux. La finestra di Windows Hello può aprirsi dietro quella di Shellvane: cercala nella barra delle applicazioni.
