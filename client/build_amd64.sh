#!/bin/bash
echo "$0"
PATH="$PATH:/usr/local/go/bin"
#export GOPATH=$(pwd)
export GO111MODULE=auto
#export GOEXPERIMENT=arenas
rm -v client_amd64
go build -trimpath -ldflags="-s -w" -o client_amd64 main.go
RET=$?
sha256sum client_amd64 > client_amd64.sha256sum
echo $(date)
test $RET -gt 0 && echo "BUILD FAILED! RET=$RET" || echo "BUILD OK!"
exit $RET
