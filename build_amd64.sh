#!/bin/bash
echo "$0"
PATH="$PATH:/usr/local/go/bin"
#export GOPATH=$(pwd)
export GO111MODULE=auto
#export GOEXPERIMENT=arenas
rm -v ndbserver_amd64
go build -trimpath -ldflags="-s -w" -o ndbserver_amd64 main.go
RET=$?
sha256sum ndbserver_amd64 > ndbserver_amd64.sha256sum
echo $(date)
test $RET -gt 0 && echo "BUILD FAILED! RET=$RET" || echo "BUILD OK!"
exit $RET
