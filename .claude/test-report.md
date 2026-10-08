# Test report — Disk Clone

> **Attenzione**: questo report si riferisce alla revisione 1 (con il *file a file*). Va rigenerato con `/test` per la revisione 2 (Unità → Immagine, ripristino immagine).

Ultimo aggiornamento: 2026-10-01, dopo le correzioni della review (R1–R8).
Macchina: Windows 10 Pro 22H2, i7-6700K, nessun Linux/WSL disponibile, 7-Zip-zstd 26.02 installato. Test di integrazione Windows eseguiti dall'utente in PowerShell **amministratore**.

## Esito suite

| Suite | Comando | Esito |
|---|---|---|
| Test Go (Windows) | `go test -count=3 ./...` | **verde**, 9 pacchetti, 73 test/subtest, **0 skip**, stabili su 3 ripetizioni |
| Compilazione test per Linux | `GOOS=linux go test -c` (tutti i pacchetti + `itest` con tag `integration`) | **compila** (non eseguibili qui) |
| Type-check frontend | `npm run check` | **0 errori, 0 warning** |
| Traduzioni + codici backend | `npm run check:i18n` | **OK, 101 chiavi** |
| Integrazione Windows (VHD) | `go test -tags integration -count=1 -v ./internal/itest/` (amministratore) | **verde**: `TestCloneGPTWithMountedVolumes` (11.8 s), `TestArchiveVolumeWithoutLetter` (4.0 s); VHD scollegati al termine |
| Integrazione Linux (loop device) | `sudo go test -tags integration ./internal/itest/` | **non eseguita** |

## Test aggiunti in questo giro

| Test | Copre |
|---|---|
| `TestExtractSymlinkChainDoesNotEscape`, `TestExtractRejectsWritingThroughSymlink`, `TestExtractKeepsLegitimateRelativeLinks`, `TestExtractReplacesExistingSymlinkInsteadOfFollowing` | R1 — nessuna scrittura attraverso symlink; i link relativi legittimi restano |
| `TestBootSystemPartitionDisk` | R2 — `SystemPartition` risolta alla partizione EFI (FAT32, 100 MB, disco 2) e marcata di sistema |
| `TestCreateSurvivesShrinkingAndUnreadableFiles` (shrink, ioerror) | R3 — file accorciato / errore di lettura: zeri + avviso, gli altri file intatti |
| `TestScanMatchesCreate`, `TestRestoreNeeded` | R4/R6 — totale = byte letti da `Create`, file sparse alla dimensione apparente, spazio di ripristino da `ContentBytes` |
| `TestCloneNotices` | R5 — avviso "destinazione offline" anche su errore/annullamento (Windows) |
| `TestEstimateCachesUsage` | R7 — una misura per partizione; disco diverso con lo stesso nome rimisurato |
| `TestCA7CancelWhileWritingArchive` | CA7 — annullamento **durante la scrittura** dell'archivio (dopo la fase "scan"): file parziale cancellato. Necessario perché con la nuova fase "scan" `TestCA7CancelArchive` annulla già durante la scansione |
| `TestCA3ArchiveAndRestore` (aggiornato) | R6 — avanzamento esattamente al 100% anche con stima del file system volutamente errata; prima fase = "scan"; `ContentBytes` nel manifest |

### Mutation check (valore dei test)

| Mutazione | Test | Esito |
|---|---|---|
| Disattivato `noLinkInParents` (R1) | `TestExtractRejectsWritingThroughSymlink` | fallisce ✔ |
| Disattivato il controllo "link attraverso link" (R1) | `TestExtractSymlinkChainDoesNotEscape` | fallisce ✔ |
| Disattivato il riempimento con zeri (R3) | `TestCreateSurvivesShrinkingAndUnreadableFiles` | fallisce ✔ |
| Archivio parziale non cancellato (CA7) | `TestCA7CancelWhileWritingArchive` | fallisce ✔ |
| Ignorato il disco della partizione di avvio (R2) | `TestBootSystemPartitionDisk` | **passa**: su questa macchina la partizione EFI è sullo stesso disco di C:, quindi il caso "EFI su altro disco" non è riproducibile qui. Il test verifica la risoluzione registro → volume EFI e lo dichiara nel log (`LIMIT: ...`) |
| (giro precedente) Rimozione cancellazione archivio parziale | `TestCA7CancelArchive` | fallisce ✔ |

## Copertura dei criteri di accettazione

| CA | Copertura | Stato |
|---|---|---|
| **CA1** clone + SHA-256 + "Verifica OK" | Unit `clone` (0 B…100 MiB, `HeadLast`, errori, verify/mismatch); **integrazione Windows verde**: clone VHD GPT a 3 partizioni (NTFS, FAT32, NTFS senza lettera + MSR) su VHD con volume FAT32 montato, verifica SHA-256 OK. | **Verde su VHD Windows**; Linux (loop) e chiavette reali (T35) da eseguire |
| **CA2** clone avviabile | Solo manuale (T35). Rischio disco di avvio Windows ridotto da R2. | **Non verificato** |
| **CA3** archivio NTFS+FAT32 (+ext4), ripristino identico | Behavior `TestCA3ArchiveAndRestore`; unit round trip, link, junction, FIFO (Linux), file illeggibili (R3); **integrazione Windows verde**: partizione NTFS senza lettera letta via GUID, archiviata e ripristinata, nessuna lettera lasciata assegnata. | **Verde** su cartelle e su volume VHD; ext4/loop Linux da eseguire |
| **CA4** apribile con strumenti standard | `TestCA4StandardTools` con 7-Zip-zstd 26.02. | **Verde su Windows**; `tar --zstd` su Linux da eseguire |
| **CA5** blocchi su destinazione piccola / spazio insufficiente | Unit `validate`; spazio di ripristino ora dalle dimensioni apparenti (R4); UI verificata a schermo. | **Verde** |
| **CA6** no stessa unità / no disco di sistema | Unit `validate`; disco di avvio EFI marcato di sistema (R2); UI verificata a schermo. | **Verde** (caso multi-disco di R2 non riproducibile qui) |
| **CA7** annulla rapido, destinazione incompleta, archivio parziale cancellato | `TestCopyCancel`, `TestVerifyCanceled`, `TestCreateFileCancelRemovesPartial`, `TestJobCancel`, `TestCA7CancelArchive`, `TestCA7CancelWhileWritingArchive`, `TestCA7CancelRestore`; avviso disco offline (R5). | **Verde** (clone su device reale da confermare) |
| **CA8** UAC / pkexec, segnalazione senza privilegi | Manifest nell'exe; `TestCA8RefusedWithoutPrivileges`; banner UI. pkexec non provato. | **Parziale** — Linux da verificare |
| **CA9** eseguibile su macchina pulita | Nessuna prova su macchina pulita; build Linux non eseguita. | **Non verificato** |
| **CA10** cambio lingua IT↔EN | `check:i18n` (101 chiavi, codici e notice del backend inclusi la nuova `dest_offline` e la fase `scan`); verifica a schermo. | **Verde** |

## Mitigazioni dell'analisi confermate dall'integrazione Windows

- **F1 (blocker)**: clone di una sorgente GPT a più partizioni su una destinazione con volume montato (`J:`) completato **senza `ACCESS_DENIED`** con lock + dismount + disco offline + `HeadLast`.
- **F2**: al termine il disco di destinazione risulta **offline** (verificato con `IOCTL_DISK_GET_DISK_ATTRIBUTES`).
- **T15**: con privilegi la tabella delle partizioni viene letta per intero (numeri 1–4, MSR senza volume marcata non supportata).
- **T19**: partizione senza lettera letta tramite `\\?\Volume{GUID}\`, `mount.Stat` coerente.

## Difetti

| # | Stato |
|---|---|
| D1 — nessuno strumento Windows apriva `.tar.zst` | **Risolto**: 7-Zip-zstd documentato in README/guida, CA4 aggiornato, test verde |
| Review R1–R8 | **Corretti** (vedi `.claude/review.md`) |

Nessun test flaky osservato (suite ripetuta 3 volte; test di annullamento CA7 ripetuti 5 volte).

## Ancora da fare per chiudere la verifica

1. ~~Windows, PowerShell amministratore: test di integrazione~~ — **fatto, verde**. Facoltativo: `$env:DISKCLONE_EXPECT_ELEVATED = "1"; go test -count=1 ./internal/privilege/` dalla stessa shell.
2. **Linux**: `go test ./...` (esegue il test FIFO e i test R1 con semantica POSIX dei symlink — il caso d'attacco reale), poi `sudo env "PATH=$PATH" go test -tags integration -count=1 -v ./internal/itest/`, `tar --zstd` per CA4.
3. **T35**: collaudo manuale CA1, CA2, CA9 e parti manuali di CA3/CA8 con chiavette reali.
