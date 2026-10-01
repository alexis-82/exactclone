# Piano tecnico — Disk Clone Tool

## Approccio

Progetto nuovo (cartella vuota, Go e Wails non ancora installati). Applicazione **Wails v2** (stabile) con backend Go e frontend **Svelte + TypeScript** (template ufficiale `svelte-ts`, il più leggero e semplice tra quelli disponibili).

Strategia **bottom-up**: prima i motori indipendenti dall'OS (clonazione, archivio, avanzamento), testati con file normali che fanno da finti dischi. Poi gli adattatori per Linux e Windows (elenco dischi, apertura dei device, montaggio), quindi il livello applicativo e la UI, infine il pacchetto distribuibile. Ogni livello è testabile senza quello sopra.

Principio chiave: i motori lavorano su interfacce Go (`io.ReaderAt`/`io.WriterAt`, `fs.FS`/path), **mai** su API dell'OS. Il codice specifico per l'OS sta solo in file `*_windows.go` / `*_linux.go` (build tag), così macOS si potrà aggiungere con `*_darwin.go`.

## Architettura

```
progetto_software/
├── main.go                     # bootstrap Wails
├── app.go                      # metodi esposti al frontend (bindings)
├── internal/
│   ├── disk/                   # modello Disk/Partition + enumerazione
│   │   ├── types.go
│   │   ├── enum_linux.go       # lsblk -J -b
│   │   └── enum_windows.go     # IOCTL su \\.\PhysicalDriveN + volumi
│   ├── rawdev/                 # apertura device raw, dimensione, lock
│   │   ├── rawdev.go           # interfaccia Device (ReaderAt/WriterAt/Size/Close)
│   │   ├── rawdev_linux.go     # /dev/sdX, BLKGETSIZE64, fsync
│   │   └── rawdev_windows.go   # CreateFile, IOCTL_DISK_GET_LENGTH_INFO, FSCTL_LOCK/DISMOUNT
│   ├── clone/                  # motore bit a bit + verifica SHA-256
│   ├── archive/                # tar.zst create/extract
│   ├── mount/                  # mount sola lettura partizioni
│   │   ├── mount_linux.go      # mount -o ro / umount
│   │   └── mount_windows.go    # lettera esistente o \\?\Volume{GUID}\
│   ├── progress/               # tracker byte/velocità/ETA, throttling eventi
│   ├── job/                    # un job alla volta, context cancel, eventi
│   ├── validate/               # regole di sicurezza (stesso disco, disco di sistema, spazio)
│   └── privilege/              # verifica admin/root
├── frontend/                   # Svelte + TS
│   └── src/
│       ├── App.svelte          # tab Backup / Ripristino
│       ├── lib/BackupTab.svelte, RestoreTab.svelte, ProgressPanel.svelte, ConfirmDialog.svelte
│       └── i18n/it.json, en.json, i18n.ts
├── build/
│   ├── windows/wails.exe.manifest   # requireAdministrator
│   └── linux/diskclone.desktop, diskclone-launch.sh, polkit policy
└── testdata/                   # fixture lsblk JSON, alberi di file di test
```

### Scelte tecniche puntuali

| Area | Scelta | Note |
|---|---|---|
| Copia bit a bit | Pipeline reader → [writer, hasher], blocchi da 16 MiB allineati a 4096, 2–4 buffer riciclati quando writer e hasher hanno finito | Hash in una goroutine separata per non frenare gli SSD veloci (F13). Opzione `HeadLast`: il primo 1 MiB (MBR/GPT) viene scritto per ultimo (F1). |
| Preparazione destinazione | `disk.PrepareForWrite`/`FinishWrite`. **Windows**: lock + dismount di tutti i volumi + disco **offline** durante la scrittura e lasciato offline alla fine. **Linux**: `umount` automatico delle partizioni montate dopo la conferma, `BLKRRPART` alla fine | F1, F2, F3: evita il rimontaggio a metà copia, la collisione di firma e l'automount udisks. |
| Verifica | Seconda passata: `dst.DropCache()` (Linux: `fsync` + `BLKFLSBUF` + `fadvise DONTNEED`; Windows: lettura `FILE_FLAG_NO_BUFFERING`), poi lettura dei primi N byte, hash, confronto | Facoltativa (D7). Senza svuotare la cache la verifica su Linux legge dalla RAM (F4). |
| Fine scrittura | `fsync` (Linux) / `FlushFileBuffers` (Windows) prima di dichiarare il successo | Altrimenti l'utente stacca la USB con la cache non ancora scritta. |
| Elenco dischi Linux | `lsblk -J -b -o NAME,PATH,SIZE,MODEL,SERIAL,TRAN,TYPE,FSTYPE,LABEL,MOUNTPOINTS,PKNAME,RM` (fallback `MOUNTPOINT` per util-linux < 2.37) | util-linux è sempre presente; il parsing JSON è testabile con fixture. `FSUSED` non usato: vale solo per FS montati (F6). |
| Spazio usato / stima archivio | `mount.Usage(path)` dopo il mount in sola lettura: `statfs` (Linux), `GetDiskFreeSpaceEx` (Windows) | F6. Regola aggiuntiva: archivio stimato > 4 GiB − 1 su destinazione FAT32 → bloccato (F5). |
| Attraversamento FS sorgente | Symlink/junction/reparse point non seguiti (salvati come symlink); hardlink → `TypeLink`; FIFO/socket/device saltati senza aprirli | F7: evita cicli infiniti e blocchi non annullabili. |
| Elenco dischi Windows | `golang.org/x/sys/windows` + IOCTL (`IOCTL_STORAGE_QUERY_PROPERTY`, `IOCTL_DISK_GET_LENGTH_INFO`, `IOCTL_DISK_GET_DRIVE_LAYOUT_EX`), volumi via `FindFirstVolume` + `IOCTL_VOLUME_GET_VOLUME_DISK_EXTENTS`, FS via `GetVolumeInformation` | Nessun PowerShell/WMI: più veloce e senza dipendenze. |
| Disco di sistema | Linux: disco padre dei mountpoint `/`, `/boot`, `/boot/efi` e swap. Windows: disco che contiene il volume di `%SystemDrive%` | Escluso come destinazione (G6). |
| Montaggio Windows | Prima prova di lettura via `\\?\Volume{GUID}\`; se fallisce, mount point temporaneo con `SetVolumeMountPoint` su una cartella in `%TEMP%` | Risolve l'ambiguità della spec; verificato dallo spike del Task 19. |
| Archivio | `archive/tar` (stdlib) + `github.com/klauspost/compress/zstd`, livello `SpeedDefault` (≈ livello 3), **non esposto in UI** | Risolve l'ambiguità della spec. Struttura: `<nomePartizione>/...`, più un `manifest.json` in testa (versione, data, disco, partizioni). |
| Estrazione | Rifiuta path assoluti e `..` (path traversal), symlink fuori dalla destinazione; su Linux i file prendono uid/gid della cartella di destinazione | Sicurezza; F11 (evita file di root nella home dell'utente). |
| Sicurezza WebView | Solo asset embedded, CSP restrittiva, link esterni via `BrowserOpenURL`, devtools off in build; ogni binding `Start*` ri-valida lato Go | F10: la WebView gira con privilegi elevati. |
| Avanzamento | `progress.Tracker` con media mobile su 3 s; eventi Wails `job:progress` al massimo 4 al secondo | |
| Annulla | `context.Context` controllato a ogni blocco o file | Archivio parziale cancellato (CA7). |
| i18n | Dizionari JSON nel frontend + store Svelte; lingua salvata in `os.UserConfigDir()/diskclone/config.json` | Il backend restituisce **codici di errore**, il frontend li traduce. |
| Privilegi | Windows: manifest `requireAdministrator`. Linux: script `diskclone-launch.sh` → `pkexec env DISPLAY=… XAUTHORITY=… WAYLAND_DISPLAY=… XDG_RUNTIME_DIR=… diskclone` + policy polkit con `allow_gui` | |

## Ordine di implementazione

1. **Fase 0 — Setup** (T1–T3): toolchain, scheletro Wails, struttura pacchetti.
2. **Fase 1 — Motori indipendenti dall'OS** (T4–T9): progress, clone, verify, archive create/extract. Tutto con test unitari su file temporanei.
3. **Fase 2 — Linux** (T10–T13, incluso T12b): elenco dischi, device raw, smontaggio/riletta partizioni, montaggio. Test di integrazione con loop device.
4. **Fase 3 — Windows** (T14–T19, incluso T17b): elenco dischi, device raw con lock/dismount/offline, accesso alle partizioni. Test di integrazione con VHD collegato.
5. **Fase 4 — Livello applicativo** (T20–T23): privilegi, validazioni, gestore dei job, binding Wails.
6. **Fase 5 — UI** (T24–T31): tab, form, conferma, avanzamento, ripristino, i18n.
7. **Fase 6 — Pacchetti** (T32–T34): build Windows e Linux, avvio con privilegi.
8. **Fase 7 — Collaudo** (T35): esecuzione di CA1–CA10 su hardware reale.

Le fasi 2 e 3 sono indipendenti tra loro: si può partire dal sistema su cui si sviluppa (Windows).

## Rischi e dipendenze

- [ ] **Rischio — Primo uso di Go e Wails.** Mitigazione: T1–T3 includono `wails doctor` e un "hello world" funzionante prima di scrivere logica; motori scritti in Go puro con test, senza Wails.
- [ ] **Rischio — Scrittura su disco Windows negata** se i volumi non sono bloccati/smontati (anche quelli senza lettera) o se Windows rimonta i volumi quando vede la nuova tabella delle partizioni (F1). Mitigazione: T17 blocca e smonta **tutti** i volumi; T17b mette il disco offline durante la scrittura; T5 `HeadLast`; T18 testa una sorgente GPT a 3 partizioni su VHD.
- [ ] **Rischio — Clone Windows non avviabile per collisione della firma del disco** (F2). Mitigazione: clone lasciato offline (T17b) + avviso a fine copia (T28); CA2 aggiornato con la condizione corrispondente.
- [ ] **Rischio — Automount Linux (udisks2)** monta le chiavette appena collegate (F3). Mitigazione: T12b smonta dopo la conferma; T13 riusa i mountpoint esistenti.
- [ ] **Rischio — Distruzione accidentale di un disco** durante lo sviluppo. Mitigazione: test di integrazione **solo** su loop device / VHD; le validazioni (T21) arrivano prima della UI; in modalità sviluppo si possono abilitare solo dischi rimovibili/USB (flag `DISKCLONE_DEV_SAFE=1`).
- [ ] **Rischio — `pkexec` + GUI su Linux**: `pkexec` cancella `DISPLAY`/`WAYLAND_DISPLAY`; su Wayland le app root possono essere rifiutate dal compositor; WebKitGTK come root può dare warning. Mitigazione: wrapper che passa le variabili d'ambiente; fallback `GDK_BACKEND=x11` + `xhost +SI:localuser:root` se Wayland rifiuta; test dedicato in T34 su GNOME Wayland e X11.
- [ ] **Rischio — Build Linux da Windows**: Wails su Linux richiede CGO + WebKitGTK, quindi niente cross-compilazione semplice. Mitigazione: build Linux su macchina Linux o in **WSL2** (Ubuntu) con `libwebkit2gtk-4.1-dev` e build tag `webkit2_41` (F8).
- [ ] **Rischio — Allineamento I/O Windows**: le letture raw devono essere multiple della dimensione del settore (512/4096); l'ultimo blocco va gestito. Mitigazione: buffer allineati a 4096 e lunghezza dell'ultimo blocco arrotondata/gestita, coperti da test.
- [ ] **Rischio — Permessi/attributi nel tar**: ACL Windows perse (accettato dai Non-Goal); file bloccati o illeggibili → saltati e riportati in un elenco di avvisi, senza far fallire l'intero archivio.
- [ ] **Dipendenza** — Go ≥ 1.22, Wails CLI v2, Node.js (presente: v22.14), WebView2 (Windows 10/11), `libwebkit2gtk-4.1` (Linux).
- [ ] **Dipendenza** — `golang.org/x/sys`, `github.com/klauspost/compress`.

## Verifica di allineamento con la spec

- [x] G1 tipo → metodo: T24, T25, T26
- [x] G2 bit a bit + verifica: T5, T6, T12, T17, T25
- [x] G3 archivio con partizioni selezionabili: T7, T9, T13, T19, T26
- [x] G4 ripristino in cartella: T8, T29
- [x] G5 avanzamento + annulla: T4, T22, T28
- [x] G6 conferma + blocco stesso disco/disco di sistema: T11, T16, T21, T27
- [x] G7 Windows + Linux, IT/EN: T23, T30, T31, T32–T34
- [x] Nessun task formatta dischi, crea immagini bit a bit su file, usa binari esterni (eccetto `lsblk`/`mount` di sistema su Linux) né tocca macOS.
