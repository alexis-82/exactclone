<div align="center">

# ExactClone

**Exact copies of your disks, compressed backup images and drive formatting, for Windows and Linux**<br>
*Copie esatte dei tuoi dischi, immagini di backup compresse e formattazione delle unità, per Windows e Linux*

[![Version](https://img.shields.io/badge/version-1.2.0-blue?style=plastic)](CHANGELOG.md)&nbsp;&nbsp;&nbsp;[![Go](https://img.shields.io/badge/Go-00ADD8?style=plastic&logo=go&logoColor=white)](#)&nbsp;&nbsp;&nbsp;[![Platform](https://img.shields.io/badge/platform-Windows%20%7C%20Linux-lightgrey?style=plastic)](#)

🇬🇧 [English](#en)&nbsp;&nbsp;·&nbsp;&nbsp;🇮🇹 [Italiano](#it)

</div>

---

<a id="en"></a>
<a id="contents"></a>
## 📑 Contents

- [✨ What it is](#what-it-is)
- [🚀 Installation](#installation)
- [🧭 How to use](#how-to-use)
- [🛟 Troubleshooting](#troubleshooting)
- [📄 License](#license)

---

<a id="what-it-is"></a>
## ✨ What it is

ExactClone is a desktop app for Windows and Linux that lets you:

- **Drive → Drive**: copy a whole disk onto another one, bit for bit (for example to move your system to a new disk).
- **Drive → Image**: save a whole disk into a compressed backup file (`.img.zst`).
- **Restore**: write a backup image back onto a drive.
- **Format**: erase a drive and prepare it with the file system you choose.

At the end of every copy ExactClone can check that the copy is identical to the original (on by default). The app is in English and Italian.

Backup images are standard compressed files: you can open them even without ExactClone, with [7-Zip-zstd](https://github.com/mcmilk/7-Zip-zstd/releases) on Windows or with `zstd` on Linux.

![devices](https://i.ibb.co/jkzpKNp0/pc-due-unita-usb.jpg)

<a id="installation"></a>
## 🚀 Installation

**What you need**

- **Windows** 10 or 11, 64-bit. Microsoft Edge WebView2 is required: up-to-date systems already have it, and the installer tells you if it is missing.
- **Linux** 64-bit, for example Ubuntu 24.04 or later, or Fedora 40 or later.
- The administrator password: ExactClone reads and writes disks directly.

Download the file for your system from the [releases page](https://github.com/alexis-82/exactclone/releases/latest).

> [!NOTE]
> Version 1.0.0 cannot update itself: install the new version by hand once. After that, ExactClone offers you new versions by itself.

**Windows**

- **Installer** (`ExactClone-<version>-amd64-installer.exe`): recommended. It adds ExactClone to the Start menu and the desktop, and it keeps itself up to date. You can uninstall it from *Settings → Apps*.
- **Portable** (`exactclone-<version>-amd64-portable.exe`): a single file you can start from anywhere, without installing. It does not update itself: when a new version is out, ExactClone opens its download page.

**Linux**

Open the Terminal in the folder where you downloaded the file and paste the command for your system.

Debian / Ubuntu:

```sh
sudo apt install ./exactclone_<version>_amd64.deb
# uninstall / disinstallazione
sudo apt remove exactclone
```

Fedora:

```sh
sudo dnf install ./exactclone-<version>-1.x86_64.rpm
# uninstall / disinstallazione
sudo dnf remove exactclone
```

Other distributions:

```sh
mkdir exactclone && tar -xzf exactclone-linux-amd64.tar.gz -C exactclone
cd exactclone
sudo sh build/linux/install.sh
# uninstall / disinstallazione
sudo sh build/linux/install.sh --uninstall
```

ExactClone then appears in the applications menu. On Linux it asks for your password every time it starts.

<a id="how-to-use"></a>
## 🧭 How to use

**Backup tab**

1. Choose **Drive → Drive** or **Drive → Image**.
2. Choose the source drive and the destination (another drive, or where to save the image).
3. Press **Start** and confirm.

**Restore tab**

1. Choose the image: ExactClone shows the disk it came from, its size and its date.
2. Choose the destination drive, press **Restore** and confirm.

**Format tab**

1. Choose the drive.
2. Choose the file system. The greyed-out ones cannot be used: the reason is written next to them.
3. Choose the partition table: **MBR** works with the most devices (drives up to 2 TiB), **GPT** is the modern choice and has no size limit.
4. Optionally, type a name for the drive.
5. Tick **Full format** to write zeros over the whole drive first: the old data can no longer be recovered, but it takes much longer. Leave it unticked for a quick format.
6. Press **Format** and confirm.

On Windows, FAT32 can be used only on drives up to 32 GB: for larger drives choose exFAT. ReFS works only on the Windows editions that can create it.

**Good to know**

- Everything on the destination drive is erased. ExactClone always asks you to confirm first.
- The disk of the system you are using cannot be copied, overwritten or formatted.
- Windows: after a copy or a restore, the written drive stays *offline*. Disconnect it without bringing it online on the same PC: otherwise Windows changes its identity and the copy will not start.
- Copying onto a larger drive: the partitions keep their original size and the extra space stays free. You can use it by enlarging a partition, for example with Disk Management or GParted.
- Images: the folder where you save the image needs as much free space as the whole disk (the final file is smaller). On a FAT32 drive you can only save images of disks up to 4 GiB. An interrupted image is deleted, a damaged one is refused when restoring.
- Updates: at startup ExactClone checks if a new version is out, shows what's new and offers **Download and install**. You can turn this off in the **Info** tab, where you can also check by hand. An update never starts while a copy or a format is running.

<a id="troubleshooting"></a>
## 🛟 Troubleshooting

**A file system is greyed out (Linux)**

Its program is not installed. Install the matching package from your software center or package manager: `e2fsprogs` (ext2/3/4), `xfsprogs` (XFS), `btrfs-progs` (Btrfs), `f2fs-tools` (F2FS), `ntfs-3g` (NTFS), `exfatprogs` (exFAT), `dosfstools` (FAT32). Then press **Refresh**.

**Linux: the window does not open**

Start `exactclone-launch` from the Terminal and read the error. Check that `xhost` is installed (package `x11-xserver-utils` or `xorg-xhost`).

**The update fails**

The update window tells you why. You can always download the new version from the [releases page](https://github.com/alexis-82/exactclone/releases/latest) and install it as described in [Installation](#installation).

<a id="license"></a>
## 📄 License

[MIT](LICENSE)

---

<a id="it"></a>
<a id="indice"></a>
## 📑 Indice

- [✨ Cos'è](#cose)
- [🚀 Installazione](#installazione)
- [🧭 Come si usa](#come-si-usa)
- [🛟 Problemi comuni](#problemi-comuni)
- [📄 Licenza](#licenza)

---

<a id="cose"></a>
## ✨ Cos'è

ExactClone è un'applicazione per Windows e Linux che ti permette di:

- **Unità → Unità**: copiare un intero disco su un altro, bit a bit (per esempio per spostare il sistema su un disco nuovo).
- **Unità → Immagine**: salvare un intero disco in un file di backup compresso (`.img.zst`).
- **Ripristino**: riscrivere un'immagine di backup su un'unità.
- **Formattazione**: cancellare un'unità e prepararla con il file system che scegli.

Alla fine di ogni copia ExactClone può controllare che la copia sia identica all'originale (attivo di default). L'applicazione è in italiano e in inglese.

Le immagini di backup sono normali file compressi: puoi aprirle anche senza ExactClone, con [7-Zip-zstd](https://github.com/mcmilk/7-Zip-zstd/releases) su Windows o con `zstd` su Linux.

![devices](https://i.ibb.co/jkzpKNp0/pc-due-unita-usb.jpg)

<a id="installazione"></a>
## 🚀 Installazione

**Cosa ti serve**

- **Windows** 10 o 11 a 64 bit. Serve Microsoft Edge WebView2: i sistemi aggiornati ce l'hanno già e l'installer ti avvisa se manca.
- **Linux** a 64 bit, per esempio Ubuntu 24.04 o successive, oppure Fedora 40 o successive.
- La password di amministratore: ExactClone legge e scrive direttamente i dischi.

Scarica il file per il tuo sistema dalla [pagina delle release](https://github.com/alexis-82/exactclone/releases/latest).

> [!NOTE]
> La versione 1.0.0 non si aggiorna da sola: installa la nuova versione a mano una volta. Dopo, è ExactClone a proporti le nuove versioni.

**Windows**

- **Installer** (`ExactClone-<versione>-amd64-installer.exe`): consigliato. Aggiunge ExactClone al menu Start e al desktop, e si aggiorna da solo. Si disinstalla da *Impostazioni → App*.
- **Portable** (`exactclone-<versione>-amd64-portable.exe`): un unico file da avviare dove vuoi, senza installazione. Non si aggiorna da solo: quando esce una nuova versione, ExactClone apre la sua pagina di download.

**Linux**

Apri il Terminale nella cartella dove hai scaricato il file e incolla il comando per il tuo sistema.

Debian / Ubuntu:

```sh
sudo apt install ./exactclone_<version>_amd64.deb
# uninstall / disinstallazione
sudo apt remove exactclone
```

Fedora:

```sh
sudo dnf install ./exactclone-<version>-1.x86_64.rpm
# uninstall / disinstallazione
sudo dnf remove exactclone
```

Altre distribuzioni:

```sh
mkdir exactclone && tar -xzf exactclone-linux-amd64.tar.gz -C exactclone
cd exactclone
sudo sh build/linux/install.sh
# uninstall / disinstallazione
sudo sh build/linux/install.sh --uninstall
```

ExactClone compare poi nel menu delle applicazioni. Su Linux chiede la password a ogni avvio.

<a id="come-si-usa"></a>
## 🧭 Come si usa

**Scheda Backup**

1. Scegli **Unità → Unità** oppure **Unità → Immagine**.
2. Scegli l'unità di origine e la destinazione (un'altra unità, oppure dove salvare l'immagine).
3. Premi **Avvia** e conferma.

**Scheda Ripristino**

1. Scegli l'immagine: ExactClone mostra il disco da cui proviene, la dimensione e la data.
2. Scegli l'unità di destinazione, premi **Ripristina** e conferma.

**Scheda Formattazione**

1. Scegli l'unità.
2. Scegli il file system. Quelli in grigio non si possono usare: il motivo è scritto accanto.
3. Scegli la tabella delle partizioni: **MBR** funziona con più dispositivi (unità fino a 2 TiB), **GPT** è la scelta moderna e non ha limiti di dimensione.
4. Se vuoi, scrivi un nome per l'unità.
5. Spunta **Formattazione completa** per scrivere prima zeri su tutta l'unità: i vecchi dati non sono più recuperabili, ma ci vuole molto più tempo. Senza spunta la formattazione è veloce.
6. Premi **Formatta** e conferma.

Su Windows FAT32 si può usare solo su unità fino a 32 GB: per quelle più grandi scegli exFAT. ReFS funziona solo sulle edizioni di Windows che possono crearlo.

**Da sapere**

- Tutto quello che c'è sull'unità di destinazione viene cancellato. ExactClone chiede sempre prima la conferma.
- Il disco del sistema che stai usando non può essere copiato, sovrascritto o formattato.
- Windows: dopo una copia o un ripristino l'unità scritta resta *offline*. Scollegala senza portarla online sullo stesso PC: altrimenti Windows ne cambia l'identità e la copia non si avvia.
- Copia su un'unità più grande: le partizioni mantengono la dimensione originale e lo spazio in più resta libero. Puoi usarlo allargando una partizione, per esempio con Gestione disco o GParted.
- Immagini: la cartella dove salvi l'immagine deve avere tanto spazio libero quanto l'intero disco (il file finale è più piccolo). Su un'unità FAT32 puoi salvare solo immagini di dischi fino a 4 GiB. Un'immagine interrotta viene cancellata, una danneggiata viene rifiutata durante il ripristino.
- Aggiornamenti: all'avvio ExactClone controlla se è uscita una nuova versione, mostra le novità e propone **Scarica e installa**. Puoi disattivarlo nella scheda **Info**, dove puoi anche controllare a mano. Un aggiornamento non parte mai durante una copia o una formattazione.

<a id="problemi-comuni"></a>
## 🛟 Problemi comuni

**Un file system è in grigio (Linux)**

Il suo programma non è installato. Installa il pacchetto corrispondente dal gestore software o dal gestore pacchetti: `e2fsprogs` (ext2/3/4), `xfsprogs` (XFS), `btrfs-progs` (Btrfs), `f2fs-tools` (F2FS), `ntfs-3g` (NTFS), `exfatprogs` (exFAT), `dosfstools` (FAT32). Poi premi **Aggiorna**.

**Linux: la finestra non si apre**

Avvia `exactclone-launch` dal Terminale e leggi l'errore. Controlla che `xhost` sia installato (pacchetto `x11-xserver-utils` oppure `xorg-xhost`).

**L'aggiornamento non riesce**

La finestra di aggiornamento ti dice il motivo. Puoi sempre scaricare la nuova versione dalla [pagina delle release](https://github.com/alexis-82/exactclone/releases/latest) e installarla come descritto in [Installazione](#installazione).

<a id="licenza"></a>
## 📄 Licenza

[MIT](LICENSE)
