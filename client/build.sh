#!/bin/bash
echo "$0"
PATH="$PATH:/usr/local/go/bin"
#export GOPATH=$(pwd)
export GO111MODULE=auto
#export GOEXPERIMENT=arenas
rm -v client
go build -o client main.go
RET=$?
echo $(date)
test $RET -gt 0 && echo "BUILD FAILED! RET=$RET" || echo "BUILD OK!"
exit $RET
