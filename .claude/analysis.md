# Analisi critica — Disk Clone Tool

Input analizzati: `.claude/spec.md`, `.claude/plan.md`, `.claude/tasks.md`. Il progetto è vuoto (nessun codice esistente) → l'asse **Impatto/regressioni** non ha rischi sul codice esistente; i rischi d'impatto riguardano i **dischi dell'utente**.

Riepilogo: **1 blocker, 6 major, 6 minor**.

---

## Blocker

### F1 — Windows: il disco destinazione viene rimontato a metà copia → scrittura negata
- **Severità**: blocker
- **Dove**: `tasks.md:30` (Task 17), `plan.md` riga "Fine scrittura"
- **Scenario**: Task 17 blocca e smonta i volumi del disco destinazione, poi la copia scrive per primi i settori 0–N (MBR/GPT della sorgente). Il Partition Manager di Windows vede la nuova tabella delle partizioni, crea nuovi volumi e li monta. I volumi nuovi **non** sono bloccati dal nostro handle → le scritture successive nelle loro aree falliscono con `ERROR_ACCESS_DENIED` a pochi MB dall'inizio. CA1 e CA2 falliscono su Windows.
- **Mitigazione proposta**: durante la scrittura mettere il disco **offline** (`IOCTL_DISK_SET_DISK_ATTRIBUTES` con `DISK_ATTRIBUTE_OFFLINE`, non persistente) oltre a lock e dismount, oppure scrivere il primo 1 MiB **per ultimo** (prima la copia da 1 MiB in poi, infine la tabella delle partizioni). Prevedere un test esplicito in Task 18 con sorgente GPT a più partizioni.

---

## Major

### F2 — Windows: collisione della firma del disco → il clone non si avvia
- **Severità**: major
- **Dove**: `spec.md:51` (CA2), `tasks.md:30` (Task 17)
- **Scenario**: si clona un disco Windows MBR mentre sorgente e destinazione sono collegati alla stessa macchina Windows. Al termine Windows rileva due dischi con la stessa disk signature, mette il clone offline e, se l'utente lo porta online, **gli assegna una nuova firma** → il BCD del clone punta alla firma vecchia e il clone non si avvia. CA2 fallisce. (Con GPT il problema è analogo per i GUID dei dischi e delle partizioni.)
- **Mitigazione proposta**: lasciare il clone offline alla fine della copia (collegato a F1) e mostrare nel riepilogo finale: "Scollega il disco clonato prima di riavviare / non portarlo online su questo PC". Aggiungere la condizione a CA2: "senza portare il clone online sulla macchina che lo ha creato".

### F3 — Linux: automount del desktop e tabella delle partizioni non riletta
- **Severità**: major
- **Dove**: `tasks.md:22` (Task 12), `tasks.md:23` (Task 13)
- **Scenario A**: l'utente collega la chiavetta USB su Ubuntu/Fedora; udisks2 la **monta in automatico** in `/media/<user>/…`. Task 12 rifiuta l'operazione perché una partizione è montata → l'utente non sa cosa fare (la spec parte dal presupposto che sia "completamente smontata", ma sui desktop Linux non è così di default).
- **Scenario B**: Task 13 esegue `mount -o ro` su una partizione già montata in lettura-scrittura da udisks → errore `already mounted` o mount incoerente.
- **Scenario C**: dopo la clonazione il kernel non rilegge la tabella delle partizioni della destinazione → le nuove partizioni non compaiono finché non si scollega la chiavetta; un "Aggiorna" subito dopo mostra dati vecchi.
- **Mitigazione proposta**: (A) per la clonazione, smontare automaticamente le partizioni del disco destinazione e sorgente dopo conferma (`umount`) invece di rifiutare; (B) in `mount.ReadOnly`, se la partizione è già montata riusare il mountpoint esistente senza smontarlo nel cleanup; (C) dopo la scrittura chiamare `ioctl(BLKRRPART)`.

### F4 — Linux: la verifica legge dalla page cache, non dal disco
- **Severità**: major
- **Dove**: `plan.md:51`, `tasks.md:13` (Task 6)
- **Scenario**: dopo la copia di una chiavetta da 8 GB su una macchina con 32 GB di RAM, la passata di verifica rilegge la destinazione con `read()` normali: gran parte dei blocchi è ancora nella page cache → l'hash coincide anche se i dati sul flash sono corrotti (chiavetta contraffatta/difettosa). La "Verifica OK" di CA1 dà una falsa garanzia.
- **Mitigazione proposta**: dopo `fsync`, eseguire `ioctl(BLKFLSBUF)` / `posix_fadvise(POSIX_FADV_DONTNEED)` sul device, oppure leggere la destinazione con `O_DIRECT` e buffer allineati. (Su Windows gli handle `\\.\PhysicalDriveN` non passano dalla cache del file system, ma usare comunque `FILE_FLAG_NO_BUFFERING` in lettura per la verifica.)

### F5 — Archivio su destinazione FAT32 oltre i 4 GiB
- **Severità**: major
- **Dove**: `tasks.md:16` (Task 9), `tasks.md:37` (Task 21), `tasks.md:45` (Task 26)
- **Scenario**: l'utente salva l'archivio su una chiavetta/HDD esterno formattato FAT32 (molto comune). Spazio libero sufficiente → la validazione passa; a 4 GiB la scrittura fallisce con `file too large` dopo, ad esempio, 20 minuti di lavoro.
- **Mitigazione proposta**: in `validate`, se il FS di destinazione è FAT32 e la stima supera 4 GiB − 1, bloccare l'avvio con un messaggio chiaro (lo split in volumi resta fuori scope). Aggiungere la regola ai test del Task 21.

### F6 — La stima dello spazio usa `FSUSED`, che lsblk riporta solo per FS montati
- **Severità**: major
- **Dove**: `plan.md:53`, `tasks.md:16` (Task 9), `tasks.md:45` (Task 26)
- **Scenario**: partizione NTFS non montata su Linux → `lsblk` restituisce `fsused: null` → stima = 0 → il controllo "spazio libero ≥ stima" passa sempre; l'archivio si interrompe per disco pieno. Su Windows le partizioni senza lettera hanno lo stesso problema se la stima viene presa solo dalle lettere.
- **Mitigazione proposta**: calcolare la stima dopo il mount in sola lettura con `statfs` (Linux) / `GetDiskFreeSpaceEx` sul path del volume (Windows): `usato = totale − libero`. Quindi la stima va fatta al momento della selezione delle partizioni (montaggio anticipato) o all'avvio del job come prima fase.

### F7 — Attraversamento del FS sorgente: symlink, junction e file speciali non specificati
- **Severità**: major
- **Dove**: `tasks.md:14` (Task 7)
- **Scenario A (Windows)**: la partizione sorgente è un disco di sistema Windows; la cartella `Documents and Settings` e le junction in `%USERPROFILE%` (es. `Application Data`) puntano a cartelle dello stesso volume. Se il walk le segue → contenuti duplicati o cicli infiniti (`Application Data\Application Data\…`) fino al riempimento della destinazione.
- **Scenario B (Linux)**: una partizione ext4 contiene socket, FIFO o device node; `os.Open` su una FIFO **blocca per sempre** → il job resta appeso e "Annulla" non funziona (la chiamata bloccante non osserva il context).
- **Mitigazione proposta**: nel Task 7 specificare: non seguire symlink/junction/reparse point (salvarli come symlink nel tar o saltarli con avviso), saltare con avviso i file non regolari (FIFO, socket, device), gestire i hardlink come `TypeLink`. Aggiungere al DoD un test con symlink ciclico e una FIFO (Linux).

---

## Minor

### F8 — Build Linux: Wails v2 usa WebKitGTK 4.0 di default
- **Dove**: `tasks.md:55` (Task 33)
- **Scenario**: build su Ubuntu 24.04 o Debian 13: il pacchetto `libwebkit2gtk-4.0-dev` non esiste più → la build fallisce o il binario non parte sulle distro recenti.
- **Mitigazione**: compilare con il build tag `webkit2_41` (`wails build -tags webkit2_41`) e documentare la dipendenza runtime `libwebkit2gtk-4.1-0`.

### F9 — CA4: 7-Zip "standard" potrebbe non aprire `.tar.zst`
- **Dove**: `spec.md:53` (CA4)
- **Scenario**: un utente Windows prova ad aprire l'archivio con una versione di 7-Zip senza supporto zstd → "formato non supportato"; CA4 risulta fallito anche se l'archivio è corretto.
- **Mitigazione**: verificare nel Task 35 quale strumento Windows apre `.tar.zst` (7-Zip recente, fork 7-Zip-zstd, `tar.exe` di Windows) e correggere CA4 citando lo strumento effettivamente verificato.

### F10 — Intera WebView eseguita come root/amministratore
- **Dove**: `spec.md:84` (D9), `plan.md:62`
- **Scenario**: in una build di produzione resta attivo il devtools / il menu contestuale "Ispeziona", oppure un link nel frontend apre contenuto remoto nella WebView → codice JS di terzi eseguito con privilegi di root, con accesso ai binding `StartClone`.
- **Mitigazione**: nessun contenuto remoto (asset solo embedded, CSP restrittiva), devtools disattivati in `wails build` (default), link esterni aperti con `BrowserOpenURL` e non nella WebView. Il backend ri-valida **sempre** gli input dei binding (Task 21 lato Go, non solo UI).

### F11 — Ripristino come root: proprietario dei file e permessi
- **Dove**: `tasks.md:15` (Task 8), `tasks.md:48` (Task 29)
- **Scenario**: su Linux l'app gira come root; l'utente ripristina in `/home/mario/restore` → tutti i file estratti appartengono a `root:root` e l'utente non può modificarli/cancellarli. Viceversa, applicare uid/gid del tar ripristina proprietari di un altro sistema che possono non esistere.
- **Mitigazione**: decidere esplicitamente: assegnare i file al proprietario della cartella di destinazione (`chown` ricorsivo all'uid/gid della cartella scelta). Annotarlo nel Task 8.

### F12 — Clone GPT su disco più grande: GPT di backup non in fondo al disco
- **Dove**: `spec.md:51` (CA2)
- **Scenario**: si clona un disco GPT da 128 GB su uno da 256 GB; il GPT secondario resta a metà disco → Linux/`gdisk` segnalano "GPT corrotta / backup header non alla fine"; lo spazio extra non è utilizzabile senza riparazione. Non è un errore dell'app ma l'utente lo percepirà come tale.
- **Mitigazione**: messaggio informativo a fine clonazione quando destinazione > origine e la sorgente è GPT. La riparazione automatica resta fuori scope (Non-Goal "ridimensionamento").

### F13 — SHA-256 nella goroutine di lettura può limitare la velocità
- **Dove**: `plan.md` riga "Copia bit a bit"
- **Scenario**: su CPU senza estensioni SHA (molti Intel pre-2019) Go fa SHA-256 a ~400–500 MB/s; con un SSD NVMe in box USB 3.2 Gen2 (~900 MB/s) l'hash calcolato nella goroutine di lettura dimezza la velocità della copia.
- **Mitigazione**: calcolare l'hash in una terza goroutine della pipeline (reader → [writer, hasher]), riusando lo stesso buffer finché entrambi non hanno finito.

---

## Verifica di copertura spec → task

| Criterio | Coperto da | Note |
|---|---|---|
| CA1 | T5, T6, T12, T17, T18 | Vedi F1, F4 |
| CA2 | T35 (manuale) | Vedi F1, F2, F12 |
| CA3 | T7, T8, T13, T19, T29 | Vedi F7 |
| CA4 | T7, T35 | Vedi F9 |
| CA5 | T21, T25, T26 | Vedi F5, F6 |
| CA6 | T11, T16, T21, T25 | OK |
| CA7 | T6, T9, T22, T28 | Vedi F7 scenario B (FIFO blocca l'annulla) |
| CA8 | T20, T32, T33, T34 | OK |
| CA9 | T32, T33 | Vedi F8 |
| CA10 | T24, T30, T31 | OK |

Nessun task viola i Non-Goals.

## Esito

**1 blocker (F1)** → secondo la procedura tornare a `/plan-tasks` per integrare le mitigazioni di F1–F7 nei task 6, 7, 8, 9, 12, 13, 17, 18, 21 (i minor possono essere integrati come note nei task esistenti).

---

## Stato mitigazioni (aggiornato dopo /plan-tasks)

Tutti i finding F1–F13 sono stati integrati in `plan.md` e `tasks.md` (nuovi task **12b** e **17b**; modificati i task 5, 6, 7, 8, 9, 10, 12, 13, 17, 18, 19, 21, 23, 26, 28, 33, 35). CA2 della spec aggiornato con la condizione di F2. Nessun blocker aperto.

## Verifica sul campo (integrazione Windows su VHD, shell amministratore)

- **F1 confermato mitigato**: nessun `ACCESS_DENIED` clonando una sorgente GPT a 3 partizioni su un disco con volume montato.
- **F2 confermato**: la destinazione resta offline dopo la clonazione.
- F3, F4 (Linux) e la parte Linux di F8 restano da verificare su Linux.
