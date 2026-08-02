#!/bin/sh
set -eu

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
    echo "usage: $0 VERSION [OUTPUT_DIRECTORY]" >&2
    exit 2
fi

VERSION=${1#v}
OUTPUT_DIRECTORY=${2:-dist}
ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"
case "$OUTPUT_DIRECTORY" in
    /*) ;;
    *) OUTPUT_DIRECTORY="$ROOT/$OUTPUT_DIRECTORY" ;;
esac
case "$OUTPUT_DIRECTORY" in
    /|"$ROOT") echo "refusing unsafe output directory: $OUTPUT_DIRECTORY" >&2; exit 2 ;;
esac
if [ ! -f ui/dist/index.js ]; then
    echo "UI SDK build is missing; run npm --prefix ui run build first" >&2
    exit 1
fi

SOURCE_DATE_EPOCH=${SOURCE_DATE_EPOCH:-$(git show -s --format=%ct HEAD 2>/dev/null || date +%s)}
rm -rf "$OUTPUT_DIRECTORY"
mkdir -p "$OUTPUT_DIRECTORY"

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath -buildvcs=false \
    -ldflags "-s -w -buildid= -X main.version=$VERSION" \
    -o "$OUTPUT_DIRECTORY/relayward-conformance-linux-amd64" ./cmd/relayward-conformance
reported_version=$("$OUTPUT_DIRECTORY/relayward-conformance-linux-amd64" version)
if [ "$reported_version" != "$VERSION" ]; then
    echo "conformance binary reports $reported_version instead of $VERSION" >&2
    exit 1
fi

staging=$(mktemp -d)
trap 'rm -rf "$staging"' EXIT HUP INT TERM
install -m 0644 ui/package.json "$staging/package.json"
cp -R ui/dist "$staging/dist"
(
    cd "$staging"
    npm pkg set "version=$VERSION"
    npm pack --ignore-scripts --pack-destination "$OUTPUT_DIRECTORY" >/dev/null
)
ui_package="relayward-ui-sdk-$VERSION.tgz"
if [ ! -f "$OUTPUT_DIRECTORY/$ui_package" ]; then
    echo "UI SDK package was not created with the expected name" >&2
    exit 1
fi

tar --sort=name --owner=0 --group=0 --numeric-owner --mtime="@$SOURCE_DATE_EPOCH" \
    -czf "$OUTPUT_DIRECTORY/relayward-sdk-fixtures-$VERSION.tar.gz" fixtures

contract_directory="$OUTPUT_DIRECTORY/contract-plugin"
./scripts/build-contract-plugin.sh "$VERSION" "$contract_directory"
for asset in \
    contract-plugin-center-linux-amd64 \
    contract-plugin-node-linux-amd64 \
    contract-plugin-ui.tar.gz \
    relayward-plugin.json; do
    mv "$contract_directory/$asset" "$OUTPUT_DIRECTORY/$asset"
done
rmdir "$contract_directory"

"$OUTPUT_DIRECTORY/relayward-conformance-linux-amd64" plugin-release "$OUTPUT_DIRECTORY"
(
    cd "$OUTPUT_DIRECTORY"
    sha256sum \
        contract-plugin-center-linux-amd64 \
        contract-plugin-node-linux-amd64 \
        contract-plugin-ui.tar.gz \
        relayward-conformance-linux-amd64 \
        "relayward-sdk-fixtures-$VERSION.tar.gz" \
        "$ui_package" \
        relayward-plugin.json > SHA256SUMS
)

rm -rf "$staging"
trap - EXIT HUP INT TERM
