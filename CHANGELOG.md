# Changelog

Tutte le modifiche rilevanti a ExactClone sono documentate in questo file.

## [1.2.0] - 2026-11-24

### Added

- Format tab: erases a drive and creates a single partition with the chosen file system (Linux: ext4, ext3, ext2, XFS, Btrfs, F2FS, NTFS, exFAT, FAT32; Windows: NTFS, exFAT, FAT32 up to 32 GB, ReFS), MBR or GPT partition table and an optional volume label. Quick format, or full format that first writes zeros over the whole drive (with progress and cancel). File systems whose program is not installed are shown disabled.

### Changed

- Cloning or restoring onto a larger disk now moves the backup GPT table to the end of the disk automatically (like `sgdisk -e`); the notice appears only if this fails.

## [1.1.0] - 2026-10-26

> ExactClone 1.0.0 cannot update itself: install 1.1.0 manually once (installer, .deb/.rpm or .tar.gz). Later versions are offered by the app.

### Added

- Automatic updates: at startup (can be turned off) and from the Info tab, ExactClone checks GitHub for a new version, shows what's new and, after confirmation, downloads and installs it. The download is verified (size and SHA-256), and no update starts while a disk operation is running.
- Windows installer for all users (Program Files) with Start menu and desktop shortcuts and an uninstaller. The portable executable is still available; it does not update itself (the app opens the release page).
- Linux packages: .deb (Debian/Ubuntu) and .rpm (Fedora 40+), besides the .tar.gz for install.sh; shown in software centers.

### Changed

- Linux: ExactClone is installed in /usr/bin and /usr/libexec instead of /usr/local, and the launcher is now `exactclone-launch`. install.sh removes a previous /usr/local installation and refuses to run when the .deb/.rpm package is installed.
- Slightly taller main window.

### Fixed

- Linux: wrong character in the Italian text of the password prompt.
- Windows: product version missing from the executable's numeric version fields.

## [1.0.0] - 2026-10-03

Primo rilascio.

### Aggiunto

- **Unità → Unità**: clonazione bit a bit di un disco su un altro.
- **Unità → Immagine**: copia bit a bit dell'intero disco in un file immagine compresso `.img.zst`.
- **Ripristino**: scrittura di un'immagine `.img.zst` su un'unità, con lettura delle informazioni dell'immagine (disco di origine, dimensione, SHA-256) prima di avviare l'operazione.
- Verifica SHA-256 opzionale al termine di ogni operazione (attiva di default).
- Barra di avanzamento e annullamento dell'operazione in corso.
- Immagini in formato zstd standard: si decomprimono anche senza ExactClone (7-Zip-zstd su Windows, `zstd` su Linux).
- Controlli di sicurezza sulle unità:
  - il disco di sistema in uso non può essere né origine né destinazione;
  - una destinazione più piccola dell'origine viene rifiutata;
  - controllo dello spazio libero e del limite di 4 GiB su FAT32 per le immagini;
  - un'immagine non può essere salvata sul disco di origine né ripristinata su un disco che la contiene;
  - le immagini interrotte vengono cancellate e quelle danneggiate rifiutate durante il ripristino.
- Conferma esplicita prima di ogni operazione che sovrascrive un'unità.
- Supporto Windows (amd64):
  - eseguibile unico con richiesta di elevazione UAC;
  - blocco e smontaggio dei volumi coinvolti, disco scritto messo offline durante la copia.
- Supporto Linux:
  - avvio come root tramite `pkexec` con policy polkit e voce di menu;
  - script `install.sh` per l'installazione;
  - smontaggio automatico delle partizioni montate e rilettura della tabella delle partizioni a fine copia;
  - compatibilità con sessioni Wayland tramite XWayland.
- Interfaccia in italiano e inglese, con la lingua salvata nel file di configurazione.
- Scheda **Info** con versione, piattaforma, privilegi e formato delle immagini.
- Avviso quando l'applicazione non ha i privilegi di amministratore/root.
- Modalità sicura per lo sviluppo (`EXACTCLONE_DEV_SAFE=1`): come destinazione sono ammessi solo dischi USB, VHD e loop.
- Test di integrazione su dischi virtuali (loop device su Linux, VHDX su Windows).

[1.0.0]: https://github.com/alexis-82/exactclone/releases/tag/v1.0.0
