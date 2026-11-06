# Importazione {#top}

**Importa** (nell'intestazione del pannello Host o dalla palette dei comandi) legge gli host da altri strumenti. Un'anteprima mostra cosa verrà creato; gli host esistenti non vengono duplicati e prima viene fatto un backup del database di Shellvane.

## Anteprima {#preview}

L'anteprima mostra gli host nelle loro cartelle, annidate come nello strumento di origine. Chiudi una cartella con la sua freccia; la sua casella sceglie o esclude tutti gli host al suo interno.

- **Utente per gli host che non ne hanno** viene impostato sugli host scelti senza utente (molte sessioni PuTTY e KiTTY non lo hanno); gli indirizzi nell'anteprima lo mostrano.
- **Metti tutto in un nuovo gruppo** (attivo di default, con il nome dello strumento) raccoglie l'import in un gruppo; disattivalo per mettere cartelle e host importati al livello principale, dove le cartelle con lo stesso nome delle tue vengono unite.

## Sorgenti {#sources}

- **OpenSSH**: `~/.ssh/config` con `Include`, pattern `Host`, `HostName`, `User`, `Port`, `IdentityFile`, `ProxyJump`, `ProxyCommand`, `SetEnv`, `RemoteCommand`, `PreferredAuthentications`, `IdentitiesOnly`, `LocalForward`/`RemoteForward`/`DynamicForward` (come tunnel).
- **PuTTY**: sessioni salvate (registro di Windows, o i file di PuTTY su Linux).
- **KiTTY**: sessioni dal registro o dalla cartella portable `Sessions`, mantenendo le cartelle di KiTTY come gruppi (anche le sottocartelle di una cartella `Sessions` portable). La cartella si può indicare anche come `.zip`.
- **MobaXterm**: sessioni SSH di `MobaXterm.ini` o di un export `.mxtsessions`, con le loro cartelle.
- **SecureCRT**: la cartella `Sessions`; le sottocartelle diventano gruppi.
- **WinSCP**: siti SFTP e SCP di `WinSCP.ini` (esportalo dal registro con Strumenti → Esporta configurazione).
- **FileZilla**: siti SFTP di `sitemanager.xml`, con le loro cartelle.
- **Tabby**: profili SSH di `config.yaml`, con gruppi e jump host.
- **mRemoteNG**: connessioni SSH di `confCons.xml`, con le loro cartelle.
- **Termius**: esporta gli host come file di configurazione SSH in Termius, poi scegli quel file.

Le sessioni di altri protocolli (RDP, VNC, FTP, Telnet, seriale…) vengono elencate come non importate; Telnet e console seriali si aprono invece dalla connessione rapida ([Connessione](shellvane:topic/connecting#telnet-serial)). Per spostare host tra due installazioni di Shellvane, esporta un gruppo ([Host](shellvane:topic/hosts#templates)) o usa la [sincronizzazione](shellvane:topic/sync). Le password salvate non vengono mai importate, nemmeno quelle offuscate: salvale nella cassaforte al primo accesso.

Le chiavi indicate da `IdentityFile` si possono poi importare in Chiavi.

[Importa host](shellvane:action/import)
