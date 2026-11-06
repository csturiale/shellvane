# Sincronizzazione tra computer {#top}

Shellvane può mantenere allineati più computer tramite una cartella che già condividono (Syncthing, Dropbox, iCloud Drive, OneDrive, Google Drive o una condivisione di rete), un server WebDAV, un repository Git o un bucket S3. Non c'è nessun server Shellvane.

## Configurazione {#setup}

1. Sblocca la cassaforte (la chiave di sincronizzazione viene conservata lì).
2. In **Impostazioni → Sincronizzazione** scegli dove vanno i file cifrati e una **passphrase di sincronizzazione**, poi **Avvia la sincronizzazione**:
   - **Cartella condivisa**: una cartella tenuta allineata da iCloud Drive, Google Drive, Dropbox, OneDrive, Syncthing o una condivisione di rete. Le cartelle dei cloud drive installati su questo computer vengono proposte come pulsanti (iCloud Drive su macOS e Windows; Google Drive per desktop, anche come lettera di unità su Windows; Dropbox; OneDrive; la `~/Sync` di Syncthing). Shellvane usa la sottocartella `Shellvane-sync`; la sincronizzazione la fa l'app del drive, che deve essere installata e collegata su ogni computer.
   - **Server WebDAV**: l'URL di una cartella su Nextcloud, ownCloud, un NAS o qualsiasi server WebDAV, con utente e password (meglio una password per app). Shellvane crea la cartella se manca; la password viene conservata nella cassaforte.
   - **Bucket S3**: Amazon S3 o un servizio compatibile (MinIO, Cloudflare R2, Backblaze B2, Wasabi): endpoint, bucket, regione, access key e secret key (meglio se limitata a quel bucket). I file vanno nella cartella `Shellvane-sync` del bucket; la secret key viene conservata nella cassaforte.
   - **Repository Git**: l'URL di un repository privato su cui puoi già fare push con `git` (chiave SSH o credential helper), per esempio `git@github.com:io/shellvane-sync.git`. Shellvane tiene un clone nella sua cartella dati, fa pull prima di ogni sincronizzazione e push dopo.
3. Su ogni altro computer scegli lo stesso posto (la cartella come appare lì, lo stesso URL) e la stessa passphrase.

La sincronizzazione avviene ogni cinque minuti mentre la cassaforte è sbloccata, e su richiesta con **Sincronizza ora**.

## Cosa viene sincronizzato {#what}

Host e gruppi (con impostazioni, tag e note), snippet, tunnel, regole di automazione, ambienti, runbook e schemi di colori. La cassaforte (password salvate, chiavi, seed TOTP), i workspace e le preferenze restano su ogni computer: un host che usa una chiave del vault ha bisogno di quella chiave su ogni computer.

## Come funziona {#how}

Ogni computer scrive solo il proprio file in `Shellvane-sync` (o nella cartella WebDAV, o fa commit solo di quel file nel repository), così il servizio non deve mai unire nulla. Un file è cifrato end-to-end con XChaCha20-Poly1305 con una chiave derivata dalla passphrase (Argon2id): il servizio della cartella, il server WebDAV, l'host Git o il servizio S3, e chiunque vi abbia accesso, non vedono nulla in chiaro. Per ogni elemento vince la modifica più recente; le eliminazioni vengono propagate per 90 giorni. Un elemento che non si può applicare (un runbook o uno schema il cui nome è già usato da un altro) viene elencato dopo la sincronizzazione.

**Interrompi la sincronizzazione su questo computer** dimentica la configurazione qui (e il clone Git locale); la cartella, il server o il repository e gli altri computer non vengono toccati.
