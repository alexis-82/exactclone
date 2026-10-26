#!/bin/sh
# Installs ExactClone system-wide, in the same paths as the .deb and .rpm
# packages. Run as root from the project root after
# `wails build -tags webkit2_41`, or from the extracted
# exactclone-linux-<arch>.tar.gz (same layout).
#
#   sudo sh build/linux/install.sh               install / update
#   sudo sh build/linux/install.sh --uninstall   remove
set -e

BIN=/usr/bin/exactclone
LAUNCHER=/usr/bin/exactclone-launch
HELPER=/usr/libexec/exactclone-root
UPDATER=/usr/bin/exactclone-update
POLICY=/usr/share/polkit-1/actions/org.exactclone.policy
DESKTOP_FILE=/usr/share/applications/exactclone.desktop
ICON=/usr/share/pixmaps/exactclone.png

if [ "$(id -u)" -ne 0 ]; then
    echo "Esegui come root: sudo sh $0 $*" >&2
    exit 1
fi

# ExactClone installed from the .deb or .rpm: the package manager owns these
# files, do not overwrite them.
owned_by_package() {
    if command -v dpkg-query >/dev/null 2>&1 && dpkg-query -S "$BIN" >/dev/null 2>&1; then
        echo "apt (sudo apt remove exactclone)"
        return 0
    fi
    if command -v rpm >/dev/null 2>&1 && rpm -qf "$BIN" >/dev/null 2>&1; then
        echo "dnf (sudo dnf remove exactclone)"
        return 0
    fi
    return 1
}

if manager=$(owned_by_package); then
    echo "ExactClone è installato come pacchetto: aggiornalo o rimuovilo con $manager." >&2
    exit 1
fi

# Installations made by older versions of this script (/usr/local) and
# before the rename (Disk Clone / diskclone).
remove_old() {
    rm -f /usr/local/bin/exactclone /usr/local/bin/exactclone-launch.sh \
        /usr/local/libexec/exactclone-root \
        /usr/local/bin/diskclone /usr/local/bin/diskclone-launch.sh \
        /usr/local/libexec/diskclone-root \
        /usr/share/polkit-1/actions/org.diskclone.policy \
        /usr/share/applications/diskclone.desktop \
        /usr/share/pixmaps/diskclone.png
}

refresh_menu() {
    if command -v update-desktop-database >/dev/null 2>&1; then
        update-desktop-database -q /usr/share/applications || true
    fi
}

if [ "$1" = "--uninstall" ]; then
    remove_old
    rm -f "$BIN" "$LAUNCHER" "$HELPER" "$UPDATER" "$POLICY" "$DESKTOP_FILE" "$ICON"
    refresh_menu
    echo "ExactClone rimosso. Le impostazioni (/root/.config/exactclone) non sono state toccate."
    exit 0
fi

if [ ! -f build/bin/exactclone ]; then
    echo "build/bin/exactclone non trovato: esegui prima 'wails build -tags webkit2_41' dalla root del progetto." >&2
    exit 1
fi

remove_old
install -m 0755 build/bin/exactclone "$BIN"
install -m 0755 build/linux/exactclone-launch.sh "$LAUNCHER"
install -D -m 0755 build/linux/exactclone-root "$HELPER"
install -m 0755 build/linux/exactclone-update.sh "$UPDATER"
install -D -m 0644 build/linux/org.exactclone.policy "$POLICY"
install -m 0644 build/linux/exactclone.desktop "$DESKTOP_FILE"
install -m 0644 build/appicon.png "$ICON"
refresh_menu
echo "ExactClone installato: lo trovi nel menu delle applicazioni."
echo "Dipendenze runtime: libwebkit2gtk-4.1-0 e pkexec (polkit)."
