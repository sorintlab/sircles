#!/usr/bin/env bash
#
# Update vendored dedendencies. Requires glide >= 0.12
#
set -e

if ! [[ "$PWD" = "$GOPATH/src/github.com/sorintlab/sircles" ]]; then
  echo "must be run from \$GOPATH/src/github.com/sorintlab/sircles"
  exit 255
fi

if [ ! $(command -v glide) ]; then
        echo "glide: command not found"
        exit 255
fi

if [ ! $(command -v glide-vc) ]; then
        echo "glide-vc: command not found"
        exit 255
fi

glide update --strip-vendor

### update sqlite3 bindings since the current ones (as of 15 Nov 2017) are old
pushd vendor/github.com/mattn/go-sqlite3
go run ./tool/upgrade.go
popd

glide-vc --only-code --no-tests
