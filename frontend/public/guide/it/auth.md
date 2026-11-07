# Autenticazione {#top}

Shellvane prova, in ordine: la chiave assegnata all'host, il tuo agent SSH, le chiavi in `~/.ssh`, keyboard-interactive (2FA), poi la password.

## Password {#password}

Quando il server chiede una password, Shellvane mostra un dialog. Con la cassaforte sbloccata, spunta **Salva nella cassaforte**: viene salvata solo dopo un login riuscito e usata in silenzio la volta successiva (se viene rifiutata, ti viene chiesta di nuovo).

## Chiavi e agent {#keys}

- Le chiavi della cassaforte si assegnano per host o gruppo (campo **Chiave SSH**).
- Il tuo agent viene usato automaticamente: `SSH_AUTH_SOCK` su macOS/Linux; su Windows il servizio agent di OpenSSH, Pageant o `SSH_AUTH_SOCK` (funzionano anche gli agent di 1Password, Bitwarden, KeePassXC).
- Le chiavi cifrate in `~/.ssh` chiedono la passphrase solo se il server accetta la chiave.
- Ogni chiave viene provata a turno finché il server ne accetta una: prima quella dell'host, poi quelle dell'agent, poi `~/.ssh/id_ed25519`, `id_ecdsa` e `id_rsa`, ciascuna proposta una volta sola. **Diagnostica** mostra quali chiavi sono state proposte.

## Kerberos e Active Directory {#kerberos}

Nelle aziende con Active Directory (o Kerberos MIT/FreeIPA), i server accettano spesso direttamente il login di dominio: niente chiavi, niente password da scrivere per ogni server. Attiva **Kerberos / Active Directory (GSSAPI)** nelle opzioni avanzate dell'host o del gruppo:

- Con un ticket Kerberos su questo computer — il login di dominio di una postazione Linux, o `kinit utente@CORP.EXAMPLE.COM` — Shellvane lo usa e si collega senza chiedere nulla.
- Senza ticket (o su Windows e macOS, i cui archivi di ticket Shellvane non può leggere), Shellvane chiede la **password di dominio** una volta per connessione e ottiene un ticket dal domain controller.
- Il realm viene preso dal ticket, da `krb5.conf` o dal dominio dell'host in maiuscolo (`srv1.corp.example.com` → `CORP.EXAMPLE.COM`); i domain controller si trovano via DNS, come li pubblica Active Directory. Imposta realm e principal solo se questi non vanno bene.

Il server deve accettare `GSSAPIAuthentication` e avere una chiave per `host/<suo nome>`. Kerberos viene provato per primo; se non riesce seguono chiavi e password. Il ticket non viene inoltrato al server.

## Metodi e loro ordine {#order}

Per impostazione predefinita Shellvane prova le chiavi, poi keyboard-interactive (codici a due fattori, richieste di PAM), poi la password, come OpenSSH. In **Avanzate**, **Scegli i metodi di autenticazione e il loro ordine** cambia l'ordine per un host o un gruppo, e i metodi non selezionati non vengono mai usati (`PreferredAuthentications` di OpenSSH).

I server con un `MaxAuthTries` basso chiudono la connessione dopo poche chiavi rifiutate. Imposta **Chiavi proposte** su **Solo la chiave dell'host** per proporre solo la chiave scelta in **Chiave SSH** (`IdentitiesOnly` di OpenSSH). Entrambe le impostazioni vengono importate da `~/.ssh/config`.

## Codici a due fattori (TOTP) {#totp}

I server che chiedono un codice di verifica (Google Authenticator, Duo…) mostrano le loro domande in un dialog. Per avere i codici compilati automaticamente, incolla il segreto TOTP (base32, oppure l'URI `otpauth://` dietro al QR code) nel campo **Codice a due fattori** dell'host: viene conservato nella cassaforte e il codice è calcolato a ogni login. Una domanda di password nella stessa richiesta viene compilata dalla cassaforte o dal comando password.

## Comando password {#password-command}

**Avanzate › Comando password** esegue un comando che stampa la password, ad esempio `pass show ssh/$SHELLVANE_HOST` o `op read "op://Private/$SHELLVANE_HOST/password"`. Le variabili `SHELLVANE_HOST`, `SHELLVANE_PORT` e `SHELLVANE_USER` descrivono la destinazione. I preset coprono pass, 1Password, Bitwarden e HashiCorp Vault.

## Certificati {#certificates}

- **Associa un certificato** a una chiave in Chiavi (il file `-cert.pub` firmato dalla tua CA): viene offerto al posto della chiave semplice finché è valido.
- **Comando certificato** (Avanzate) chiede alla tua CA un certificato a breve durata a ogni login: la chiave pubblica è in `$SHELLVANE_PUBLIC_KEY` e nel file `$SHELLVANE_PUBLIC_KEY_FILE`; il comando stampa il certificato. Preset: motore SSH di Vault, smallstep step-ca.
- **Credenziali da un plugin** (Avanzate): HashiCorp Vault, CyberArk, Conjur, Teleport o Boundary forniscono l'intero accesso, senza script: vedi [Plugin](shellvane:topic/plugins#credentials).

## Inoltro dell'agent {#forwarding}

**Avanzate › Inoltro agent** permette ai programmi sul server (git, ssh verso un altro host) di usare le chiavi del tuo agent. Poiché chiunque abbia root sul server potrebbe usarlo, Shellvane chiede prima di **ogni** firma: *Consenti una volta*, *Consenti per questa connessione* o *Rifiuta*. Aggiungere o rimuovere chiavi dal server è sempre rifiutato.

[Jump host e proxy](shellvane:topic/jump-proxy)
