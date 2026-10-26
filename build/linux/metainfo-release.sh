#!/bin/sh
# Adds the release being built to ExactClone's AppStream metadata, so software
# centers show the right version and date. Run before packaging, e.g.
#   sh build/linux/metainfo-release.sh build/linux/io.github.alexis_82.exactclone.metainfo.xml 1.3.0 2026-10-18
# The file in the repository has no <releases>: the workflow adds it on its
# own copy at every release.
set -e

if [ $# -ne 3 ]; then
    echo "Uso: $0 <file metainfo> <versione X.Y.Z> <data AAAA-MM-GG>" >&2
    exit 1
fi
FILE=$1
VERSION=$2
DATE=$3

if grep -q "^ *<releases> *$" "$FILE"; then
    echo "$FILE contiene già <releases>" >&2
    exit 1
fi
if ! grep -q "^</component>" "$FILE"; then
    echo "$FILE: </component> non trovato" >&2
    exit 1
fi

TMP="$FILE.tmp"
awk -v version="$VERSION" -v date="$DATE" '
    /^<\/component>/ {
        print "  <releases>"
        print "    <release version=\"" version "\" date=\"" date "\"/>"
        print "  </releases>"
    }
    { print }
' "$FILE" > "$TMP"
mv "$TMP" "$FILE"
