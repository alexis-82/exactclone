# Disk Clone — Guida a compilazione e installazione

Questa guida spiega come preparare l'ambiente, compilare e installare Disk Clone su **Windows** e su **Linux**.

> Disk Clone scrive direttamente sui dischi. Prima di usarlo su dischi con dati importanti, esegui i test di integrazione (sezione 4), che lavorano solo su dischi virtuali.

---

## Indice

1. [Cosa serve in breve](#1-cosa-serve-in-breve)
2. [Windows](#2-windows)
3. [Linux](#3-linux)
4. [Test](#4-test)
5. [Problemi comuni](#5-problemi-comuni)

---

## 1. Cosa serve in breve

| Componente | Versione | A cosa serve |
|---|---|---|
| Go | ≥ 1.25 (consigliata l'ultima) | backend e compilazione |
| Node.js | 22 LTS (minimo 20.19) | build del frontend (Vite 8) |
| Wails CLI | v2 | compila l'app desktop |
| Git | qualsiasi | scaricare il progetto (facoltativo) |
| WebView2 | — | solo Windows: motore dell'interfaccia (già presente su Windows 10/11) |
| WebKitGTK 4.1 + GTK 3 | — | solo Linux: motore dell'interfaccia |

Linux va compilato **su Linux** (o in WSL2): Wails su Linux usa CGO e WebKitGTK e non si compila da Windows.

---

## 2. Windows

Testato su Windows 10 22H2 e Windows 11.

### 2.1 Installare gli strumenti

Apri **PowerShell** (non serve l'amministratore per questa fase):

```powershell
winget install --id GoLang.Go -e
winget install --id OpenJS.NodeJS.LTS -e
winget install --id Git.Git -e
```

**Chiudi e riapri** PowerShell, così il `PATH` si aggiorna. Poi installa la Wails CLI:

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

Il comando `wails` viene installato in `%USERPROFILE%\go\bin`. Se PowerShell non lo trova, vedi la [sezione 5](#wails-non-trovato).

Verifica l'ambiente:

```powershell
go version
node --version
wails doctor
```

`wails doctor` deve terminare con `SUCCESS  Your system is ready for Wails development!`.

### 2.2 Ottenere il progetto

```powershell
cd $HOME\Desktop
git clone <url-del-repository> progetto_software
cd progetto_software
```

(oppure copia la cartella del progetto ed entra con `cd`.)

### 2.3 Compilare

```powershell
wails build -platform windows/amd64
```

Al primo avvio Wails esegue `npm install` nel frontend: ci vuole qualche minuto.

Risultato: **`build\bin\diskclone.exe`** (circa 12 MB, eseguibile unico, non richiede installazione).

Varianti utili:

| Comando | Effetto |
|---|---|
| `wails build -clean` | ricompila da zero |
| `wails build -nsis` | crea anche un installer (richiede [NSIS](https://nsis.sourceforge.io) installato) |
| `wails build -upx` | comprime l'exe (richiede [UPX](https://upx.github.io)) |

### 2.4 Installare e avviare

Non serve installazione: copia `diskclone.exe` dove preferisci (es. `C:\Program Files\DiskClone\`) e avvialo.

- All'avvio Windows mostra la richiesta **UAC**: l'app richiede sempre i privilegi di amministratore per accedere ai dischi.
- Se su un PC manca WebView2 (raro), installalo da <https://developer.microsoft.com/microsoft-edge/webview2/>.
- Le impostazioni (lingua) sono salvate in `%APPDATA%\diskclone\config.json`.

Per disinstallare basta cancellare l'exe e, se vuoi, la cartella `%APPDATA%\diskclone`.

### 2.5 Sviluppo con ricarica automatica

```powershell
wails dev
```

Apre l'app; le modifiche al frontend si ricaricano subito, quelle al codice Go ricompilano l'app. Per provare le operazioni sui dischi avvia PowerShell **come amministratore** prima di `wails dev`.

---

## 3. Linux

Le istruzioni coprono Ubuntu/Debian, Fedora e Arch. Su altre distribuzioni i nomi dei pacchetti cambiano ma servono gli stessi componenti.

### 3.1 Pacchetti di sistema

**Ubuntu 22.04+ / Debian 12+**

```sh
sudo apt update
sudo apt install -y build-essential pkg-config git curl \
    libgtk-3-dev libwebkit2gtk-4.1-dev x11-xserver-utils
```

**Fedora**

```sh
sudo dnf install -y gcc gcc-c++ make pkgconf-pkg-config git curl \
    gtk3-devel webkit2gtk4.1-devel xhost
```

**Arch Linux / Manjaro**

```sh
sudo pacman -S --needed base-devel git curl gtk3 webkit2gtk-4.1 xorg-xhost
```

`xhost` serve al launcher per aprire la finestra come root nelle sessioni Wayland (vedi 3.6).

### 3.2 Go

I pacchetti `golang` delle distribuzioni sono spesso troppo vecchi. Installa Go dal sito ufficiale (sostituisci la versione con l'ultima indicata su <https://go.dev/dl/>):

```sh
GO_VERSION=1.27.0
curl -LO https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.profile
source ~/.profile
go version
```

### 3.3 Node.js

Serve Node 22 LTS. Il modo più semplice, indipendente dalla distribuzione, è **nvm**:

```sh
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.1/install.sh | bash
source ~/.bashrc
nvm install 22
node --version
```

### 3.4 Wails CLI

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails doctor
```

`wails doctor` può segnalare `libwebkit2gtk-4.0` mancante: è normale, il progetto usa la **4.1** tramite il build tag `webkit2_41` (vedi sotto).

### 3.5 Compilare

```sh
git clone <url-del-repository> diskclone
cd diskclone
wails build -tags webkit2_41
```

Il tag `-tags webkit2_41` è **obbligatorio** sulle distribuzioni recenti (Ubuntu 24.04, Debian 13, Fedora 40+), che non hanno più WebKitGTK 4.0.

Risultato: **`build/bin/diskclone`**.

### 3.6 Installare

Dalla cartella del progetto:

```sh
sudo sh build/linux/install.sh
```

Lo script copia:

| File | Destinazione |
|---|---|
| eseguibile | `/usr/local/bin/diskclone` |
| launcher | `/usr/local/bin/diskclone-launch.sh` |
| helper eseguito come root | `/usr/local/libexec/diskclone-root` |
| policy polkit (richiesta password) | `/usr/share/polkit-1/actions/org.diskclone.policy` |
| voce nel menu applicazioni | `/usr/share/applications/diskclone.desktop` |
| icona | `/usr/share/pixmaps/diskclone.png` |

Su una macchina diversa da quella di compilazione serve solo la libreria a runtime:

| Distribuzione | Pacchetto |
|---|---|
| Ubuntu / Debian | `libwebkit2gtk-4.1-0` |
| Fedora | `webkit2gtk4.1` |
| Arch | `webkit2gtk-4.1` |

### 3.7 Avviare

Dal menu applicazioni: **Disk Clone**. Oppure da terminale:

```sh
diskclone-launch.sh
```

Cosa succede:

1. compare la finestra di polkit che chiede la password di amministratore;
2. l'app parte come **root** (necessario per leggere e scrivere i dischi);
3. nelle sessioni **Wayland** il launcher usa XWayland (`GDK_BACKEND=x11`) e autorizza root sul display con `xhost +SI:localuser:root`, perché molti compositor rifiutano finestre di applicazioni root.

Per provare Wayland nativo: `DISKCLONE_WAYLAND=1 diskclone-launch.sh`.

Le impostazioni sono salvate in `/root/.config/diskclone/config.json` (l'app gira come root).

### 3.8 Disinstallare

```sh
sudo rm -f /usr/local/bin/diskclone /usr/local/bin/diskclone-launch.sh \
    /usr/local/libexec/diskclone-root \
    /usr/share/polkit-1/actions/org.diskclone.policy \
    /usr/share/applications/diskclone.desktop \
    /usr/share/pixmaps/diskclone.png
sudo rm -rf /root/.config/diskclone
```

### 3.9 Sviluppo

```sh
sudo -E env "PATH=$PATH" wails dev -tags webkit2_41
```

`sudo -E` mantiene le variabili del display; serve solo per provare le operazioni sui dischi. Per lavorare sull'interfaccia basta `wails dev -tags webkit2_41` senza sudo.

---

## 4. Test

### 4.1 Test unitari (nessun privilegio, nessun disco toccato)

Su entrambi i sistemi, dalla cartella del progetto:

```sh
go test ./...
cd frontend
npm run check        # controllo dei tipi Svelte/TypeScript
npm run check:i18n   # traduzioni IT/EN complete
```

### 4.2 Test di integrazione (dischi virtuali)

Creano dischi virtuali, li clonano, li archiviano e li cancellano. **Non toccano i dischi reali.**

**Windows** — PowerShell **come amministratore**:

```powershell
go test -tags integration -count=1 -v ./internal/itest/
$env:DISKCLONE_EXPECT_ELEVATED = "1"; go test -count=1 ./internal/privilege/
```

Usa `diskpart` per creare due VHDX da 256 MiB. Verifica la clonazione di un disco GPT con volumi montati, che la destinazione resti offline e la lettura di una partizione senza lettera.

**Linux** — come root (servono `losetup`, `sfdisk`, `mkfs.ext4`, `mkfs.vfat`):

```sh
sudo apt install -y util-linux fdisk e2fsprogs dosfstools   # Debian/Ubuntu
sudo env "PATH=$PATH" go test -tags integration -count=1 -v ./internal/itest/
```

Usa i loop device. Verifica clonazione e verifica SHA-256 (compresa una corruzione rilevata), smontaggio automatico, rilettura della tabella partizioni e montaggio in sola lettura di ext4 e FAT.

### 4.3 Modalità sicura per le prove manuali

Con questa variabile come destinazione sono ammessi solo dischi **USB, VHD e loop**: utile per provare l'app senza rischiare i dischi interni.

```powershell
# Windows (PowerShell amministratore)
$env:DISKCLONE_DEV_SAFE = "1"; .\build\bin\diskclone.exe
```

```sh
# Linux
DISKCLONE_DEV_SAFE=1 diskclone-launch.sh
```

---

## 5. Problemi comuni

### `wails` non trovato

Il comando è in `~/go/bin` (Linux) o `%USERPROFILE%\go\bin` (Windows).

- Linux: `export PATH=$PATH:$HOME/go/bin` (aggiungilo a `~/.profile`).
- Windows: `[Environment]::SetEnvironmentVariable("Path", $env:Path + ";$env:USERPROFILE\go\bin", "User")`, poi riapri PowerShell.

### `npm error ERESOLVE … @sveltejs/vite-plugin-svelte`

Il template Wails originale usava una versione del plugin Svelte incompatibile con Vite 8. Nel progetto è già corretto (`^7.3.1` in `frontend/package.json`). Se ricompare dopo un aggiornamento delle dipendenze: `cd frontend && npm install @sveltejs/vite-plugin-svelte@latest -D`.

### Linux: `Package webkit2gtk-4.0 was not found`

Manca il build tag: compila con `wails build -tags webkit2_41`.

### Linux: la finestra non si apre dopo la password

1. Avvia `diskclone-launch.sh` da terminale e leggi l'errore.
2. `cannot open display` → installa `xhost` (sezione 3.1) e riprova.
3. In alternativa: `sudo -E /usr/local/bin/diskclone`.

### Windows: "L'applicazione non ha i privilegi di amministratore"

L'app è stata avviata senza elevazione (ad esempio con `wails dev` da una shell normale). Riavviala come amministratore.

### Windows: il disco clonato non compare in Esplora risorse

È voluto: al termine della clonazione il disco di destinazione resta **offline**. Se fosse portato online sullo stesso PC dell'originale, Windows ne cambierebbe la firma e il clone non si avvierebbe più. Scollegalo e usalo su un'altra macchina; se ti serve online, da *Gestione disco* → tasto destro sul disco → *Online*.

### `go test -race` non funziona su Windows

Il race detector richiede CGO e un compilatore C (es. MSYS2/MinGW). Non è necessario per i test del progetto.
