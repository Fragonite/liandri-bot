#!/usr/bin/env bash

export GIT_COMMIT_INFO_LONG=$(git describe --dirty --broken --tags --always --abbrev=1000 --long)
export GIT_COMMIT_INFO_SHORT=$(git describe --dirty --broken --tags --always)

if go build -ldflags="-X 'main.gVersionLong=$GIT_COMMIT_INFO_LONG' -X 'main.gVersionShort=$GIT_COMMIT_INFO_SHORT'" ; then
        ./liandri-bot $1 $2 $3 $4 $5 $6 $7 $8
fi