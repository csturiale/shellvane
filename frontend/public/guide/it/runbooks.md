# Runbook {#top}

Un runbook è una sequenza salvata di comandi da eseguire su molti host insieme. I risultati appaiono in una griglia: una riga per host, una colonna per passo, con il codice di uscita e l'output di ogni cella.

Apri **Automazione → Runbook** per crearne uno, oppure usa **Esegui su più host** su uno snippet per partire da quello. I runbook si esportano e importano in YAML, così puoi tenerli in un repository e condividerli.

[Apri Automazione](shellvane:view/automation)

## Destinazioni {#targets}

Scegli dove eseguire un runbook:

- **Host**: qualsiasi host salvato;
- **Gruppi**: tutti gli host del gruppo e dei suoi sottogruppi;
- **Tag**: tutti gli host con uno dei tag.

Un host selezionato più volte viene eseguito una volta sola. La finestra di avvio mostra quanti host corrispondono prima di eseguire.

Gli host vengono eseguiti **in parallelo** (con un eventuale numero massimo contemporaneo) o **in sequenza**. In sequenza puoi fermarti al primo host che fallisce, utile per una modifica progressiva: se il primo server si rompe, gli altri restano intatti.

## Passi {#steps}

Ogni passo è un comando shell eseguito a sé (un nuovo canale exec, non la shell interattiva). Opzioni:

- **Esegui quando**: sempre, solo se il passo precedente è riuscito, solo se è fallito, o solo se il suo output corrisponde a un'espressione regolare. "Passo precedente" è l'ultimo passo effettivamente eseguito su quell'host.
- **Continua se fallisce**: altrimenti un passo fallito salta i passi rimanenti su quell'host.
- **Chiedi prima di eseguire**: la cella ti aspetta; scegli **Continua** o **Ferma** per ogni host, oppure continua su tutti gli host insieme.
- **Timeout**: il passo fallisce se dura di più.

Scrivi `{{name}}` in un comando per chiedere un valore all'avvio del runbook; le variabili dichiarate possono avere un valore predefinito e un elenco di scelte. `{{steps.<nome passo>.output}}` inserisce l'output (senza spazi iniziali e finali) di un passo precedente sullo stesso host.

**Prova a secco** mostra il comando risolto di ogni passo per ogni host senza eseguire nulla. A esecuzione finita, esporta i risultati in CSV o JSON. Ogni comando eseguito viene scritto nel registro di audit.
