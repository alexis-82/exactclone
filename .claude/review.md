# Code review — Disk Clone

Ambito: tutto il codice dell'implementazione (progetto nuovo: `app.go`, `jobs.go`, `config.go`, `internal/**`, `frontend/src/**`, `build/linux/*`). Base: commit `e0ccc6a`.
Metodo: lettura del codice + verifiche mirate (probe temporaneo per R1, prova dell'interfaccia per un sospetto poi scartato). Nessuna modifica al codice in questa fase.

Riepilogo: **1 blocker, 3 major, 4 minor.**

| # | Severità | Asse | Titolo |
|---|---|---|---|
| R1 | blocker | Sicurezza | Ripristino su Linux: scrittura fuori dalla cartella passando per symlink creati dall'archivio |
| R2 | major | Correttezza | Windows: il disco con la partizione EFI di avvio non è marcato "di sistema" |
| R3 | major | Edge case | Un file che si accorcia durante il backup fa fallire l'intero archivio |
| R4 | major | Edge case | File sparse: stima e controllo spazio del ripristino non affidabili |
| R5 | minor | Correttezza/UX | Windows: clonazione annullata o fallita lascia la destinazione offline senza avviso |
| R6 | minor | Correttezza/UX | Avanzamento dell'archivio con totale incoerente |
| R7 | minor | Prestazioni | Le partizioni vengono montate due volte all'avvio del backup e a ogni modifica della selezione |
| R8 | minor | Manutenibilità | Requisito di privilegi del ripristino diverso fra frontend e backend |

---

## Blocker

### R1 — Ripristino su Linux: scrittura fuori dalla cartella passando per symlink creati dall'archivio
- **Locazione**: `internal/archive/extract.go:95` (creazione symlink) e `:87`/`:108` (scrittura di file e hard link nel percorso già validato); `checkLinkTarget` a `:155`.
- **Problema**: i controlli sono **lessicali**: `safeJoin` verifica il nome dell'entry e `checkLinkTarget` verifica il target di ogni link. Su Linux il kernel risolve `..` **dopo** aver seguito un symlink, quindi una catena di link singolarmente "interni" può puntare fuori.
- **Scenario** (archivio malevolo, ripristino eseguito come root):
  1. `y` → symlink a `.` → controllo: `dest/.` = `dest`, interno ✔
  2. `x` → symlink a `y/..` → controllo lessicale: `dest/y/..` = `dest`, interno ✔; risoluzione reale: `y` = `dest`, quindi `x` = **genitore di `dest`**
  3. `x/evil.txt` file regolare → `safeJoin` dà `dest/x/evil.txt`, interno ✔ → `os.OpenFile` segue `x` e scrive **`<genitore>/evil.txt`**.

  Allungando la catena (`x2` → `x/..`, ecc.) si risale fino a `/`: un archivio può scrivere file in qualunque percorso, come root.
- **Verifica**: probe eseguito su Windows → nessuna fuga (Windows normalizza `..` nei target in modo lessicale). Su Linux non eseguibile qui; la semantica POSIX di risoluzione dei percorsi rende lo scenario sfruttabile. Il probe va aggiunto come test di regressione (gira su entrambi gli OS).
- **Suggerimento**: non scrivere mai attraverso un symlink. Due misure complementari: (1) creare i symlink **alla fine**, dopo tutti i file e le cartelle (approccio di GNU tar con `--delay-directory-restore`/bsdtar `--secure`); (2) prima di creare ogni entry verificare con `Lstat` che nessun componente del percorso sotto `dest` sia un symlink.

## Major

### R2 — Windows: il disco con la partizione EFI di avvio non è marcato "di sistema"
- **Locazione**: `internal/disk/enum_windows.go:312` (`systemDiskNumbers`).
- **Problema**: è considerato di sistema solo il disco che contiene il volume di `%SystemDrive%` (C:). Su molte installazioni con più dischi la partizione EFI (boot manager) è su **un altro disco** (l'installer di Windows la mette sul disco 0).
- **Scenario**: Windows su disco 2 (C:), partizione EFI sul disco 0 insieme ai dati → il disco 0 risulta normale e selezionabile come destinazione → la clonazione sovrascrive la partizione EFI → **il PC non si avvia più**. Su Linux il caso è coperto (`/boot/efi` è fra i mountpoint di sistema).
- **Suggerimento**: marcare di sistema anche il disco della "system partition" di Windows (valore `SystemPartition` in `HKLM\SYSTEM\Setup`, es. `\Device\HarddiskVolume1`, mappato al disco tramite i disk extents) e, se presente, quello del volume del pagefile.

### R3 — Un file che si accorcia durante il backup fa fallire l'intero archivio
- **Locazione**: `internal/archive/archive.go:241-246` (`addFile`).
- **Problema**: l'header tar dichiara `fi.Size()`; se durante la lettura il file risulta più corto, `Create` restituisce errore e il job termina con `archive_failed`. Su Windows i volumi vengono letti "dal vivo" (non sono rimontati in sola lettura), e lo stesso vale su Linux quando la partizione era già montata in lettura-scrittura dall'automount.
- **Scenario**: backup di una partizione dati di 200 GiB; all'80% un programma tronca un file di log → errore "file shrank" → l'archivio parziale viene cancellato, si perdono ore di lavoro.
- **Suggerimento**: completare l'entry con zeri fino alla dimensione dichiarata e registrare un `Warning` per quel file, invece di interrompere il job.

### R4 — File sparse: stima e controllo spazio del ripristino non affidabili
- **Locazione**: `internal/archive/archive.go` (`addFile`: `tar.FileInfoHeader` senza supporto sparse), `app.go:210` (`needed += p.UsedBytes`), `internal/archive/extract.go:87` (`writeFile`).
- **Problema**: lo spazio necessario al ripristino è stimato dallo spazio **occupato** sul file system di origine, ma un file sparse viene archiviato ed estratto con la sua dimensione **apparente**.
- **Scenario**: partizione ext4 con un'immagine di VM sparse da 100 GiB che occupa 3 GiB; spazio usato totale 10 GiB. Ripristino su una chiavetta da 64 GiB: il controllo (64 ≥ 10) passa, l'estrazione scrive 100 GiB di dati e fallisce a metà per disco pieno. Inoltre l'avanzamento del backup supera il 100% (vedi R6).
- **Suggerimento**: salvare nel manifest la somma delle dimensioni apparenti dei file (calcolata durante `Create`) e usarla per il controllo del ripristino; in estrazione, saltare con `Seek` i blocchi di zeri per ricreare file sparse dove il file system lo consente.

## Minor

### R5 — Windows: clonazione annullata o fallita lascia la destinazione offline senza avviso
- **Locazione**: `jobs.go:36` (`PrepareForWrite` mette offline la destinazione) e `:84` (notice `clone_offline` solo in caso di successo).
- **Problema**: se l'utente annulla o la copia fallisce, il disco di destinazione resta offline e sparisce da Esplora risorse, ma il messaggio finale dice solo "destinazione incompleta".
- **Suggerimento**: aggiungere la notice `clone_offline` (o una variante "scollega e ricollega il disco") anche in caso di errore o annullamento, dopo che `PrepareForWrite` è riuscito; `job.Result` viene già inoltrato anche con errore.

### R6 — Avanzamento dell'archivio con totale incoerente
- **Locazione**: `jobs.go:111`.
- **Problema**: il totale della fase è lo spazio occupato dal file system (metadati, pagefile, cluster parzialmente pieni), mentre l'avanzamento conta i byte del contenuto dei file. La percentuale finisce sotto il 100% (spesso di molto su NTFS di sistema) o sopra (file sparse), e l'ETA è sbagliata.
- **Suggerimento**: totale = somma delle dimensioni dei file, ottenuta con una prima passata veloce di sola scansione dei metadati, oppure mostrare l'avanzamento come byte elaborati senza percentuale.

### R7 — Partizioni montate due volte all'avvio e a ogni modifica della selezione
- **Locazione**: `app.go:111` (`EstimateArchive`), `app.go:169` (richiamata da `StartArchive`), `frontend/src/lib/BackupTab.svelte:80/97` (richiamata a ogni checkbox).
- **Problema**: su Linux ogni chiamata esegue `mount`/`umount` per ogni partizione non montata; `StartArchive` ripete la stima e poi `runArchive` monta di nuovo. Con dischi USB lenti o NTFS (ntfs-3g) ogni montaggio costa secondi.
- **Suggerimento**: calcolare lo spazio usato una sola volta per partizione e riutilizzarlo (cache per ID nella sessione), oppure fare la stima all'interno del job come prima fase.

### R8 — Requisito di privilegi del ripristino diverso fra frontend e backend
- **Locazione**: `frontend/src/lib/RestoreTab.svelte:19` (`canStart` richiede `elevated`) vs `app.go:199` (`StartRestore` non lo richiede).
- **Problema**: la regola vive in due posti con valori diversi; chi modifica uno dei due non sa quale sia quello vero. Il ripristino in una cartella dell'utente non avrebbe bisogno di privilegi.
- **Suggerimento**: decidere la regola (consiglio: nessun privilegio richiesto per il ripristino) e allineare il frontend; idealmente il backend espone se un'operazione è consentita.

---

## Assi senza ulteriori finding

- **Prestazioni (motore di copia)**: pipeline reader → [writer, hasher] con buffer riciclati; nessuna allocazione nel ciclo caldo; eventi di avanzamento limitati a 4/s.
- **Sicurezza (frontend/launcher)**: CSP restrittiva, nessun contenuto remoto; i binding ri-validano lato Go; l'helper root riceve solo variabili di display e lingua, nessun `LD_PRELOAD`/`GTK_MODULES`.
- **Concorrenza**: un solo job alla volta (`job.Manager`), stato protetto da mutex, evento `done` emesso dopo l'aggiornamento dello stato.

## Sospetti verificati e scartati

- *L'interfaccia non calcola la stima se si sceglie l'origine prima di passare a "Unità → File"*: provato sull'app compilata → la stima compare correttamente.

## Fuori scope — segnalazioni

Nessuna: il progetto è nuovo e non c'è codice preesistente.

## Prossimo passo

Ci sono blocker e major → tornare a `/implement` per R1–R4 (R5–R8 si possono includere nello stesso giro), poi rilanciare `/test`. R1 va corretto **prima** di qualsiasi uso del ripristino su Linux.
