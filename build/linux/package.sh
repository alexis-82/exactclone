#!/bin/sh
# Creates build/bin/exactclone-linux-<arch>.tar.gz, the Linux file attached to
# a GitHub release for installations made with install.sh. Run from the
# project root after the Linux build, e.g.
#   wails build -tags webkit2_41 -ldflags "-X main.version=v1.2.3"
#   sh build/linux/package.sh
# The archive keeps the project layout expected by install.sh.
set -e

if [ ! -f build/bin/exactclone ]; then
    echo "build/bin/exactclone non trovato: esegui prima 'wails build' dalla root del progetto." >&2
    exit 1
fi

case "$(uname -m)" in
    x86_64) ARCH=amd64 ;;
    aarch64 | arm64) ARCH=arm64 ;;
    *)
        echo "Architettura non supportata: $(uname -m)" >&2
        exit 1
        ;;
esac

OUT="build/bin/exactclone-linux-$ARCH.tar.gz"
tar -czf "$OUT" \
    build/bin/exactclone \
    build/linux/install.sh \
    build/linux/exactclone-launch.sh \
    build/linux/exactclone-root \
    build/linux/exactclone-update.sh \
    build/linux/org.exactclone.policy \
    build/linux/exactclone.desktop \
    build/appicon.png
echo "Creato $OUT"
