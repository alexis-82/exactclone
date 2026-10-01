# Specifica — Disk Clone Tool (GUI multipiattaforma)

> **Revisione 2 (2026-10-01)**: su richiesta dello sviluppatore la funzione *file a file* (archivio `.tar.zst` e ripristino in cartella) è stata **rimossa** e sostituita da **Unità → Immagine** bit a bit (`.img.zst`) e **Ripristino Immagine → Unità**. Le decisioni superate sono marcate nella tabella "Decisioni prese".

## Obiettivo

Offrire un'applicazione desktop con interfaccia grafica, multipiattaforma (Windows + Linux), per copiare bit a bit un'unità disco su un'altra unità o in un file immagine compresso, e per ripristinare tale immagine su un'unità, senza dover usare la riga di comando.

## Goals (cosa DEVE fare)

- [ ] G1 — **Tab "Backup"**: l'utente sceglie il tipo: *Unità → Unità* oppure *Unità → Immagine*. Entrambi sono **bit a bit**.
- [ ] G2 — **Unità → Unità**: copia l'intero disco sorgente sul disco destinazione, con verifica SHA-256 post-copia opzionale (checkbox attiva di default).
- [ ] G3 — **Unità → Immagine**: copia l'intero disco sorgente in un file immagine compresso `.img.zst` nella cartella e col nome scelti dall'utente; il file contiene dimensione, modello del disco di origine e SHA-256 dei dati; verifica post-creazione opzionale (attiva di default).
- [ ] G4 — **Tab "Ripristino"**: l'utente seleziona un'immagine `.img.zst` creata dall'app e un'unità di destinazione ≥ dimensione originale; l'app la scrive bit a bit, controlla lo SHA-256 interno e, se richiesto (default sì), rilegge l'unità per la verifica.
- [ ] G5 — **Avanzamento**: durante ogni operazione l'app mostra percentuale, byte elaborati, velocità e tempo stimato, e permette di annullare.
- [ ] G6 — **Sicurezza operazioni distruttive**: prima di scrivere su un'unità (clonazione, ripristino) l'app mostra un riepilogo e richiede conferma esplicita; impedisce che origine e destinazione coincidano, di scrivere sul disco di sistema in uso e di salvare l'immagine su una partizione del disco di origine.
- [ ] G7 — **Multipiattaforma**: lo stesso codice gira su Windows 10/11 e Linux, distribuito come eseguibile unico senza binari esterni; UI in italiano e inglese.

## Non-Goals (cosa NON deve fare) — scope hard

- [ ] NON backup *file a file* / archivi di file (rimosso nella revisione 2), NON ripristino di singoli file.
- [ ] NON backup incrementali/differenziali né pianificazione (scheduler) — solo copie complete avviate manualmente.
- [ ] NON copia su disco/immagine ripristinata su disco più piccolo dell'origine, NON ridimensionamento partizioni, NON copia dei soli settori usati (stile partclone).
- [ ] NON suddivisione dell'immagine in più file (es. per FAT32), NON livello di compressione configurabile, NON cifratura dell'immagine.
- [ ] NON formati di terzi (VHD/VMDK/E01/`.img` grezza) né montaggio delle immagini come unità virtuali.
- [ ] NON destinazioni di rete/cloud, NON clonazione a caldo del disco di sistema in uso.
- [ ] NON supporto macOS in questa iterazione (il codice dipendente dall'OS resta isolato per aggiungerlo in futuro).

## Utenti / consumatori

Utente tecnico o semi-tecnico (sistemista, tecnico PC, utente avanzato) che collega i dischi via USB 3.x a una macchina Windows o Linux e preferisce una GUI a `dd` da terminale. Esegue l'app con privilegi di amministratore/root.

## Interfaccia (schermate e flussi)

**Tab 1 — Backup**
1. Tipo: `( ) Unità → Unità` / `( ) Unità → Immagine` (entrambi bit a bit)
2. Origine: lista dischi fisici (modello, dimensione, bus) + pulsante "Aggiorna"
3. Destinazione:
   - Unità → Unità: lista dischi fisici (esclusi l'origine, il disco di sistema e i dischi troppo piccoli)
   - Unità → Immagine: scelta del file `.img.zst` (con spazio libero e file system della destinazione)
4. `[x] Verifica dopo la copia`
5. "Avvia" → riepilogo/conferma (con conferma esplicita per la clonazione) → barra di avanzamento con "Annulla"

**Tab 2 — Ripristino**
1. Selezione immagine `.img.zst` → mostra disco di origine, dimensione, data
2. Selezione unità di destinazione (stessi filtri della clonazione)
3. `[x] Verifica dopo il ripristino`
4. "Ripristina" → conferma esplicita → avanzamento con "Annulla"

**Impostazioni minime**: lingua (IT/EN).

## Criteri di accettazione

- [ ] CA1 — Clonazione Unità → Unità di una chiavetta USB su una seconda chiavetta ≥ dimensione: lo SHA-256 dei primi N byte (N = dimensione origine) delle due unità coincide; con verifica attiva l'app lo riporta come "Verifica OK".
- [ ] CA2 — Un disco clonato o ripristinato da immagine con sistema operativo avviabile resta avviabile (a condizione che non venga portato online sulla stessa macchina Windows dell'originale — vedi analisi F2).
- [ ] CA3 — Unità → Immagine di una chiavetta e Ripristino dell'immagine su un'altra chiavetta ≥ dimensione: lo SHA-256 dei primi N byte della destinazione coincide con quello dell'origine; un'immagine alterata viene rilevata (errore "immagine danneggiata").
- [ ] CA4 — L'immagine `.img.zst` si decomprime con strumenti standard in un `.img` identico al disco di origine (`zstd -d` su Linux, [7-Zip-zstd](https://github.com/mcmilk/7-Zip-zstd/releases) su Windows).
- [ ] CA5 — L'avvio è bloccato con messaggio chiaro se: destinazione < origine (clonazione/ripristino); spazio libero < dimensione del disco di origine (immagine); destinazione FAT32 e disco > 4 GiB (immagine); il file di destinazione si trova su una partizione del disco di origine.
- [ ] CA6 — Non è possibile selezionare la stessa unità come origine e destinazione, né il disco di sistema come destinazione o origine.
- [ ] CA7 — "Annulla" interrompe l'operazione entro pochi secondi; l'app segnala che la destinazione è incompleta e cancella l'immagine parziale.
- [ ] CA8 — Su Windows l'avvio chiede elevazione UAC; su Linux il launcher avvia l'app via `pkexec`. Senza privilegi l'app lo segnala invece di fallire durante la copia.
- [ ] CA9 — L'eseguibile per Windows e per Linux funziona su una macchina pulita senza installare dipendenze (salvo il runtime WebView di sistema: WebView2 su Windows, WebKitGTK su Linux).
- [ ] CA10 — Cambiando lingua tutti i testi della UI passano da italiano a inglese e viceversa.

## Vincoli tecnici

- Stack: **Go + Wails** (frontend web nella WebView di sistema).
- Compatibilità: Windows 10/11 e Linux desktop. Dischi collegati via USB 3.x.
- Accesso raw: `\\.\PhysicalDriveN` su Windows, `/dev/sdX` / `/dev/nvmeXnY` su Linux. Su Windows i volumi dei dischi coinvolti vanno bloccati/smontati (`FSCTL_LOCK_VOLUME` / `FSCTL_DISMOUNT_VOLUME`); la destinazione di una scrittura va messa offline.
- Lettura/scrittura a blocchi grandi (16 MiB), allineati al settore.
- Compressione: zstd tramite libreria pure Go (`klauspost/compress`), livello default, nessun binario esterno.
- Formato immagine: stream zstd standard; metadati JSON in **frame skippable** zstd (ignorati dai decompressori standard): intestazione all'inizio (formato, versione, disco, dimensione, data) e chiusura di dimensione fissa alla fine (SHA-256, dimensione).
- Privilegi: Windows manifest `requireAdministrator`; Linux avvio tramite `pkexec`.
- Codice dipendente dall'OS isolato in file `*_windows.go` / `*_linux.go`.

## Decisioni prese

| # | Decisione | Alternative valutate | Motivazione |
|---|---|---|---|
| D1 | UI con **Go + Wails** | Python+PySide6, Tauri+Rust, Avalonia | Scelta dello sviluppatore: binario leggero, cross-compilazione semplice. |
| D2 | **Copia raw implementata in Go**, niente `dd`/`dd.exe` | `dd` Linux + port `dd.exe` | Port Windows abbandonato; in-process avanzamento, annulla e hash uniformi. |
| D3 | ~~Tipo determina il metodo (Unità→File = file a file)~~ → **Rev. 2**: due tipi, entrambi bit a bit (Unità→Unità, Unità→Immagine) | File a file | Sostituito su richiesta dello sviluppatore (rev. 2). |
| D4 | ~~Archivio `.tar.zst`~~ → **Rev. 2**: immagine **`.img.zst`** | `.img` grezza; VHD | Lo spazio vuoto si comprime quasi a zero; stesso motore zstd; apribile con zstd/7-Zip-zstd. |
| D5 | ~~FS leggibili dall'OS~~ | — | Superata: il bit a bit non legge i file system. |
| D6 | ~~Ripristino in cartella~~ → **Rev. 2**: ripristino immagine **su unità** | — | Sostituita. |
| D7 | Verifica SHA-256 **opzionale, attiva di default** per clonazione, creazione immagine e ripristino | Sempre; mai | Garanzia di integrità senza imporre il raddoppio del tempo. Il ripristino controlla comunque lo SHA-256 interno dell'immagine. |
| D8 | ~~Partizioni selezionabili~~ | — | Superata (il bit a bit copia l'intero disco). |
| D9 | Linux: **intera app avviata via `pkexec`** | GUI non root + helper privilegiato | Più semplice. |
| D10 | UI **italiano + inglese** | Solo IT; solo EN | Richiesta dello sviluppatore. |
| D11 | Metadati in **frame skippable** zstd (intestazione + chiusura a dimensione fissa) | File `.json` separato | Un solo file; verificato che il decoder Go e 7-Zip-zstd 26.02 li ignorano e ricostruiscono l'immagine identica. |
| D12 | Spazio per l'immagine: **blocco rigido** se spazio libero < dimensione del disco | Solo avviso | Scelta dello sviluppatore: mai un'immagine interrotta per disco pieno. |
| D13 | Immagine con destinazione **FAT32 bloccata** se il disco supera 4 GiB − 1 | Suddivisione in più file | Coerente con D12 (la dimensione compressa non è nota prima); split fuori scope. |

## Ambiguità aperte

Nessuna.
