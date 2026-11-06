# Tunnel {#top}

I tunnel inoltrano porte di rete attraverso una connessione SSH.

## Tipi {#types}

- **Locale** (`-L`): una porta di questo computer raggiunge un host e una porta visti dal server. Esempio: locale `5432` → `db.interno:5432`, poi collega il client SQL a `localhost:5432`.
- **Remoto** (`-R`): una porta del server raggiunge un host e una porta visti da questo computer.
- **Dinamico** (`-D`): un proxy SOCKS5 su questo computer; puntaci il browser per navigare come il server.

## Uso {#use}

Avvia e ferma i tunnel dal pannello Tunnel; **Avvia con l'app** li apre all'avvio. Ogni tunnel mostra stato, connessioni e byte trasferiti. La barra di stato conta i tunnel attivi.

I tunnel condividono la connessione con i terminali verso lo stesso host. Se cade, il tunnel mostra *In riconnessione* e si riapre dopo 1, 2, 4… secondi (al massimo 30) finché il server non è raggiungibile; gli errori che un nuovo tentativo non risolve (password errata, porta già in uso) lo fermano.

[Apri Tunnel](shellvane:view/tunnels)

## Porte sul server {#ports}

L'icona della porta nell'intestazione del pannello di un host elenca le porte TCP in ascolto su quel server (lette con `ss`, `netstat` o `/proc`, su un canale separato). **Inoltra** apre un inoltro locale sulla stessa porta se è libera, altrimenti su una porta libera qualsiasi, e mostra l'indirizzo locale: cliccalo per aprirlo nel browser.

Quando un programma nel pannello stampa un indirizzo locale come `http://localhost:5173`, Shellvane propone di inoltrare quella porta.

Questi inoltri sono **temporanei**: compaiono in Tunnel con l'etichetta "temporaneo" e terminano quando vengono fermati. **Salva come tunnel** ne trasforma uno in un profilo salvato.

## Socket Unix {#sockets}

Una destinazione che inizia con `/` è un socket Unix, senza porta: un tunnel locale verso `/var/run/docker.sock` rende raggiungibile il Docker del server su `127.0.0.1:<porta>` (`DOCKER_HOST=tcp://127.0.0.1:2375 docker ps`), e lo stesso vale per i socket di PostgreSQL o MySQL.
