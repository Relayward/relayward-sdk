#!/bin/sh
set -eu

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
    echo "usage: $0 VERSION [OUTPUT_DIRECTORY]" >&2
    exit 2
fi

VERSION=${1#v}
OUTPUT_DIRECTORY=${2:-dist/contract-plugin}
ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"
case "$OUTPUT_DIRECTORY" in
    /*) ;;
    *) OUTPUT_DIRECTORY="$ROOT/$OUTPUT_DIRECTORY" ;;
esac
case "$OUTPUT_DIRECTORY" in
    /|"$ROOT") echo "refusing unsafe output directory: $OUTPUT_DIRECTORY" >&2; exit 2 ;;
esac

rm -rf "$OUTPUT_DIRECTORY"
mkdir -p "$OUTPUT_DIRECTORY"
if [ ! -f ui/dist/index.js ]; then
    echo "UI SDK build is missing; run npm --prefix ui run build first" >&2
    exit 1
fi
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath -buildvcs=false -ldflags "-s -w -buildid= -X main.version=$VERSION" \
    -o "$OUTPUT_DIRECTORY/contract-plugin-center-linux-amd64" ./cmd/relayward-contract-plugin
cp "$OUTPUT_DIRECTORY/contract-plugin-center-linux-amd64" "$OUTPUT_DIRECTORY/contract-plugin-node-linux-amd64"
chmod 0755 "$OUTPUT_DIRECTORY/contract-plugin-node-linux-amd64"

STAGING=$(mktemp -d)
trap 'rm -rf "$STAGING"' EXIT HUP INT TERM
cp fixtures/contract-plugin/ui/index.html "$STAGING/index.html"
cp fixtures/contract-plugin/ui/app.js "$STAGING/app.js"
cp fixtures/contract-plugin/ui/styles.css "$STAGING/styles.css"
cp ui/dist/index.js "$STAGING/relayward-ui-sdk.js"
SOURCE_DATE_EPOCH=${SOURCE_DATE_EPOCH:-0}
tar --sort=name --owner=0 --group=0 --numeric-owner --mtime="@$SOURCE_DATE_EPOCH" \
    -C "$STAGING" -czf "$OUTPUT_DIRECTORY/contract-plugin-ui.tar.gz" \
    app.js index.html relayward-ui-sdk.js styles.css
rm -rf "$STAGING"
trap - EXIT HUP INT TERM

go run ./cmd/relayward-contract-release -dist "$OUTPUT_DIRECTORY" -version "$VERSION"
