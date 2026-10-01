# Disk Clone

Applicazione desktop (Go + Wails v2 + Svelte) per Windows e Linux:

- **Unità → Unità**: clonazione bit a bit di un disco su un altro, con verifica SHA-256 opzionale.
- **Unità → File**: archivio compresso `.tar.zst` dei file delle partizioni selezionate.
- **Ripristino**: estrazione di un archivio in una cartella di un volume già montato.

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

Modalità sicura per lo sviluppo: con `DISKCLONE_DEV_SAFE=1` come destinazione sono ammessi solo dischi USB, VHD e loop (su Linux compaiono anche i loop device).

## Build

### Windows

```sh
wails build -platform windows/amd64
```

Produce `build/bin/diskclone.exe` (eseguibile unico). Il manifest richiede l'elevazione: all'avvio compare la richiesta UAC.

### Linux

Va compilato su Linux (o in WSL2): Wails usa CGO e WebKitGTK, quindi non si cross-compila da Windows.

```sh
wails build -tags webkit2_41
sudo sh build/linux/install.sh
```

`install.sh` installa:

| File | Destinazione |
|---|---|
| eseguibile | `/usr/local/bin/diskclone` |
| launcher | `/usr/local/bin/diskclone-launch.sh` |
| helper root | `/usr/local/libexec/diskclone-root` |
| policy polkit | `/usr/share/polkit-1/actions/org.diskclone.policy` |
| voce di menu | `/usr/share/applications/diskclone.desktop` |

Dipendenza a runtime: `libwebkit2gtk-4.1-0`.

#### Avvio come root (pkexec)

La voce di menu esegue `diskclone-launch.sh`, che chiama `pkexec` (richiesta password con la finestra di polkit) e passa all'helper root le variabili necessarie ad aprire la finestra (`DISPLAY`, `XAUTHORITY`, `WAYLAND_DISPLAY`, `XDG_RUNTIME_DIR`, lingua).

Sessione **Wayland**: molti compositor rifiutano finestre di client root, quindi il launcher usa XWayland (`GDK_BACKEND=x11`) e autorizza root sul display con `xhost +SI:localuser:root`. Per provare Wayland nativo: `DISKCLONE_WAYLAND=1 diskclone-launch.sh`.

Se la finestra non si apre:

1. avviare `diskclone-launch.sh` da terminale e leggere l'errore;
2. verificare che `xhost` sia installato (pacchetto `x11-xserver-utils` / `xorg-xhost`);
3. in alternativa avviare direttamente `sudo -E diskclone`.

## Test di integrazione (dischi virtuali)

Toccano dispositivi a blocchi: si eseguono solo su dischi virtuali creati dal test.

**Linux** (root, loop device; servono `losetup`, `sfdisk`, `mkfs.ext4`, `mkfs.vfat`):

```sh
sudo go test -tags integration -count=1 -v ./internal/itest/
```

**Windows** (shell amministratore; crea e collega VHDX con `diskpart`):

```powershell
go test -tags integration -count=1 -v ./internal/itest/
$env:DISKCLONE_EXPECT_ELEVATED = "1"; go test -count=1 ./internal/privilege/
```

## Aprire gli archivi senza Disk Clone

Gli archivi `.tar.zst` sono tar standard compressi con zstd e si possono aprire anche con altri programmi:

- **Windows**: [7-Zip-zstd](https://github.com/mcmilk/7-Zip-zstd/releases) — il 7-Zip ufficiale e il `tar.exe` di Windows 10 non supportano zstd. Si apre in due passaggi: `.tar.zst` → `.tar` → file.
- **Linux**: `tar --zstd -xf archivio.tar.zst` (serve il pacchetto `zstd`).

## Note di comportamento

- Windows: durante la clonazione i volumi di origine e destinazione vengono bloccati e smontati e la destinazione viene messa **offline**. Al termine il clone resta offline: va scollegato senza portarlo online sullo stesso PC, altrimenti Windows ne cambia la firma e il clone non si avvia.
- Linux: le partizioni montate automaticamente da origine e destinazione vengono smontate dopo la conferma; a fine copia il kernel rilegge la tabella delle partizioni.
- Archivi: symlink e junction vengono salvati come link e non seguiti; FIFO, socket e device vengono saltati con un avviso. Nel ripristino i link che puntano fuori dalla cartella di destinazione non vengono creati.
