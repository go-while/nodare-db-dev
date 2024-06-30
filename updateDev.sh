#!/bin/bash
test -z "$1" && echo "usage: $0 commit" && exit 1

go get github.com/go-while/nodare-db-dev/client/clilib@$1 && \
go mod tidy && go mod vendor
