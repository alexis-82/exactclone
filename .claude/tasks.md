# Task — Disk Clone Tool

> Legenda: `[x]` fatto e DoD verificato · `[~]` codice fatto, DoD di integrazione da verificare (Linux: `sudo go test -tags integration -count=1 -v ./internal/itest/`; Windows: sessione amministratore) · `[ ]` da fare

## Fase 0 — Setup

- [x] Task 1: Installare Go (≥ 1.22) e la Wails CLI v2 (`go install github.com/wailsapp/wails/v2/cmd/wails@latest`) — DoD: `go version` e `wails doctor` non riportano errori bloccanti su Windows.
- [x] Task 2: Creare il progetto con `wails init -n diskclone -t svelte-ts` nella cartella del progetto, `git init`, `.gitignore` — DoD: `wails dev` apre la finestra demo; primo commit fatto.
- [x] Task 3: Creare la struttura `internal/{disk,rawdev,clone,archive,mount,progress,job,validate,privilege}` con `doc.go` e i tipi base (`disk.Disk`, `disk.Partition`, `rawdev.Device`) — DoD: `go build ./...` e `go vet ./...` passano.

## Fase 1 — Motori indipendenti dall'OS

- [x] Task 4: `progress.Tracker` (byte totali/fatti, velocità con media mobile su 3 s, ETA, throttling a 4 eventi/s) — DoD: test unitari con orologio finto verificano velocità, ETA e throttling.
- [x] Task 5: `clone.Copy(ctx, src io.ReaderAt, dst io.WriterAt, size, opts, onProgress)` con pipeline reader → [writer, hasher] (SHA-256 in goroutine separata, buffer riciclato solo quando entrambi hanno finito — F13), blocchi da 16 MiB, buffer allineati, opzione `HeadLast` per scrivere il primo 1 MiB alla fine (F1) — DoD: test copia file da 0 B, 1 B, 16 MiB+1, 100 MiB casuali, con e senza `HeadLast` → contenuto identico e hash corretto.
- [x] Task 6: Annulla e gestione errori nel clone (ctx annullato, errore di lettura/scrittura a metà) + `clone.Verify(ctx, dst, size, expectedHash)` che chiama `dst.DropCache()` prima di rileggere (F4; no-op sui file di test) — DoD: test: annulla ferma entro un blocco e ritorna `ErrCanceled`; un byte alterato nella destinazione fa fallire la verifica.
- [x] Task 7: `archive.Create(ctx, roots []Root{Name, Path}, w io.Writer, onProgress)` → tar + zstd, sottocartella per partizione, `manifest.json` iniziale, file illeggibili raccolti come avvisi; symlink/junction/reparse point **non seguiti** (salvati come symlink), hardlink come `TypeLink`, FIFO/socket/device saltati con avviso senza aprirli (F7) — DoD: test su albero di file in `t.TempDir()`; l'archivio si legge con `tar` + zstd e contiene `manifest.json` + tutti i file; un symlink ciclico non causa ricorsione; (Linux) una FIFO viene saltata senza bloccare.
- [x] Task 8: `archive.Extract(ctx, archivePath, destDir, onProgress)` con protezione da path traversal (`..`, path assoluti, symlink in uscita) e avanzamento sui byte compressi letti; su Linux i file estratti prendono uid/gid della cartella di destinazione, non quelli del tar né root (F11) — DoD: test round-trip create→extract identico; un archivio malevolo con `../x` o con symlink che esce dalla destinazione viene rifiutato.
- [x] Task 9: Annulla in Create con cancellazione del file parziale + `archive.Estimate(usages []uint64)` = somma dello spazio usato delle partizioni (fattore prudenziale 1.0; lo spazio usato arriva da `mount.Usage`, non da `lsblk FSUSED` — F6) — DoD: test: annulla a metà → nessun file `.tar.zst` residuo; la stima restituisce la somma.

## Fase 2 — Linux

- [x] Task 10: `disk.List()` Linux tramite `lsblk -J -b …` → `[]Disk` con partizioni, FS, mountpoint, flag rimovibile/USB; colonna `MOUNTPOINTS` con fallback a `MOUNTPOINT` per util-linux < 2.37; `FSUSED` non usato (F6) — DoD: test di parsing su 3 fixture JSON (USB singola, NVMe con 3 partizioni, disco con LVM/crypto marcato come non supportato).
- [~] Task 11: Rilevamento del disco di sistema su Linux (padre di `/`, `/boot`, `/boot/efi`, swap) — DoD: test su fixture; sulla macchina reale il disco di sistema risulta `IsSystem=true`.
- [~] Task 12: `rawdev.Open` Linux (read-only / write, dimensione via `BLKGETSIZE64`, `fsync` alla chiusura, `DropCache()` = `fsync` + `ioctl(BLKFLSBUF)` + `posix_fadvise(DONTNEED)` — F4) — DoD: test di integrazione (build tag `integration`, root) con due loop device da 256 MiB: clone + verifica OK; dopo `DropCache` una corruzione scritta direttamente sul file di backing del loop viene rilevata dalla verifica.
- [~] Task 12b: `disk.PrepareForWrite` / `disk.FinishWrite` Linux: smontaggio automatico (`umount`) di tutte le partizioni montate di origine e destinazione dopo la conferma; a fine scrittura `ioctl(BLKRRPART)` sulla destinazione (F3) — DoD: test di integrazione: con una partizione del loop montata la clonazione procede dopo lo smontaggio; dopo la clonazione `lsblk` mostra le partizioni nuove senza riconnettere.
- [~] Task 13: `mount.ReadOnly(partition) (path, cleanup)` Linux con `mount -o ro` in una cartella temporanea + `umount` nel cleanup; se la partizione è **già montata** (automount udisks) riusa il mountpoint e il cleanup non smonta (F3); `mount.Usage(path)` via `statfs` (F6) — DoD: test di integrazione su loop con ext4 e vfat: file leggibili, dopo il cleanup la cartella è smontata e rimossa; con partizione già montata il mountpoint originale resta intatto; `Usage` coincide con `df`.

## Fase 3 — Windows

- [x] Task 14: `disk.List()` Windows: enumerazione di `\\.\PhysicalDriveN` con modello, seriale, bus (USB/SATA/NVMe), dimensione — DoD: sulla macchina di sviluppo l'elenco coincide con "Gestione disco" (dimensioni e modelli).
- [x] Task 15: Partizioni e volumi Windows (layout + mappa volume→disco via disk extents, FS e etichetta via `GetVolumeInformation`, lettera se presente) — DoD: per ogni disco le partizioni e i FS coincidono con "Gestione disco"; le partizioni senza FS riconosciuto sono marcate come non supportate.
- [x] Task 16: Rilevamento del disco di sistema Windows (volume di `%SystemDrive%`) — DoD: sulla macchina di sviluppo il disco con `C:` risulta `IsSystem=true`.
- [x] Task 17: `rawdev.Open` Windows: `CreateFile` su `\\.\PhysicalDriveN`, dimensione via `IOCTL_DISK_GET_LENGTH_INFO`, in scrittura lock + dismount di **tutti** i volumi del disco, `FlushFileBuffers` alla chiusura; `DropCache()` = no-op (handle raw non in cache), lettura di verifica con `FILE_FLAG_NO_BUFFERING` (F4) — DoD: l'apertura in scrittura di un disco con volume montato riesce dopo il lock e fallisce con errore chiaro se il lock è negato.
- [x] Task 17b: `disk.PrepareForWrite` / `disk.FinishWrite` Windows: disco destinazione messo **offline** (`IOCTL_DISK_SET_DISK_ATTRIBUTES`, `DISK_ATTRIBUTE_OFFLINE`) prima della scrittura e **lasciato offline** a fine clonazione (F1, F2); la clonazione usa `HeadLast` (Task 5) — DoD: durante la copia nessun nuovo volume appare in "Gestione disco"; a fine copia il disco destinazione risulta offline.
- [x] Task 18: Test di integrazione Windows con due VHD da 256 MiB collegati (script `diskpart` in `scripts/`); il VHD sorgente ha tabella **GPT con 3 partizioni** (NTFS, FAT32, una senza lettera) — DoD: clone + verifica tra i due VHD OK senza `ACCESS_DENIED` (F1); nessun disco reale toccato.
- [x] Task 19: `mount.ReadOnly` Windows: spike `\\?\Volume{GUID}\` vs mount point temporaneo, poi implementazione della soluzione funzionante — DoD: su un VHD con partizione NTFS senza lettera, `filepath.WalkDir` elenca i file; nessuna lettera lasciata assegnata dopo il cleanup; `mount.Usage(path)` via `GetDiskFreeSpaceEx` coincide con "Gestione disco" (F6).

## Fase 4 — Livello applicativo

- [x] Task 20: `privilege.IsElevated()` (token amministratore su Windows, `euid==0` su Linux) — DoD: restituisce false/true eseguendo l'app senza e con privilegi.
- [x] Task 21: `validate` (origine≠destinazione, destinazione non di sistema, destinazione ≥ origine, spazio libero ≥ stima, archivio stimato > 4 GiB − 1 su destinazione FAT32 bloccato (F5), almeno una partizione selezionata, flag `DISKCLONE_DEV_SAFE` = solo dischi rimovibili) con codici di errore; le regole sono richiamate **anche** nei binding Go prima di avviare ogni job, non solo nella UI (F10) — DoD: test unitari per ogni regola. **Nota impl.:** dev-safe filtra per bus (usb/loop/virtual) invece del flag rimovibile, perche Windows segnala come rimovibile anche l'SSD SATA di sistema; aggiunta regola dest_on_source (archivio salvato dentro una partizione selezionata).
- [x] Task 22: `job.Manager` (un job alla volta, `Cancel()`, stati idle/running/done/failed/canceled, eventi `job:progress`/`job:done` via Wails runtime) — DoD: test con job finto: un secondo avvio durante un job è rifiutato; annulla porta allo stato `canceled`.
- [x] Task 23: Binding in `app.go`: `ListDisks`, `IsElevated`, `StartClone`, `StartArchive`, `StartRestore`, `Cancel`, `PickSaveFile`, `PickArchive`, `PickFolder`, `GetConfig`/`SetLanguage`; ogni `Start*` ri-valida con `validate`; frontend solo con asset embedded, CSP restrittiva, link esterni via `BrowserOpenURL`, devtools disattivati in build (F10) — DoD: `wails dev` genera i binding TS; chiamata a `ListDisks` dal frontend restituisce i dischi reali.

## Fase 5 — UI (Svelte)

- [x] Task 24: Struttura con due tab, i18n (`it.json`/`en.json` + store), banner se non elevato — DoD: i tab funzionano; senza privilegi appare il banner tradotto.
- [x] Task 25: Tab Backup: selettore tipo, lista dischi origine con "Aggiorna"; per Unità→Unità lista destinazioni filtrata (no origine, no disco di sistema) + checkbox "Verifica dopo la copia" attiva — DoD: i filtri si aggiornano cambiando l'origine; i dischi non selezionabili sono disabilitati con motivo.
- [~] Task 26: Tab Backup Unità→File: partizioni con checkbox (non supportate disabilitate), scelta cartella e nome `.tar.zst`, spazio libero vs stima (calcolata montando le partizioni in sola lettura alla selezione), avviso se la destinazione è FAT32 — DoD: "Avvia" è disabilitato se nessuna partizione è selezionata, se lo spazio è insufficiente o se l'archivio stimato supera 4 GiB su FAT32.
- [~] Task 27: Dialog di conferma con riepilogo (origine → destinazione, modello, dimensione) e, per la clonazione, conferma esplicita (digitare il numero del disco o una checkbox "Ho capito che i dati verranno cancellati") — DoD: senza conferma esplicita il job non parte.
- [~] Task 28: Pannello avanzamento (%, byte, velocità, ETA, fase "copia/verifica"), "Annulla", esito finale con avvisi; a fine clonazione su Windows: "Il disco clonato è offline: scollegalo prima di riavviare e non portarlo online su questo PC" (F2); se sorgente GPT e destinazione più grande: avviso sul GPT di backup (F12) — DoD: con un job reale su loop/VHD i valori si aggiornano e l'annulla mostra "Operazione annullata — destinazione incompleta".
- [~] Task 29: Tab Ripristino: selezione archivio, lettura del `manifest.json` (mostra data/partizioni), selezione cartella di destinazione, conferma, avanzamento — DoD: il ripristino di un archivio creato al Task 7 produce i file attesi.
- [x] Task 30: Selettore lingua persistito in `config.json` — DoD: cambiando lingua e riavviando l'app la lingua resta quella scelta.
- [x] Task 31: Traduzioni complete IT/EN di tutti i testi e codici di errore — DoD: uno script/test verifica che `it.json` e `en.json` abbiano le stesse chiavi e che nessuna chiave usata nel codice manchi.

## Fase 6 — Pacchetti

- [~] Task 32: Build Windows (`wails build -platform windows/amd64`) con manifest `requireAdministrator` e controllo WebView2 — DoD: l'exe su una macchina Windows pulita chiede UAC e si avvia.
- [~] Task 33: Build Linux (in WSL2 o su Linux) con `wails build -tags webkit2_41` (F8) + `diskclone-launch.sh` (pkexec con variabili d'ambiente), `.desktop`, policy polkit — DoD: dal menu applicazioni l'app chiede la password e si apre come root.
- [~] Task 34: Test dell'avvio Linux su GNOME Wayland e su sessione X11; applicazione del fallback (`GDK_BACKEND=x11`/`xhost`) se necessario — DoD: l'app si apre in entrambe le sessioni; procedura documentata nel README.

## Fase 7 — Collaudo

- [ ] Task 35: Eseguire CA1–CA10 della spec con chiavette USB reali su Windows e Linux; per CA4 su Windows verificare e annotare quale strumento apre il `.tar.zst` (7-Zip recente / 7-Zip-zstd / `tar.exe`) e correggere CA4 di conseguenza (F9); registrare gli esiti in `.claude/test-report.md` — DoD: tutti i CA superati o con difetti aperti come nuovi task.

## Fase 8 — Correzioni dalla review (`.claude/review.md`)

- [x] Task 36 (R1): `Extract` rifiuta entry il cui percorso passa per un symlink e symlink il cui target attraversa un altro symlink; un file esistente che è un symlink viene sostituito, non seguito â€” DoD: test di regressione `y â†’ .`, `x â†’ y/..`, `x/evil.txt` senza fughe; file sotto un symlink rifiutato con `ErrUnsafePath`.
- [x] Task 37 (R2): Windows: disco della system partition EFI (`HKLM\SYSTEM\Setup\SystemPartition`) marcato `IsSystem` — DoD: test che risolve la system partition a un disco marcato di sistema.
- [x] Task 38 (R3): file che si accorcia o con errore di lettura durante il backup: completato con zeri + avviso, il job continua — DoD: test con lettura corta e con errore di lettura simulati.
- [x] Task 39 (R4+R6): scansione preliminare dei file → `ContentBytes` nel manifest (dimensioni apparenti), usato come totale dell'avanzamento e come spazio necessario al ripristino — DoD: test: avanzamento al 100% esatto; `needed` = somma delle dimensioni apparenti.
- [x] Task 40 (R5): Windows: notice "destinazione offline" anche quando la clonazione fallisce o viene annullata dopo `PrepareForWrite` — DoD: notice tradotta (check:i18n) e presente nel risultato su errore.
- [x] Task 41 (R7): cache dello spazio usato per partizione (invalidata da `ListDisks`), riusata da `StartArchive` â€” DoD: test: seconda stima non rimonta le partizioni già misurate.
- [x] Task 42 (R8): ripristino senza requisito di privilegi anche nel frontend — DoD: `RestoreTab` non dipende da `elevated`; `npm run check` verde.

## Fase 9 — Revisione 2: Unità → Immagine al posto del file a file

> Superati dalla revisione 2 (codice rimosso, resta nella cronologia git): Task 7, 8, 9, 13 (solo `ReadOnly`), 19, 26, 29, 36, 38, 39, 41, 42 e i finding S1–S4 del secondo giro di review (tutti relativi al file a file).

- [x] Task 43: Rimuovere il file a file: pacchetto `internal/archive`, `mount.ReadOnly`, stima/cache dello spazio usato, partizioni e ripristino in cartella nella UI, relativi test e traduzioni — DoD: `go build`, `go vet` (Windows e Linux), `go test`, `npm run check` verdi; nessun riferimento a `tar`/`archive` nel codice.
- [x] Task 44: Pacchetto `internal/image`: creazione `.img.zst` (frame skippable di intestazione, dati zstd tramite `clone.Copy`, chiusura con SHA-256), lettura delle informazioni, lettore sequenziale per il ripristino, verifica del file — DoD: test round trip, immagine alterata rilevata, file non valido rifiutato, annullamento, compatibilità 7-Zip-zstd.
- [x] Task 45: `disk.PrepareForRead` (blocca/smonta solo l'origine) per la creazione dell'immagine — DoD: vet Windows/Linux; coperto dai test di integrazione.
- [x] Task 46: Regole `validate.Image` (spazio ≥ dimensione disco, FAT32 > 4 GiB, file su partizione dell'origine, disco di sistema come origine) e `validate.RestoreImage` (destinazione ≥ immagine, non di sistema, modalità sicura) — DoD: test unitari per ogni regola.
- [x] Task 47: Binding e job: `StartImage`, `ReadImageInfo`, `StartRestoreImage`, notice offline/GPT — DoD: test dei job con dispositivi simulati da file.
- [x] Task 48: UI: modalità Unità → Immagine nel tab Backup, tab Ripristino immagine → unità, traduzioni — DoD: `npm run check`, `check:i18n` verdi; verifica a schermo.
- [~] Task 49: Test di integrazione immagine (VHD Windows, loop Linux) — DoD: scritti e compilano; esecuzione da parte dell'utente.
- [ ] Task 50: Aggiornare README, guida di installazione e test report — DoD: nessun riferimento al file a file.

