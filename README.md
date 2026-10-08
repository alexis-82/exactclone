<p align="center">
  <img src="https://i.ibb.co/1J7Tr5dn/appicon.png" alt="Logo ExactClone" width="128">
</p>

# ExactClone

Applicazione desktop (Go + Wails v2 + Svelte) per Windows e Linux:

- **Unità → Unità**: clonazione bit a bit di un disco su un altro.
- **Unità → Immagine**: copia bit a bit dell'intero disco in un file immagine compresso `.img.zst`.
- **Ripristino**: scrittura di un'immagine `.img.zst` su un'unità.

Tutte le operazioni hanno la verifica SHA-256 opzionale (attiva di default).

Richiede privilegi di amministratore (Windows) o root (Linux).

## Indice


- [Linux](#linux)
- [Immagini `.img.zst` senza ExactClone](#immagini-imgzst-senza-exactclone)
- [Note di comportamento](#note-di-comportamento)

Modalità sicura per lo sviluppo: con `EXACTCLONE_DEV_SAFE=1` come destinazione sono ammessi solo dischi USB, VHD e loop (su Linux compaiono anche i loop device).


### Linux

Installazione del lanciatore:

```sh
sudo sh build/linux/install.sh
```

Per disinstallare:
```sh
sudo sh build/linux/install.sh --uninstall
```

#### Avvio come root (pkexec)

La voce di menu esegue `exactclone-launch.sh`, che chiama `pkexec` (richiesta password con la finestra di polkit) e passa all'helper root le variabili necessarie ad aprire la finestra (`DISPLAY`, `XAUTHORITY`, `WAYLAND_DISPLAY`, `XDG_RUNTIME_DIR`, lingua).

Sessione **Wayland**: molti compositor rifiutano finestre di client root, quindi il launcher usa XWayland (`GDK_BACKEND=x11`) e autorizza root sul display con `xhost +SI:localuser:root`. Per provare Wayland nativo: `EXACTCLONE_WAYLAND=1 exactclone-launch.sh`.

Se la finestra non si apre:

1. avviare `exactclone-launch.sh` da terminale e leggere l'errore;
2. verificare che `xhost` sia installato (pacchetto `x11-xserver-utils` / `xorg-xhost`);
3. in alternativa avviare direttamente `sudo -E exactclone`.

## Immagini `.img.zst` senza ExactClone

Un'immagine è uno stream zstd standard dell'intero disco; le informazioni di ExactClone (disco di origine, dimensione, SHA-256) stanno in frame che i decompressori standard ignorano. Decomprimendola si ottiene un `.img` grezzo, identico al disco:

- **Windows**: [7-Zip-zstd](https://github.com/mcmilk/7-Zip-zstd/releases) — il 7-Zip ufficiale e il `tar.exe` di Windows 10 non supportano zstd.
- **Linux**: `zstd -d disco.img.zst` (pacchetto `zstd`). Ripristino manuale su un disco: `zstd -dc disco.img.zst | sudo dd of=/dev/sdX bs=16M status=progress conv=fsync`.

## Note di comportamento

- Windows: durante la copia i volumi dei dischi coinvolti vengono bloccati e smontati e il disco che viene scritto (clonazione, ripristino) viene messo **offline**. Al termine resta offline: va scollegato senza portarlo online sullo stesso PC, altrimenti Windows ne cambia la firma e il clone non si avvia.
- Linux: le partizioni montate automaticamente da origine e destinazione vengono smontate dopo la conferma; a fine copia il kernel rilegge la tabella delle partizioni.
- Immagini: lo spazio libero della destinazione deve essere almeno pari alla dimensione del disco (la dimensione compressa non è nota in anticipo); su FAT32 sono ammessi solo dischi fino a 4 GiB. L'immagine non può essere salvata sul disco di origine né ripristinata su un disco che la contiene. Un'immagine interrotta viene cancellata; una danneggiata viene rifiutata durante il ripristino.
- Il disco di sistema in uso non può essere né origine né destinazione.
