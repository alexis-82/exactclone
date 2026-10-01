# Test report — Disk Clone

Data: 2026-10-01 · Macchina: Windows 10 Pro 22H2, i7-6700K, shell **non amministratore**, nessun Linux/WSL disponibile.

## Esito suite

| Suite | Comando | Esito |
|---|---|---|
| Test unitari + behavior Go (Windows) | `go test ./...` | **verde** — 9 pacchetti |
| Compilazione test per Linux | `GOOS=linux go test -c` (tutti i pacchetti + `itest` con tag `integration`) | **compila** (non eseguibili qui) |
| Type-check frontend | `npm run check` | **0 errori, 0 warning** |
| Traduzioni IT/EN + codici errore backend | `npm run check:i18n` | **OK, 99 chiavi** |
| Integrazione Windows (VHD) | `go test -tags integration ./internal/itest/` | **skip** — serve shell amministratore |
| Integrazione Linux (loop device) | `sudo go test -tags integration ./internal/itest/` | **non eseguita** — serve Linux + root |

Race detector (`-race`) non disponibile: richiede CGO/gcc, assente su questa macchina.

## Test aggiunti in questa fase

- `acceptance_test.go` (behavior test a livello applicazione, partizioni simulate con cartelle):
  - `TestCA3ArchiveAndRestore` — backup di una partizione "NTFS" e una "FAT32" tramite job manager → manifest, una cartella per partizione, avanzamento al 100%, ripristino con dimensioni e SHA-256 identici (inclusi file vuoti, nomi con spazi e accenti).
  - `TestCA4StandardTools` — apertura dell'archivio con 7-Zip o `tar --zstd`, solo se lo strumento supporta zstd.
  - `TestCA7CancelArchive` — annulla durante l'archiviazione: stato `canceled` entro 5 s, archivio parziale cancellato.
  - `TestCA7CancelRestore` — annulla durante il ripristino: stato `canceled`.
  - `TestCA8RefusedWithoutPrivileges` — `StartClone`/`StartArchive` senza privilegi → `not_elevated` prima di toccare i dischi.
  - `TestRestoreRejectsForeignFile` — file che non è un archivio Disk Clone → `not_diskclone_archive`.
- `internal/archive/archive_test.go`: `TestLongPaths` — percorsi oltre 100 byte (limite dell'header tar classico).

### Verifica del valore dei test (mutation check manuale)

| Mutazione | Test atteso | Risultato |
|---|---|---|
| Rimossa la cancellazione dell'archivio parziale in `CreateFile` | `TestCA7CancelArchive` | **fallisce** ✔ |
| Disattivato il controllo esplicito dei componenti `..` in `safeJoin` | `TestExtractRejectsUnsafePaths` | passa: il secondo controllo (`insideDir`) blocca comunque tutti i casi → protezione ridondante, non test vuoto |

## Copertura dei criteri di accettazione

| CA | Copertura | Stato |
|---|---|---|
| **CA1** clone + SHA-256 + "Verifica OK" | Unit: `clone` (copia 0 B…100 MiB, `HeadLast`, errori, verify + mismatch). Integrazione: `TestCloneAndVerifyLoop` (Linux, include corruzione rilevata dopo drop cache), `TestCloneGPTWithMountedVolumes` (Windows VHD). UI: messaggio `result.verifyOk`. | **Parziale** — logica verificata; integrazione su device da eseguire (admin / Linux); chiavette reali in T35 |
| **CA2** clone avviabile | Solo manuale (T35). | **Non verificato** |
| **CA3** archivio NTFS+FAT32 (+ext4) e ripristino | Behavior: `TestCA3ArchiveAndRestore`. Unit: round trip, symlink/junction, FIFO (Linux). Integrazione: `TestArchiveVolumeWithoutLetter` (VHD), `TestMountReadOnly` ext4/vfat (Linux). | **Parziale** — verde su cartelle; partizioni reali da eseguire |
| **CA4** apribile con strumenti standard | `TestCA4StandardTools`: estrazione con 7-Zip-zstd 26.02 (`C:\Program Files\7-Zip-Zstandard\7z.exe`), file identici per dimensione e SHA-256, `manifest.json` visibile. | **Verde su Windows** (con 7-Zip-zstd, vedi D1); Linux `tar --zstd` da eseguire |
| **CA5** blocco destinazione piccola / spazio insufficiente | Unit: `validate` (`dest_too_small`, `insufficient_space`, `fat32_limit`); UI verificata a schermo (dischi troppo piccoli disabilitati). | **Verde** |
| **CA6** no stessa unità, no disco di sistema | Unit: `validate`; UI verificata a schermo (origine e disco di sistema disabilitati come destinazione, disco di sistema non selezionabile come origine). | **Verde** |
| **CA7** annulla entro pochi secondi, destinazione incompleta, archivio parziale cancellato | Unit: `TestCopyCancel`, `TestVerifyCanceled`, `TestCreateFileCancelRemovesPartial`, `TestJobCancel`. Behavior: `TestCA7CancelArchive`, `TestCA7CancelRestore`. | **Verde** (clone su device reale da confermare) |
| **CA8** UAC / pkexec, segnalazione senza privilegi | Manifest `requireAdministrator` presente nell'exe (verificato sul binario); `TestCA8RefusedWithoutPrivileges`; banner UI verificato a schermo; `TestIsElevated`. pkexec non provato. | **Parziale** — Linux da verificare (T33/T34) |
| **CA9** eseguibile su macchina pulita | Windows: exe unico 12 MB. Nessuna prova su macchina pulita; build Linux non eseguita. | **Non verificato** |
| **CA10** cambio lingua IT↔EN | `check:i18n` (chiavi identiche, nessuna chiave mancante, codici errore/notice del backend tradotti); verifica a schermo del cambio lingua e della persistenza dopo riavvio. | **Verde** |

## Difetti / problemi aperti

### D1 — CA4: su questo Windows nessuno strumento standard apre il `.tar.zst` — **RISOLTO (opzione a)**

Aggiornamento: installato [7-Zip-zstd](https://github.com/mcmilk/7-Zip-zstd/releases) 26.02; `TestCA4StandardTools` ora **passa**. Il test cerca la prima installazione di 7-Zip con codec zstd (anche in `C:\Program Files\7-Zip-Zstandard\`). CA4 nella spec aggiornato per indicare 7-Zip-zstd; link aggiunto a `README.md` e `docs/INSTALLAZIONE.md`.

Situazione iniziale:
- 7-Zip installato: **22.01** (2022), elenco formati senza zstd → `7z x backup.tar.zst` esce con codice 2.
- `tar.exe` di Windows 10: bsdtar 3.5.2 compilato **senza** libzstd.
- `tar` di Git for Windows: GNU tar, richiede il programma `zstd` esterno, non installato.
- L'archivio in sé è corretto (letto dal nostro codice e da `archive/tar` + decoder zstd), ma il CA4 così com'è scritto non è soddisfatto su Windows senza software aggiuntivo. Era il rischio F9 dell'analisi.
- **Da decidere** (non risolto in questa fase): (a) indicare nel CA4 e nella guida lo strumento richiesto (7-Zip-zstd, oppure `zstd` + tar) e rieseguire il test dopo averlo installato; (b) cambiare formato (es. zip) — tocca la decisione D4 della spec, va riportato a `/clarify`.

### D2 — Test di integrazione non eseguiti
`TestCloneGPTWithMountedVolumes` è quello che verifica davvero la mitigazione del blocker F1 (disco offline + `HeadLast` contro `ACCESS_DENIED`) e il comportamento offline del clone (F2). Finché non gira da shell amministratore, F1/F2 restano **non confermati**.

### Nessun test flaky osservato
Test di annullamento del motore di copia ripetuti 20 volte in fase di implementazione senza fallimenti.

## Come completare la verifica

1. PowerShell **amministratore**, nella cartella del progetto:
   ```powershell
   go test -tags integration -count=1 -v ./internal/itest/
   $env:DISKCLONE_EXPECT_ELEVATED = "1"; go test -count=1 ./internal/privilege/
   ```
2. Linux: `sudo env "PATH=$PATH" go test -tags integration -count=1 -v ./internal/itest/` e `go test ./...` (esegue anche il test FIFO).
3. CA4: installare uno strumento con supporto zstd e rieseguire `go test -run CA4 -v .`.
4. CA1, CA2, CA9 e le parti manuali di CA3/CA8: collaudo T35 con chiavette reali.
