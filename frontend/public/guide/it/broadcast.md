# Broadcast {#top}

Il broadcast invia ciò che scrivi in un pannello a tutti i pannelli della tab, ad esempio per eseguire lo stesso comando su più server.

## Attivalo {#use}

Clicca l'icona dell'antenna nella barra delle tab o premi [[B]]. La tab e i suoi pannelli ricevono un bordo arancione e il badge **BROADCAST**.

## Scegli i pannelli {#select}

L'intestazione di ogni pannello ha la casella **Ricevi**: togli la spunta per escludere un pannello.

La barra del broadcast sotto le tab sceglie anche i pannelli in un colpo: **Tutti i pannelli**, oppure i pannelli di un **gruppo**, o quelli degli host con un **tag**. Host → **Connetti a tutti** apre un gruppo o un elenco filtrato in una tab a griglia con il broadcast già attivo.

## Barra del broadcast {#bar}

Con il broadcast attivo compare una barra sotto le tab. Scrivi un comando e premi Invio: va a tutti i pannelli che ricevono, qualunque pannello abbia il focus. Togli la spunta da **Invio** per inviare il testo senza eseguirlo. ↑ e ↓ scorrono i comandi già inviati; **Ctrl+C** interrompe il comando in tutti i pannelli che ricevono.

Scrivere direttamente in un pannello funziona ancora e viene replicato negli altri.

## Sicurezza {#safety}

- I comandi che sembrano pericolosi (`rm -rf`, `shutdown`, `DROP TABLE`…) chiedono conferma quando raggiungerebbero più pannelli, dalla barra del broadcast, dagli snippet e dalla palette. L'elenco è un insieme di espressioni regolari (senza distinzione tra maiuscole e minuscole) in **Impostazioni → Broadcast**: aggiungi le tue, ad esempio `kubectl\s+delete`, o ripristina quelle predefinite.
- Le [risposte automatiche](shellvane:topic/automation#respond) non scattano mai nei pannelli che ricevono il broadcast.
