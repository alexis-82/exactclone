# ExactClone

Applicazione desktop (Go + Wails v2 + Svelte) per Windows e Linux:

- **Unità → Unità**: clonazione bit a bit di un disco su un altro.
- **Unità → Immagine**: copia bit a bit dell'intero disco in un file immagine compresso `.img.zst`.
- **Ripristino**: scrittura di un'immagine `.img.zst` su un'unità.

Tutte le operazioni hanno la verifica SHA-256 opzionale (attiva di default).

Richiede privilegi di amministratore (Windows) o root (Linux).

Guida completa a compilazione e installazione: [docs/INSTALLAZIONE.md](docs/INSTALLAZIONE.md).

## Requisiti di sviluppo

- Go ≥ 1.25, Node.js 22 LTS (minimo 20.19), Wails CLI v2: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- Windows: WebView2 (già presente su Windows 10/11)
- Linux: `libgtk-3-dev libwebkit2gtk-4.1-dev build-essential pkg-config`

`wails doctor` verifica l'ambiente.

## Sviluppo

```sh
wails dev            # app con hot reload
go test ./...        # test unitari Go
cd frontend && npm run check && npm run check:i18n
```

Modalità sicura per lo sviluppo: con `EXACTCLONE_DEV_SAFE=1` come destinazione sono ammessi solo dischi USB, VHD e loop (su Linux compaiono anche i loop device).

## Build

### Windows

```sh
wails build -platform windows/amd64
```

Produce `build/bin/exactclone.exe` (eseguibile unico). Il manifest richiede l'elevazione: all'avvio compare la richiesta UAC.

### Linux

Va compilato su Linux (o in WSL2): Wails usa CGO e WebKitGTK, quindi non si cross-compila da Windows.

```sh
wails build -tags webkit2_41
sudo sh build/linux/install.sh
```

`install.sh` installa:

| File | Destinazione |
|---|---|
| eseguibile | `/usr/local/bin/exactclone` |
| launcher | `/usr/local/bin/exactclone-launch.sh` |
| helper root | `/usr/local/libexec/exactclone-root` |
| policy polkit | `/usr/share/polkit-1/actions/org.exactclone.policy` |
| voce di menu | `/usr/share/applications/exactclone.desktop` |

Dipendenza a runtime: `libwebkit2gtk-4.1-0`.

#### Avvio come root (pkexec)

La voce di menu esegue `exactclone-launch.sh`, che chiama `pkexec` (richiesta password con la finestra di polkit) e passa all'helper root le variabili necessarie ad aprire la finestra (`DISPLAY`, `XAUTHORITY`, `WAYLAND_DISPLAY`, `XDG_RUNTIME_DIR`, lingua).

Sessione **Wayland**: molti compositor rifiutano finestre di client root, quindi il launcher usa XWayland (`GDK_BACKEND=x11`) e autorizza root sul display con `xhost +SI:localuser:root`. Per provare Wayland nativo: `EXACTCLONE_WAYLAND=1 exactclone-launch.sh`.

Se la finestra non si apre:

1. avviare `exactclone-launch.sh` da terminale e leggere l'errore;
2. verificare che `xhost` sia installato (pacchetto `x11-xserver-utils` / `xorg-xhost`);
3. in alternativa avviare direttamente `sudo -E exactclone`.

## Test di integrazione (dischi virtuali)

Toccano dispositivi a blocchi: si eseguono solo su dischi virtuali creati dal test.

**Linux** (root, loop device; servono `losetup`, `sfdisk`, `mkfs.ext4`, `mkfs.vfat`):

```sh
sudo go test -tags integration -count=1 -v ./internal/itest/
```

**Windows** (shell amministratore; crea e collega VHDX con `diskpart`):

```powershell
go test -tags integration -count=1 -v ./internal/itest/
$env:EXACTCLONE_EXPECT_ELEVATED = "1"; go test -count=1 ./internal/privilege/
```

## Immagini `.img.zst` senza ExactClone

Un'immagine è uno stream zstd standard dell'intero disco; le informazioni di ExactClone (disco di origine, dimensione, SHA-256) stanno in frame che i decompressori standard ignorano. Decomprimendola si ottiene un `.img` grezzo, identico al disco:

- **Windows**: [7-Zip-zstd](https://github.com/mcmilk/7-Zip-zstd/releases) — il 7-Zip ufficiale e il `tar.exe` di Windows 10 non supportano zstd.
- **Linux**: `zstd -d disco.img.zst` (pacchetto `zstd`). Ripristino manuale su un disco: `zstd -dc disco.img.zst | sudo dd of=/dev/sdX bs=16M status=progress conv=fsync`.

## Note di comportamento

- Windows: durante la copia i volumi dei dischi coinvolti vengono bloccati e smontati e il disco che viene scritto (clonazione, ripristino) viene messo **offline**. Al termine resta offline: va scollegato senza portarlo online sullo stesso PC, altrimenti Windows ne cambia la firma e il clone non si avvia.
- Linux: le partizioni montate automaticamente da origine e destinazione vengono smontate dopo la conferma; a fine copia il kernel rilegge la tabella delle partizioni.
- Immagini: lo spazio libero della destinazione deve essere almeno pari alla dimensione del disco (la dimensione compressa non è nota in anticipo); su FAT32 sono ammessi solo dischi fino a 4 GiB. L'immagine non può essere salvata sul disco di origine né ripristinata su un disco che la contiene. Un'immagine interrotta viene cancellata; una danneggiata viene rifiutata durante il ripristino.
- Il disco di sistema in uso non può essere né origine né destinazione.
