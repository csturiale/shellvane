# Host e gruppi {#top}

Un **host** è un server a cui ti connetti. Un **gruppo** è una cartella di host e sottogruppi che condividono le impostazioni.

## Crea, modifica, sposta {#manage}

- **Nuovo host** / **Nuovo gruppo** sono nell'intestazione del pannello Host; l'icona sul gruppo crea un host al suo interno.
- Trascina host e gruppi per spostarli. I pulsanti su ogni riga la modificano o la eliminano; eliminando un gruppo ti viene detto quanti sottogruppi e host contiene, che vengono eliminati con lui.
- Il filtro cerca nomi, indirizzi e note.
- L'**etichetta** di un host è facoltativa: senza, viene mostrato l'hostname.

[Nuovo host](shellvane:action/newHost) · [Nuovo gruppo](shellvane:action/newGroup)

## Menu contestuali {#menus}

Con il tasto destro su un host puoi connetterti, aprirlo con Mosh o desktop remoto, sfogliarne i file, svegliarlo, copiarne l'indirizzo, segnarlo come preferito, leggerne le note, duplicarlo, modificarlo o eliminarlo. Su un gruppo puoi connetterti a tutti i suoi host, aggiungervi un host o un sottogruppo, copiarlo come modello, modificarlo o eliminarlo.

## Impostazioni ereditate {#inheritance}

Le impostazioni lasciate vuote su un host vengono dal suo gruppo, poi dai gruppi superiori, poi dai valori predefiniti. Il testo d'esempio di ogni campo mostra il valore ereditato e quale gruppo lo ha impostato.

Impostazioni ereditabili: utente, porta, chiave SSH, script di avvio, colori del terminale, jump host, proxy, ProxyCommand, comandi per password e certificato, inoltro dell'agent, tmux/screen, riconnessione automatica, contesto Kubernetes.

Esempio: imposta `utente: deploy`, la chiave `prod` e il bastion come jump host sul gruppo **Produzione**; ogni host al suo interno ha bisogno solo dell'indirizzo.

## Esportare e condividere gruppi {#templates}

Per passare a un collega un insieme di connessioni (per esempio un albero `AWS/prod` e `AWS/staging` con utente, porta e comando proxy giusti), usa il pulsante degli appunti su un gruppo: **Esporta il gruppo** copia come testo il gruppo, i suoi sottogruppi e le loro impostazioni. Con **Includi gli host** (attivo di default) vanno con loro anche gli host: nome, indirizzo, tag, impostazioni e jump host, ricollegati per nome. Disattivalo per condividere solo la struttura, come modello da riempire con i propri host. Chiavi della cassaforte, password, note, tema del terminale locale, indirizzi per Wake-on-LAN e sequenze di knocking non vengono mai inclusi.

Sull'altro computer, il pulsante degli appunti in cima al pannello, **Importa gruppi**, li crea al livello principale o dentro un gruppo che scegli. Un gruppo che esiste già con lo stesso nome resta com'è e riceve solo ciò che gli manca; un host già presente con lo stesso nome e indirizzo viene saltato, quindi importare due volte non aggiunge nulla. Le chiavi si assegnano poi con **Chiave SSH** come al solito.

Per sicurezza, i comandi di un'esportazione importata **non** vengono importati — ProxyCommand, comandi per password, certificato e pre-connessione (girerebbero sul tuo computer), comandi di avvio e remoti, e l'inoltro X11. Il messaggio dopo l'importazione li elenca: aggiungili a mano se ti fidi della provenienza.

## Colori {#colors}

Il colore di un gruppo compare su ogni pannello, tab e voce della barra di stato dei suoi host, così un terminale di produzione si riconosce a colpo d'occhio. I sottogruppi ereditano il colore più vicino.

## Note {#notes}

Ogni host ha note libere, anch'esse cercate dal filtro.

## Riconoscere i pannelli {#labels}

Ogni pannello mostra l'etichetta, `utente@host:porta`, il percorso del gruppo con il suo colore, i jump host ("via bastion") e lo stato. Passa sopra una tab per gli stessi dettagli.

## Tag, preferiti, recenti {#tags}

- Assegna dei **tag** agli host (separati da virgole) nel dialog dell'host; compaiono sotto il nome. Clicca un tag, o scrivi `#tag` nel filtro, per vedere gli host che lo hanno. Il filtro cerca anche nelle note.
- La stella segna un **preferito**; i preferiti e gli ultimi host a cui ti sei connesso sono elencati in cima al pannello.
- **Duplica** copia un host con impostazioni, tag e note.
- **Connetti a tutti**: il pulsante griglia su un gruppo, o il pulsante sotto il filtro, apre tutti gli host in una tab, in griglia, con il broadcast attivo.
- Le **note** sono in Markdown: il pulsante note di un host le mostra formattate.
- La barra di connessione rapida ricorda gli ultimi indirizzi scritti.
