# Riconnessione, tmux e workspace {#top}

## Riconnessione automatica {#reconnect}

Quando una connessione cade (cambio di rete, sospensione, riavvio del server), il pannello mostra **Connessione persa** e riprova dopo 1, 2, 4… secondi, fino a 30, al massimo 8 volte, e subito quando la rete torna. **Riconnetti ora** e **Annulla** sono nel pannello. Gli errori che un nuovo tentativo non risolve (password errata, chiave dell'host cambiata) fermano i tentativi.

Si disattiva in Impostazioni › Sessioni, o per host/gruppo in Avanzate. Il vecchio output resta sullo schermo con una riga "riconnesso alle".

## Sessioni persistenti con tmux o screen {#tmux}

Imposta **Sessione persistente** (Avanzate) su tmux o GNU screen: la shell gira in una sessione con nome sul server, così dopo una disconnessione — o un riavvio di Shellvane — ritrovi gli stessi programmi ancora in esecuzione, con il loro output. Ogni pannello ha il proprio nome di sessione. Se il multiplexer non è installato, si apre la shell normale. Un badge "tmux" contrassegna questi pannelli.

## Workspace {#workspaces}

L'icona del layout nella barra delle tab salva con un nome le tab, gli split e le connessioni aperte; riaprilo più tardi dallo stesso menu, dalla schermata iniziale o dalla palette dei comandi. Salvare con un nome esistente lo sostituisce.

## Ripristino all'avvio {#restore}

Con Impostazioni › Sessioni › **Ripristina le tab all'avvio**, le tab aperte alla chiusura tornano all'avvio successivo (le sessioni tmux/screen vengono riprese).

[Apri le impostazioni](shellvane:view/settings)
