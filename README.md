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
- [Installare zstd](#installare-zstd)
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

## Installare zstd

Le immagini `.img.zst` sono stream zstd standard: con lo strumento `zstd` si possono decomprimere anche senza ExactClone.

### Windows

Da PowerShell o dal Prompt dei comandi, con **winget** (incluso in Windows 10/11):

```powershell
winget install Meta.Zstandard
```

In alternativa:

- **Scoop**: `scoop install zstd`
- **Chocolatey** (da amministratore): `choco install zstandard`
- **Manuale**: scaricare lo zip `zstd-vX.Y.Z-win64.zip` dalle [release ufficiali](https://github.com/facebook/zstd/releases), estrarlo e aggiungere la cartella al `PATH`.

Chiudere e riaprire il terminale, poi verificare:

```powershell
zstd --version
```

Per un'interfaccia grafica: [7-Zip-zstd](https://github.com/mcmilk/7-Zip-zstd/releases) (`winget install mcmilk.7zip-zstd`). Il 7-Zip ufficiale e il `tar.exe` di Windows non supportano zstd.

### Linux

```sh
# Debian / Ubuntu / Mint
sudo apt install zstd

# Fedora / RHEL / Rocky / Alma
sudo dnf install zstd

# Arch / Manjaro
sudo pacman -S zstd

# openSUSE
sudo zypper install zstd
```

Verifica:

```sh
zstd --version
```

### Uso

```sh
# Decomprimere in un .img grezzo, identico al disco
zstd -d disco.img.zst

# Linux: ripristino manuale su un disco (ATTENZIONE: sovrascrive /dev/sdX)
zstd -dc disco.img.zst | sudo dd of=/dev/sdX bs=16M status=progress conv=fsync
```

## Note di comportamento

- Windows: durante la copia i volumi dei dischi coinvolti vengono bloccati e smontati e il disco che viene scritto (clonazione, ripristino) viene messo **offline**. Al termine resta offline: va scollegato senza portarlo online sullo stesso PC, altrimenti Windows ne cambia la firma e il clone non si avvia.
- Linux: le partizioni montate automaticamente da origine e destinazione vengono smontate dopo la conferma; a fine copia il kernel rilegge la tabella delle partizioni.
- Immagini: lo spazio libero della destinazione deve essere almeno pari alla dimensione del disco (la dimensione compressa non è nota in anticipo); su FAT32 sono ammessi solo dischi fino a 4 GiB. L'immagine non può essere salvata sul disco di origine né ripristinata su un disco che la contiene. Un'immagine interrotta viene cancellata; una danneggiata viene rifiutata durante il ripristino.
- Il disco di sistema in uso non può essere né origine né destinazione.

---

![devices](https://i.ibb.co/jkzpKNp0/pc-due-unita-usb.jpg)