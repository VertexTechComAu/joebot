#!/bin/bash

set -e

# --- Configuration ---
GO_VERSION="1.24.6"
GO_ARCHIVE="go${GO_VERSION}.linux-amd64.tar.gz"
# --- End Configuration ---

SCRIPT=`readlink -f $0`
SCRIPT_DIR=`dirname $SCRIPT`

echo "Downloading Go version ${GO_VERSION}..."
curl -O --silent --show-error "https://dl.google.com/go/${GO_ARCHIVE}"
tar -zxf "${GO_ARCHIVE}"

echo "Building joebot..."
platforms=("windows/amd64" "linux/amd64" "darwin/amd64")

echo "Building and embeding HTML resource"
pushd $SCRIPT_DIR/src/joebot
$SCRIPT_DIR/go/bin/go generate

echo "Start cross-compilation"
rm -rf $SCRIPT_DIR/output
for platform in "${platforms[@]}"
do
    platform_split=(${platform//\// })
    GOOS=${platform_split[0]}
    GOARCH=${platform_split[1]}
    output_name='joebot-'$GOOS'-'$GOARCH
    if [ $GOOS = "windows" ]; then
        output_name+='.exe'
    fi

    echo "Compiling for ${platform}..."
    env GOOS=$GOOS GOARCH=$GOARCH $SCRIPT_DIR/go/bin/go build -o $SCRIPT_DIR/output/$output_name
    if [ $? -ne 0 ]; then
        echo 'An error has occurred! Aborting the script execution...'
        exit 1
    fi
done

popd

echo "Clean up"
rm -f "${GO_ARCHIVE}"
rm -rf go

echo "Build complete. Binaries are in the 'output' directory."
