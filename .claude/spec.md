# Specifica — Disk Clone Tool (GUI multipiattaforma)

## Obiettivo

Offrire un'applicazione desktop con interfaccia grafica, multipiattaforma (Windows + Linux), per clonare bit a bit un'unità disco su un'altra unità, creare un archivio compresso file a file di un'unità e ripristinare tale archivio, senza dover usare la riga di comando.

## Goals (cosa DEVE fare)

- [ ] G1 — **Tab "Backup"**: l'utente sceglie il tipo: *Unità → Unità* oppure *Unità → File*. Il tipo determina il metodo: *Unità → Unità* = **bit a bit**; *Unità → File* = **archivio compresso file a file** (`.tar.zst`). Nessun selettore di metodo separato.
- [ ] G2 — **Unità → Unità (bit a bit)**: copia l'intero disco sorgente sul disco destinazione, con verifica SHA-256 post-copia opzionale (checkbox attiva di default).
- [ ] G3 — **Unità → File (file a file)**: l'app elenca le partizioni del disco sorgente con il loro file system; l'utente spunta quelle da includere (tutte di default, solo quelle leggibili dall'OS); l'app le monta in sola lettura e crea un unico archivio `.tar.zst` con una sottocartella per partizione, nella cartella e col nome scelti dall'utente.
- [ ] G4 — **Tab "Ripristino"**: l'utente seleziona un archivio `.tar.zst` creato dall'app e una cartella di destinazione su un volume già formattato e montato; l'app vi estrae i file.
- [ ] G5 — **Avanzamento**: durante ogni operazione l'app mostra percentuale, byte elaborati, velocità e tempo stimato, e permette di annullare.
- [ ] G6 — **Sicurezza operazioni distruttive**: prima della clonazione l'app mostra un riepilogo (origine → destinazione, modelli, dimensioni) e richiede conferma esplicita; impedisce che origine e destinazione coincidano e impedisce di scrivere sul disco di sistema in uso.
- [ ] G7 — **Multipiattaforma**: lo stesso codice gira su Windows 10/11 e Linux, distribuito come eseguibile unico senza binari esterni; UI in italiano e inglese.

## Non-Goals (cosa NON deve fare) — scope hard

- [ ] NON backup incrementali/differenziali né pianificazione (scheduler) — solo backup completi avviati manualmente.
- [ ] NON immagine bit a bit su file né suo ripristino — il bit a bit è solo Unità → Unità.
- [ ] NON clonazione bit a bit su disco più piccolo dell'origine, NON ridimensionamento partizioni, NON copia dei soli settori usati (stile partclone).
- [ ] NON formattazione/partizionamento dell'unità di destinazione nel ripristino — si estrae in una cartella di un volume esistente; il ripristino file a file NON rende il disco avviabile e NON preserva ACL Windows / attributi estesi.
- [ ] NON lettura di file system non supportati nativamente dall'OS (es. ext4 su Windows, APFS/HFS+), NON volumi cifrati (BitLocker/LUKS), RAID o dischi dinamici — mostrati come "non supportati".
- [ ] NON cifratura degli archivi, NON destinazioni di rete/cloud, NON formati di terzi (VHD/VMDK/7z/zip) né montaggio degli archivi.
- [ ] NON supporto macOS in questa iterazione (il codice dipendente dall'OS resta comunque isolato per aggiungerlo in futuro).

## Utenti / consumatori

Utente tecnico o semi-tecnico (sistemista, tecnico PC, utente avanzato) che collega i dischi via USB 3.x a una macchina Windows o Linux e preferisce una GUI a `dd`/`tar` da terminale. Esegue l'app con privilegi di amministratore/root.

## Interfaccia (schermate e flussi)

**Tab 1 — Backup**
1. Tipo: `( ) Unità → Unità (bit a bit)` / `( ) Unità → File (archivio compresso)`
2. Origine: lista dischi fisici (modello, dimensione, bus, partizioni) + pulsante "Aggiorna"
3. Destinazione:
   - Unità → Unità: lista dischi fisici (esclusa l'origine e il disco di sistema) + checkbox `[x] Verifica dopo la copia`
   - Unità → File: lista partizioni con checkbox (FS indicato, non leggibili disabilitate) + scelta cartella e nome file (con spazio libero)
4. "Avvia" → riepilogo/conferma → barra di avanzamento con "Annulla"

**Tab 2 — Ripristino**
1. Selezione archivio `.tar.zst`
2. Selezione cartella di destinazione (volume montato)
3. "Ripristina" → conferma → avanzamento con "Annulla"

**Impostazioni minime**: lingua (IT/EN).

## Criteri di accettazione

- [ ] CA1 — Clonazione Unità → Unità di una chiavetta USB su una seconda chiavetta ≥ dimensione: lo SHA-256 dei primi N byte (N = dimensione origine) delle due unità coincide; con verifica attiva l'app lo riporta come "Verifica OK".
- [ ] CA2 — Un disco clonato bit a bit con sistema operativo avviabile resta avviabile (a condizione che il clone non venga portato online sulla stessa macchina Windows che lo ha creato — vedi analisi F2).
- [ ] CA3 — Backup Unità → File di una partizione NTFS e una FAT32 (e ext4 su Linux): l'archivio contiene una sottocartella per ciascuna partizione selezionata con tutti i file; dopo il ripristino in una cartella, file e dimensioni coincidono e un campione verificato via hash è identico.
- [ ] CA4 — L'archivio `.tar.zst` prodotto si apre con strumenti standard (`tar --zstd -xf` su Linux, 7-Zip su Windows).
- [ ] CA5 — Se destinazione < origine (clonazione) o spazio libero insufficiente per la stima dell'archivio, l'avvio è bloccato con messaggio chiaro.
- [ ] CA6 — Non è possibile selezionare la stessa unità come origine e destinazione, né il disco di sistema come destinazione.
- [ ] CA7 — "Annulla" interrompe l'operazione entro pochi secondi; l'app segnala che la destinazione è incompleta (e cancella l'archivio parziale in Unità → File).
- [ ] CA8 — Su Windows l'avvio chiede elevazione UAC; su Linux il launcher avvia l'app via `pkexec`. Senza privilegi l'app lo segnala invece di fallire durante la copia.
- [ ] CA9 — L'eseguibile per Windows e per Linux funziona su una macchina pulita senza installare dipendenze (salvo il runtime WebView di sistema: WebView2 su Windows, WebKitGTK su Linux).
- [ ] CA10 — Cambiando lingua tutti i testi della UI passano da italiano a inglese e viceversa.

## Vincoli tecnici

- Stack: **Go + Wails** (frontend web nella WebView di sistema). Primo utilizzo da parte dello sviluppatore → preferire soluzioni semplici e ben documentate.
- Compatibilità: Windows 10/11 e Linux desktop. Dischi collegati via USB 3.x e non montati (per la clonazione).
- Accesso raw: `\\.\PhysicalDriveN` su Windows, `/dev/sdX` / `/dev/nvmeXnY` su Linux. Su Windows, prima di scrivere, i volumi del disco destinazione vanno bloccati/smontati (`FSCTL_LOCK_VOLUME` / `FSCTL_DISMOUNT_VOLUME`) anche se non hanno lettera.
- Lettura/scrittura a blocchi grandi (4–64 MiB), allineati al settore.
- Compressione: zstd tramite libreria pure Go (`klauspost/compress`), nessun binario esterno.
- Montaggio partizioni per file a file: in sola lettura (Linux: `mount -o ro`; Windows: lettere/volumi già esposti dall'OS o assegnati temporaneamente).
- Privilegi: Windows manifest `requireAdministrator`; Linux avvio tramite `pkexec`.
- Codice dipendente dall'OS isolato dietro un'interfaccia (enumerazione dischi, apertura device, montaggio) per poter aggiungere macOS in futuro.

## Decisioni prese

| # | Decisione | Alternative valutate | Motivazione |
|---|---|---|---|
| D1 | UI con **Go + Wails** | Python+PySide6, Tauri+Rust, Avalonia | Scelta dello sviluppatore: binario leggero (~10–20 MB), cross-compilazione semplice, Go più accessibile di Rust. |
| D2 | **Copia raw implementata in Go**, niente `dd`/`dd.exe` | `dd` Linux + port `dd.exe` (chrysocome) | Il port Windows è abbandonato e non dà avanzamento affidabile; in-process si ottengono avanzamento, annulla e hash uniformi sui due OS, senza problemi di licenza. |
| D3 | Tipo determina il metodo: Unità→Unità = bit a bit, Unità→File = file a file; ripristino solo da archivio | Selettore metodo indipendente (4 combinazioni) | UI più semplice; copre i casi d'uso reali dello sviluppatore. |
| D4 | Archivio **`.tar.zst`** | zip, 7z | Compressione molto veloce e in streaming, libreria pure Go; zip 2–4× più lento, 7z richiederebbe un binario esterno. |
| D5 | FS supportati = **quelli leggibili nativamente dall'OS** | Solo NTFS/FAT32/exFAT ovunque | Nessun driver extra; su Linux anche ext4. Le partizioni non leggibili sono mostrate come "non supportate". |
| D6 | Ripristino in **cartella di un volume già montato** | App formatta + estrae; entrambe | Meno codice OS-specifico, nessuna formattazione distruttiva. |
| D7 | Verifica SHA-256 post-clonazione **opzionale, attiva di default** | Sempre; mai | Garanzia di integrità senza imporre il raddoppio del tempo. |
| D8 | Partizioni **selezionabili** (tutte di default), un archivio con sottocartella per partizione | Sempre tutte | Permette di escludere partizioni inutili (es. recovery). |
| D9 | Linux: **intera app avviata via `pkexec`** | GUI non root + helper privilegiato | Molto più semplice; accettabile per uno strumento usato da tecnici. |
| D10 | UI **italiano + inglese** | Solo IT; solo EN | Richiesta dello sviluppatore. |

## Ambiguità aperte

- [?risolvere-dopo: dettaglio di implementazione] Livello di compressione zstd (default proposto: livello 3 "default" di zstd) e se esporlo in UI.
- [?risolvere-dopo: dipende dall'API Windows scelta in plan] Su Windows, come montare in sola lettura partizioni senza lettera assegnata (assegnazione temporanea vs accesso via `\\?\Volume{GUID}\`).
