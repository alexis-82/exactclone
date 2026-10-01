#!/bin/sh
# Installs Disk Clone system-wide. Run from the project root as root after
# `wails build -tags webkit2_41`.
set -e
install -m 0755 build/bin/diskclone /usr/local/bin/diskclone
install -m 0755 build/linux/diskclone-launch.sh /usr/local/bin/diskclone-launch.sh
install -D -m 0755 build/linux/diskclone-root /usr/local/libexec/diskclone-root
install -m 0644 build/linux/org.diskclone.policy /usr/share/polkit-1/actions/org.diskclone.policy
install -m 0644 build/linux/diskclone.desktop /usr/share/applications/diskclone.desktop
install -m 0644 build/appicon.png /usr/share/pixmaps/diskclone.png
echo "Disk Clone installed. Runtime dependency: libwebkit2gtk-4.1-0"
