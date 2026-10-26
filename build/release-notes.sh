#!/bin/sh
# Prints the release notes of version X.Y.Z: the "## [X.Y.Z]" section of
# CHANGELOG.md, without its heading and the link list at the end of the file.
# Exits with 1 if the section is missing or empty, so that the release
# workflow stops before publishing a release without notes.
#
#   sh build/release-notes.sh 1.2.0 > notes.md
set -e

if [ $# -ne 1 ]; then
    echo "Uso: $0 <versione X.Y.Z>" >&2
    exit 1
fi
VERSION=$1
CHANGELOG=${CHANGELOG:-CHANGELOG.md}

# Section body, leading blank lines removed; $(...) drops the trailing ones.
NOTES=$(awk -v version="$VERSION" '
    index($0, "## [" version "]") == 1 { found = 1; next }
    found && (/^## \[/ || /^\[[^]]+\]: /) { exit }
    found { print }
' "$CHANGELOG" | sed -e '/./,$!d')

if [ -z "$(printf '%s' "$NOTES" | tr -d '[:space:]')" ]; then
    echo "$CHANGELOG: nessuna sezione \"## [$VERSION]\" (o è vuota)" >&2
    exit 1
fi
printf '%s\n' "$NOTES"
