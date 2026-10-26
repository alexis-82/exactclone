<div align="center">

# ExactClone

**Clonazione bit a bit di dischi e immagini compresse, per Windows e Linux**

[![Version](https://img.shields.io/badge/version-1.1.0-blue?style=plastic)](CHANGELOG.md)&nbsp;&nbsp;&nbsp;[![Go](https://img.shields.io/badge/Go-00ADD8?style=plastic&logo=go&logoColor=white)](#)&nbsp;&nbsp;&nbsp;[![Wails](https://img.shields.io/badge/Wails-DF0000?style=plastic&logo=wails&logoColor=white)](#)&nbsp;&nbsp;&nbsp;[![Svelte](https://img.shields.io/badge/Svelte-FF3E00?style=plastic&logo=svelte&logoColor=white)](#)&nbsp;&nbsp;&nbsp;[![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20Linux-lightgrey?style=plastic)](#)&nbsp;&nbsp;&nbsp;[![License](https://img.shields.io/badge/license-MIT-green?style=plastic)](LICENSE)

</div>

---

<a id="indice"></a>
## 📑 Indice

- [✨ Cos'è](#cose)
- [💻 Requisiti di sistema](#requisiti-di-sistema)
- [🚀 Installazione](#installazione)
- [🔄 Aggiornamenti](#aggiornamenti)
- [🐧 Avvio su Linux](#avvio-su-linux)
- [🗜️ Installare zstd](#installare-zstd)
- [📝 Note di comportamento](#note-di-comportamento)
- [🛠️ Risoluzione problemi](#risoluzione-problemi)
- [📄 Licenza](#licenza)

---

<a id="cose"></a>
## ✨ Cos'è

ExactClone è un'applicazione desktop per Windows e Linux:

- **Unità → Unità**: clonazione bit a bit di un disco su un altro.
- **Unità → Immagine**: copia bit a bit dell'intero disco in un file immagine compresso `.img.zst`.
- **Ripristino**: scrittura di un'immagine `.img.zst` su un'unità.

Tutte le operazioni hanno la verifica SHA-256 opzionale (attiva di default). Interfaccia in italiano e inglese.

![devices](https://i.ibb.co/jkzpKNp0/pc-due-unita-usb.jpg)

<a id="requisiti-di-sistema"></a>
## 💻 Requisiti di sistema

- **Windows** 10 o 11 a 64 bit, con Microsoft Edge WebView2 Runtime (già presente nei sistemi aggiornati; l'installer avvisa se manca).
- **Linux** a 64 bit con GTK 3, WebKitGTK 4.1 e polkit (`pkexec`), per esempio Ubuntu 24.04 o successive e Fedora 40 o successive.
- Privilegi di amministratore (Windows) o root (Linux): ExactClone legge e scrive direttamente i dischi.

<a id="installazione"></a>
## 🚀 Installazione

I file si scaricano dalla [pagina delle release](https://github.com/alexis-82/exactclone/releases/latest). Il file `SHA256SUMS.txt` della release contiene i checksum di tutti i file.

> [!NOTE]
> La versione 1.0.0 non si aggiorna da sola: per passare alla 1.1.0 installala a mano una volta, nel modo che preferisci tra quelli sotto. Le versioni successive te le propone ExactClone.

### Windows

- **Installer** `ExactClone-<versione>-amd64-installer.exe`: installa ExactClone per tutti gli utenti in `C:\Program Files\ExactClone`, con collegamenti nel menu Start e sul desktop. Si disinstalla da *Impostazioni → App*. È il modo consigliato: solo la copia installata si aggiorna da sola.
- **Portable** `exactclone-<versione>-amd64-portable.exe`: un unico eseguibile da avviare dove vuoi, senza installazione. Non si aggiorna da solo: ExactClone segnala la nuova versione e apre la pagina della release.

### Linux

**Debian / Ubuntu** (`.deb`):

```sh
sudo apt install ./exactclone_<versione>_amd64.deb
# disinstallazione
sudo apt remove exactclone
```

**Fedora** (`.rpm`):

```sh
sudo dnf install ./exactclone-<versione>-1.x86_64.rpm
# disinstallazione
sudo dnf remove exactclone
```

**Altre distribuzioni** (`exactclone-linux-amd64.tar.gz`):

```sh
mkdir exactclone && tar -xzf exactclone-linux-amd64.tar.gz -C exactclone
cd exactclone
sudo sh build/linux/install.sh
# disinstallazione
sudo sh build/linux/install.sh --uninstall
```

`install.sh` rimuove un'eventuale installazione precedente in `/usr/local` (versione 1.0.0) e si rifiuta di procedere se ExactClone è già installato come pacchetto `.deb` o `.rpm`: in quel caso aggiornalo o rimuovilo con `apt`/`dnf`.

Con tutti e tre i metodi ExactClone compare nel menu delle applicazioni.

<a id="aggiornamenti"></a>
## 🔄 Aggiornamenti

All'avvio ExactClone controlla se su GitHub c'è una versione più recente e, se c'è, mostra le novità e propone **Scarica e installa**. Il controllo si disattiva nella scheda **Info** (casella *Controlla gli aggiornamenti all'avvio*), dove c'è anche il pulsante **Controlla aggiornamenti**.

- Il file scaricato viene verificato (dimensione e SHA-256) prima dell'installazione.
- Durante una clonazione, una creazione di immagine o un ripristino l'aggiornamento non parte: attendi la fine dell'operazione.
- **Windows**: si apre l'installer della nuova versione e ExactClone si chiude; a fine installazione puoi riavviarlo dall'ultima pagina.
- **Linux**: l'aggiornamento usa lo stesso metodo dell'installazione (`apt`, `dnf` oppure `install.sh`) e ExactClone si riavvia da solo.
- Senza connessione, o se GitHub non risponde, all'avvio non compare nulla.

<a id="avvio-su-linux"></a>
## 🐧 Avvio su Linux

La voce di menu esegue `exactclone-launch`, che chiama `pkexec` (richiesta della password con la finestra di polkit) e passa a ExactClone le variabili necessarie ad aprire la finestra (`DISPLAY`, `XAUTHORITY`, `WAYLAND_DISPLAY`, `XDG_RUNTIME_DIR`, lingua).

Sessione **Wayland**: molti compositor rifiutano finestre di client root, quindi il launcher usa XWayland (`GDK_BACKEND=x11`) e autorizza root sul display con `xhost +SI:localuser:root`. Per provare Wayland nativo: `EXACTCLONE_WAYLAND=1 exactclone-launch`.

<a id="installare-zstd"></a>
## 🗜️ Installare zstd

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

<a id="note-di-comportamento"></a>
## 📝 Note di comportamento

- Windows: durante la copia i volumi dei dischi coinvolti vengono bloccati e smontati e il disco che viene scritto (clonazione, ripristino) viene messo **offline**. Al termine resta offline: va scollegato senza portarlo online sullo stesso PC, altrimenti Windows ne cambia la firma e il clone non si avvia.
- Linux: le partizioni montate automaticamente da origine e destinazione vengono smontate dopo la conferma; a fine copia il kernel rilegge la tabella delle partizioni.
- Immagini: lo spazio libero della destinazione deve essere almeno pari alla dimensione del disco (la dimensione compressa non è nota in anticipo); su FAT32 sono ammessi solo dischi fino a 4 GiB. L'immagine non può essere salvata sul disco di origine né ripristinata su un disco che la contiene. Un'immagine interrotta viene cancellata; una danneggiata viene rifiutata durante il ripristino.
- Il disco di sistema in uso non può essere né origine né destinazione.

<a id="risoluzione-problemi"></a>
## 🛠️ Risoluzione problemi

**Linux: la finestra non si apre**

1. avviare `exactclone-launch` da terminale e leggere l'errore;
2. verificare che `xhost` sia installato (pacchetto `x11-xserver-utils` / `xorg-xhost`);
3. in alternativa avviare direttamente `sudo -E exactclone`.

**L'aggiornamento non riesce**

Il messaggio nella finestra di aggiornamento indica la causa (connessione, file non valido, installazione non riuscita). Puoi sempre scaricare la nuova versione dalla [pagina delle release](https://github.com/alexis-82/exactclone/releases/latest) e installarla come descritto in [Installazione](#installazione).

<a id="licenza"></a>
## 📄 Licenza

[MIT](LICENSE)
