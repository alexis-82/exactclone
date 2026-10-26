; =====================================================================
;  ExactClone - installer NSIS autonomo
;
;  Non usa wails_tools.nsh (rigenerato da Wails a ogni build): tutto
;  il comportamento è in questo file. Ricavato da quello di Zenith, ma
;  per macchina: ExactClone richiede comunque i diritti di amministratore.
;
;  Funzionalità:
;    - scelta della lingua all'avvio (italiano / inglese), ricordata
;      anche dal disinstallatore
;    - installazione per tutti gli utenti in Program Files
;    - componenti: programma, collegamento sul desktop, collegamento
;      nel menu Start
;    - voce in "App installate" / "Programmi e funzionalità"; il suo
;      InstallLocation dice all'app se è installata o portable
;    - attesa della chiusura di ExactClone (aggiornamento dall'app)
;    - controllo del runtime WebView2
;    - disinstallatore, con opzione per rimuovere anche le impostazioni
;
;  Build (dalla root del progetto):
;    wails build
;    makensis build\windows\installer\exactclone.nsi
;  Versione da riga di comando:
;    makensis /DVERSION=1.2.0 build\windows\installer\exactclone.nsi
;  Output: build\bin\ExactClone-<versione>-amd64-installer.exe
;
;  makensis lavora nella cartella di questo file: i percorsi relativi
;  qui sotto partono da build\windows\installer.
; =====================================================================

Unicode true
ManifestDPIAware true
SetCompressor /SOLID lzma

; ---------------------------------------------------------------------
;  Configurazione
; ---------------------------------------------------------------------
!define APP_NAME     "ExactClone"
!define APP_EXE      "exactclone.exe"
!define COMPANY      "alexis-82"
!define APP_URL      "https://github.com/alexis-82/exactclone"
!ifndef VERSION
  !define VERSION    "1.0.0"          ; formato x.y.z
!endif

!ifndef SOURCE_EXE                    ; eseguibile prodotto da `wails build`
  !define SOURCE_EXE "..\..\bin\${APP_EXE}"
!endif
!define ICON_FILE    "..\icon.ico"

; Chiavi di registro (HKLM, vista a 64 bit: SetRegView 64 in .onInit).
; UNINST_KEY è letta anche dall'app (update_windows.go): stessa vista.
!define APP_REG_KEY  "Software\${APP_NAME}"
!define UNINST_KEY   "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}"

; Cartelle dati dell'app (dell'utente che disinstalla)
!define SETTINGS_DIR "$APPDATA\exactclone"       ; config.json di ExactClone
!define WEBVIEW_DIR  "$APPDATA\${APP_EXE}"      ; cache WebView2 creata da Wails

; Runtime WebView2 (Evergreen)
!define WEBVIEW2_GUID "{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"
!define WEBVIEW2_URL  "https://go.microsoft.com/fwlink/p/?LinkId=2124703"

; ---------------------------------------------------------------------
;  Impostazioni generali
; ---------------------------------------------------------------------
Name "${APP_NAME}"
OutFile "..\..\bin\${APP_NAME}-${VERSION}-amd64-installer.exe"
InstallDir "$PROGRAMFILES64\${APP_NAME}"
RequestExecutionLevel admin
BrandingText "${APP_NAME} ${VERSION}"
ShowInstDetails show
ShowUninstDetails show

!include "MUI2.nsh"
!include "FileFunc.nsh"
!include "LogicLib.nsh"
!include "x64.nsh"

; ---------------------------------------------------------------------
;  Interfaccia (Modern UI 2)
; ---------------------------------------------------------------------
!define MUI_ICON   "${ICON_FILE}"
!define MUI_UNICON "${ICON_FILE}"
!define MUI_ABORTWARNING
!define MUI_UNABORTWARNING

; Lingua scelta: salvata in HKLM, così il disinstallatore la riusa
!define MUI_LANGDLL_REGISTRY_ROOT      HKLM
!define MUI_LANGDLL_REGISTRY_KEY       "${APP_REG_KEY}"
!define MUI_LANGDLL_REGISTRY_VALUENAME "Language"
!define MUI_LANGDLL_ALLLANGUAGES       ; mostra sempre tutte le lingue

; Ultima pagina: casella "Avvia ExactClone" (l'installer è già elevato)
!define MUI_FINISHPAGE_RUN "$INSTDIR\${APP_EXE}"

; Pagine dell'installer
!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH

; Pagine del disinstallatore
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_COMPONENTS
!insertmacro MUI_UNPAGE_INSTFILES

; Lingue (la prima è quella predefinita)
!insertmacro MUI_LANGUAGE "Italian"
!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_RESERVEFILE_LANGDLL

; ---------------------------------------------------------------------
;  Testi tradotti
; ---------------------------------------------------------------------
LangString SecAppName      ${LANG_ITALIAN} "${APP_NAME} (obbligatorio)"
LangString SecAppName      ${LANG_ENGLISH} "${APP_NAME} (required)"
LangString SecAppDesc      ${LANG_ITALIAN} "Il programma ${APP_NAME}."
LangString SecAppDesc      ${LANG_ENGLISH} "The ${APP_NAME} application."
LangString SecDesktopName  ${LANG_ITALIAN} "Collegamento sul desktop"
LangString SecDesktopName  ${LANG_ENGLISH} "Desktop shortcut"
LangString SecDesktopDesc  ${LANG_ITALIAN} "Crea un collegamento a ${APP_NAME} sul desktop di tutti gli utenti."
LangString SecDesktopDesc  ${LANG_ENGLISH} "Creates a ${APP_NAME} shortcut on the desktop of all users."
LangString SecStartName    ${LANG_ITALIAN} "Collegamento nel menu Start"
LangString SecStartName    ${LANG_ENGLISH} "Start menu shortcut"
LangString SecStartDesc    ${LANG_ITALIAN} "Aggiunge ${APP_NAME} al menu Start."
LangString SecStartDesc    ${LANG_ENGLISH} "Adds ${APP_NAME} to the Start menu."

LangString UnSecAppName       ${LANG_ITALIAN} "${APP_NAME}"
LangString UnSecAppName       ${LANG_ENGLISH} "${APP_NAME}"
LangString UnSecAppDesc       ${LANG_ITALIAN} "Rimuove il programma, i collegamenti e la cache dell'interfaccia."
LangString UnSecAppDesc       ${LANG_ENGLISH} "Removes the application, its shortcuts and the UI cache."
LangString UnSecSettingsName  ${LANG_ITALIAN} "Impostazioni"
LangString UnSecSettingsName  ${LANG_ENGLISH} "Settings"
LangString UnSecSettingsDesc  ${LANG_ITALIAN} "Rimuove anche le impostazioni (lingua, controllo degli aggiornamenti). Le immagini .img.zst non vengono mai eliminate."
LangString UnSecSettingsDesc  ${LANG_ENGLISH} "Also removes the settings (language, update check). The .img.zst images are never deleted."

LangString MsgWebView2 ${LANG_ITALIAN} "${APP_NAME} richiede Microsoft Edge WebView2 Runtime, che non risulta installato.$\n$\nVuoi aprire la pagina di download di Microsoft?"
LangString MsgWebView2 ${LANG_ENGLISH} "${APP_NAME} requires the Microsoft Edge WebView2 Runtime, which does not seem to be installed.$\n$\nDo you want to open the Microsoft download page?"
LangString MsgAppRunning ${LANG_ITALIAN} "${APP_NAME} è ancora in esecuzione.$\n$\nChiudilo e premi Riprova."
LangString MsgAppRunning ${LANG_ENGLISH} "${APP_NAME} is still running.$\n$\nClose it and press Retry."
LangString Msg64Bit ${LANG_ITALIAN} "${APP_NAME} richiede Windows a 64 bit."
LangString Msg64Bit ${LANG_ENGLISH} "${APP_NAME} requires 64-bit Windows."

; Proprietà dell'installer (tasto destro > Proprietà > Dettagli)
VIProductVersion "${VERSION}.0"
VIFileVersion    "${VERSION}.0"
VIAddVersionKey /LANG=${LANG_ITALIAN} "ProductName"     "${APP_NAME}"
VIAddVersionKey /LANG=${LANG_ITALIAN} "CompanyName"     "${COMPANY}"
VIAddVersionKey /LANG=${LANG_ITALIAN} "FileDescription" "${APP_NAME} Setup"
VIAddVersionKey /LANG=${LANG_ITALIAN} "FileVersion"     "${VERSION}"
VIAddVersionKey /LANG=${LANG_ITALIAN} "ProductVersion"  "${VERSION}"
VIAddVersionKey /LANG=${LANG_ITALIAN} "LegalCopyright"  "${COMPANY}"

; ---------------------------------------------------------------------
;  Attesa della chiusura di ExactClone
; ---------------------------------------------------------------------
; Avviato dall'aggiornamento, l'installer parte mentre l'app si sta
; chiudendo: un exe in esecuzione non si apre in scrittura. Aspetta fino a
; ~10 s, poi chiede di chiuderlo (Riprova / Annulla).
!macro WAIT_FOR_APP un
Function ${un}WaitForApp
  IfFileExists "$INSTDIR\${APP_EXE}" 0 done
  StrCpy $R1 0
  retry:
    ClearErrors
    FileOpen $R0 "$INSTDIR\${APP_EXE}" a
    IfErrors locked
    FileClose $R0
    Goto done
  locked:
    IntOp $R1 $R1 + 1
    ${If} $R1 < 20
      Sleep 500
      Goto retry
    ${EndIf}
    MessageBox MB_RETRYCANCEL|MB_ICONEXCLAMATION "$(MsgAppRunning)" /SD IDCANCEL IDRETRY retry
    Abort
  done:
FunctionEnd
!macroend
!insertmacro WAIT_FOR_APP ""
!insertmacro WAIT_FOR_APP "un."

; ---------------------------------------------------------------------
;  Installazione
; ---------------------------------------------------------------------
Section "!$(SecAppName)" SecApp
  SectionIn RO
  SetShellVarContext all
  SetOutPath "$INSTDIR"

  Call WaitForApp
  File "/oname=${APP_EXE}" "${SOURCE_EXE}"
  WriteUninstaller "$INSTDIR\uninstall.exe"

  WriteRegStr HKLM "${APP_REG_KEY}" "InstallDir" "$INSTDIR"

  ; Voce in "App installate" / "Programmi e funzionalità"
  WriteRegStr   HKLM "${UNINST_KEY}" "DisplayName"          "${APP_NAME}"
  WriteRegStr   HKLM "${UNINST_KEY}" "DisplayVersion"       "${VERSION}"
  WriteRegStr   HKLM "${UNINST_KEY}" "DisplayIcon"          "$INSTDIR\${APP_EXE}"
  WriteRegStr   HKLM "${UNINST_KEY}" "Publisher"            "${COMPANY}"
  WriteRegStr   HKLM "${UNINST_KEY}" "URLInfoAbout"         "${APP_URL}"
  WriteRegStr   HKLM "${UNINST_KEY}" "InstallLocation"      "$INSTDIR"
  WriteRegStr   HKLM "${UNINST_KEY}" "UninstallString"      "$\"$INSTDIR\uninstall.exe$\""
  WriteRegStr   HKLM "${UNINST_KEY}" "QuietUninstallString" "$\"$INSTDIR\uninstall.exe$\" /S"
  WriteRegDWORD HKLM "${UNINST_KEY}" "NoModify" 1
  WriteRegDWORD HKLM "${UNINST_KEY}" "NoRepair" 1
  ${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
  IntFmt $0 "0x%08X" $0
  WriteRegDWORD HKLM "${UNINST_KEY}" "EstimatedSize" $0

  Call CheckWebView2
SectionEnd

Section "$(SecDesktopName)" SecDesktop
  SetShellVarContext all
  CreateShortcut "$DESKTOP\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}"
SectionEnd

Section "$(SecStartName)" SecStart
  SetShellVarContext all
  CreateShortcut "$SMPROGRAMS\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}"
SectionEnd

!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
  !insertmacro MUI_DESCRIPTION_TEXT ${SecApp}     $(SecAppDesc)
  !insertmacro MUI_DESCRIPTION_TEXT ${SecDesktop} $(SecDesktopDesc)
  !insertmacro MUI_DESCRIPTION_TEXT ${SecStart}   $(SecStartDesc)
!insertmacro MUI_FUNCTION_DESCRIPTION_END

Function .onInit
  ; NSIS is 32-bit: without this HKLM\SOFTWARE would go to WOW6432Node,
  ; where the 64-bit app does not look for InstallLocation.
  SetRegView 64
  ${IfNot} ${RunningX64}
    MessageBox MB_OK|MB_ICONSTOP "$(Msg64Bit)" /SD IDOK
    Abort
  ${EndIf}
  ; Update: reuse the folder of the current installation
  ReadRegStr $0 HKLM "${APP_REG_KEY}" "InstallDir"
  ${If} $0 != ""
    StrCpy $INSTDIR $0
  ${EndIf}
  !insertmacro MUI_LANGDLL_DISPLAY
FunctionEnd

; WebView2 is preinstalled on up-to-date Windows 10/11; if missing, offer
; the official download page (the installer itself stays offline).
; Its registration is in the 32-bit view (WOW6432Node).
Function CheckWebView2
  SetRegView 32
  ReadRegStr $0 HKLM "SOFTWARE\Microsoft\EdgeUpdate\Clients\${WEBVIEW2_GUID}" "pv"
  SetRegView 64
  ${If} $0 == ""
  ${OrIf} $0 == "0.0.0.0"
    ReadRegStr $0 HKCU "Software\Microsoft\EdgeUpdate\Clients\${WEBVIEW2_GUID}" "pv"
  ${EndIf}
  ${If} $0 == ""
  ${OrIf} $0 == "0.0.0.0"
    MessageBox MB_YESNO|MB_ICONEXCLAMATION "$(MsgWebView2)" /SD IDNO IDNO +2
      ExecShell "open" "${WEBVIEW2_URL}"
  ${EndIf}
FunctionEnd

; ---------------------------------------------------------------------
;  Disinstallazione
; ---------------------------------------------------------------------
Section "un.$(UnSecAppName)" UnSecApp
  SectionIn RO
  SetShellVarContext all

  Call un.WaitForApp

  Delete "$DESKTOP\${APP_NAME}.lnk"
  Delete "$SMPROGRAMS\${APP_NAME}.lnk"

  Delete "$INSTDIR\${APP_EXE}"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"                      ; solo se vuota: non cancella file estranei

  SetShellVarContext current
  RMDir /r "${WEBVIEW_DIR}"

  DeleteRegKey HKLM "${UNINST_KEY}"
  DeleteRegKey HKLM "${APP_REG_KEY}"
SectionEnd

Section /o "un.$(UnSecSettingsName)" UnSecSettings
  SetShellVarContext current
  RMDir /r "${SETTINGS_DIR}"
SectionEnd

!insertmacro MUI_UNFUNCTION_DESCRIPTION_BEGIN
  !insertmacro MUI_DESCRIPTION_TEXT ${UnSecApp}      $(UnSecAppDesc)
  !insertmacro MUI_DESCRIPTION_TEXT ${UnSecSettings} $(UnSecSettingsDesc)
!insertmacro MUI_UNFUNCTION_DESCRIPTION_END

Function un.onInit
  SetRegView 64
  !insertmacro MUI_UNGETLANGUAGE
FunctionEnd
