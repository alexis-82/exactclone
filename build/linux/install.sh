#!/bin/sh
# Installs ExactClone system-wide. Run from the project root as root after
# `wails build -tags webkit2_41`.
set -e

# Remove an installation made before the rename (Disk Clone / diskclone).
rm -f /usr/local/bin/diskclone /usr/local/bin/diskclone-launch.sh \
    /usr/local/libexec/diskclone-root \
    /usr/share/polkit-1/actions/org.diskclone.policy \
    /usr/share/applications/diskclone.desktop \
    /usr/share/pixmaps/diskclone.png

install -m 0755 build/bin/exactclone /usr/local/bin/exactclone
install -m 0755 build/linux/exactclone-launch.sh /usr/local/bin/exactclone-launch.sh
install -D -m 0755 build/linux/exactclone-root /usr/local/libexec/exactclone-root
install -m 0644 build/linux/org.exactclone.policy /usr/share/polkit-1/actions/org.exactclone.policy
install -m 0644 build/linux/exactclone.desktop /usr/share/applications/exactclone.desktop
install -m 0644 build/appicon.png /usr/share/pixmaps/exactclone.png
echo "ExactClone installed. Runtime dependency: libwebkit2gtk-4.1-0"
