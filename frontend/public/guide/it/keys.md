# Chiavi SSH {#top}

La sezione Chiavi gestisce chiavi private conservate cifrate nella cassaforte (che deve essere sbloccata).

## Genera {#generate}

**Genera** crea chiavi Ed25519 (consigliate), RSA 3072/4096 o ECDSA. Il commento aiuta a riconoscere la chiave sui server.

## Importa {#import}

Incolla una chiave privata OpenSSH, PEM o PuTTY `.ppk` (v2 o v3); le chiavi cifrate richiedono la passphrase una volta. I duplicati vengono riconosciuti dall'impronta.

## Token hardware (YubiKey, smart card) {#token}

Una chiave conservata su una YubiKey, una smart card o un HSM non lascia mai il dispositivo: il token firma ogni accesso dopo il suo PIN. Shellvane ci parla tramite il modulo PKCS#11 del produttore: OpenSC (la maggior parte delle smart card e l'applet PIV della YubiKey), `ykcs11` di Yubico, o la libreria del tuo HSM.

1. Installa il modulo (per esempio OpenSC) e collega il token.
2. In **Chiavi**, il pulsante USB **Aggiungi una chiave da un token hardware** elenca i moduli trovati nelle posizioni abituali; se ne può scrivere un altro percorso. **Cerca token**, scegli il token e la chiave, dalle un nome.
3. Assegnala a host o gruppi con **Chiave SSH**, come qualsiasi altra chiave.

Al primo accesso viene chiesto il PIN, che si può salvare nella cassaforte. Un PIN errato viene richiesto fino a tre volte, poi Shellvane si ferma, prima che il token si blocchi. Appena la connessione è aperta, il token viene disconnesso. Una chiave su token non si può esportare né ruotare, e l'agent di Shellvane non la offre agli altri programmi. La chiave pubblica, da installare sui server, si copia come le altre.

## Installa su un host {#deploy}

**Installa** si connette a un host (con qualsiasi metodo che funzioni ora, ad esempio la password) e aggiunge la chiave pubblica a `~/.ssh/authorized_keys`, creandolo con i permessi giusti. Eseguirlo due volte non aggiunge la chiave due volte.

## Usala {#assign}

Scegli la chiave nel campo **Chiave SSH** di un host o gruppo. **Copia** mette la chiave pubblica negli appunti.

## Certificati {#certificates}

L'icona del badge associa un certificato OpenSSH firmato dalla tua CA. Vengono mostrati principal e scadenza; un certificato scaduto viene ignorato e si usa la chiave semplice. Per certificati a breve durata emessi a ogni login, vedi il [comando certificato](shellvane:topic/auth#certificates).

[Apri Chiavi](shellvane:view/keys)

## Dove è usata, esportazione e rotazione {#rotation}

L'icona della rete di una chiave elenca i gruppi che la impostano e gli host che la usano, direttamente o tramite il loro gruppo.

**Ruota la chiave** la sostituisce in sicurezza: Shellvane crea una nuova chiave dello stesso tipo, la installa su ogni host che usa la vecchia, accede a ciascun host solo con la nuova chiave (senza agent né password) per dimostrare che funziona, e solo allora fa usare la nuova chiave a quegli host e gruppi. Se un host non la accetta, non cambia nulla. Facoltativamente la vecchia chiave viene poi rimossa dagli `authorized_keys` degli host. La vecchia chiave resta in Shellvane finché non la elimini; eliminare una chiave usata da host chiede conferma.

**Esporta** fornisce la chiave privata come testo OpenSSH o PuTTY (PPK v3), cifrata con una passphrase se la imposti. Serve anche a convertire tra i due formati: importa un `.ppk` ed esportalo come OpenSSH, o viceversa.

## Agent di Shellvane {#agent}

**Agent di Shellvane** (pannello Chiavi) rende disponibili le chiavi del vault agli altri programmi di questo computer, come `ssh`, `git` e `scp`, senza copiarle su disco. Clicca **SSH_AUTH_SOCK** per copiare la riga da incollare in un terminale (o nel profilo della shell); su Windows è una named pipe.

Ogni firma chiede prima conferma, indicando quale chiave viene usata; il vault deve essere sbloccato, e un vault bloccato non offre chiavi. I programmi non possono aggiungere né rimuovere chiavi.
